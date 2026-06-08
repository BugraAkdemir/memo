package memory

import (
	"context"
	"fmt"

	"memo/internal/api"
)

func NewEmbeddingFunc(client *api.Client, model string) EmbeddingFunc {
	return func(ctx context.Context, text string) ([]float32, error) {
		embedding, err := client.CreateEmbedding(ctx, model, text)
		if err != nil {
			return nil, fmt.Errorf("memory.Embedding: %w", err)
		}
		return embedding, nil
	}
}

var NewLMStudioEmbeddingFunc = NewEmbeddingFunc
