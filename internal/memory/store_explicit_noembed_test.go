package memory

import (
	"context"
	"errors"
	"testing"
)

// TestSaveExplicit_WithoutAnEmbedderStillSavesAndRecalls was found live: a
// backend with only cloud providers and no local embedding model answered
// every "remember this" with a 500 carrying a raw "connection refused" to
// 127.0.0.1:8081. The fact must be saved anyway and still reach later turns
// through pinned-fact recall (which falls back to recency without vectors).
func TestSaveExplicit_WithoutAnEmbedderStillSavesAndRecalls(t *testing.T) {
	store, err := NewStore(StoreConfig{Dir: t.TempDir(), Dimension: 3, EmbeddingFunc: func(ctx context.Context, text string) ([]float32, error) {
		return nil, errors.New(`Post "http://127.0.0.1:8081/v1/embeddings": connection refused`)
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.SaveExplicit(ctx, "The user's favourite colour is orange.", "profile"); err != nil {
		t.Fatalf("SaveExplicit() error = %v, want the fact saved without a vector", err)
	}
	got, err := store.GetPinnedFactsRanked(ctx, "what colour do I like?", 5)
	if err != nil {
		t.Fatalf("GetPinnedFactsRanked() error = %v", err)
	}
	if len(got) != 1 || got[0].Content != "The user's favourite colour is orange." {
		t.Errorf("pinned facts = %+v, want the saved fact", got)
	}
}
