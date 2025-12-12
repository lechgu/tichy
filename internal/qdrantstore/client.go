package qdrantstore

// code is based on
// https://github.com/qdrant/go-client

import (
	"log"

	"github.com/lechgu/tichy/internal/config"
	"github.com/qdrant/go-client/qdrant"
)

// NewQdrantClient provides instance of new Qdrant client
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
	return client, err
}
