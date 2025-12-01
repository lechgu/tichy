package fetchers

import (
	"context"
	"os"
	"path/filepath"
	"slices"

	"github.com/lechgu/tichy/internal/config"
	"github.com/lechgu/tichy/internal/models"
	"github.com/samber/do/v2"
)

type TextFetcher struct {
	FileExtensions []string
}

func NewText(i do.Injector) (Fetcher, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, err
	}
	if len(cfg.FileExtensions) > 0 {
		return &TextFetcher{FileExtensions: cfg.FileExtensions}, nil
	}
	return &TextFetcher{FileExtensions: []string{".txt", ".md"}}, nil
}

func (t *TextFetcher) Fetch(ctx context.Context, source string) ([]models.Document, error) {
	var docs []models.Document

	err := filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if slices.Contains(t.FileExtensions, ext) {
			return nil
		}
		/*
			if ext != ".txt" && ext != ".md" {
				return nil
			}
		*/

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(source, path)

		docType := "document"
		dir := filepath.Dir(relPath)
		if dir != "." && dir != "" {
			normalizedPath := filepath.ToSlash(relPath)
			for i, c := range normalizedPath {
				if c == '/' {
					docType = normalizedPath[:i]
					break
				}
			}
		}

		docs = append(docs, models.Document{
			Content: string(content),
			ID:      path,
			Metadata: map[string]string{
				"filename":      filepath.Base(path),
				"relative_path": relPath,
				"type":          docType,
			},
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return docs, nil
}
