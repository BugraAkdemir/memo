package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"memo/internal/models"
	"memo/internal/truncate"
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

// TestRecentSince covers the pure time-window fetch added for the
// self-insight digest feature — no query/semantic ranking, just "everything
// since this cutoff", unlike RetrieveContext/FilteredSearch.
func TestRecentSince(t *testing.T) {
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

	if err := store.SaveInteraction(ctx, "old fact", "noted"); err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}

	future := time.Now().Add(time.Hour)
	results, err := store.RecentSince(ctx, future, 10)
	if err != nil {
		t.Fatalf("RecentSince(future) error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("RecentSince(future) len = %d, want 0 (cutoff is after the only saved memory)", len(results))
	}

	past := time.Now().Add(-time.Hour)
	results, err = store.RecentSince(ctx, past, 10)
	if err != nil {
		t.Fatalf("RecentSince(past) error = %v", err)
	}
	if len(results) != 1 || !strings.Contains(results[0].Content, "old fact") {
		t.Fatalf("RecentSince(past) = %+v, want 1 result containing %q", results, "old fact")
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

	// Regression: SUM() over zero rows returns SQL NULL regardless of what
	// the CASE expression inside it would produce, which used to fail this
	// Scan outright ("converting NULL to int is unsupported") on a
	// completely empty table — i.e. every fresh install, before the first
	// memory is ever saved.
	stats := store.Stats()
	if stats.Dimension != 3 {
		t.Fatalf("Dimension = %d, want 3", stats.Dimension)
	}
	if stats.Count != 0 || stats.ExplicitCount != 0 || stats.PinnedTokens != 0 {
		t.Fatalf("empty-store stats = %+v, want all-zero counts", stats)
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
	if stats.PinnedTokens != 0 {
		t.Fatalf("PinnedTokens = %d, want 0 (SaveInteraction doesn't pin)", stats.PinnedTokens)
	}
}

// TestStats_PinnedTokens covers PinnedTokens specifically: only
// source='explicit' rows count toward it, and the estimate must match
// truncate.EstimateTokens' own len/3 heuristic exactly (same divisor,
// CharsPerTokenEstimate) so this number is directly comparable to what
// BuildSystemPrompt's own token-budget reasoning uses elsewhere.
func TestStats_PinnedTokens(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	// Not pinned — must not count toward PinnedTokens.
	if err := store.SaveInteraction(ctx, "some casual chat turn", "ok"); err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}

	fact1 := "User's dog is named Zeytin"
	fact2 := "User works as a backend engineer"
	if err := store.SaveExplicit(ctx, fact1, ""); err != nil {
		t.Fatalf("SaveExplicit(fact1) error = %v", err)
	}
	if err := store.SaveExplicit(ctx, fact2, ""); err != nil {
		t.Fatalf("SaveExplicit(fact2) error = %v", err)
	}

	want := (len(fact1) + len(fact2)) / truncate.CharsPerTokenEstimate
	stats := store.Stats()
	if stats.PinnedTokens != want {
		t.Fatalf("PinnedTokens = %d, want %d", stats.PinnedTokens, want)
	}
	if stats.ExplicitCount != 2 {
		t.Fatalf("ExplicitCount = %d, want 2", stats.ExplicitCount)
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

// TestSaveExplicit is a regression test: the INSERT's VALUES tuple used to
// list 16 comma-separated items against the 15-column list (a stray extra
// `?` placeholder ahead of chunk_index's literal 0), which SQLite rejects
// at prepare time with "16 values for 15 columns" — so every single call to
// SaveExplicit (and therefore the /remember chat command, and the memory
// import feature built on top of it) silently failed to save anything.
// Never caught before because SaveExplicit had zero test coverage.
func TestSaveExplicit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveExplicit(ctx, "user likes short answers", "imported"); err != nil {
		t.Fatalf("SaveExplicit() error = %v", err)
	}
	if store.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", store.Count())
	}

	results := store.DebugSearch(ctx, "user likes short answers", 5)
	if len(results) != 1 {
		t.Fatalf("DebugSearch() returned %d results, want 1", len(results))
	}
	if results[0].Source != "explicit" {
		t.Errorf("Source = %q, want explicit", results[0].Source)
	}
	if results[0].Importance != 5 {
		t.Errorf("Importance = %d, want 5", results[0].Importance)
	}
	if results[0].Tags != "imported" {
		t.Errorf("Tags = %q, want imported", results[0].Tags)
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

// TestSaveAndRetrieve_LongUnbrokenBlob is the regression test for BUG-H3's
// live-observed retrieve-path gap: chunkText's splitLongWord fix bounded
// SaveInteraction's userChunk, but saveChunk still appended the full,
// unbounded assistantMsg onto every chunk's embed text, and RetrieveContext
// embedded its raw query (plus expandQuery/splitCompoundQuery derivatives)
// directly with no bound at all. A fake embedding func that rejects any
// input over a batch-size-like byte limit (mirroring the real embedding
// server's "too large to process" error) reproduces both failures if
// capForEmbedding isn't applied at every one of those call sites.
func TestSaveAndRetrieve_LongUnbrokenBlob(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	const batchByteLimit = 1024
	limited := func(_ context.Context, text string) ([]float32, error) {
		if len(text) > batchByteLimit {
			return nil, fmt.Errorf("input (%d bytes) is too large to process (current batch size: %d)", len(text), batchByteLimit)
		}
		return []float32{1, 0, 0}, nil
	}

	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: limited})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	blob := strings.Repeat("a", 40000) // no spaces — exactly the reported repro shape
	longReply := strings.Repeat("b", 40000)

	if err := store.SaveInteraction(ctx, blob+" birkaç normal kelime", longReply); err != nil {
		t.Fatalf("SaveInteraction() with long unbroken blob + long reply error = %v, want nil", err)
	}

	if _, err := store.RetrieveContext(ctx, blob+" ve normal bir soru daha", 5, 0); err != nil {
		t.Fatalf("RetrieveContext() with long unbroken blob query error = %v, want nil", err)
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

func TestRecencyFactor(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	rfc := func(d time.Duration) string { return now.Add(-d).Format(time.RFC3339) }

	if got := recencyFactor(rfc(200*24*time.Hour), 0, now); got != 1 {
		t.Errorf("halfLifeDays=0 must disable: got %v, want 1", got)
	}
	if got := recencyFactor("", 30, now); got != 1 {
		t.Errorf("empty timestamp: got %v, want 1", got)
	}
	if got := recencyFactor("not-a-date", 30, now); got != 1 {
		t.Errorf("bad timestamp: got %v, want 1", got)
	}
	if got := recencyFactor(rfc(0), 30, now); got != 1 {
		t.Errorf("fresh memory: got %v, want 1", got)
	}
	if got := recencyFactor(rfc(-48*time.Hour), 30, now); got != 1 {
		t.Errorf("future timestamp: got %v, want 1", got)
	}
	if got := recencyFactor(rfc(30*24*time.Hour), 30, now); got < 0.49 || got > 0.51 {
		t.Errorf("one half-life old: got %v, want ~0.5", got)
	}
	if got := recencyFactor(rfc(300*24*time.Hour), 30, now); got != 0.15 {
		t.Errorf("ten half-lives old must hit the 0.15 floor: got %v", got)
	}
}
