package qdrantstore

// code is based on
// https://github.com/qdrant/go-client

import (
	"context"
	"log"

	"github.com/lechgu/tichy/internal/config"
	"github.com/qdrant/go-client/qdrant"
)

// NewQdrantClient provides instance of new Qdrant client
// Upstream code should provide createCollection flag to enforce
// collection creation in Qdrant database
func NewQdrantClient(cfg *config.Config, createCollection bool) (*qdrant.Client, error) {
	qdrantCfg := &qdrant.Config{
		Host: cfg.Qdrant.Host,
		Port: cfg.Qdrant.Port,
	}
	client, err := qdrant.NewClient(qdrantCfg)
	if err != nil {
		log.Println("ERROR: unable to create new Qdrant client", err)
		return nil, err
	}
	if createCollection {
		err = createQdrantCollection(cfg, client)
	}
	return client, err
}

// helper function to create Qdrant collection
func createQdrantCollection(cfg *config.Config, client *qdrant.Client) error {
	var err error
	collection := cfg.Qdrant.Collection
	ctx := context.Background()
	colClient := client.GetCollectionsClient()
	_, err = colClient.Get(ctx, &qdrant.GetCollectionInfoRequest{
		CollectionName: collection,
	})
	if err == nil {
		if cfg.Qdrant.Recreate {
			// drop collection
			log.Printf("INFO: recreate '%s' collection", collection)
			_, err = colClient.Delete(ctx, &qdrant.DeleteCollection{
				CollectionName: collection,
			})
			if err != nil {
				return err
			}
		} else {
			return nil
		}
	}
	csize := cfg.Qdrant.CollectionSize
	if csize == 0 {
		csize = 1024 // default collection size
	}
	log.Printf("INFO: create '%s' collection with size %d", collection, csize)
	params := &qdrant.VectorParams{
		Size:     csize,
		Distance: qdrant.Distance_Cosine,
	}
	_, err = colClient.Create(ctx, &qdrant.CreateCollection{
		CollectionName: collection,
		VectorsConfig:  qdrant.NewVectorsConfig(params),
	})
	if err != nil {
		log.Fatalf("unable to create collection %s, error: %v", collection, err)
		return err
	}
	return nil
}
