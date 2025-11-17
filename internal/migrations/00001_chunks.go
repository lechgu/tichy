package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upChunks, downChunks)
}

func upChunks(ctx context.Context, tx *sql.Tx) error {
	dimension := 768
	if envDim := os.Getenv("EMBEDDING_DIMENSION"); envDim != "" {
		if d, err := strconv.Atoi(envDim); err == nil {
			dimension = d
		}
	}

	_, err := tx.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`
		CREATE TABLE chunks (
			id SERIAL PRIMARY KEY,
			text TEXT NOT NULL,
			source TEXT NOT NULL,
			chunk_index INTEGER NOT NULL,
			metadata JSONB,
			embedding vector(%d)
		)`, dimension)

	_, err = tx.ExecContext(ctx, query)
	return err
}

func downChunks(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS chunks")
	return err
}
