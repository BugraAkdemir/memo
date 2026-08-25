package websearch

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/BugraAkdemir/gosearch"
)

func TestFetch_MapsPage(t *testing.T) {
	orig := gosearchFetch
	defer func() { gosearchFetch = orig }()

	gosearchFetch = func(ctx context.Context, url string, opts ...gosearch.Option) (*gosearch.Page, error) {
		return &gosearch.Page{URL: url, Title: "T", Content: "# Heading\ncontent"}, nil
	}

	page, err := Fetch(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Title != "T" || page.Content != "# Heading\ncontent" || page.URL != "https://example.com" {
		t.Errorf("unexpected page: %+v", page)
	}
}

func TestFetch_TruncatesLongContent(t *testing.T) {
	orig := gosearchFetch
	defer func() { gosearchFetch = orig }()

	long := strings.Repeat("a", maxFetchContentRunes+500)
	gosearchFetch = func(ctx context.Context, url string, opts ...gosearch.Option) (*gosearch.Page, error) {
		return &gosearch.Page{URL: url, Content: long}, nil
	}

	page, err := Fetch(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len([]rune(page.Content)) > maxFetchContentRunes+10 {
		t.Errorf("content not truncated: %d runes", len([]rune(page.Content)))
	}
}

func TestFetch_WrapsError(t *testing.T) {
	orig := gosearchFetch
	defer func() { gosearchFetch = orig }()

	sentinel := errors.New("blocked")
	gosearchFetch = func(ctx context.Context, url string, opts ...gosearch.Option) (*gosearch.Page, error) {
		return nil, sentinel
	}

	_, err := Fetch(context.Background(), "https://example.com")
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got %v", err)
	}
}

func TestFetch_EmptyStaticContentFallsBackToBrowser(t *testing.T) {
	origFetch, origBrowser := gosearchFetch, browserFallbackFetch
	defer func() { gosearchFetch, browserFallbackFetch = origFetch, origBrowser }()

	gosearchFetch = func(ctx context.Context, url string, opts ...gosearch.Option) (*gosearch.Page, error) {
		return &gosearch.Page{URL: url, Title: "static title"}, nil // empty Content
	}
	var gotURL string
	browserFallbackFetch = func(ctx context.Context, url string) (*gosearch.Page, error) {
		gotURL = url
		return &gosearch.Page{URL: url, Title: "rendered title", Content: "rendered content"}, nil
	}

	page, err := Fetch(context.Background(), "https://example.com/js-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotURL != "https://example.com/js-app" {
		t.Errorf("browser fallback got url %q, want the original url", gotURL)
	}
	if page.Content != "rendered content" || page.Title != "rendered title" {
		t.Errorf("expected the browser-rendered page to win, got %+v", page)
	}
}

func TestFetch_BrowserFallbackUnavailable_KeepsEmptyResult(t *testing.T) {
	origFetch, origBrowser := gosearchFetch, browserFallbackFetch
	defer func() { gosearchFetch, browserFallbackFetch = origFetch, origBrowser }()

	gosearchFetch = func(ctx context.Context, url string, opts ...gosearch.Option) (*gosearch.Page, error) {
		return &gosearch.Page{URL: url, Title: "static title"}, nil // empty Content
	}
	browserFallbackFetch = func(ctx context.Context, url string) (*gosearch.Page, error) {
		return nil, errors.New("no browser installed")
	}

	page, err := Fetch(context.Background(), "https://example.com/js-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Content != "" || page.Title != "static title" {
		t.Errorf("expected the original (empty) static result when the browser fallback fails, got %+v", page)
	}
}

func TestFetch_NonEmptyStaticContentSkipsBrowser(t *testing.T) {
	origFetch, origBrowser := gosearchFetch, browserFallbackFetch
	defer func() { gosearchFetch, browserFallbackFetch = origFetch, origBrowser }()

	gosearchFetch = func(ctx context.Context, url string, opts ...gosearch.Option) (*gosearch.Page, error) {
		return &gosearch.Page{URL: url, Title: "T", Content: "already has content"}, nil
	}
	called := false
	browserFallbackFetch = func(ctx context.Context, url string) (*gosearch.Page, error) {
		called = true
		return &gosearch.Page{}, nil
	}

	if _, err := Fetch(context.Background(), "https://example.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("browser fallback should not run when the static fetch already returned content")
	}
}
