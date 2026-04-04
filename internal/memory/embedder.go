package memory

import (
	"context"
	"fmt"

	"cortex/internal/api"

	chromem "github.com/philippgille/chromem-go"
)

func NewLMStudioEmbeddingFunc(client *api.Client, model string) chromem.EmbeddingFunc {
	return func(ctx context.Context, text string) ([]float32, error) {
		embedding, err := client.CreateEmbedding(ctx, model, text)
		if err != nil {
			return nil, fmt.Errorf("memory.LMStudioEmbedding: %w", err)
		}
		return embedding, nil
	}
}
