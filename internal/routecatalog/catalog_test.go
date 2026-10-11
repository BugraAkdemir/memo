package routecatalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCatalogOfflineFallbackAndExactEndpoint(t *testing.T) {
	var fail bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "unavailable", 503)
			return
		}
		w.Write([]byte(`{"one":{"api":"https://one/v1","models":{"shared":{"id":"shared","tool_call":true,"limit":{"context":100000},"modalities":{"input":["text"],"output":["text"]}}}},"two":{"api":"https://two/v1","models":{"shared":{"id":"shared","tool_call":true,"limit":{"context":200000},"modalities":{"input":["text"],"output":["text"]}}}}}`))
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "cache.json")
	c := New(path, srv.URL, srv.Client())
	c.Ensure(context.Background())
	if m, ok := c.Lookup("https://one/v1", "shared"); !ok || m.Limit.Context != 100000 {
		t.Fatalf("endpoint metadata lost: %+v %v", m, ok)
	}
	if _, ok := c.Lookup("", "shared"); ok {
		t.Fatal("ambiguous capacity treated as a fact")
	}
	if _, ok := c.Lookup("https://one/v1", "shared-alias"); ok {
		t.Fatal("invented alias match")
	}
	fail = true
	past := time.Now().Add(-24 * time.Hour)
	if e := os.Chtimes(path, past, past); e != nil {
		t.Fatal(e)
	}
	offline := New(path, srv.URL, srv.Client())
	offline.Ensure(context.Background())
	if m, ok := offline.Lookup("https://two/v1", "shared"); !ok || m.Limit.Context != 200000 {
		t.Fatalf("last good snapshot lost on outage: %+v %v", m, ok)
	}
	empty := New(filepath.Join(t.TempDir(), "missing.json"), srv.URL, srv.Client())
	empty.Ensure(context.Background())
	if _, ok := empty.Lookup("", "shared"); ok {
		t.Fatal("missing cache must remain unknown")
	}
}
