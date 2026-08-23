package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memo/internal/config"
	"memo/internal/provider"
)

func TestExtractJSONMemoryImport(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain object", `{"facts":["a"],"style_summary":""}`, `{"facts":["a"],"style_summary":""}`},
		{"prose wrapped", "Sure, here you go:\n```json\n{\"facts\":[],\"style_summary\":\"casual\"}\n```", `{"facts":[],"style_summary":"casual"}`},
		{"brace inside string", `{"facts":["likes emoji :)}"],"style_summary":""}`, `{"facts":["likes emoji :)}"],"style_summary":""}`},
		{"no json", "no object here", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractJSON(tt.in); got != tt.want {
				t.Errorf("extractJSON(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestImportMemoryFromTextEmptyInput(t *testing.T) {
	a := &App{}
	_, _, err := a.ImportMemoryFromText(context.Background(), "   ")
	if err == nil {
		t.Fatal("expected error for empty text, got nil")
	}
}

// TestImportMemoryFromTextNoModelFailsFastWithClearMessage is a regression
// test: an earlier version called a.providerRouter.ChatCompletion directly,
// bypassing callLLM's Orchestra/provider/local routing chain entirely — so
// with nothing connected (no local model, no external provider), it hung on
// a live network call until the request's own timeout, and the user saw no
// indication of what was actually wrong. ImportMemoryFromText now routes
// through callLLM like every other LLM call in this codebase, which fails
// immediately via its local.client nil-check and returns a clear, actionable
// message instead of attempting a network call at all.
func TestImportMemoryFromTextNoModelFailsFastWithClearMessage(t *testing.T) {
	// UILanguage pinned to "tr" — see the twin test in memory_test.go: the
	// Turkish assertion below now requires the Turkish UI-language branch.
	a := &App{cfg: &config.AppConfig{Identity: config.IdentityConfig{UILanguage: "tr"}}}
	_, _, err := a.ImportMemoryFromText(context.Background(), "some pasted AI answer about me")
	if err == nil {
		t.Fatal("expected error when no model/provider is connected, got nil")
	}
	if !strings.Contains(err.Error(), "yüklenmemiş") {
		t.Errorf("error = %q, want it to contain callLLM's clear no-model-loaded message", err.Error())
	}
}

// newImportTestRouter builds a real *provider.Router pointed at an httptest
// server whose chat completion content is the JSON body ImportMemoryFromText
// expects ({"facts":[...],"style_summary":"..."}), marshaled here so tests
// don't hand-escape JSON-inside-JSON.
func newImportTestRouter(t *testing.T, facts []string, styleSummary string) *provider.Router {
	t.Helper()
	body, err := json.Marshal(importedMemory{Facts: facts, StyleSummary: styleSummary})
	if err != nil {
		t.Fatalf("marshal importedMemory: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, string(body))
	}))
	t.Cleanup(srv.Close)

	return provider.NewRouter([]provider.ProviderConfig{{
		Type:    provider.ProviderCustom,
		Name:    "test",
		BaseURL: srv.URL,
		Model:   "test-model",
		Enabled: true,
	}})
}

// TestImportMemoryFromText_SkipsAlreadyPinnedDuplicate mirrors
// TestExtractAndPinFacts_SkipsAlreadyPinnedDuplicate (memory_test.go) for the
// settings "import memory from another AI" feature — it saves pinned facts
// through the exact same SaveExplicitMemory path but, before this fix,
// never checked against already-pinned facts first. A user pasting the same
// "what do you know about me" export twice (e.g. after re-asking the other
// AI) would otherwise get a fresh duplicate pinned entry every time.
func TestImportMemoryFromText_SkipsAlreadyPinnedDuplicate(t *testing.T) {
	store := newExtractionTestStore(t)
	if err := store.SaveExplicit(context.Background(), "User's name is Ece.", "imported"); err != nil {
		t.Fatalf("SaveExplicit() error = %v", err)
	}

	router := newImportTestRouter(t, []string{"User's name is Ece.", "User's favorite color is orange"}, "")
	a := &App{
		store:              store,
		providerRouter:     router,
		activeProviderName: "test",
		cfg:                &config.AppConfig{},
	}

	factsSaved, _, err := a.ImportMemoryFromText(context.Background(), "pasted profile text")
	if err != nil {
		t.Fatalf("ImportMemoryFromText() error = %v", err)
	}
	if factsSaved != 1 {
		t.Errorf("factsSaved = %d, want 1 (duplicate must not count as newly saved)", factsSaved)
	}

	pinned, err := store.GetPinnedFacts(context.Background())
	if err != nil {
		t.Fatalf("GetPinnedFacts() error = %v", err)
	}
	if len(pinned) != 2 {
		t.Fatalf("len(pinned) = %d, want 2 (1 pre-existing + 1 new, duplicate skipped): %+v", len(pinned), pinned)
	}
	nameCount := 0
	for _, p := range pinned {
		if p.Content == "User's name is Ece." {
			nameCount++
		}
	}
	if nameCount != 1 {
		t.Errorf("\"User's name is Ece.\" pinned %d times, want exactly 1 (duplicate must be skipped)", nameCount)
	}
}

// TestImportMemoryFromText_CapsFactCount is a regression test for the
// missing bound on parsed.Facts before this fix: a single import previously
// saved every fact the model's JSON returned, unbounded, straight into the
// pinned-facts list — which has no relevance decay to age out unchecked
// growth the way ordinary RAG memories do. maxImportedFactsPerCall must stop
// a hallucinating/unbounded model response from flooding it.
func TestImportMemoryFromText_CapsFactCount(t *testing.T) {
	store := newExtractionTestStore(t)

	facts := make([]string, maxImportedFactsPerCall+5)
	for i := range facts {
		facts[i] = fmt.Sprintf("User's fact number %d is unique", i)
	}
	router := newImportTestRouter(t, facts, "")
	a := &App{
		store:              store,
		providerRouter:     router,
		activeProviderName: "test",
		cfg:                &config.AppConfig{},
	}

	factsSaved, _, err := a.ImportMemoryFromText(context.Background(), "pasted profile text")
	if err != nil {
		t.Fatalf("ImportMemoryFromText() error = %v", err)
	}
	if factsSaved != maxImportedFactsPerCall {
		t.Errorf("factsSaved = %d, want %d (capped)", factsSaved, maxImportedFactsPerCall)
	}

	pinned, err := store.GetPinnedFacts(context.Background())
	if err != nil {
		t.Fatalf("GetPinnedFacts() error = %v", err)
	}
	if len(pinned) != maxImportedFactsPerCall {
		t.Fatalf("len(pinned) = %d, want %d", len(pinned), maxImportedFactsPerCall)
	}
}
