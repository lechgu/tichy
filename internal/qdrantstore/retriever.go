package qdrantstore

import (
	"context"
	"log"
	"strings"

	"github.com/lechgu/tichy/internal/auth"
	"github.com/lechgu/tichy/internal/config"
	"github.com/lechgu/tichy/internal/interfaces"
	"github.com/lechgu/tichy/internal/models"
	"github.com/qdrant/go-client/qdrant"
)

// QdrantRetriever defines Qdrant retriever structure
type QdrantRetriever struct {
	client            *qdrant.Client
	defaultCollection string
	embedder          interfaces.Embedder
	LLMServerURL      string
}

// NewQdrantRetriever provides new Qdrant retriever
func NewQdrantRetriever(cfg *config.Config,
	client *qdrant.Client,
	collection string,
	embedder interfaces.Embedder) *QdrantRetriever {
	return &QdrantRetriever{
		client:            client,
		defaultCollection: collection,
		embedder:          embedder,
		LLMServerURL:      cfg.LLMServerURL,
	}
}

// Query retrieves chunks using the collection resolved from context or the default.
func (r *QdrantRetriever) Query(ctx context.Context, query string, topK int) ([]models.Chunk, error) {
	collection := r.defaultCollection
	if user, ok := auth.UserFromContext(ctx); ok {
		if len(user.VectorDBs) != 0 {
			log.Printf("INFO: receive request from %+v", user)
			collection = user.VectorDBs[0]
		}
	}
	return r.queryCollection(ctx, collection, query, topK)
}

// QueryCollection retrieves chunks from an explicitly named collection.
func (r *QdrantRetriever) QueryCollection(ctx context.Context, collection, query string, topK int) ([]models.Chunk, error) {
	if collection == "" {
		collection = r.defaultCollection
	}
	return r.queryCollection(ctx, collection, query, topK)
}

// queryCollection is the internal implementation that hits a specific Qdrant collection.
func (r *QdrantRetriever) queryCollection(ctx context.Context, collection, query string, topK int) ([]models.Chunk, error) {
	emb, err := r.embedder.Embed(ctx, []models.Chunk{{Text: query}})
	if err != nil {
		return nil, err
	}

	pclient := r.client.GetPointsClient()
	sel := qdrant.WithPayloadSelector{
		SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: true},
	}
	spoint := &qdrant.SearchPoints{
		CollectionName: collection,
		Vector:         emb[0],
		Limit:          uint64(topK),
		WithPayload:    &sel,
	}
	if strings.Contains(collection, "elog") {
		if qp, err := ExtractQueryParams(ctx, r.LLMServerURL, query); err == nil {
			if filter := BuildDynamicFilter(qp); filter != nil {
				spoint.Filter = filter
			}
		}
	}
	search, err := pclient.Search(ctx, spoint)
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
			Metadata: map[string]string{"collection": collection},
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
