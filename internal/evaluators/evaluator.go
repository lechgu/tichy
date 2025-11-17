package evaluators

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/lechgu/tichy/internal/config"
	"github.com/lechgu/tichy/internal/models"
	"github.com/lechgu/tichy/internal/responders"
	"github.com/lechgu/tichy/internal/retrievers"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/samber/do/v2"
)

type Evaluator struct {
	cfg       *config.Config
	retriever *retrievers.Retriever
	responder *responders.Responder
	client    openai.Client
}

func New(di do.Injector) (*Evaluator, error) {
	cfg, err := do.Invoke[*config.Config](di)
	if err != nil {
		return nil, err
	}

	retriever, err := do.Invoke[*retrievers.Retriever](di)
	if err != nil {
		return nil, err
	}

	responder, err := do.Invoke[*responders.Responder](di)
	if err != nil {
		return nil, err
	}

	client := openai.NewClient(
		option.WithBaseURL(cfg.LLMServerURL+"/v1"),
		option.WithAPIKey("not-needed"),
	)

	return &Evaluator{
		cfg:       cfg,
		retriever: retriever,
		responder: responder,
		client:    client,
	}, nil
}

func (e *Evaluator) EvaluateRetrieval(ctx context.Context, test models.TestQuestion) (*models.RetrievalEval, error) {
	chunks, err := e.retriever.Query(ctx, test.Question, e.cfg.TopK)
	if err != nil {
		return nil, err
	}

	var mrrScores []float64
	var ndcgScores []float64
	keywordsFound := 0

	for _, keyword := range test.Keywords {
		mrrScore := calculateMRR(keyword, chunks)
		mrrScores = append(mrrScores, mrrScore)

		ndcgScore := calculateNDCG(keyword, chunks, e.cfg.TopK)
		ndcgScores = append(ndcgScores, ndcgScore)

		if mrrScore > 0 {
			keywordsFound++
		}
	}

	avgMRR := 0.0
	avgNDCG := 0.0
	if len(mrrScores) > 0 {
		for _, score := range mrrScores {
			avgMRR += score
		}
		avgMRR /= float64(len(mrrScores))

		for _, score := range ndcgScores {
			avgNDCG += score
		}
		avgNDCG /= float64(len(ndcgScores))
	}

	keywordCoverage := 0.0
	if len(test.Keywords) > 0 {
		keywordCoverage = float64(keywordsFound) / float64(len(test.Keywords)) * 100
	}

	return &models.RetrievalEval{
		MRR:             avgMRR,
		NDCG:            avgNDCG,
		KeywordCoverage: keywordCoverage,
	}, nil
}

func (e *Evaluator) EvaluateAnswer(ctx context.Context, test models.TestQuestion) (*models.AnswerEval, string, []models.Chunk, error) {
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(test.Question),
	}
	generatedAnswer, err := e.responder.Respond(ctx, messages, test.Question)
	if err != nil {
		return nil, "", nil, err
	}

	chunks, err := e.retriever.Query(ctx, test.Question, e.cfg.TopK)
	if err != nil {
		return nil, generatedAnswer, nil, err
	}

	systemPrompt := "You are an expert evaluator assessing the quality of answers. Evaluate the generated answer by comparing it to the reference answer. Only give 5/5 scores for perfect answers. Respond ONLY with valid JSON."

	userPrompt := fmt.Sprintf(`Question:
%s

Generated Answer:
%s

Reference Answer:
%s

Evaluate the generated answer on three dimensions:
1. Accuracy: How factually correct is it compared to the reference answer? Only give 5/5 scores for perfect answers.
2. Completeness: How thoroughly does it address all aspects of the question, covering all the information from the reference answer?
3. Relevance: How well does it directly answer the specific question asked, giving no additional information?

Respond with ONLY valid JSON in this format:
{
  "feedback": "detailed feedback here",
  "accuracy": 5.0,
  "completeness": 5.0,
  "relevance": 5.0
}

If the answer is wrong, accuracy must be 1.`,
		test.Question, generatedAnswer, test.ReferenceAnswer)

	resp, err := e.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4o,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Temperature: openai.Float(0.0),
	})
	if err != nil {
		return nil, generatedAnswer, chunks, err
	}

	if len(resp.Choices) == 0 {
		return nil, generatedAnswer, chunks, fmt.Errorf("no response from LLM judge")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)

	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) > 2 {
			content = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var answerEval models.AnswerEval
	if err := json.Unmarshal([]byte(content), &answerEval); err != nil {
		return nil, generatedAnswer, chunks, fmt.Errorf("failed to parse judge response: %w", err)
	}

	return &answerEval, generatedAnswer, chunks, nil
}

func calculateMRR(keyword string, chunks []models.Chunk) float64 {
	keywordLower := strings.ToLower(keyword)
	for rank, chunk := range chunks {
		if strings.Contains(strings.ToLower(chunk.Text), keywordLower) {
			return 1.0 / float64(rank+1)
		}
	}
	return 0.0
}

func calculateDCG(relevances []int, k int) float64 {
	dcg := 0.0
	for i := 0; i < min(k, len(relevances)); i++ {
		dcg += float64(relevances[i]) / math.Log2(float64(i+2))
	}
	return dcg
}

func calculateNDCG(keyword string, chunks []models.Chunk, k int) float64 {
	keywordLower := strings.ToLower(keyword)

	relevances := make([]int, 0, min(k, len(chunks)))
	for i := 0; i < min(k, len(chunks)); i++ {
		if strings.Contains(strings.ToLower(chunks[i].Text), keywordLower) {
			relevances = append(relevances, 1)
		} else {
			relevances = append(relevances, 0)
		}
	}

	dcg := calculateDCG(relevances, k)

	idealRelevances := make([]int, len(relevances))
	copy(idealRelevances, relevances)
	for i := 0; i < len(idealRelevances); i++ {
		for j := i + 1; j < len(idealRelevances); j++ {
			if idealRelevances[i] < idealRelevances[j] {
				idealRelevances[i], idealRelevances[j] = idealRelevances[j], idealRelevances[i]
			}
		}
	}

	idcg := calculateDCG(idealRelevances, k)

	if idcg == 0 {
		return 0.0
	}

	return dcg / idcg
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
