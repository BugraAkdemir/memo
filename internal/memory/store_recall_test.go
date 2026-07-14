package memory

import (
	"context"
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

func containsMemory(results []MemoryResult, substr string) bool {
	for _, r := range results {
		if strings.Contains(r.Content, substr) {
			return true
		}
	}
	return false
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
		"isim":     "kullanicinin adi Ahmet",
		"dogum":    "kullanicinin dogum gunu 5 Mayis 1995",
		"renk":     "kullanicinin favori rengi kirmizi",
		"meslek":   "kullanici yazilim muhendisi olarak calisiyor",
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
