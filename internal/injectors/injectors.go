package injectors

import (
	"database/sql"
	"fmt"

	"github.com/lechgu/tichy/internal/chunkers"
	"github.com/lechgu/tichy/internal/config"
	"github.com/lechgu/tichy/internal/conversations"
	"github.com/lechgu/tichy/internal/databases"
	"github.com/lechgu/tichy/internal/embedders"
	"github.com/lechgu/tichy/internal/fetchers"
	"github.com/lechgu/tichy/internal/loggers"
	"github.com/lechgu/tichy/internal/pgvectorstore"
	"github.com/lechgu/tichy/internal/qdrantstore"
	"github.com/lechgu/tichy/internal/responders"
	"github.com/lechgu/tichy/internal/servers"
	"github.com/lechgu/tichy/internal/vectorstore"
	"github.com/samber/do/v2"
)

var Default do.Injector

func init() {
	Default = do.New()
	do.Provide(Default, config.New)
	do.Provide(Default, loggers.New)
	do.Provide(Default, databases.New)
	do.Provide(Default, chunkers.New)
	do.Provide(Default, embedders.New)
	//do.Provide(Default, ingestors.New)
	//do.Provide(Default, retrievers.New)

	// backend-selection provider will give us either pgvectorstore or chromadb one
	do.Provide(Default, provideIngestor)
	do.Provide(Default, provideRetriever)

	do.Provide(Default, responders.New)
	do.Provide(Default, conversations.New)
	do.Provide(Default, servers.New)
	do.ProvideNamed(Default, "text", fetchers.NewText)
}

func provideIngestor(i do.Injector) (vectorstore.Ingestor, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, err
	}

	switch cfg.VectorBackend {
	case "pgvector":
		db, _ := do.Invoke[*sql.DB](i)
		return pgvectorstore.NewPgIngestor(db), nil

	case "qdrant":
		client, err := qdrantstore.NewQdrantClient(cfg)
		if err != nil {
			return nil, err
		}
		collection := "collection" // TODO
		return qdrantstore.NewQdrantIngestor(client, collection), nil
	}

	return nil, fmt.Errorf("unknown backend %q", cfg.VectorBackend)
}

func provideRetriever(i do.Injector) (vectorstore.Retriever, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, err
	}

	switch cfg.VectorBackend {
	case "pgvector":
		db, _ := do.Invoke[*sql.DB](i)
		embed, _ := do.Invoke[*embedders.Embedder](i)
		return pgvectorstore.NewPgRetriever(db, embed), nil

	case "qdrant":
		client, err := qdrantstore.NewQdrantClient(cfg)
		if err != nil {
			return nil, err
		}
		collection := "collection" // TODO
		embed, _ := do.Invoke[*embedders.Embedder](i)
		return qdrantstore.NewQdrantRetriever(client, collection, embed), nil
	}

	return nil, fmt.Errorf("unknown backend %q", cfg.VectorBackend)
}
