package memory

import (
	"context"
	"strings"
	"testing"

	chromem "github.com/philippgille/chromem-go"
)

func TestSeparatedMemorySearchLoadsOnlyMatchingDocument(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(t.TempDir(), testEmbedding)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if err := store.SaveInteraction(ctx, "coffee beans", "arabica"); err != nil {
		t.Fatalf("SaveInteraction(coffee) error = %v", err)
	}
	if err := store.SaveInteraction(ctx, "mountain hiking", "boots"); err != nil {
		t.Fatalf("SaveInteraction(hiking) error = %v", err)
	}

	reloaded, err := NewStore(store.persistDir, testEmbedding)
	if err != nil {
		t.Fatalf("reload NewStore() error = %v", err)
	}
	if reloaded.Count() != 2 {
		t.Fatalf("Count() = %d, want 2", reloaded.Count())
	}

	results, err := reloaded.RetrieveContext(ctx, "coffee grinder", 1, 0)
	if err != nil {
		t.Fatalf("RetrieveContext() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if !strings.Contains(results[0].Content, "coffee beans") {
		t.Fatalf("result content = %q, want coffee memory", results[0].Content)
	}
}

func TestDeleteGobFileRemovesDiskAndIndex(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(t.TempDir(), testEmbedding)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	if err := store.SaveInteraction(ctx, "coffee beans", "arabica"); err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}

	files := store.ListGobFiles()
	if len(files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(files))
	}
	if err := store.DeleteGobFile(files[0].Path); err != nil {
		t.Fatalf("DeleteGobFile() error = %v", err)
	}
	if store.Count() != 0 {
		t.Fatalf("Count() = %d, want 0", store.Count())
	}
}

func testEmbedding(_ context.Context, text string) ([]float32, error) {
	text = strings.ToLower(text)
	switch {
	case strings.Contains(text, "coffee") || strings.Contains(text, "bean") || strings.Contains(text, "grinder"):
		return []float32{1, 0, 0}, nil
	case strings.Contains(text, "mountain") || strings.Contains(text, "hiking"):
		return []float32{0, 1, 0}, nil
	default:
		return []float32{0, 0, 1}, nil
	}
}

var _ chromem.EmbeddingFunc = testEmbedding
