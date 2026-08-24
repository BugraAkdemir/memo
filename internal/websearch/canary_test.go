//go:build canary

package websearch

import (
	"context"
	"testing"
	"time"
)

// TestCanary_Search gerçek Bing/DuckDuckGo aramasına çıkar ve Search'ün hâlâ
// en az bir sonuç döndürdüğünü doğrular. gosearch'ün HTML parser'ları
// kırılırsa (motorlar markup değiştirirse) bu test patlar.
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
