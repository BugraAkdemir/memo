//go:build upstream

package modelcatalog

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// Explicit network smoke test; normal CI tests use only httptest fixtures.
func TestPublishedCatalogSchema(t *testing.T) {
	c := New(SourceURL, "", &http.Client{Timeout: 10 * time.Second})
	if err := c.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(c.data.Groups) == 0 {
		t.Fatal("published catalog has no groups")
	}
	for _, id := range []string{"gpt-6.1-sol", "claude-opus-4-6"} {
		if cap, ok := c.Cached(id, ""); ok {
			t.Logf("%s: context=%d efforts=%v", id, cap.Context, cap.Thinking.Levels)
		}
	}
}
