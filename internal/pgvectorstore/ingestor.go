package pgvectorstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/lechgu/tichy/internal/models"
	"github.com/pgvector/pgvector-go"
)

var ErrLengthMismatch = errors.New("chunks and embeddings length mismatch")

// PgIngestor defines PostgresSQL ingestor structure
type PgIngestor struct {
	db *sql.DB
}

// NewPgIngestor defines new PostgresSQL ingestor
func NewPgIngestor(db *sql.DB) *PgIngestor {
	return &PgIngestor{db: db}
}

// Ingest implements vectorstore.Ingest method for PostgresSQL ingestor
func (ing *PgIngestor) Ingest(ctx context.Context, chunks []models.Chunk, embeddings [][]float32) error {
	if len(chunks) != len(embeddings) {
		return ErrLengthMismatch
	}

	tx, err := ing.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO chunks (text, source, chunk_index, metadata, embedding)
        VALUES ($1, $2, $3, $4, $5)
    `)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i, chunk := range chunks {
		md := []byte(nil)
		if chunk.Metadata != nil {
			md, err = json.Marshal(chunk.Metadata)
			if err != nil {
				return err
			}
		}

		_, err = stmt.ExecContext(ctx,
			chunk.Text,
			chunk.Source,
			chunk.Index,
			md,
			pgvector.NewVector(embeddings[i]),
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
