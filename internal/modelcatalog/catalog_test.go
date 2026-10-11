package modelcatalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestOnlineCapabilitiesAndOfflineRestart(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"claude":[{"id":"future-model","owned_by":"anthropic","context_length":1000000,"thinking":{"levels":["low","high","max"]}}]}`))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "catalog.json")
	c := New(server.URL, path, server.Client())
	if err := c.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	cap, ok := c.Cached("future-model", "anthropic")
	if !ok || cap.Context != 1000000 || len(cap.Thinking.Levels) != 3 {
		t.Fatalf("capabilities: %+v / %v", cap, ok)
	}
	cap.Thinking.Levels[0] = "mutated"
	if cap, _ = c.Cached("future-model", "anthropic"); cap.Thinking.Levels[0] != "low" {
		t.Fatal("caller mutated cached data")
	}
	if _, ok := c.Cached("unlisted-model", ""); ok {
		t.Fatal("guessed unknown model")
	}
	if err := c.Ensure(context.Background()); err != nil || calls.Load() != 1 {
		t.Fatalf("cache hit: %v / %d", err, calls.Load())
	}
	server.Close()
	restarted := New(server.URL, path, server.Client())
	cap, ok = restarted.Cached("future-model", "")
	if !ok || cap.Context != 1000000 {
		t.Fatal("offline restart lost last good capability")
	}
	restarted.data.At = time.Time{}
	if err := restarted.refresh(context.Background()); err == nil {
		t.Fatal("expected offline error")
	}
	if cap, _ = restarted.Cached("future-model", ""); cap.Context != 1000000 {
		t.Fatal("failure replaced good snapshot")
	}
}
func TestAmbiguousTiersAndMalformedRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"wrong_shape":{}}`)) }))
	defer server.Close()
	c := New(server.URL, "", server.Client())
	c.data.Groups = map[string][]Capability{
		"codex-plus": {{ID: "tier-model", Owner: "openai", Context: 900000}},
		"codex-free": {{ID: "tier-model", Owner: "openai", Context: 200000}},
	}
	cap, _ := c.Cached("tier-model", "openai")
	if cap.Context != 200000 {
		t.Fatalf("ambiguous tiers=%d", cap.Context)
	}
	if err := c.refresh(context.Background()); err == nil {
		t.Fatal("malformed catalog accepted")
	}
	if cap, _ = c.Cached("tier-model", ""); cap.Context != 200000 {
		t.Fatal("malformed refresh destroyed cache")
	}
}
