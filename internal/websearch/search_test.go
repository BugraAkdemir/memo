package websearch

import (
	"context"
	"errors"
	"testing"

	"github.com/BugraAkdemir/gosearch"
)

func TestSearch_MapsResultsAndUsesDuckDuckGoOnly(t *testing.T) {
	orig := gosearchSearch
	defer func() { gosearchSearch = orig }()

	var gotEngine gosearch.Engine
	var gotOptCount int
	gosearchSearch = func(ctx context.Context, query string, engine gosearch.Engine, opts ...gosearch.Option) ([]gosearch.Result, error) {
		gotEngine = engine
		gotOptCount = len(opts)
		return []gosearch.Result{
			{Title: "t1", URL: "u1", Snippet: "s1", Date: "2026-08-24"},
		}, nil
	}

	results, err := Search(context.Background(), "query", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := Result{Title: "t1", URL: "u1", Snippet: "s1"}
	if len(results) != 1 || results[0] != want {
		t.Errorf("unexpected mapped result: %+v, want [%+v]", results, want)
	}
	if gotEngine != gosearch.DuckDuckGo {
		t.Errorf("expected DuckDuckGo as the (sole) engine, got %v", gotEngine)
	}
	// Only WithMaxResults, deliberately no WithFallback — see Search's doc
	// comment: Bing proved actively misleading rather than erroring cleanly,
	// so no fallback engine is configured at all.
	if gotOptCount != 1 {
		t.Errorf("expected exactly 1 option (WithMaxResults, no fallback), got %d", gotOptCount)
	}
}

func TestSearch_DefaultMaxResults(t *testing.T) {
	orig := gosearchSearch
	defer func() { gosearchSearch = orig }()

	called := false
	gosearchSearch = func(ctx context.Context, query string, engine gosearch.Engine, opts ...gosearch.Option) ([]gosearch.Result, error) {
		called = true
		return nil, nil
	}

	if _, err := Search(context.Background(), "query", 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("gosearchSearch was not called")
	}
}

func TestSearch_WrapsError(t *testing.T) {
	orig := gosearchSearch
	defer func() { gosearchSearch = orig }()

	sentinel := errors.New("boom")
	gosearchSearch = func(ctx context.Context, query string, engine gosearch.Engine, opts ...gosearch.Option) ([]gosearch.Result, error) {
		return nil, sentinel
	}

	_, err := Search(context.Background(), "query", 5)
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got %v", err)
	}
}

func TestFormatForContext(t *testing.T) {
	results := []Result{{Title: "Go Dili", URL: "https://example.com", Snippet: "Güzel bir dil"}}
	out := FormatForContext("golang", results)
	if out == "" {
		t.Error("sonuçlu FormatForContext boş string döndürmemeli")
	}
	if out == FormatForContext("golang", nil) {
		t.Error("sonuçsuz FormatForContext boş string döndürmeli")
	}
}
