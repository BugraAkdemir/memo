package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/BugraAkdemir/gosearch"
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

func TestFetchPage_UsedBrowser_AddsNote(t *testing.T) {
	orig := websearchFetch
	defer func() { websearchFetch = orig }()
	websearchFetch = func(ctx context.Context, url string) (*websearch.Page, error) {
		return &websearch.Page{URL: url, Title: "Live Scores", Content: "table data", UsedBrowser: true}, nil
	}

	args, _ := json.Marshal(FetchPageArgs{URL: "https://example.com/js-app"})
	out, err := FetchPage(context.Background(), args, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "chromium") && !strings.Contains(out, "tarayıcı") {
		t.Errorf("expected a browser-render note when UsedBrowser=true, got: %q", out)
	}
}

func TestFetchPage_StaticFetch_NoNote(t *testing.T) {
	orig := websearchFetch
	defer func() { websearchFetch = orig }()
	websearchFetch = func(ctx context.Context, url string) (*websearch.Page, error) {
		return &websearch.Page{URL: url, Title: "Docs", Content: "static content", UsedBrowser: false}, nil
	}

	args, _ := json.Marshal(FetchPageArgs{URL: "https://example.com"})
	out, err := FetchPage(context.Background(), args, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(strings.ToLower(out), "chromium") || strings.Contains(out, "tarayıcı motoruyla") {
		t.Errorf("did not expect a browser-render note when UsedBrowser=false, got: %q", out)
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

// fakeBrowserChecker satisfies both websearch.BrowserFetcher and this
// package's browserInstallChecker, so tests can control IsInstalled's
// answer independently of Fetch (which is never actually called here —
// FetchPage only consults websearch.Browser's install status, it never
// calls Fetch on it directly; that happens inside websearch.Fetch itself,
// mocked out via websearchFetch in these tests).
type fakeBrowserChecker struct{ installed bool }

func (f fakeBrowserChecker) Fetch(ctx context.Context, url string) (*gosearch.Page, error) {
	return nil, errors.New("not used in these tests")
}
func (f fakeBrowserChecker) IsInstalled(ctx context.Context) bool { return f.installed }

func TestFetchPage_EmptyContent_BrowserNotInstalled_MentionsInstall(t *testing.T) {
	origFetch, origBrowser := websearchFetch, websearch.Browser
	defer func() { websearchFetch, websearch.Browser = origFetch, origBrowser }()

	websearchFetch = func(ctx context.Context, url string) (*websearch.Page, error) {
		return &websearch.Page{URL: url, Content: ""}, nil
	}
	websearch.Browser = fakeBrowserChecker{installed: false}

	args, _ := json.Marshal(FetchPageArgs{URL: "https://example.com"})
	out, err := FetchPage(context.Background(), args, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "install") && !strings.Contains(out, "kur") {
		t.Errorf("expected the not-installed hint to mention installing, got: %q", out)
	}
}

func TestFetchPage_EmptyContent_BrowserInstalled_GenericMessage(t *testing.T) {
	origFetch, origBrowser := websearchFetch, websearch.Browser
	defer func() { websearchFetch, websearch.Browser = origFetch, origBrowser }()

	websearchFetch = func(ctx context.Context, url string) (*websearch.Page, error) {
		return &websearch.Page{URL: url, Content: ""}, nil
	}
	websearch.Browser = fakeBrowserChecker{installed: true}

	args, _ := json.Marshal(FetchPageArgs{URL: "https://example.com"})
	out, err := FetchPage(context.Background(), args, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(strings.ToLower(out), "install") || strings.Contains(out, "kur") {
		t.Errorf("browser IS installed — should not suggest installing it, got: %q", out)
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
