package qdrantstore

// code is based on
// https://github.com/qdrant/go-client

import (
	"context"
	"log"

	"github.com/lechgu/tichy/internal/config"
	"github.com/qdrant/go-client/qdrant"
)

func NewQdrantClient(cfg *config.Config) (*qdrant.Client, error) {
	qdrantCfg := &qdrant.Config{
		Host: cfg.Qdrant.Host,
		Port: cfg.Qdrant.Port,
	}
	client, err := qdrant.NewClient(qdrantCfg)
	if err != nil {
		log.Println("ERROR: unable to create new Qdrant client", err)
		return nil, err
	}
	err = createQdrantCollection(cfg, client)
	return client, err
}

func createQdrantCollection(cfg *config.Config, client *qdrant.Client) error {
	var err error
	collection := cfg.Qdrant.Collection
	ctx := context.Background()
	colClient := client.GetCollectionsClient()
	_, err = colClient.Get(ctx, &qdrant.GetCollectionInfoRequest{
		CollectionName: collection,
	})
	if err == nil {
		if !cfg.Qdrant.Recreate {
			return nil
		}
		// drop collection
		_, err = colClient.Delete(ctx, &qdrant.DeleteCollection{
			CollectionName: collection,
		})
	}
	log.Printf("INFO: creation '%s' collection", collection)
	csize := cfg.Qdrant.CollectionSize
	if csize == 0 {
		log.Println("WARNING: Qdrant collection size is not configured, using 1024")
		csize = 1024
	}
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
