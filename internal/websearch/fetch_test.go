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
