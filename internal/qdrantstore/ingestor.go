package qdrantstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/lechgu/tichy/internal/config"
	"github.com/lechgu/tichy/internal/models"
	"github.com/qdrant/go-client/qdrant"
)

var ErrLengthMismatch = errors.New("chunks and embeddings length mismatch")

// QdrantIngestor structure defines Qdrant ingestor
type QdrantIngestor struct {
	client         *qdrant.Client
	collection     string
	recreate       bool
}

// NewQdrantIngestor provides new Qdrant ingestor
func NewQdrantIngestor(cfg *config.Config, client *qdrant.Client, collection string) *QdrantIngestor {
	return &QdrantIngestor{
		client:         client,
		collection:     collection,
		recreate:       cfg.Qdrant.Recreate,
	}
}

// CreateCollection creates new collection in Qdrant store if it does not exist
func (ing *QdrantIngestor) CreateCollection(csize uint64) error {
	var err error
	ctx := context.Background()
	colClient := ing.client.GetCollectionsClient()
	_, err = colClient.Get(ctx, &qdrant.GetCollectionInfoRequest{
		CollectionName: ing.collection,
	})
	if err == nil {
		if ing.recreate {
			// drop collection
			fmt.Printf("INFO: recreate '%s' collection", ing.collection)
			_, err = colClient.Delete(ctx, &qdrant.DeleteCollection{
				CollectionName: ing.collection,
			})
			if err != nil {
				return err
			}
		} else {
			return nil
		}
	}
	fmt.Printf("INFO: create '%s' collection with size %d", ing.collection, csize)
	params := &qdrant.VectorParams{
		Size:     csize,
		Distance: qdrant.Distance_Cosine,
	}
	_, err = colClient.Create(ctx, &qdrant.CreateCollection{
		CollectionName: ing.collection,
		VectorsConfig:  qdrant.NewVectorsConfig(params),
	})
	if err != nil {
		fmt.Printf("unable to create collection %s, error: %v", ing.collection, err)
		return err
	}
	return nil
}

// Ingest implements vectorestores Ingest method
func (ing *QdrantIngestor) Ingest(ctx context.Context, chunks []models.Chunk, embeddings [][]float32) error {
	fmt.Println("ingesting docs into qdrant", len(chunks), len(embeddings))
	if len(chunks) != len(embeddings) {
		return ErrLengthMismatch
	}

	// we should determine collection size from embeddings and create it appropriately
	// the embeddings vector dimension represents collection size
	csize := uint64(len(embeddings[0]))
	if err := ing.CreateCollection(csize); err != nil {
		return err
	}

	points := make([]*qdrant.PointStruct, len(chunks))

	for i, ch := range chunks {

		payloadMap := map[string]interface{}{
			"text":        ch.Text,
			"source":      ch.Source,
			"chunk_index": ch.Index,
		}
		for k, v := range ch.Metadata {
			payloadMap[k] = v
		}

		data := embeddings[i]
		points[i] = &qdrant.PointStruct{
			Id: qdrant.NewIDNum(uint64(i)),
			Vectors: qdrant.NewVectors(
				data...,
			),
			Payload: toQdrantPayload(payloadMap),
		}

	}

	_, err := ing.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: ing.collection,
		Points:         points,
	})
	return err
}

// helper function to provide Qdrant payload
func toQdrantPayload(fields map[string]interface{}) map[string]*qdrant.Value {
	out := make(map[string]*qdrant.Value, len(fields))
	for k, v := range fields {
		switch v := v.(type) {
		case int:
			out[k] = qdrant.NewValueInt(int64(v))
		case string:
			out[k] = qdrant.NewValueString(v)
		default:
			out[k] = qdrant.NewValueString(fmt.Sprint(v))
		}
	}
	return out
}
