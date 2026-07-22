package memory

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"testing"
)

// bagOfWordsEmbedding returns a deterministic EmbeddingFunc that behaves
// enough like a real sentence embedding to exercise realistic RAG failure
// modes: cosine similarity roughly tracks shared-word overlap, and a
// multi-topic (compound) question's vector is the normalized sum of all its
// topics' word vectors — so a topic mentioned alongside others gets diluted
// the same way a real embedding model dilutes it. The trivial fixed-vector
// testEmbedding used elsewhere in this package (3 dims, one axis per topic)
// can't reproduce that dilution; this one can, which is what makes
// TestRecall_CompoundQuery_ShortFactSurvivesNoise meaningful.
func bagOfWordsEmbedding(dim int) EmbeddingFunc {
	return func(_ context.Context, text string) ([]float32, error) {
		vec := make([]float32, dim)
		for w := range strings.FieldsSeq(strings.ToLower(text)) {
			h := fnv.New32a()
			_, _ = h.Write([]byte(w))
			idx := int(h.Sum32() % uint32(dim))
			vec[idx]++
		}
		var norm float64
		for _, v := range vec {
			norm += float64(v) * float64(v)
		}
		norm = math.Sqrt(norm)
		if norm == 0 {
			norm = 1
		}
		for i := range vec {
			vec[i] = float32(float64(vec[i]) / norm)
		}
		return vec, nil
	}
}

func newRecallStore(t *testing.T, embed EmbeddingFunc, dim int) *Store {
	t.Helper()
	store, err := NewStore(StoreConfig{
		Dir:           t.TempDir(),
		Dimension:     dim,
		EmbeddingFunc: embed,
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// newRecallStoreGoFallback is newRecallStore but forces the Go-fallback
// vector search (goSearch) via StoreConfig.ForceGoFallback — the path CI
// always exercises, since GitHub Actions' runner never has the sqlite-vec
// extension compiled in (see vecSearch's own comment for how differently
// the two implementations can rank near-identical candidates). Any dev
// machine that *does* have sqlite-vec available — this one included —
// silently only exercises vecSearch in every test that uses plain
// newRecallStore, so a goSearch-specific regression can pass locally and
// still fail in CI. Use this for recall tests where that distinction
// actually matters (ranking behavior), not for tests that only check
// plumbing (save/error paths).
//
// Deliberately goes through NewStore's own ForceGoFallback config rather
// than constructing a store and then setting store.useVec = false
// directly: NewStore starts a background goroutine (the vec-migration
// pass) that reads useVec concurrently, so mutating it from outside after
// construction returns is a genuine data race, caught directly by -race —
// not a hypothetical, this was the actual bug the first version of this
// helper had.
func newRecallStoreGoFallback(t *testing.T, embed EmbeddingFunc, dim int) *Store {
	t.Helper()
	store, err := NewStore(StoreConfig{
		Dir:             t.TempDir(),
		Dimension:       dim,
		EmbeddingFunc:   embed,
		ForceGoFallback: true,
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func containsMemory(results []MemoryResult, substr string) bool {
	for _, r := range results {
		if strings.Contains(r.Content, substr) {
			return true
		}
	}
	return false
}

// --- Duplicate-interaction skipping (BUG: repeated "selam" turns piling up) -

func runSkipsNearDuplicateRepeatedGreeting(t *testing.T, store *Store) {
	ctx := context.Background()

	// Mirrors the reported bug directly: the user sends the same generic
	// greeting several times in a row and the model keeps giving back the
	// exact same reply (see identity.go's anti-verbatim-copy fix for the
	// prompt-side half of this) — these turns must collapse to one memory
	// instead of five near-identical rows.
	for i := range 5 {
		if err := store.SaveInteraction(ctx, "selam", "selam! nasilsin"); err != nil {
			t.Fatalf("SaveInteraction() [%d] error = %v", i, err)
		}
	}
	if got := store.Count(); got != 1 {
		t.Fatalf("Count() after 5 near-identical greetings = %d, want 1 (later turns should be skipped as duplicates)", got)
	}

	if err := store.SaveInteraction(ctx, "dogum gunum 5 mayis", "not aldim"); err != nil {
		t.Fatalf("SaveInteraction() distinct turn error = %v", err)
	}
	if got := store.Count(); got != 2 {
		t.Fatalf("Count() after a genuinely distinct turn = %d, want 2 (distinct content must still be saved)", got)
	}
}

func TestSaveInteraction_SkipsNearDuplicateRepeatedGreeting_VecSearch(t *testing.T) {
	runSkipsNearDuplicateRepeatedGreeting(t, newRecallStore(t, bagOfWordsEmbedding(32), 32))
}

func TestSaveInteraction_SkipsNearDuplicateRepeatedGreeting_GoFallback(t *testing.T) {
	runSkipsNearDuplicateRepeatedGreeting(t, newRecallStoreGoFallback(t, bagOfWordsEmbedding(32), 32))
}

func TestSaveInteraction_NeverTreatsPinnedFactAsDuplicateTarget(t *testing.T) {
	ctx := context.Background()
	store := newRecallStore(t, bagOfWordsEmbedding(32), 32)

	if err := store.SaveExplicit(ctx, "kullanicinin adi Ahmet", "profile"); err != nil {
		t.Fatalf("SaveExplicit() error = %v", err)
	}
	// Same wording as the pinned fact, but going through the ordinary chat
	// path — this must still be saved as its own conversation memory, not
	// silently skipped because it resembles the pinned fact.
	if err := store.SaveInteraction(ctx, "kullanicinin adi Ahmet", ""); err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}

	if got := store.Count(); got != 2 {
		t.Fatalf("Count() = %d, want 2 (pinned fact + conversation turn, neither skipped)", got)
	}
	pinned, err := store.GetPinnedFacts(ctx)
	if err != nil {
		t.Fatalf("GetPinnedFacts() error = %v", err)
	}
	if len(pinned) != 1 {
		t.Fatalf("len(pinned) = %d, want 1 (pinned fact must be untouched by the duplicate check)", len(pinned))
	}
}

// --- Pinned facts (always-injected, bypass RAG ranking) ---------------------

func TestGetPinnedFacts_OnlyExplicitSource(t *testing.T) {
	ctx := context.Background()
	store := newRecallStore(t, bagOfWordsEmbedding(16), 16)

	if err := store.SaveInteraction(ctx, "kanka naber", "iyilik kanka"); err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}
	if err := store.SaveExplicit(ctx, "kullanicinin adi Ahmet", "profile"); err != nil {
		t.Fatalf("SaveExplicit() error = %v", err)
	}

	pinned, err := store.GetPinnedFacts(ctx)
	if err != nil {
		t.Fatalf("GetPinnedFacts() error = %v", err)
	}
	if len(pinned) != 1 {
		t.Fatalf("len(pinned) = %d, want 1 (routine conversation must not be pinned)", len(pinned))
	}
	if pinned[0].Source != "explicit" {
		t.Errorf("Source = %q, want explicit", pinned[0].Source)
	}
	if !strings.Contains(pinned[0].Content, "Ahmet") {
		t.Errorf("Content = %q, want it to contain Ahmet", pinned[0].Content)
	}
}

func TestGetPinnedFacts_EmptyStoreReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	store := newRecallStore(t, bagOfWordsEmbedding(16), 16)

	pinned, err := store.GetPinnedFacts(ctx)
	if err != nil {
		t.Fatalf("GetPinnedFacts() error = %v", err)
	}
	if len(pinned) != 0 {
		t.Fatalf("len(pinned) = %d, want 0", len(pinned))
	}
}

func TestGetPinnedFacts_RespectsCap(t *testing.T) {
	ctx := context.Background()
	store := newRecallStore(t, bagOfWordsEmbedding(16), 16)

	for i := range pinnedFactsLimit + 10 {
		if err := store.SaveExplicit(ctx, fmt.Sprintf("gercek numara %d", i), "profile"); err != nil {
			t.Fatalf("SaveExplicit(%d) error = %v", i, err)
		}
	}

	pinned, err := store.GetPinnedFacts(ctx)
	if err != nil {
		t.Fatalf("GetPinnedFacts() error = %v", err)
	}
	if len(pinned) != pinnedFactsLimit {
		t.Fatalf("len(pinned) = %d, want %d (cap must hold even with more explicit facts saved)", len(pinned), pinnedFactsLimit)
	}
}

// TestFindMergeCandidates_ExcludesExplicitFacts is a regression test: the
// candidate pool used to exclude only source='merged', so two near-duplicate
// pinned (source='explicit') facts could be selected as a merge pair —
// saveMerged writes the result as source='merged'/importance=4, which
// GetPinnedFacts' WHERE clause no longer matches, silently un-pinning a fact
// that auto-extraction (internal/app/memory.go) had just pinned. Two
// identical-embedding explicit facts here would otherwise score cosine
// similarity 1.0, comfortably above the 0.92 merge threshold.
func TestFindMergeCandidates_ExcludesExplicitFacts(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(StoreConfig{
		Dir:       t.TempDir(),
		Dimension: 4,
		EmbeddingFunc: func(_ context.Context, _ string) ([]float32, error) {
			return []float32{1, 0, 0, 0}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveExplicit(ctx, "kullanicinin adi Ahmet", "profile"); err != nil {
		t.Fatalf("SaveExplicit(1) error = %v", err)
	}
	if err := store.SaveExplicit(ctx, "kullanicinin ismi Ahmet", "profile"); err != nil {
		t.Fatalf("SaveExplicit(2) error = %v", err)
	}

	candidates, err := store.FindMergeCandidates(ctx, 5)
	if err != nil {
		t.Fatalf("FindMergeCandidates() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("FindMergeCandidates() = %+v, want none — pinned facts must never be auto-merged/un-pinned", candidates)
	}
}

// TestFindPinnedMergeCandidates_MatchesNearDuplicatePinnedFacts is the direct
// counterpart to TestFindMergeCandidates_ExcludesExplicitFacts above: the two
// near-duplicate pinned facts that FindMergeCandidates correctly refuses to
// touch must still be discoverable through the pinned-only path, or nothing
// in the codebase actually dedups the pinned set (see pinnedFactsLimit's doc
// comment for the gap this closes).
func TestFindPinnedMergeCandidates_MatchesNearDuplicatePinnedFacts(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(StoreConfig{
		Dir:       t.TempDir(),
		Dimension: 4,
		EmbeddingFunc: func(_ context.Context, _ string) ([]float32, error) {
			return []float32{1, 0, 0, 0}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveExplicit(ctx, "kullanicinin adi Ahmet", "profile"); err != nil {
		t.Fatalf("SaveExplicit(1) error = %v", err)
	}
	if err := store.SaveExplicit(ctx, "kullanicinin ismi Ahmet", "profile"); err != nil {
		t.Fatalf("SaveExplicit(2) error = %v", err)
	}

	candidates, err := store.FindPinnedMergeCandidates(ctx, 5)
	if err != nil {
		t.Fatalf("FindPinnedMergeCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("FindPinnedMergeCandidates() = %+v, want exactly 1 pair", candidates)
	}
}

// TestFindPinnedMergeCandidates_ExcludesNonExplicit is the mirror image of
// the test above: a near-duplicate pair of ordinary (non-pinned) memories
// must never be surfaced by the pinned-only finder, even at cosine
// similarity 1.0 — this path exists specifically for the small, curated
// explicit set, not the much larger general conversational pool (that's
// FindMergeCandidates' job).
func TestFindPinnedMergeCandidates_ExcludesNonExplicit(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(StoreConfig{
		Dir:       t.TempDir(),
		Dimension: 4,
		EmbeddingFunc: func(_ context.Context, _ string) ([]float32, error) {
			return []float32{1, 0, 0, 0}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveInteraction(ctx, "kanka naber", "iyilik kanka"); err != nil {
		t.Fatalf("SaveInteraction(1) error = %v", err)
	}
	if err := store.SaveInteraction(ctx, "naber kanka", "kanka iyilik"); err != nil {
		t.Fatalf("SaveInteraction(2) error = %v", err)
	}

	candidates, err := store.FindPinnedMergeCandidates(ctx, 5)
	if err != nil {
		t.Fatalf("FindPinnedMergeCandidates() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("FindPinnedMergeCandidates() = %+v, want none — only source=explicit/importance=5 rows are eligible", candidates)
	}
}

// TestSavePinnedMerged_StaysPinned confirms savePinnedMerged's whole reason
// to exist: unlike saveMerged (source='merged'/importance=4), its result
// must still satisfy GetPinnedFacts' WHERE clause, and the two originals it
// replaces must no longer appear there (pending_deletion=1).
func TestSavePinnedMerged_StaysPinned(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(StoreConfig{
		Dir:       t.TempDir(),
		Dimension: 4,
		EmbeddingFunc: func(_ context.Context, _ string) ([]float32, error) {
			return []float32{1, 0, 0, 0}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveExplicit(ctx, "kullanicinin adi Ahmet", "profile"); err != nil {
		t.Fatalf("SaveExplicit(1) error = %v", err)
	}
	if err := store.SaveExplicit(ctx, "kullanicinin ismi Ahmet", "profile"); err != nil {
		t.Fatalf("SaveExplicit(2) error = %v", err)
	}

	candidates, err := store.FindPinnedMergeCandidates(ctx, 5)
	if err != nil {
		t.Fatalf("FindPinnedMergeCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("FindPinnedMergeCandidates() = %+v, want exactly 1 pair", candidates)
	}
	c := candidates[0]

	if err := store.savePinnedMerged(ctx, "Kullanicinin adi Ahmet", c.ID1, c.ID2); err != nil {
		t.Fatalf("savePinnedMerged() error = %v", err)
	}

	pinned, err := store.GetPinnedFacts(ctx)
	if err != nil {
		t.Fatalf("GetPinnedFacts() error = %v", err)
	}
	if len(pinned) != 1 {
		t.Fatalf("len(pinned) = %d, want 1 (merge result must stay pinned, originals must drop out)", len(pinned))
	}
	if pinned[0].Source != "explicit" {
		t.Errorf("Source = %q, want explicit", pinned[0].Source)
	}
	if !strings.Contains(pinned[0].Content, "Ahmet") {
		t.Errorf("Content = %q, want it to contain Ahmet", pinned[0].Content)
	}
}

// TestRunPinnedConsolidation_MergesAndKeepsPinned exercises the full
// runPinnedConsolidation path (find → LLM merge → save) with a fake
// ConsolidationFunc standing in for mergeMemoriesLLM, confirming the merge
// result is reachable via GetPinnedFacts afterward — the same guarantee
// TestSavePinnedMerged_StaysPinned checks one level lower, here through the
// actual scheduled entry point (applyImportanceRules calls this).
func TestRunPinnedConsolidation_MergesAndKeepsPinned(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(StoreConfig{
		Dir:       t.TempDir(),
		Dimension: 4,
		EmbeddingFunc: func(_ context.Context, _ string) ([]float32, error) {
			return []float32{1, 0, 0, 0}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveExplicit(ctx, "kullanicinin adi Ahmet", "profile"); err != nil {
		t.Fatalf("SaveExplicit(1) error = %v", err)
	}
	if err := store.SaveExplicit(ctx, "kullanicinin ismi Ahmet", "profile"); err != nil {
		t.Fatalf("SaveExplicit(2) error = %v", err)
	}

	fakeMerge := func(_ context.Context, content1, _ string) (string, error) {
		return content1, nil
	}
	store.runPinnedConsolidation(ctx, fakeMerge)

	pinned, err := store.GetPinnedFacts(ctx)
	if err != nil {
		t.Fatalf("GetPinnedFacts() error = %v", err)
	}
	if len(pinned) != 1 {
		t.Fatalf("len(pinned) = %d, want 1 after pinned consolidation merges the near-duplicate pair", len(pinned))
	}
	if pinned[0].Source != "explicit" {
		t.Errorf("Source = %q, want explicit", pinned[0].Source)
	}
}

// TestRunConsolidation_NeverTouchesPinnedFacts confirms runConsolidation
// (the general-pool path) leaves the pinned set completely alone even when
// a fake ConsolidationFunc is wired up — FindMergeCandidates already
// excludes source='explicit' (TestFindMergeCandidates_ExcludesExplicitFacts),
// this just checks the same guarantee holds through the actual scheduled
// entry point, not just the candidate-finding step in isolation.
func TestRunConsolidation_NeverTouchesPinnedFacts(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(StoreConfig{
		Dir:       t.TempDir(),
		Dimension: 4,
		EmbeddingFunc: func(_ context.Context, _ string) ([]float32, error) {
			return []float32{1, 0, 0, 0}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveExplicit(ctx, "kullanicinin adi Ahmet", "profile"); err != nil {
		t.Fatalf("SaveExplicit(1) error = %v", err)
	}
	if err := store.SaveExplicit(ctx, "kullanicinin ismi Ahmet", "profile"); err != nil {
		t.Fatalf("SaveExplicit(2) error = %v", err)
	}

	fakeMerge := func(_ context.Context, content1, _ string) (string, error) {
		return content1, nil
	}
	store.runConsolidation(ctx, fakeMerge)

	pinned, err := store.GetPinnedFacts(ctx)
	if err != nil {
		t.Fatalf("GetPinnedFacts() error = %v", err)
	}
	if len(pinned) != 2 {
		t.Fatalf("len(pinned) = %d, want 2 — the general consolidation pass must never merge/un-pin explicit facts", len(pinned))
	}
}

// runRetrieveContextExcludesPendingDeletion is the regression test for
// BUG-H5: none of vecSearch/goSearch/ftsSearch filtered pending_deletion,
// and RetrieveContext didn't filter it downstream either — so a row a
// consolidation merge marks pending_deletion=1 (saveMergedAs sets the flag
// but leaves the row, its vector, and its FTS entry fully intact —
// PurgePendingDeletions is the only thing that ever removes them, up to
// ~187 days later) kept resurfacing in ordinary RAG retrieval as a
// near-duplicate of the very merged row that was supposed to replace it.
// Directly marks a saved row pending_deletion=1 (matching exactly the
// state saveMergedAs leaves originals in) rather than going through a real
// consolidation run, to isolate this from FindMergeCandidates' own
// similarity-threshold behavior — that path is already covered by
// TestFindMergeCandidates_ExcludesExplicitFacts and friends.
func runRetrieveContextExcludesPendingDeletion(t *testing.T, store *Store) {
	t.Helper()
	ctx := context.Background()

	if err := store.SaveInteraction(ctx, "kullanicinin kedisinin adi Pamuk", "kaydedildi"); err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}

	before, err := store.RetrieveContext(ctx, "kedimin adi ne", 5, 0)
	if err != nil {
		t.Fatalf("RetrieveContext() (before) error = %v", err)
	}
	if !containsMemory(before, "Pamuk") {
		t.Fatalf("expected the cat fact to be retrievable before marking pending_deletion, got %+v", before)
	}

	if err := store.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE memories SET pending_deletion = 1 WHERE content LIKE '%Pamuk%'")
		return err
	}); err != nil {
		t.Fatalf("mark pending_deletion: %v", err)
	}

	after, err := store.RetrieveContext(ctx, "kedimin adi ne", 5, 0)
	if err != nil {
		t.Fatalf("RetrieveContext() (after) error = %v", err)
	}
	if containsMemory(after, "Pamuk") {
		t.Fatalf("RetrieveContext() still returned a pending_deletion=1 row, got %+v", after)
	}
}

func TestRetrieveContext_ExcludesPendingDeletion(t *testing.T) {
	runRetrieveContextExcludesPendingDeletion(t, newRecallStore(t, bagOfWordsEmbedding(32), 32))
}

func TestRetrieveContext_ExcludesPendingDeletion_GoFallback(t *testing.T) {
	runRetrieveContextExcludesPendingDeletion(t, newRecallStoreGoFallback(t, bagOfWordsEmbedding(32), 32))
}

// --- Single/multi-fact recall -------------------------------------------------

func TestRecall_SingleExplicitFact(t *testing.T) {
	ctx := context.Background()
	store := newRecallStore(t, bagOfWordsEmbedding(32), 32)

	if err := store.SaveExplicit(ctx, "kullanicinin favori rengi kirmizidir", "profile"); err != nil {
		t.Fatalf("SaveExplicit() error = %v", err)
	}

	results, err := store.RetrieveContext(ctx, "favori rengi nedir", 3, 0)
	if err != nil {
		t.Fatalf("RetrieveContext() error = %v", err)
	}
	if !containsMemory(results, "kirmizidir") {
		t.Fatalf("expected the color fact in results, got %+v", results)
	}
}

// TestRecall_MultipleDistinctFacts_EachIndependentlyRetrievable saves several
// unrelated facts about the same user (name, birthday, color, job, pet) and
// checks each is retrievable on its own — the baseline a RAG memory system
// must clear before compound-query behavior matters at all.
func TestRecall_MultipleDistinctFacts_EachIndependentlyRetrievable(t *testing.T) {
	ctx := context.Background()
	store := newRecallStore(t, bagOfWordsEmbedding(48), 48)

	facts := map[string]string{
		"isim":        "kullanicinin adi Ahmet",
		"dogum":       "kullanicinin dogum gunu 5 Mayis 1995",
		"renk":        "kullanicinin favori rengi kirmizi",
		"meslek":      "kullanici yazilim muhendisi olarak calisiyor",
		"evcilhayvan": "kullanicinin bir kedisi var adi Pamuk",
	}
	for _, content := range facts {
		if err := store.SaveExplicit(ctx, content, "profile"); err != nil {
			t.Fatalf("SaveExplicit(%q) error = %v", content, err)
		}
	}

	tests := []struct {
		query string
		want  string
	}{
		{"kullanicinin adi ne", "Ahmet"},
		{"dogum gunu ne zaman", "1995"},
		{"favori renk hangisi", "kirmizi"},
		{"kullanici ne is yapiyor", "muhendisi"},
		{"evcil hayvani var mi", "Pamuk"},
	}
	for _, tc := range tests {
		results, err := store.RetrieveContext(ctx, tc.query, 2, 0)
		if err != nil {
			t.Fatalf("RetrieveContext(%q) error = %v", tc.query, err)
		}
		if !containsMemory(results, tc.want) {
			t.Errorf("query %q: expected %q among top results, got %+v", tc.query, tc.want, results)
		}
	}
}

// TestRecall_CompoundQuery_ShortFactSurvivesNoise reproduces the exact
// reported bug: a user tells Memo a short, single fact ("favorite color:
// red"), then in a later, unrelated compound question that also touches
// other topics, the fact must still surface. The corpus is engineered so
// several "noise" memories partially overlap the query's other topics (name,
// birthday) — mimicking real casual conversation that mentions a name or a
// date in passing — which is exactly the kind of partial-overlap noise that
// can outrank a precise single-topic fact under pure vector cosine
// similarity. This is the regression test for both bugs fixed alongside it:
// FTS5 never being compiled into any build (see fts5-build-tag-required
// memory / commit e4889e1), and escapeFTSQuery's implicit AND semantics
// silently discarding the FTS candidate list for any multi-word natural
// -language question (see escapeFTSQuery's doc comment).
func TestRecall_CompoundQuery_ShortFactSurvivesNoise(t *testing.T) {
	ctx := context.Background()
	store := newRecallStore(t, bagOfWordsEmbedding(64), 64)

	if !store.useFTS {
		t.Skip(`requires -tags "sqlite_fts5" to compile FTS5 support into the test binary; ` +
			`without it, this scenario is exactly the reported bug — see fts5-build-tag-required memory`)
	}

	// Noise: casual conversation snippets that share the "name"/"birthday"
	// topic words with the eventual compound query, without being the fact
	// we care about.
	for i := range 15 {
		if err := store.SaveInteraction(ctx,
			fmt.Sprintf("Ahmet bugun dogum gunu partisi hakkinda konustu sohbet %d", i),
			"ilginc bir sohbetti",
		); err != nil {
			t.Fatalf("SaveInteraction(noise %d) error = %v", i, err)
		}
	}

	// The actual fact, saved once, exactly like the real /remember flow.
	if err := store.SaveExplicit(ctx, "kullanicinin en sevdigi renk kirmizidir", "profile"); err != nil {
		t.Fatalf("SaveExplicit(color fact) error = %v", err)
	}

	// A brand-new-session, single compound question — the literal repro.
	results, err := store.RetrieveContext(ctx, "adimi ve dogum gunumu ve en sevdigim rengi biliyor musun", 5, 0)
	if err != nil {
		t.Fatalf("RetrieveContext() error = %v", err)
	}
	if !containsMemory(results, "kirmizidir") {
		t.Fatalf("compound query failed to recall the color fact among noise; got %+v", results)
	}
}

// --- Persistence across sessions ---------------------------------------------

// TestRecall_PersistsAcrossStoreReopen simulates the real-world scenario: a
// fact is saved in one process/session, the app is closed, and a brand-new
// session (new Store instance, same on-disk DB) must still recall it.
func TestRecall_PersistsAcrossStoreReopen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	embed := bagOfWordsEmbedding(32)

	store1, err := NewStore(StoreConfig{Dir: dir, Dimension: 32, EmbeddingFunc: embed})
	if err != nil {
		t.Fatalf("NewStore() (session 1) error = %v", err)
	}
	if err := store1.SaveExplicit(ctx, "kullanicinin favori rengi kirmizidir", "profile"); err != nil {
		t.Fatalf("SaveExplicit() error = %v", err)
	}
	if err := store1.Close(); err != nil {
		t.Fatalf("Close() (session 1) error = %v", err)
	}

	store2, err := NewStore(StoreConfig{Dir: dir, Dimension: 32, EmbeddingFunc: embed})
	if err != nil {
		t.Fatalf("NewStore() (session 2) error = %v", err)
	}
	defer store2.Close()

	if store2.Count() != 1 {
		t.Fatalf("Count() after reopen = %d, want 1", store2.Count())
	}
	results, err := store2.RetrieveContext(ctx, "favori rengi nedir", 3, 0)
	if err != nil {
		t.Fatalf("RetrieveContext() (session 2) error = %v", err)
	}
	if !containsMemory(results, "kirmizidir") {
		t.Fatalf("fact saved in session 1 not recalled in session 2, got %+v", results)
	}
}

// --- Ranking / importance --------------------------------------------------

// TestRecall_ExplicitFactOutranksCasualMention checks the importance boost
// in RetrieveContext (importance=5 for explicit saves vs. importance=3 for
// casual conversation) actually changes ranking when both are relevant.
func TestRecall_ExplicitFactOutranksCasualMention(t *testing.T) {
	ctx := context.Background()
	store := newRecallStore(t, bagOfWordsEmbedding(16), 16)

	if err := store.SaveInteraction(ctx, "kahve hakkinda gecici bir sohbet ettik", "guzel bir sohbetti"); err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}
	if err := store.SaveExplicit(ctx, "kahve", "profile"); err != nil {
		t.Fatalf("SaveExplicit() error = %v", err)
	}

	results, err := store.RetrieveContext(ctx, "kahve", 2, 0)
	if err != nil {
		t.Fatalf("RetrieveContext() error = %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d: %+v", len(results), results)
	}
	if results[0].Source != "explicit" {
		t.Errorf("top result Source = %q, want explicit (importance boost should rank it first): %+v", results[0].Source, results)
	}
}

// TestRecall_CasualFactNotCrowdedOutByRoutineNoise reproduces the exact
// failure pattern from a real user session: a personal fact ("my dog's name
// is Zeytin") stated in normal chat — saved via the ordinary per-turn
// SaveInteraction path with the default importance=3, completely
// indistinguishable in priority from routine greetings, since Memo has no
// mechanism to auto-detect "this is a durable fact worth extra weight" (see
// internal/app/memory.go's saveMemoryAsync, which saves every single turn
// unconditionally, greetings included) — is asked about later inside a
// COMPOUND question alongside an unrelated topic. Before splitCompoundQuery,
// this was confirmed to fail: the single blended embedding for the whole
// compound question ranked dozens of near-duplicate routine "kanka naber"
// turns above the one specific fact, crowding it out of the topK=5 cut
// entirely (verified by temporarily disabling the segment-decomposition
// loop in RetrieveContext and re-running this exact test — it failed, 0/5
// results contained the fact).
// TestRecall_CasualFactNotCrowdedOutByRoutineNoise_VecSearch runs the shared
// scenario (see runCasualFactNotCrowdedOutByRoutineNoise) against whichever
// vector search path this environment naturally selects — vecSearch when
// the sqlite-vec extension is available (true on this dev machine), goSearch
// otherwise. Kept alongside the _GoFallback variant below specifically
// *because* the two paths can rank near-identical candidates differently
// (see vecSearch's own comment on this) — a name reflecting "whatever the
// environment gives us" would hide that this test's real path depends on
// local setup.
func TestRecall_CasualFactNotCrowdedOutByRoutineNoise_VecSearch(t *testing.T) {
	runCasualFactNotCrowdedOutByRoutineNoise(t, newRecallStore(t, bagOfWordsEmbedding(256), 256))
}

// TestRecall_CasualFactNotCrowdedOutByRoutineNoise_GoFallback runs the same
// scenario forced onto goSearch — the path CI actually exercises (GitHub
// Actions runners never have the sqlite-vec extension compiled in). This is
// the regression test for a real gap: the original version of this test
// passed reliably on every local run (sqlite-vec available here) while
// intermittently/consistently failing in CI, because the two search
// implementations aren't guaranteed to rank near-identical candidates the
// same way. Without a test that explicitly forces the Go fallback, a
// goSearch-specific regression can pass locally and only ever surface in CI.
func TestRecall_CasualFactNotCrowdedOutByRoutineNoise_GoFallback(t *testing.T) {
	runCasualFactNotCrowdedOutByRoutineNoise(t, newRecallStoreGoFallback(t, bagOfWordsEmbedding(256), 256))
}

// runCasualFactNotCrowdedOutByRoutineNoise reproduces the exact failure
// pattern from a real user session: a personal fact ("my dog's name is
// Zeytin") stated in normal chat — saved via the ordinary per-turn
// SaveInteraction path with the default importance=3, completely
// indistinguishable in priority from routine greetings, since Memo has no
// mechanism to auto-detect "this is a durable fact worth extra weight" (see
// internal/app/memory.go's saveMemoryAsync, which saves every single turn
// unconditionally, greetings included) — is asked about later inside a
// COMPOUND question alongside an unrelated topic. Before splitCompoundQuery,
// this was confirmed to fail: the single blended embedding for the whole
// compound question ranked dozens of near-duplicate routine "kanka naber"
// turns above the one specific fact, crowding it out of the topK=5 cut
// entirely (verified by temporarily disabling the segment-decomposition
// loop in RetrieveContext and re-running this exact test — it failed, 0/5
// results contained the fact).
//
// Dimension is deliberately 256, not 64: bagOfWordsEmbedding hashes words
// into buckets by fnv32a(word) % dim, and at only 64 buckets this specific
// test's word set has a real collision — "bugun"/"hava"/"nasil"/"sohbet"/a
// noise index number happen to land on buckets that overlap enough with
// "kopeğimin"/"adı" to spuriously outscore the actual fact against the
// "kopeğimin adı neydi" query segment (verified directly: at dim=64 two
// noise entries scored 0.480 similarity against that segment vs the real
// fact's 0.471; at dim=256 the fact scores 0.471 and the best-matching
// noise entry drops to 0.160). This is an artifact of the low-dimensional
// hash-based test double, not the production ranking algorithm — a real
// embedding model operates in a much higher-dimensional, semantically
// structured space where this kind of collision doesn't happen.
func runCasualFactNotCrowdedOutByRoutineNoise(t *testing.T, store *Store) {
	t.Helper()
	ctx := context.Background()

	for i := range 30 {
		if err := store.SaveInteraction(ctx,
			fmt.Sprintf("kanka naber bugun hava nasil sohbet %d", i),
			"iyilik kanka",
		); err != nil {
			t.Fatalf("SaveInteraction(noise %d) error = %v", i, err)
		}
	}

	// Mirrors the real bug exactly: a normal chat turn, not SaveExplicit.
	if err := store.SaveInteraction(ctx, "kopeğimin adı zeytin", "ne güzel isim"); err != nil {
		t.Fatalf("SaveInteraction(fact) error = %v", err)
	}

	// A small topK, matching the real-world default, and a genuinely
	// compound question — the decomposition only activates for these.
	results, err := store.RetrieveContext(ctx, "kanka naber ve kopeğimin adı neydi", 5, 0)
	if err != nil {
		t.Fatalf("RetrieveContext() error = %v", err)
	}
	if !containsMemory(results, "zeytin") {
		t.Fatalf("casual-chat fact crowded out by routine conversational noise; got %+v", results)
	}
}

func TestSplitCompoundQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"single topic, no split", "favori rengim ne", nil},
		{
			"two topics joined by ve",
			"adımı ve doğum günümü biliyor musun",
			[]string{"adımı", "doğum günümü biliyor musun"},
		},
		{
			"three topics joined by ve",
			"adımı ve doğum günümü ve en sevdiğim rengimi biliyor musun",
			[]string{"adımı", "doğum günümü", "en sevdiğim rengimi biliyor musun"},
		},
		{
			"comma-separated topics",
			"adım, doğum günüm, favori rengim ne",
			[]string{"adım", "doğum günüm", "favori rengim ne"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitCompoundQuery(tc.query)
			if len(got) != len(tc.want) {
				t.Fatalf("splitCompoundQuery(%q) = %v, want %v", tc.query, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("segment[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestRecall_TopKLimitRespected checks RetrieveContext never returns more
// than the requested topK, even with many equally-relevant candidates.
func TestRecall_TopKLimitRespected(t *testing.T) {
	ctx := context.Background()
	store := newRecallStore(t, bagOfWordsEmbedding(24), 24)

	for i := range 10 {
		if err := store.SaveExplicit(ctx, fmt.Sprintf("kahve turu numara %d", i), "profile"); err != nil {
			t.Fatalf("SaveExplicit(%d) error = %v", i, err)
		}
	}

	results, err := store.RetrieveContext(ctx, "kahve turu", 3, 0)
	if err != nil {
		t.Fatalf("RetrieveContext() error = %v", err)
	}
	if len(results) > 3 {
		t.Fatalf("len(results) = %d, want <= 3", len(results))
	}
}

// TestRecall_MinSimilarityExcludesUnrelated checks the minSimilarity floor
// actually filters out memories that share no meaningful content with the
// query, rather than always padding results up to topK regardless of
// relevance.
func TestRecall_MinSimilarityExcludesUnrelated(t *testing.T) {
	ctx := context.Background()
	store := newRecallStore(t, bagOfWordsEmbedding(32), 32)

	if err := store.SaveExplicit(ctx, "dag yuruyusu botlari cok rahatti", "profile"); err != nil {
		t.Fatalf("SaveExplicit() error = %v", err)
	}

	results, err := store.RetrieveContext(ctx, "kahve fasulyesi kavurma", 5, 0.9)
	if err != nil {
		t.Fatalf("RetrieveContext() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results above a 0.9 similarity floor for an unrelated query, got %+v", results)
	}
}

// TestRecall_EmptyStoreReturnsEmpty checks a fresh store with nothing saved
// returns cleanly with no error and no results, rather than panicking on
// empty tables.
func TestRecall_EmptyStoreReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	store := newRecallStore(t, bagOfWordsEmbedding(16), 16)

	results, err := store.RetrieveContext(ctx, "anything at all", 5, 0)
	if err != nil {
		t.Fatalf("RetrieveContext() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results from an empty store, got %+v", results)
	}
}

// --- Chunking integration ----------------------------------------------------

// TestRecall_LongFactChunked_DetailBuriedAtEndStillRetrievable checks that a
// long piece of text gets split into multiple embedded chunks (chunker.go),
// and that a specific detail buried near the end of the text — which would
// be diluted into irrelevance if the whole text were embedded as one vector
// — is still retrievable because its own chunk carries its own embedding.
func TestRecall_LongFactChunked_DetailBuriedAtEndStillRetrievable(t *testing.T) {
	ctx := context.Background()
	store := newRecallStore(t, bagOfWordsEmbedding(64), 64)

	filler := make([]string, 320)
	for i := range filler {
		filler[i] = fmt.Sprintf("dolgu%d", i)
	}
	longText := strings.Join(filler, " ") + " ozel detay kod kelimesi Zebra1234"

	if err := store.SaveInteraction(ctx, longText, "tamam"); err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}
	if store.Count() <= 1 {
		t.Fatalf("Count() = %d, want >1 (long text should have been chunked)", store.Count())
	}

	results, err := store.RetrieveContext(ctx, "ozel detay kod kelimesi Zebra1234", 3, 0)
	if err != nil {
		t.Fatalf("RetrieveContext() error = %v", err)
	}
	if !containsMemory(results, "Zebra1234") {
		t.Fatalf("buried detail not retrievable after chunking, got %+v", results)
	}
}

// --- Pure-function unit coverage --------------------------------------------

func TestExpandQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"short query, no expansion", "favori rengim ne", ""},
		{"exactly 7 words, no expansion", "bir iki uc dort bes alti yedi", ""},
		{
			"long query expands to first 5 words",
			"adimi ve dogum gunumu ve en sevdigim rengi biliyor musun",
			"adimi ve dogum gunumu ve",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := expandQuery(tc.query)
			if got != tc.want {
				t.Errorf("expandQuery(%q) = %q, want %q", tc.query, got, tc.want)
			}
		})
	}
}

func TestEscapeFTSQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"single word", "kirmizi", `"kirmizi"`},
		{"multiple words joined with OR", "favori renk kirmizi", `"favori" OR "renk" OR "kirmizi"`},
		{"embedded quote escaped", `renk"test`, `"renk""test"`},
		{"empty query returned as-is", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := escapeFTSQuery(tc.query)
			if got != tc.want {
				t.Errorf("escapeFTSQuery(%q) = %q, want %q", tc.query, got, tc.want)
			}
		})
	}
}

func TestReciprocalRankFusion_HybridMatchType(t *testing.T) {
	vecResults := []MemoryResult{
		{ID: "a", Content: "shared"},
		{ID: "b", Content: "vector-only"},
	}
	ftsResults := []MemoryResult{
		{ID: "a", Content: "shared"},
		{ID: "c", Content: "fts-only"},
	}

	fused := reciprocalRankFusion(vecResults, ftsResults, 10)
	if len(fused) != 3 {
		t.Fatalf("len(fused) = %d, want 3", len(fused))
	}

	byID := make(map[string]MemoryResult, len(fused))
	for _, r := range fused {
		byID[r.ID] = r
	}

	if byID["a"].MatchType != "hybrid" {
		t.Errorf(`id "a" (in both) MatchType = %q, want "hybrid"`, byID["a"].MatchType)
	}
	if byID["b"].MatchType != "vector" {
		t.Errorf(`id "b" (vector-only) MatchType = %q, want "vector"`, byID["b"].MatchType)
	}
	if byID["c"].MatchType != "fts" {
		t.Errorf(`id "c" (fts-only) MatchType = %q, want "fts"`, byID["c"].MatchType)
	}

	// "a" appears in both lists at rank 0, so it must score higher than
	// items appearing in only one list and rank first.
	if fused[0].ID != "a" {
		t.Errorf("fused[0].ID = %q, want %q (highest combined RRF score)", fused[0].ID, "a")
	}
}

// TestReciprocalRankFusion_DeterministicOnTiedScores is the regression test
// for an intermittent CI failure (TestRecall_CasualFactNotCrowdedOutByRoutineNoise,
// observed on a real CI run — go build 471c0eb — but not reproducible with
// straight repetition locally): reciprocalRankFusion built its ranked list
// by iterating a map[string]float64, and Go deliberately randomizes map
// iteration order per process. When two candidates land at the exact same
// combined RRF score (routine, not rare — see the two vec/fts result sets
// below, each id lands at a different rank in its own list but an
// identical *combined* rank position across the two), the old plain
// `score >` comparator left their relative order dependent on that random
// iteration order — meaning which one survives a topK cutoff could flip
// from one process run to the next for identical input. This calls the
// function 200 times with input guaranteed to produce several exactly-tied
// pairs and asserts every single call returns the identical order — this
// reliably catches the bug because each call operates on a freshly
// constructed map with its own random iteration seed.
func TestReciprocalRankFusion_DeterministicOnTiedScores(t *testing.T) {
	// a/d, b/e, and c/f are each ranked identically (position 0/1/2) in
	// their own list, so every pair in each column ties exactly on
	// 1/(60+rank+1) — three separate exactly-tied pairs at three different
	// score levels, not just one edge case.
	vecResults := []MemoryResult{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	ftsResults := []MemoryResult{{ID: "d"}, {ID: "e"}, {ID: "f"}}

	first := reciprocalRankFusion(vecResults, ftsResults, 6)
	firstIDs := make([]string, len(first))
	for i, r := range first {
		firstIDs[i] = r.ID
	}

	for i := range 200 {
		got := reciprocalRankFusion(vecResults, ftsResults, 6)
		gotIDs := make([]string, len(got))
		for j, r := range got {
			gotIDs[j] = r.ID
		}
		if strings.Join(gotIDs, ",") != strings.Join(firstIDs, ",") {
			t.Fatalf("run %d order = %v, want the same order as run 0 = %v (non-deterministic tie-break)", i, gotIDs, firstIDs)
		}
	}
}

func TestHybridSearch_MatchTypeIsHybridWhenFTSCompiled(t *testing.T) {
	ctx := context.Background()
	store := newRecallStore(t, bagOfWordsEmbedding(16), 16)
	if !store.useFTS {
		t.Skip(`requires -tags "sqlite_fts5"`)
	}

	if err := store.SaveInteraction(ctx, "coffee beans arabica", "great choice"); err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}

	results, err := store.RetrieveContext(ctx, "coffee beans arabica", 5, 0)
	if err != nil {
		t.Fatalf("RetrieveContext() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results, got none")
	}
	// An exact-text query should be found by both the vector pass (identical
	// embedding) and the FTS pass (identical words) — so it must fuse to "hybrid".
	if results[0].MatchType != "hybrid" {
		t.Errorf("MatchType = %q, want hybrid (exact-text query should match both vector and FTS passes)", results[0].MatchType)
	}
}
