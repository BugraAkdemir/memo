//go:build canary

package websearch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BugraAkdemir/gosearch"
)

// TestCanary_Search gerçek DuckDuckGo aramasına çıkar ve Search'ün hâlâ
// en az bir sonuç döndürdüğünü doğrular. gosearch'ün HTML parser'ı
// kırılırsa (motor markup değiştirirse) bu test patlar. Paylaşımlı CI runner
// IP'lerinden gelen isteklerde DuckDuckGo bot/captcha koruması (ErrChallenge /
// ErrBlocked) dönerse bu bir parser kırılması olmadığından test skip edilir.
func TestCanary_Search(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := Search(ctx, "golang", 5)
	if err != nil {
		if errors.Is(err, gosearch.ErrChallenge) || errors.Is(err, gosearch.ErrBlocked) {
			t.Skipf("skipping canary on shared runner IP challenge/block: %v", err)
		}
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
