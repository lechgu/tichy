package qdrantstore

import (
	"context"

	"github.com/lechgu/tichy/internal/models"
	"github.com/qdrant/go-client/qdrant"
)

type Embedder interface {
	Embed(ctx context.Context, chunks []models.Chunk) ([][]float32, error)
}

type QdrantRetriever struct {
	client     *qdrant.Client
	collection string
	embedder   Embedder
}

func NewQdrantRetriever(client *qdrant.Client, collection string, embedder Embedder) *QdrantRetriever {
	return &QdrantRetriever{client: client, collection: collection, embedder: embedder}
}

func (r *QdrantRetriever) Query(ctx context.Context, query string, topK int) ([]models.Chunk, error) {
	emb, err := r.embedder.Embed(ctx, []models.Chunk{{Text: query}})
	if err != nil {
		return nil, err
	}

	pclient := r.client.GetPointsClient()
	search, err := pclient.Search(ctx, &qdrant.SearchPoints{
		CollectionName: r.collection,
		Vector:         emb[0],
		Limit:          uint64(topK),
		WithPayload: &qdrant.WithPayloadSelector{
			SelectorOptions: &qdrant.WithPayloadSelector_Include{},
		},
	})

	if err != nil {
		return nil, err
	}

	var out []models.Chunk
	for _, point := range search.Result {
		p := point.Payload
		c := models.Chunk{
			Text:     p["text"].GetStringValue(),
			Source:   p["source"].GetStringValue(),
			Index:    int(p["chunk_index"].GetIntegerValue()),
			Metadata: map[string]string{},
		}

		for k, v := range p {
			if k != "text" && k != "source" && k != "chunk_index" {
				c.Metadata[k] = v.GetStringValue()
			}
		}

		out = append(out, c)
	}

	return out, nil
}
