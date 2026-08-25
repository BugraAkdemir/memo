//go:build canary

package websearch

import (
	"context"
	"testing"
	"time"
)

// TestCanary_Search gerçek DuckDuckGo aramasına çıkar ve Search'ün hâlâ
// en az bir sonuç döndürdüğünü doğrular. gosearch'ün HTML parser'ı
// kırılırsa (motor markup değiştirirse) bu test patlar.
func TestCanary_Search(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := Search(ctx, "golang", 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search returned zero results — engine HTML may have changed")
	}

	for _, r := range results {
		if r.Title != "" && r.URL != "" {
			t.Logf("ok: got %d results, first usable: title=%q url=%q", len(results), r.Title, r.URL)
			return
		}
	}
	t.Fatalf("no result with both Title and URL set: %+v", results)
}

// TestCanary_SearchSuperLig is a one-off, not a permanent regression test:
// reproduces the exact live-reported search-quality issue this session
// found with Bing (queries like this returned unrelated results — Süper
// Loto, Windows help forums, random-language pages — depending on exact
// wording) to confirm DuckDuckGo actually returns on-topic results for it.
func TestCanary_SearchSuperLig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := Search(ctx, "Süper Lig 2026 2027 2. hafta maç sonuçları skorları", 8)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search returned zero results")
	}
	for i, r := range results {
		t.Logf("[%d] %s — %s", i+1, r.Title, r.URL)
	}
}

// TestCanary_Fetch gerçek bir sayfaya çıkar ve Fetch'in okunabilir içerik
// döndürdüğünü doğrular.
func TestCanary_Fetch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page, err := Fetch(ctx, "https://go.dev/doc/")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if page.Content == "" {
		t.Fatal("Fetch returned empty content — extractor may have changed")
	}
	t.Logf("ok: fetched %q, %d chars", page.Title, len(page.Content))
}
