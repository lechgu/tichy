package pgvectorstore

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/lechgu/tichy/internal/models"
	"github.com/pgvector/pgvector-go"
)

type PgRetriever struct {
	db       *sql.DB
	embedder Embedder // your existing embedder interface
}

type Embedder interface {
	Embed(ctx context.Context, chunks []models.Chunk) ([][]float32, error)
}

func NewPgRetriever(db *sql.DB, embedder Embedder) *PgRetriever {
	return &PgRetriever{db: db, embedder: embedder}
}

func (r *PgRetriever) Query(ctx context.Context, query string, topK int) ([]models.Chunk, error) {
	embeddings, err := r.embedder.Embed(ctx, []models.Chunk{{Text: query}})
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
        SELECT text, source, chunk_index, metadata
        FROM chunks
        ORDER BY embedding <=> $1
        LIMIT $2
    `, pgvector.NewVector(embeddings[0]), topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Chunk
	for rows.Next() {
		var c models.Chunk
		var md []byte
		if err := rows.Scan(&c.Text, &c.Source, &c.Index, &md); err != nil {
			return nil, err
		}
		if md != nil {
			if err := json.Unmarshal(md, &c.Metadata); err != nil {
				return nil, err
			}
		}
		out = append(out, c)
	}

	return out, rows.Err()
}
