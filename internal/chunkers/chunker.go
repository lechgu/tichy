package chunkers

import (
	"github.com/lechgu/tichy/internal/config"
	"github.com/lechgu/tichy/internal/models"
	"github.com/samber/do/v2"
	"github.com/tmc/langchaingo/textsplitter"
)

type Chunker struct {
	cfg      *config.Config
	splitter textsplitter.TextSplitter
}

func New(i do.Injector) (*Chunker, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, err
	}
	splitter := textsplitter.NewRecursiveCharacter(
		textsplitter.WithChunkSize(cfg.ChunkSize),
		textsplitter.WithChunkOverlap(cfg.ChunkOverlap),
		textsplitter.WithSeparators([]string{
			"\n## ", "\n### ", "\n#### ", "\n##### ", "\n###### ",
			"```\n\n", "\n\n", "\n", " ", "",
		}),
	)
	return &Chunker{
		cfg:      cfg,
		splitter: splitter,
	}, nil
}

func (c *Chunker) Chunk(doc models.Document) ([]models.Chunk, error) {
	texts, err := c.splitter.SplitText(doc.Content)
	if err != nil {
		return nil, err
	}

	chunks := make([]models.Chunk, 0, len(texts))
	for i, text := range texts {
		chunks = append(chunks, models.Chunk{
			Text:     text,
			Source:   doc.ID,
			Index:    i,
			Metadata: doc.Metadata,
		})
	}

	return chunks, nil
}
