package ingest

import (
	"errors"

	"github.com/lechgu/tichy/internal/chunkers"
	"github.com/lechgu/tichy/internal/embedders"
	"github.com/lechgu/tichy/internal/fetchers"
	"github.com/lechgu/tichy/internal/injectors"
	"github.com/lechgu/tichy/internal/models"
	"github.com/lechgu/tichy/internal/vectorstore"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"
)

var (
	docType string
	source  string
)

var Cmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingest documents into the vector store",
	RunE:  doIngest,
}

func init() {
	Cmd.Flags().StringVarP(&docType, "mode", "m", "", "Document fetch mode")
	Cmd.Flags().StringVarP(&source, "source", "s", "", "Source")
	_ = Cmd.MarkFlagRequired("mode")
	_ = Cmd.MarkFlagRequired("source")
}

func doIngest(cmd *cobra.Command, args []string) error {
	if docType != "text" {
		return errors.New("unsupported type: " + docType)
	}

	ctx := cmd.Context()

	fetcher, err := do.InvokeNamed[fetchers.Fetcher](injectors.Default, docType)
	if err != nil {
		return err
	}

	docs, err := fetcher.Fetch(ctx, source)
	if err != nil {
		return err
	}

	chunker, err := do.Invoke[*chunkers.Chunker](injectors.Default)
	if err != nil {
		return err
	}

	var allChunks []models.Chunk
	for _, doc := range docs {
		chunks, err := chunker.Chunk(doc)
		if err != nil {
			return err
		}
		allChunks = append(allChunks, chunks...)
	}

	embedder, err := do.Invoke[*embedders.Embedder](injectors.Default)
	if err != nil {
		return err
	}

	embeddings, err := embedder.Embed(ctx, allChunks)
	if err != nil {
		return err
	}

	ingestor, err := do.Invoke[vectorstore.Ingestor](injectors.Default)
	if err != nil {
		return err
	}

	if err := ingestor.Ingest(ctx, allChunks, embeddings); err != nil {
		return err
	}

	cmd.Println("OK")
	return nil
}
