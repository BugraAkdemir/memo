package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"memo/internal/websearch"
)

func TestFetchPage_ReturnsFormattedContent(t *testing.T) {
	orig := websearchFetch
	defer func() { websearchFetch = orig }()
	websearchFetch = func(ctx context.Context, url string) (*websearch.Page, error) {
		return &websearch.Page{URL: url, Title: "Go Docs", Content: "# Intro\nSome content."}, nil
	}

	args, _ := json.Marshal(FetchPageArgs{URL: "https://go.dev/doc/"})
	out, err := FetchPage(context.Background(), args, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty result")
	}
}

func TestFetchPage_RequiresURL(t *testing.T) {
	args, _ := json.Marshal(FetchPageArgs{URL: ""})
	_, err := FetchPage(context.Background(), args, "", nil)
	if err == nil {
		t.Fatal("expected error for empty url")
	}
}

func TestFetchPage_WrapsFetchError(t *testing.T) {
	orig := websearchFetch
	defer func() { websearchFetch = orig }()
	sentinel := errors.New("blocked")
	websearchFetch = func(ctx context.Context, url string) (*websearch.Page, error) {
		return nil, sentinel
	}

	args, _ := json.Marshal(FetchPageArgs{URL: "https://example.com"})
	_, err := FetchPage(context.Background(), args, "", nil)
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got %v", err)
	}
}

func TestFetchPage_EmptyContentReturnsHint(t *testing.T) {
	orig := websearchFetch
	defer func() { websearchFetch = orig }()
	websearchFetch = func(ctx context.Context, url string) (*websearch.Page, error) {
		return &websearch.Page{URL: url, Content: ""}, nil
	}

	args, _ := json.Marshal(FetchPageArgs{URL: "https://example.com"})
	out, err := FetchPage(context.Background(), args, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("expected a non-empty hint message for empty page content")
	}
}

func TestFetchPage_RespectsBudget(t *testing.T) {
	orig := websearchFetch
	defer func() { websearchFetch = orig }()
	calls := 0
	websearchFetch = func(ctx context.Context, url string) (*websearch.Page, error) {
		calls++
		return &websearch.Page{URL: url, Content: "content"}, nil
	}

	ctx := WithFetchBudget(context.Background())
	for i := 0; i < maxFetchDomains; i++ {
		url := "https://site" + string(rune('a'+i)) + ".example"
		args, _ := json.Marshal(FetchPageArgs{URL: url})
		if _, err := FetchPage(ctx, args, "", nil); err != nil {
			t.Fatalf("attempt %d: unexpected error: %v", i, err)
		}
	}
	if calls != maxFetchDomains {
		t.Fatalf("expected %d underlying fetch calls, got %d", maxFetchDomains, calls)
	}

	args, _ := json.Marshal(FetchPageArgs{URL: "https://one-too-many.example"})
	out, err := FetchPage(ctx, args, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != maxFetchDomains {
		t.Fatalf("expected the underlying fetch NOT to be called once budget is exhausted, calls=%d", calls)
	}
	if out == "" {
		t.Fatal("expected a non-empty budget-exhausted message")
	}
}
