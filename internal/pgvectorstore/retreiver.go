package pgvectorstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/lechgu/tichy/internal/interfaces"
	"github.com/lechgu/tichy/internal/embedders"
	"github.com/lechgu/tichy/internal/models"
	"github.com/pgvector/pgvector-go"
	"github.com/samber/do/v2"
)

type PgRetriever struct {
	db       *sql.DB
	embedder interfaces.Embedder
}

func NewPgRetriever(db *sql.DB, embedder interfaces.Embedder) *PgRetriever {
	return &PgRetriever{db: db, embedder: embedder}
}

// New provides a new PgRetriever via DI
func New(di do.Injector) (*PgRetriever, error) {
	db, err := do.Invoke[*sql.DB](di)
	if err != nil {
		return nil, err
	}
	embed, err := do.Invoke[*embedders.Embedder](di)
	if err != nil {
		return nil, err
	}
	return NewPgRetriever(db, embed), nil
}

// Query retrieves chunks using the default "chunks" table.
func (r *PgRetriever) Query(ctx context.Context, query string, topK int) ([]models.Chunk, error) {
	return r.query(ctx, query, topK)
}

// QueryCollection for pgvector ignores the collection parameter (single schema),
// but satisfies the vectorstore.Retriever interface.
func (r *PgRetriever) QueryCollection(ctx context.Context, collection, query string, topK int) ([]models.Chunk, error) {
	// pgvector uses a single table; collection parameter is intentionally ignored.
	if collection != "" {
		// Optionally surface which collection was requested in metadata.
		_ = fmt.Sprintf("pgvector does not support named collections; ignoring %q", collection)
	}
	return r.query(ctx, query, topK)
}

func (r *PgRetriever) query(ctx context.Context, query string, topK int) ([]models.Chunk, error) {
	embeddings, err := r.embedder.Embed(ctx, []models.Chunk{{Text: query}})
	if err != nil {
		return nil, err
	}

	queryEmbedding := pgvector.NewVector(embeddings[0])

	rows, err := r.db.QueryContext(ctx, `
		SELECT text, source, chunk_index, metadata
		FROM chunks
		ORDER BY embedding <=> $1
		LIMIT $2
	`, queryEmbedding, topK)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var chunks []models.Chunk
	for rows.Next() {
		var chunk models.Chunk
		var metadataBytes []byte
		if err := rows.Scan(&chunk.Text, &chunk.Source, &chunk.Index, &metadataBytes); err != nil {
			return nil, err
		}
		if metadataBytes != nil {
			if err := json.Unmarshal(metadataBytes, &chunk.Metadata); err != nil {
				return nil, err
			}
		}
		chunks = append(chunks, chunk)
	}

	return chunks, rows.Err()
}
