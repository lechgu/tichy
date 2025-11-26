package qdrantstore

// code is based on
// https://github.com/qdrant/go-client

import (
	"github.com/lechgu/tichy/internal/config"
	"github.com/qdrant/go-client/qdrant"
)

func NewQdrantClient(cfg *config.Config) (*qdrant.Client, error) {
	qdrantCfg := &qdrant.Config{
		Host: cfg.Qdrant.Host,
		Port: cfg.Qdrant.Port,
	}
	return qdrant.NewClient(qdrantCfg)
}
