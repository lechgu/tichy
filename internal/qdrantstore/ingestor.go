package qdrantstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/lechgu/tichy/internal/models"
	"github.com/qdrant/go-client/qdrant"
)

var ErrLengthMismatch = errors.New("chunks and embeddings length mismatch")

type QdrantIngestor struct {
	client     *qdrant.Client
	collection string
}

func NewQdrantIngestor(client *qdrant.Client, collection string) *QdrantIngestor {
	return &QdrantIngestor{client: client, collection: collection}
}

func (ing *QdrantIngestor) Ingest(ctx context.Context, chunks []models.Chunk, embeddings [][]float32) error {
	fmt.Println("ingesting docs into qdrant", len(chunks), len(embeddings))
	if len(chunks) != len(embeddings) {
		return ErrLengthMismatch
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

		/*
		points[i] = &qdrant.PointStruct{
			Id: &qdrant.PointId{PointIdOptions: &qdrant.PointId_Num{Num: uint64(i)}},
			Vectors: &qdrant.Vectors{
				VectorsOptions: &qdrant.Vectors_Vector{
					Vector: &qdrant.Vector{Data: embeddings[i]},
				},
			},
			Payload: toQdrantPayload(payloadMap),
		}
		*/
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

/*
func toQdrantPayload(fields map[string]interface{}) map[string]*qdrant.Value {
	out := make(map[string]*qdrant.Value, len(fields))
	for k, v := range fields {
		out[k] = &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: fmt.Sprint(v)}}
	}
	return out
}
*/
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
