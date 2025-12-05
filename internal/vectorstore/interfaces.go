package vectorstore

import (
    "context"

    "github.com/lechgu/tichy/internal/models"
)

type Ingestor interface {
    Ingest(ctx context.Context, chunks []models.Chunk, embeddings [][]float32) error
}

type Retriever interface {
    Query(ctx context.Context, query string, topK int) ([]models.Chunk, error)
}

