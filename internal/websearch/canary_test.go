//go:build canary

package websearch

import (
	"context"
	"testing"
	"time"
)

// TestCanary_DuckDuckGoSearch hits the real DuckDuckGo HTML endpoint and
// asserts that Search still returns at least one result with Title+URL set.
// Catches silent scrape breakage when DDG changes its HTML structure.
// Only runs under -tags canary (see .github/workflows/canary.yml).
func TestCanary_DuckDuckGoSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := Search(ctx, "test", 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search returned zero results — DuckDuckGo HTML may have changed")
	}

	for _, r := range results {
		if r.Title != "" && r.URL != "" {
			t.Logf("ok: got %d results, first usable: title=%q url=%q", len(results), r.Title, r.URL)
			return
		}
	}
	t.Fatalf("no result with both Title and URL set: %+v", results)
}
