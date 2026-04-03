package responders

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/lechgu/tichy/internal/config"
	"github.com/lechgu/tichy/internal/interfaces"
	"github.com/lechgu/tichy/internal/models"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/samber/do/v2"
)

// collectionResult holds the outcome of querying a single collection.
type collectionResult struct {
	collection string
	chunks     []models.Chunk
	err        error
}

// Responder wires retrieval + LLM response together.
type Responder struct {
	cfg                  *config.Config
	retriever            interfaces.Retriever
	client               openai.Client
	systemPromptTemplate string
}

func New(di do.Injector) (*Responder, error) {
	cfg, err := do.Invoke[*config.Config](di)
	if err != nil {
		return nil, err
	}

	retriever, err := do.Invoke[interfaces.Retriever](di)
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

// Respond handles a query against the default collection (backward-compatible).
func (r *Responder) Respond(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, query string) (string, error) {
	return r.RespondMulti(ctx, messages, query, nil)
}

// RespondMulti handles a query against one or more named collections concurrently.
// When collections is empty or nil the default collection is used.
func (r *Responder) RespondMulti(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, query string, collections []string) (string, error) {
	chunks, err := r.fetchChunks(ctx, query, collections)
	if err != nil {
		return "", err
	}

	ragContext := buildContext(chunks)
	systemPrompt := formatSystemPrompt(r.systemPromptTemplate, ragContext)

	llmMessages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
	}
	llmMessages = append(llmMessages, messages...)

	return callLLM(ctx, r.client, llmMessages)
}

// fetchChunks queries one collection (default) or many concurrently and merges results.
func (r *Responder) fetchChunks(ctx context.Context, query string, collections []string) ([]models.Chunk, error) {
	if len(collections) == 0 {
		// Legacy path – single default collection via Query().
		return r.retriever.Query(ctx, query, r.cfg.TopK)
	}

	results := make(chan collectionResult, len(collections))
	var wg sync.WaitGroup

	for _, col := range collections {
		wg.Add(1)
		go func(collection string) {
			defer wg.Done()
			chunks, err := r.retriever.QueryCollection(ctx, collection, query, r.cfg.TopK)
			results <- collectionResult{collection: collection, chunks: chunks, err: err}
		}(col)
	}

	// Close channel once all goroutines finish.
	go func() {
		wg.Wait()
		close(results)
	}()

	var allChunks []models.Chunk
	var errs []string
	for res := range results {
		if res.err != nil {
			errs = append(errs, fmt.Sprintf("collection %q: %v", res.collection, res.err))
			continue
		}
		allChunks = append(allChunks, res.chunks...)
	}

	// Return partial results if at least one collection succeeded.
	if len(allChunks) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("all collection queries failed: %s", strings.Join(errs, "; "))
	}

	return allChunks, nil
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
		text := chunk.Text
		if col, ok := chunk.Metadata["collection"]; ok && col != "" {
			text = fmt.Sprintf("[collection: %s]\n%s", col, chunk.Text)
		}
		parts = append(parts, text)
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
