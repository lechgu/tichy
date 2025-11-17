package fetchers

import (
	"context"

	"github.com/lechgu/tichy/internal/models"
)

type Fetcher interface {
	Fetch(ctx context.Context, source string) ([]models.Document, error)
}
