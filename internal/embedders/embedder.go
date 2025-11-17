package embedders

import (
	"context"

	"github.com/lechgu/tichy/internal/config"
	"github.com/lechgu/tichy/internal/models"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/samber/do/v2"
)

type Embedder struct {
	cfg    *config.Config
	client openai.Client
}

func New(i do.Injector) (*Embedder, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, err
	}
	client := openai.NewClient(
		option.WithBaseURL(cfg.EmbeddingServerURL+"/v1"),
		option.WithAPIKey("not-needed"),
	)
	return &Embedder{
		cfg:    cfg,
		client: client,
	}, nil
}

func (e *Embedder) Embed(ctx context.Context, chunks []models.Chunk) ([][]float32, error) {
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Text
	}

	resp, err := e.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: texts,
		},
		Model: openai.EmbeddingModel("not-used"),
	})
	if err != nil {
		return nil, err
	}

	embeddings := make([][]float32, len(resp.Data))
	for i, data := range resp.Data {
		embedding32 := make([]float32, len(data.Embedding))
		for j, val := range data.Embedding {
			embedding32[j] = float32(val)
		}
		embeddings[i] = embedding32
	}

	return embeddings, nil
}
