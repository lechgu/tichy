package vectorstore

import (
	"database/sql"
	"strings"

	"github.com/lechgu/tichy/internal/config"
	"github.com/lechgu/tichy/internal/embedders"
	"github.com/lechgu/tichy/internal/pgvectorstore"
	"github.com/lechgu/tichy/internal/qdrantstore"
	"github.com/qdrant/go-client/qdrant"
	"github.com/samber/do/v2"
)

// schemaRegistry maps collection name prefixes to the FilterSchema that
// should be applied during retrieval.  Register new schemas here as you add
// new collection types.
//
// Matching is by exact collection name first; if not found, by prefix.
// Add an entry with key "" as a final catch-all if every collection should
// use a default schema.
//
// Examples:
//
//	"elogs"        → ElogSchema       (exact match)
//	"images"       → ImageSchema      (exact match)
//	"fabric"       → SPARQLNodeSchema (exact match for Fabric / KG nodes)
//
// To disable filtering for a collection entirely, do not add it here.
var schemaRegistry = map[string]*qdrantstore.FilterSchema{
	"elogs":  &qdrantstore.ElogSchema,
	"images": &qdrantstore.ImageSchema,
	"fabric": &qdrantstore.SPARQLNodeSchema,
}

// ResolveSchema looks up the FilterSchema for a given collection name.
// Returns nil when no schema is registered (disables dynamic filtering).
func ResolveSchema(collection string) *qdrantstore.FilterSchema {
	var fKey string
	// if our collection name matches the filter key we'll use it
	for k := range schemaRegistry {
		if strings.Contains(collection, k) {
			fKey = k
			break
		}
	}
	if s, ok := schemaRegistry[fKey]; ok {
		return s
	}
	return nil
}

// Configure wires vector-store ingestor and retriever into the DI container.
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
			return qdrantstore.NewQdrantIngestor(cfg, client, cfg.Qdrant.Collection), nil
		})

		do.Provide(di, func(i do.Injector) (Retriever, error) {
			client, _ := do.Invoke[*qdrant.Client](i)
			embed, _ := do.Invoke[*embedders.Embedder](i)
			r := qdrantstore.NewQdrantRetriever(cfg, client, cfg.Qdrant.Collection, embed)
			// Attach the domain schema for the configured collection if one exists.
			// This enables dynamic LLM-based filter extraction without any
			// collection-name string matching in the retrieval hot path.
			if schema := ResolveSchema(cfg.Qdrant.Collection); schema != nil {
				r = r.WithSchema(schema)
			}
			return r, nil
		})
	}
}
