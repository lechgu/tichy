package responders

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lechgu/tichy/internal/config"
	"github.com/lechgu/tichy/internal/models"
	"github.com/lechgu/tichy/internal/retrievers"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/samber/do/v2"
)

type Responder struct {
	cfg                  *config.Config
	retriever            *retrievers.Retriever
	client               openai.Client
	systemPromptTemplate string
}

func New(di do.Injector) (*Responder, error) {
	cfg, err := do.Invoke[*config.Config](di)
	if err != nil {
		return nil, err
	}

	retriever, err := do.Invoke[*retrievers.Retriever](di)
	if err != nil {
		return nil, err
	}

	client := openai.NewClient(
		option.WithBaseURL(cfg.LLMServerURL+"/v1"),
		option.WithAPIKey("not-needed"),
	)

	systemPromptTemplate, err := loadSystemPromptTemplate(cfg)
	if err != nil {
		return nil, err
	}

	return &Responder{
		cfg:                  cfg,
		retriever:            retriever,
		client:               client,
		systemPromptTemplate: systemPromptTemplate,
	}, nil
}

func (r *Responder) Respond(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, query string) (string, error) {
	chunks, err := r.retriever.Query(ctx, query, r.cfg.TopK)
	if err != nil {
		return "", err
	}

	context := buildContext(chunks)
	systemPrompt := formatSystemPrompt(r.systemPromptTemplate, context)

	llmMessages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
	}
	llmMessages = append(llmMessages, messages...)

	response, err := callLLM(ctx, r.client, llmMessages)
	if err != nil {
		return "", err
	}

	return response, nil
}

func loadSystemPromptTemplate(cfg *config.Config) (string, error) {
	const defaultTemplate = `You are a helpful assistant. Answer questions based on the provided context.
If you don't know the answer, say so.

Context:
{context}`

	if cfg.SystemPromptTemplate == "" {
		return defaultTemplate, nil
	}

	content, err := os.ReadFile(cfg.SystemPromptTemplate)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func buildContext(chunks []models.Chunk) string {
	var parts []string
	for _, chunk := range chunks {
		parts = append(parts, chunk.Text)
	}
	return strings.Join(parts, "\n\n---\n\n")
}

func formatSystemPrompt(template, context string) string {
	return strings.ReplaceAll(template, "{context}", context)
}

func callLLM(ctx context.Context, client openai.Client, messages []openai.ChatCompletionMessageParamUnion) (string, error) {
	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    openai.ChatModelGPT4o,
		Messages: messages,
	})
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return resp.Choices[0].Message.Content, nil
}
