package ingestors

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/lechgu/tichy/internal/models"
	"github.com/pgvector/pgvector-go"
	"github.com/samber/do/v2"
)

var ErrLengthMismatch = errors.New("chunks and embeddings length mismatch")

type Ingestor struct {
	db *sql.DB
}

func New(i do.Injector) (*Ingestor, error) {
	db, err := do.Invoke[*sql.DB](i)
	if err != nil {
		return nil, err
	}
	return &Ingestor{
		db: db,
	}, nil
}

func (ing *Ingestor) Ingest(ctx context.Context, chunks []models.Chunk, embeddings [][]float32) error {
	if len(chunks) != len(embeddings) {
		return ErrLengthMismatch
	}

	tx, err := ing.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO chunks (text, source, chunk_index, metadata, embedding)
		VALUES ($1, $2, $3, $4, $5)
	`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for i, chunk := range chunks {
		var metadata []byte
		if chunk.Metadata != nil {
			metadata, err = json.Marshal(chunk.Metadata)
			if err != nil {
				return err
			}
		}

		_, err := stmt.ExecContext(ctx,
			chunk.Text,
			chunk.Source,
			chunk.Index,
			metadata,
			pgvector.NewVector(embeddings[i]),
		)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}
