package memory

import (
	"context"
	"errors"
	"strings"
	"testing"

	"memo/internal/models"
)

func TestSaveAndRetrieve(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewStore(StoreConfig{
		Dir:           dir,
		Dimension:     3,
		EmbeddingFunc: testEmbedding,
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveInteraction(ctx, "coffee beans", "arabica"); err != nil {
		t.Fatalf("SaveInteraction(coffee) error = %v", err)
	}
	if err := store.SaveInteraction(ctx, "mountain hiking", "boots"); err != nil {
		t.Fatalf("SaveInteraction(hiking) error = %v", err)
	}

	if store.Count() != 2 {
		t.Fatalf("Count() = %d, want 2", store.Count())
	}

	results, err := store.RetrieveContext(ctx, "coffee grinder", 1, 0)
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

func TestDeleteMemory(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewStore(StoreConfig{
		Dir:           dir,
		Dimension:     3,
		EmbeddingFunc: testEmbedding,
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

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

func TestClearAll(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewStore(StoreConfig{
		Dir:           dir,
		Dimension:     3,
		EmbeddingFunc: testEmbedding,
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveInteraction(ctx, "coffee beans", "arabica"); err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}
	if store.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", store.Count())
	}

	if err := store.ClearAll(); err != nil {
		t.Fatalf("ClearAll() error = %v", err)
	}
	if store.Count() != 0 {
		t.Fatalf("after ClearAll Count() = %d, want 0", store.Count())
	}
}

func TestStats(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewStore(StoreConfig{
		Dir:           dir,
		Dimension:     3,
		EmbeddingFunc: testEmbedding,
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	stats := store.Stats()
	if stats.Dimension != 3 {
		t.Fatalf("Dimension = %d, want 3", stats.Dimension)
	}

	if err := store.SaveInteraction(ctx, "test", "response"); err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}

	stats = store.Stats()
	if stats.Count != 1 {
		t.Fatalf("Count = %d, want 1", stats.Count)
	}
	if stats.VecCount != 1 {
		t.Fatalf("VecCount = %d, want 1", stats.VecCount)
	}
}

func TestFormatMemoriesForPrompt(t *testing.T) {
	memories := []models.MemoryResult{
		{Content: "test content", Similarity: 0.95, ID: "1"},
		{Content: "more content", Similarity: 0.50, ID: "2"},
	}

	result := FormatMemoriesForPrompt(memories)
	if !strings.Contains(result, "RELEVANT MEMORIES") {
		t.Error("should contain header")
	}
	if !strings.Contains(result, "95%") {
		t.Error("should contain 95% relevance")
	}
	if !strings.Contains(result, "50%") {
		t.Error("should contain 50% relevance")
	}

	empty := FormatMemoriesForPrompt(nil)
	if empty != "" {
		t.Error("should be empty for nil input")
	}
}

func TestHybridSearch_MatchTypeSet(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveInteraction(ctx, "coffee beans arabica", "great choice"); err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}

	results, err := store.RetrieveContext(ctx, "coffee", 5, 0)
	if err != nil {
		t.Fatalf("RetrieveContext() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results, got none")
	}
	// useFTS false ise (test ortamında fts5 yok) → "vector", true ise "hybrid" veya "fts" veya "vector"
	if results[0].MatchType == "" {
		t.Fatal("MatchType should not be empty")
	}
}

func TestSaveInteraction_Chunking(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	// 350 words, each well over the chunk token budget in aggregate (chunking
	// is now token-estimated, not word-counted — see chunker.go), so this
	// must split into multiple chunks rather than staying a single row.
	words := make([]string, 350)
	for i := range words {
		words[i] = "coffee"
	}
	if err := store.SaveInteraction(ctx, strings.Join(words, " "), "reply"); err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}
	if store.Count() <= 1 {
		t.Fatalf("Count() = %d, want >1 (chunked)", store.Count())
	}
}

func TestSaveInteraction_ShortNoChunk(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveInteraction(ctx, "coffee beans", "arabica"); err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}
	if store.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", store.Count())
	}
}

// TestSaveMerged_EmbedFailure_StillSaves is a regression guard for BUG-M3:
// when embedding the merged content fails, saveMerged used to save the
// merged row anyway (correct — losing the merge attempt would be worse than
// losing just its vector-searchability) but with zero indication anywhere
// that it happened, so the row silently and permanently never surfaces in
// similarity search. This test only asserts the (unchanged, intentional)
// save-without-a-vector behavior still holds; the accompanying log line
// isn't independently observable from here without a logx capture hook.
func TestSaveMerged_EmbedFailure_StillSaves(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	wantErr := errors.New("embedding service unavailable")
	store, err := NewStore(StoreConfig{
		Dir:       dir,
		Dimension: 3,
		EmbeddingFunc: func(context.Context, string) ([]float32, error) {
			return nil, wantErr
		},
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.saveMerged(ctx, "merged content", 1, 2); err != nil {
		t.Fatalf("saveMerged() error = %v, want nil (must save without a vector, not fail)", err)
	}

	var embedding []byte
	row := store.db.QueryRowContext(ctx, "SELECT embedding FROM memories WHERE source = 'merged'")
	if err := row.Scan(&embedding); err != nil {
		t.Fatalf("query saved merged row: %v", err)
	}
	if embedding != nil {
		t.Errorf("embedding = %v, want nil — a failed embed must not silently produce a stored vector", embedding)
	}
}

func testEmbedding(_ context.Context, text string) ([]float32, error) {
	text = strings.ToLower(text)
	switch {
	case strings.Contains(text, "coffee") || strings.Contains(text, "bean") || strings.Contains(text, "grinder"):
		return []float32{1, 0, 0}, nil
	case strings.Contains(text, "mountain") || strings.Contains(text, "hiking"):
		return []float32{0, 1, 0}, nil
	case strings.Contains(text, "test"):
		return []float32{0.5, 0.5, 0}, nil
	default:
		return []float32{0, 0, 1}, nil
	}
}
