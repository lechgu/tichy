// Package interfaces defines the shared behavioural contracts used across
// tichy's internal packages. Keeping all interfaces here ensures that:
//
//   - Implementations (pgvectorstore, qdrantstore, embedders, …) depend only
//     on this package and on models — never on each other.
//   - Higher-level packages (responders, injectors, commands, …) reference a
//     single canonical type rather than re-defining it locally.
//
// Dependency rule: interfaces/ → models only. No other internal import is
// permitted here, which keeps this package at the bottom of the import graph
// and guarantees it can never create a cycle.
package interfaces

import (
	"context"

	"github.com/lechgu/tichy/internal/models"
)

// Embedder is the minimal interface required by vector-store retrievers to
// convert a text query (or chunk) into a dense embedding vector.
// It is satisfied by *embedders.Embedder without that package needing to be
// imported directly by qdrantstore or pgvectorstore.
type Embedder interface {
	Embed(ctx context.Context, chunks []models.Chunk) ([][]float32, error)
}

// Ingestor writes chunks together with their pre-computed embeddings into a
// vector store. Implementations live in pgvectorstore and qdrantstore.
type Ingestor interface {
	Ingest(ctx context.Context, chunks []models.Chunk, embeddings [][]float32) error
}

// Retriever performs a semantic similarity search against a vector store and
// returns the topK most relevant chunks for a given query string.
//
// Query uses the collection that is resolved from the caller's context (e.g.
// via a JWT claim) or falls back to the backend's default collection.
//
// QueryCollection targets an explicitly named collection; pass an empty string
// to use the default. This is the entry-point used for concurrent multi-
// collection fan-out from responders.RespondMulti.
type Retriever interface {
	Query(ctx context.Context, query string, topK int) ([]models.Chunk, error)
	QueryCollection(ctx context.Context, collection, query string, topK int) ([]models.Chunk, error)
}
