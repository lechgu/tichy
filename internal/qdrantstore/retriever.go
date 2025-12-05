package qdrantstore

import (
	"context"
	"log"

	"github.com/lechgu/tichy/internal/auth"
	"github.com/lechgu/tichy/internal/models"
	"github.com/qdrant/go-client/qdrant"
)

// Embedder interace
type Embedder interface {
	Embed(ctx context.Context, chunks []models.Chunk) ([][]float32, error)
}

// QdrantRetriever defines Qdrant retriever structure
type QdrantRetriever struct {
	client     *qdrant.Client
	collection string
	embedder   Embedder
}

// NewQdrantRetriever provides new Qdrant retriever
func NewQdrantRetriever(client *qdrant.Client, collection string, embedder Embedder) *QdrantRetriever {
	return &QdrantRetriever{client: client, collection: collection, embedder: embedder}
}

// Query provides query method of retriever
func (r *QdrantRetriever) Query(ctx context.Context, query string, topK int) ([]models.Chunk, error) {
	emb, err := r.embedder.Embed(ctx, []models.Chunk{{Text: query}})
	if err != nil {
		return nil, err
	}

	// Check given context and extract user's collection to use
	collection := r.collection
	if user, ok := auth.UserFromContext(ctx); ok {
		if user.Collection != "" {
			log.Printf("INFO: receive request to user custom collection=%s from user=%s", user.Collection, user.Name)
			collection = user.Collection
		}

	}

	pclient := r.client.GetPointsClient()
	sel := qdrant.WithPayloadSelector{
		SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: true},
	}
	search, err := pclient.Search(ctx, &qdrant.SearchPoints{
		CollectionName: collection,
		Vector:         emb[0],
		Limit:          uint64(topK),
		WithPayload:    &sel,
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
