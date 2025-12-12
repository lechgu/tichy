package vectorstore

import (
	"database/sql"

	"github.com/lechgu/tichy/internal/config"
	"github.com/lechgu/tichy/internal/embedders"
	"github.com/lechgu/tichy/internal/pgvectorstore"
	"github.com/lechgu/tichy/internal/qdrantstore"
	"github.com/qdrant/go-client/qdrant"
	"github.com/samber/do/v2"
)

func Configure(di do.Injector, cfg *config.Config) {
	switch cfg.VectorBackend {
	case "pgvector":
		do.Provide(di, func(i do.Injector) (Ingestor, error) {
			db, _ := do.Invoke[*sql.DB](i)
			return pgvectorstore.NewPgIngestor(db), nil
		})
		do.Provide(di, func(i do.Injector) (Retriever, error) {
			db, _ := do.Invoke[*sql.DB](i)
			embed, _ := do.Invoke[*embedders.Embedder](i)
			return pgvectorstore.NewPgRetriever(db, embed), nil
		})

	case "qdrant":
		do.Provide(di, func(i do.Injector) (Ingestor, error) {
			client, _ := do.Invoke[*qdrant.Client](i)
			collection := cfg.Qdrant.Collection
			return qdrantstore.NewQdrantIngestor(cfg, client, collection), nil
		})

		do.Provide(di, func(i do.Injector) (Retriever, error) {
			client, _ := do.Invoke[*qdrant.Client](i)
			embed, _ := do.Invoke[*embedders.Embedder](i)
			collection := cfg.Qdrant.Collection
			return qdrantstore.NewQdrantRetriever(client, collection, embed), nil
		})
	}
}
