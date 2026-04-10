package qdrantstore

import (
	"context"
	"log"

	"github.com/lechgu/tichy/internal/auth"
	"github.com/lechgu/tichy/internal/config"
	"github.com/lechgu/tichy/internal/interfaces"
	"github.com/lechgu/tichy/internal/models"
	"github.com/qdrant/go-client/qdrant"
)

// QdrantRetriever defines Qdrant retriever structure.
type QdrantRetriever struct {
	client            *qdrant.Client
	defaultCollection string
	embedder          interfaces.Embedder
	llmServerURL      string

	// schema describes the filterable payload fields for this retriever's
	// collection.  When nil, no LLM-based filter extraction is performed and
	// the search is pure vector similarity.  Set a non-nil schema to enable
	// dynamic metadata filtering for any domain (elogs, images, SPARQL nodes…).
	schema *FilterSchema
}

// NewQdrantRetriever provides a new Qdrant retriever with no filter schema
// (pure vector search).  Use WithSchema to attach a domain schema.
func NewQdrantRetriever(
	cfg *config.Config,
	client *qdrant.Client,
	collection string,
	embedder interfaces.Embedder,
) *QdrantRetriever {
	return &QdrantRetriever{
		client:            client,
		defaultCollection: collection,
		embedder:          embedder,
		llmServerURL:      cfg.LLMServerURL,
	}
}

// WithSchema returns a copy of the retriever with the given FilterSchema
// attached.  Call this at wire-up time to enable domain-specific filtering:
//
//	retriever := NewQdrantRetriever(cfg, client, "elogs", embedder).
//	               WithSchema(&ElogSchema)
//
// Passing nil disables filter extraction (equivalent to the no-schema default).
func (r *QdrantRetriever) WithSchema(schema *FilterSchema) *QdrantRetriever {
	r.schema = schema
	return r
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

// queryCollection is the internal implementation shared by Query and QueryCollection.
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

	// Apply dynamic metadata filter when a schema is attached.
	// This replaces the previous "strings.Contains(collection, 'elog')" check,
	// which was fragile and prevented any other collection from using qdrantstore.
	if r.schema != nil && r.llmServerURL != "" {
		if params, err := ExtractQueryParams(ctx, r.llmServerURL, *r.schema, query); err == nil {
			if filter := BuildDynamicFilter(*r.schema, params); filter != nil {
				spoint.Filter = filter
			}
		} else {
			// Log but don't fail — fall back to unfiltered vector search.
			log.Printf("WARN: filter extraction failed for collection %q: %v", collection, err)
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
