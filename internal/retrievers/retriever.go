package retrievers

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/lechgu/tichy/internal/config"
	"github.com/lechgu/tichy/internal/embedders"
	"github.com/lechgu/tichy/internal/models"
	"github.com/pgvector/pgvector-go"
	"github.com/samber/do/v2"
)

type Retriever struct {
	cfg      *config.Config
	db       *sql.DB
	embedder *embedders.Embedder
}

func New(di do.Injector) (*Retriever, error) {
	cfg, err := do.Invoke[*config.Config](di)
	if err != nil {
		return nil, err
	}

	db, err := do.Invoke[*sql.DB](di)
	if err != nil {
		return nil, err
	}

	embedder, err := do.Invoke[*embedders.Embedder](di)
	if err != nil {
		return nil, err
	}

	return &Retriever{
		cfg:      cfg,
		db:       db,
		embedder: embedder,
	}, nil
}

func (r *Retriever) Query(ctx context.Context, query string, topK int) ([]models.Chunk, error) {
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
	defer func() {
		_ = rows.Close()
	}()

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

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return chunks, nil
}
