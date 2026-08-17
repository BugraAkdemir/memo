package memory

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// pinnedContents is a small test helper: GetPinnedFacts' content strings,
// in whatever order the store returns them.
func pinnedContents(t *testing.T, ctx context.Context, store *Store) []string {
	t.Helper()
	facts, err := store.GetPinnedFacts(ctx)
	if err != nil {
		t.Fatalf("GetPinnedFacts() error = %v", err)
	}
	out := make([]string, len(facts))
	for i, f := range facts {
		out[i] = f.Content
	}
	return out
}

func seedPinnedFacts(t *testing.T, ctx context.Context, store *Store, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := store.SaveExplicit(ctx, fmt.Sprintf("fact number %d", i), ""); err != nil {
			t.Fatalf("SaveExplicit(%d) error = %v", i, err)
		}
	}
}

// TestRunDream_CompressesRelatedFacts covers the main path:
// once the pinned set is at/above dreamThreshold, the whole set is
// handed to the summarize function in one batch, and — as long as it
// returns fewer, non-empty facts — the old rows are retired and the new,
// compressed set takes their place.
func TestRunDream_CompressesRelatedFacts(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	seedPinnedFacts(t, ctx, store, dreamThreshold)

	var gotBatchSize int
	compressed := []string{
		"User has a 3-year-old golden retriever named Zeytin, walks him every morning at 7am",
		"User works as a backend engineer",
	}
	fn := func(_ context.Context, facts []string) ([]string, error) {
		gotBatchSize = len(facts)
		return compressed, nil
	}

	store.runDream(ctx, fn)

	if gotBatchSize != dreamThreshold {
		t.Fatalf("summarize fn saw %d facts, want %d", gotBatchSize, dreamThreshold)
	}

	got := pinnedContents(t, ctx, store)
	if len(got) != len(compressed) {
		t.Fatalf("pinned facts after summarization = %d, want %d (got %v)", len(got), len(compressed), got)
	}
	want := map[string]bool{compressed[0]: true, compressed[1]: true}
	for _, c := range got {
		if !want[c] {
			t.Errorf("unexpected pinned fact after summarization: %q", c)
		}
	}
}

// TestRunDream_BelowThresholdSkipsLLM asserts the cheap guard
// actually guards: with fewer pinned facts than dreamThreshold,
// the (expensive) summarize function must never be invoked at all, and the
// existing set must be left completely untouched.
func TestRunDream_BelowThresholdSkipsLLM(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	seedPinnedFacts(t, ctx, store, dreamThreshold-1)
	before := pinnedContents(t, ctx, store)

	called := false
	fn := func(_ context.Context, facts []string) ([]string, error) {
		called = true
		return nil, fmt.Errorf("must not be called below threshold")
	}

	store.runDream(ctx, fn)

	if called {
		t.Fatal("summarize fn was called below dreamThreshold")
	}
	after := pinnedContents(t, ctx, store)
	if len(after) != len(before) {
		t.Fatalf("pinned fact count changed with no summarization run: before=%d after=%d", len(before), len(after))
	}
}

// TestRunDream_RejectsResultThatDoesNotShrink covers the
// "fails closed" contract: a summarize fn that doesn't actually reduce the
// count (buggy provider, or a model that just echoed the input back) must
// not be applied — losing nothing is worse than gaining nothing.
func TestRunDream_RejectsResultThatDoesNotShrink(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	seedPinnedFacts(t, ctx, store, dreamThreshold)
	before := pinnedContents(t, ctx, store)

	fn := func(_ context.Context, facts []string) ([]string, error) {
		return facts, nil // same count back — no real compression
	}

	store.runDream(ctx, fn)

	after := pinnedContents(t, ctx, store)
	if len(after) != len(before) {
		t.Fatalf("non-shrinking result was applied: before=%d after=%d", len(before), len(after))
	}
}

// TestRunDream_RejectsEmptyResult covers the other fail-closed
// case: an empty or whitespace-only result (extraction failure, provider
// error surfaced as an empty string) must never wipe out the pinned set.
func TestRunDream_RejectsEmptyResult(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	seedPinnedFacts(t, ctx, store, dreamThreshold)
	before := pinnedContents(t, ctx, store)

	fn := func(_ context.Context, facts []string) ([]string, error) {
		return []string{"", "   "}, nil
	}

	store.runDream(ctx, fn)

	after := pinnedContents(t, ctx, store)
	if len(after) != len(before) {
		t.Fatalf("empty result was applied: before=%d after=%d", len(before), len(after))
	}
}

// TestRunDreamNow_BypassesSchedulerThreshold covers the manual-trigger path
// (Settings tab's "run now" button): unlike the automatic schedule,
// RunDreamNow only needs a couple of facts to bother, not dreamThreshold.
func TestRunDreamNow_BypassesSchedulerThreshold(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	const seeded = 5
	if seeded >= dreamThreshold {
		t.Fatalf("test setup: seeded (%d) must be below dreamThreshold (%d)", seeded, dreamThreshold)
	}
	seedPinnedFacts(t, ctx, store, seeded)

	store.SetDreamFunc(func(_ context.Context, facts []string) ([]string, error) {
		return []string{"one compressed fact"}, nil
	})

	before, after, ran, err := store.RunDreamNow(ctx)
	if err != nil {
		t.Fatalf("RunDreamNow() error = %v", err)
	}
	if !ran {
		t.Fatal("RunDreamNow() ran = false, want true (below scheduler threshold but above manual minimum)")
	}
	if before != seeded {
		t.Errorf("before = %d, want %d", before, seeded)
	}
	if after != 1 {
		t.Errorf("after = %d, want 1", after)
	}
}

// TestRunDreamNow_DisabledReturnsError covers Dream being off
// (SetDreamFunc never called, or called with nil) — the manual trigger must
// report this clearly rather than silently no-op.
func TestRunDreamNow_DisabledReturnsError(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	seedPinnedFacts(t, ctx, store, 5)

	_, _, ran, err := store.RunDreamNow(ctx)
	if err == nil {
		t.Fatal("RunDreamNow() error = nil, want an error when Dream is disabled")
	}
	if ran {
		t.Fatal("RunDreamNow() ran = true while disabled")
	}
}

// TestRunDreamNow_TooFewFactsSkipsWithoutError covers the manual path's own
// (lower) floor: 1 pinned fact can't meaningfully be "compressed", so this
// should cleanly skip — not error, since nothing went wrong, there just
// wasn't enough to do.
func TestRunDreamNow_TooFewFactsSkipsWithoutError(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewStore(StoreConfig{Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	seedPinnedFacts(t, ctx, store, 1)
	store.SetDreamFunc(func(_ context.Context, facts []string) ([]string, error) {
		t.Fatal("dream fn must not be called with only 1 pinned fact")
		return nil, nil
	})

	before, after, ran, err := store.RunDreamNow(ctx)
	if err != nil {
		t.Fatalf("RunDreamNow() error = %v, want nil", err)
	}
	if ran {
		t.Fatal("RunDreamNow() ran = true with only 1 pinned fact")
	}
	if before != 1 || after != 1 {
		t.Errorf("before=%d after=%d, want 1/1", before, after)
	}
}

// TestRunDreamScheduler_RespectsEnabledFlag exercises the actual background
// goroutine (not just runDream/dreamPass directly): a very short initial
// delay + enabled=true must result in a call; enabled=false must not, even
// after the same wait — proving the scheduler re-reads DreamSettingsFunc
// rather than deciding once at startup.
func TestRunDreamScheduler_RespectsEnabledFlag(t *testing.T) {
	ctx := context.Background()

	t.Run("enabled", func(t *testing.T) {
		dir := t.TempDir()
		called := make(chan struct{}, 1)

		store, err := NewStore(StoreConfig{
			Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding,
			DreamSettings: func() (time.Duration, time.Duration, bool) {
				return 10 * time.Millisecond, time.Hour, true
			},
		})
		if err != nil {
			t.Fatalf("NewStore() error = %v", err)
		}
		defer store.Close()

		seedPinnedFacts(t, ctx, store, dreamThreshold)
		store.SetDreamFunc(func(_ context.Context, facts []string) ([]string, error) {
			select {
			case called <- struct{}{}:
			default:
			}
			return []string{"compressed"}, nil
		})

		select {
		case <-called:
		case <-time.After(3 * time.Second):
			t.Fatal("scheduler never called the dream fn while enabled")
		}
	})

	t.Run("disabled", func(t *testing.T) {
		dir := t.TempDir()
		called := make(chan struct{}, 1)

		store, err := NewStore(StoreConfig{
			Dir: dir, Dimension: 3, EmbeddingFunc: testEmbedding,
			DreamSettings: func() (time.Duration, time.Duration, bool) {
				return 10 * time.Millisecond, time.Hour, false
			},
		})
		if err != nil {
			t.Fatalf("NewStore() error = %v", err)
		}
		defer store.Close()

		seedPinnedFacts(t, ctx, store, dreamThreshold)
		store.SetDreamFunc(func(_ context.Context, facts []string) ([]string, error) {
			select {
			case called <- struct{}{}:
			default:
			}
			return []string{"compressed"}, nil
		})

		select {
		case <-called:
			t.Fatal("scheduler called the dream fn while disabled")
		case <-time.After(200 * time.Millisecond):
		}
	})
}
