package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lechgu/tichy/internal/config"
	"github.com/lechgu/tichy/internal/models"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/samber/do/v2"
)

type GeneratorConfig struct {
	QuestionsPerDoc int
	MinConfidence   float64
	ContextSize     int
	ContextOverlap  int
	Categories      []string
	MaxTests        int
	OnProgress      func()
}

type Generator struct {
	cfg    *config.Config
	client openai.Client
}

func NewGenerator(di do.Injector) (*Generator, error) {
	cfg, err := do.Invoke[*config.Config](di)
	if err != nil {
		return nil, err
	}

	client := openai.NewClient(
		option.WithBaseURL(cfg.LLMServerURL+"/v1"),
		option.WithAPIKey("not-needed"),
	)

	return &Generator{
		cfg:    cfg,
		client: client,
	}, nil
}

func (g *Generator) Generate(ctx context.Context, documents []models.Document, genCfg GeneratorConfig) ([]models.TestQuestion, error) {
	var allTests []models.TestQuestion

	for _, doc := range documents {
		if genCfg.MaxTests > 0 && len(allTests) >= genCfg.MaxTests {
			break
		}

		contexts := g.createContextWindows(doc.Content, genCfg.ContextSize, genCfg.ContextOverlap)

		for _, contextText := range contexts {
			if genCfg.MaxTests > 0 && len(allTests) >= genCfg.MaxTests {
				break
			}

			questions, err := g.generateQuestionsForContext(ctx, contextText, doc.ID, genCfg)
			if err != nil {
				continue
			}

			for _, q := range questions {
				if genCfg.MaxTests > 0 && len(allTests) >= genCfg.MaxTests {
					break
				}

				q.Category = g.normalizeCategory(q.Category, genCfg.Categories)
				q.ExpectedSources = []string{doc.ID}

				if g.validateTestCase(contextText, q) {
					allTests = append(allTests, q)
					if genCfg.OnProgress != nil {
						genCfg.OnProgress()
					}
				}
			}
		}
	}

	return allTests, nil
}

func (g *Generator) createContextWindows(content string, size, overlap int) []string {
	var windows []string

	if len(content) <= size {
		return []string{content}
	}

	for i := 0; i < len(content); i += (size - overlap) {
		end := i + size
		if end > len(content) {
			end = len(content)
		}

		window := content[i:end]
		windows = append(windows, window)

		if end == len(content) {
			break
		}
	}

	return windows
}

func (g *Generator) generateQuestionsForContext(ctx context.Context, contextText, sourceFile string, genCfg GeneratorConfig) ([]models.TestQuestion, error) {
	systemPrompt := `You are a test case generator for a RAG (Retrieval-Augmented Generation) system.
Your task is to generate high-quality, factual questions that can be DIRECTLY answered from the given text.

Requirements:
- Questions must be answerable ONLY from the provided text passage
- Do NOT create questions that require external knowledge
- Include a complete reference answer extracted from the text
- Extract 2-4 key facts or keywords that must appear in a correct answer
- Categorize each question as one of: direct_fact, temporal, numerical, comparative, relationship

Guidelines:
- Direct fact: Simple factual questions (Who is X? What is Y?)
- Temporal: Questions about time/dates (When did X happen?)
- Numerical: Questions involving numbers (How many X?)
- Comparative: Questions comparing entities (How does X compare to Y?)
- Relationship: Questions about connections (Who reports to X?)

Generate diverse, non-trivial questions. Avoid yes/no questions.`

	userPrompt := fmt.Sprintf(`Text passage:
---
%s
---

Source: %s

Generate %d test questions in JSON format:
{
  "questions": [
    {
      "question": "Who is the CEO of Insurellm?",
      "category": "direct_fact",
      "reference_answer": "Avery Lancaster is the Co-Founder and Chief Executive Officer (CEO) of Insurellm.",
      "keywords": ["Avery Lancaster", "CEO", "Co-Founder"]
    }
  ]
}

Respond with ONLY the JSON object, no other text.`, contextText, sourceFile, genCfg.QuestionsPerDoc)

	resp, err := g.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4o,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Temperature: openai.Float(0.7), // Some creativity but not too much
	})
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)

	// Strip markdown code blocks if present
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) > 2 {
			// Remove first line (```json or ```) and last line (```)
			content = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var genTest models.GeneratedTest
	if err := json.Unmarshal([]byte(content), &genTest); err != nil {
		return nil, err
	}

	return genTest.Questions, nil
}

func (g *Generator) validateTestCase(contextText string, testCase models.TestQuestion) bool {
	contextLower := strings.ToLower(contextText)

	foundKeywords := 0
	for _, keyword := range testCase.Keywords {
		if strings.Contains(contextLower, strings.ToLower(keyword)) {
			foundKeywords++
		}
	}

	if len(testCase.Keywords) > 0 {
		keywordCoverage := float64(foundKeywords) / float64(len(testCase.Keywords))
		if keywordCoverage < 0.5 {
			return false
		}
	}

	answerWords := strings.Fields(strings.ToLower(testCase.ReferenceAnswer))
	meaningfulWords := 0
	foundWords := 0

	for _, word := range answerWords {
		if len(word) > 3 && !isCommonWord(word) {
			meaningfulWords++
			if strings.Contains(contextLower, word) {
				foundWords++
			}
		}
	}

	if meaningfulWords > 0 {
		answerGrounding := float64(foundWords) / float64(meaningfulWords)
		if answerGrounding < 0.3 {
			return false
		}
	}

	if len(testCase.Question) < 10 {
		return false
	}

	if len(testCase.ReferenceAnswer) < 10 {
		return false
	}

	if len(testCase.Keywords) == 0 {
		return false
	}

	return true
}

func (g *Generator) normalizeCategory(category string, allowedCategories []string) string {
	category = strings.ToLower(strings.TrimSpace(category))

	if len(allowedCategories) == 0 {
		return category
	}

	for _, allowed := range allowedCategories {
		if category == strings.ToLower(allowed) {
			return category
		}
	}

	return "direct_fact"
}

func isCommonWord(word string) bool {
	commonWords := map[string]bool{
		"the": true, "is": true, "are": true, "was": true, "were": true,
		"and": true, "or": true, "but": true, "in": true, "on": true,
		"at": true, "to": true, "for": true, "of": true, "with": true,
		"a": true, "an": true, "as": true, "by": true, "from": true,
		"has": true, "have": true, "had": true, "that": true, "this": true,
	}
	return commonWords[word]
}
