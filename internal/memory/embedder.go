package memory

import (
	"context"
	"fmt"

	"memo/internal/api"

	chromem "github.com/philippgille/chromem-go"
)

// NewEmbeddingFunc creates an embedding function that uses any OpenAI-compatible API client.
func NewEmbeddingFunc(client *api.Client, model string) chromem.EmbeddingFunc {
	return func(ctx context.Context, text string) ([]float32, error) {
		embedding, err := client.CreateEmbedding(ctx, model, text)
		if err != nil {
			return nil, fmt.Errorf("memory.Embedding: %w", err)
		}
		return embedding, nil
	}
}

// NewLMStudioEmbeddingFunc is kept for backwards compatibility.
// Deprecated: Use NewEmbeddingFunc instead.
var NewLMStudioEmbeddingFunc = NewEmbeddingFunc
