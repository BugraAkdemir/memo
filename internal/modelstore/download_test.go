package modelstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// redirectToTestServerTransport rewrites every request's scheme/host to
// point at a local httptest.Server, leaving path/query untouched — the only
// seam needed to exercise doDownload's real HTTP call sites (the HF tree
// API and the file download itself, both hardcoded to huggingface.co)
// against a fake server instead of the real network.
type redirectToTestServerTransport struct {
	base *url.URL
}

func (t *redirectToTestServerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	redirected := req.Clone(req.Context())
	newURL := *req.URL
	newURL.Scheme = t.base.Scheme
	newURL.Host = t.base.Host
	redirected.URL = &newURL
	redirected.Host = t.base.Host
	return http.DefaultTransport.RoundTrip(redirected)
}

func newTestStore(t *testing.T, mux *http.ServeMux) (*Store, string) {
	t.Helper()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	tsURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	store := New(dir)
	transport := &redirectToTestServerTransport{base: tsURL}
	store.transport = transport
	store.client.Transport = transport
	return store, dir
}

func hfTreeHandler(repoID, filename string, size int, lfsOID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item := map[string]any{"path": filename, "size": size}
		if lfsOID != "" {
			item["lfs"] = map[string]string{"oid": lfsOID}
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{item})
	}
}

// TestDoDownload_RejectsContentThatFailsChecksumVerification proves a
// same-size-but-corrupted transfer is caught: the old code only checked
// downloaded byte count against Content-Length/expectedSize, so a transfer
// that came back the right length but with flipped/wrong bytes (a buggy
// proxy, a bit flip) was silently promoted to the final .gguf path and
// treated as a valid model.
func TestDoDownload_RejectsContentThatFailsChecksumVerification(t *testing.T) {
	const repoID = "acme/model"
	const filename = "model.gguf"
	correct := []byte("this is the correct model content, long enough!")
	corrupted := []byte("this is the WRONG!! model content, long enough!")
	if len(correct) != len(corrupted) {
		t.Fatalf("test setup: fixtures must be equal length to isolate the checksum check from the byte-count check (got %d vs %d)", len(correct), len(corrupted))
	}
	sum := sha256.Sum256(correct)
	correctHex := hex.EncodeToString(sum[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/api/models/"+repoID+"/tree/main", hfTreeHandler(repoID, filename, len(correct), correctHex))
	mux.HandleFunc("/"+repoID+"/resolve/main/"+filename, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(corrupted)
	})

	store, dir := newTestStore(t, mux)

	err := store.doDownload(context.Background(), &downloadEntry{progress: &DownloadProgress{}}, repoID, filename, int64(len(correct)))
	if err == nil {
		t.Fatal("doDownload succeeded despite a checksum mismatch, want an error")
	}
	if !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("error doesn't mention sha256 mismatch: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, filename)); !os.IsNotExist(statErr) {
		t.Fatal("corrupted download was promoted to the final .gguf path")
	}
	if _, statErr := os.Stat(filepath.Join(dir, filename+".downloading")); !os.IsNotExist(statErr) {
		t.Fatal("temp file was left behind after a checksum failure")
	}
}

// TestDoDownload_AcceptsContentMatchingChecksum is the happy-path
// counterpart: a download whose bytes match HF's reported LFS hash must
// still succeed and land at the final path.
func TestDoDownload_AcceptsContentMatchingChecksum(t *testing.T) {
	const repoID = "acme/model"
	const filename = "model.gguf"
	content := []byte("this is the correct model content, long enough!")
	sum := sha256.Sum256(content)
	correctHex := hex.EncodeToString(sum[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/api/models/"+repoID+"/tree/main", hfTreeHandler(repoID, filename, len(content), correctHex))
	mux.HandleFunc("/"+repoID+"/resolve/main/"+filename, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	})
	// fetchModelMeta hits /api/models/{repo} (no /tree/main suffix) after a
	// successful download — serve it too so doDownload's post-verify step
	// doesn't spuriously log an unrelated error.
	mux.HandleFunc("/api/models/"+repoID, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"tags": []string{}})
	})

	store, dir := newTestStore(t, mux)

	if err := store.doDownload(context.Background(), &downloadEntry{progress: &DownloadProgress{}}, repoID, filename, int64(len(content))); err != nil {
		t.Fatalf("doDownload failed for a valid, checksum-matching transfer: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		t.Fatalf("final file missing: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("final file content mismatch: got %q", got)
	}
}

// TestDoDownload_MissingLFSInfoSkipsVerification proves a repo file HF
// doesn't report as LFS-tracked (no "lfs" object in the tree response)
// downloads normally rather than being treated as unverifiable-therefore-
// rejected — checksum verification is a best-effort addition, not a new
// hard requirement every download must satisfy.
func TestDoDownload_MissingLFSInfoSkipsVerification(t *testing.T) {
	const repoID = "acme/model"
	const filename = "model.gguf"
	content := []byte("small non-lfs file content")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/models/"+repoID+"/tree/main", hfTreeHandler(repoID, filename, len(content), ""))
	mux.HandleFunc("/"+repoID+"/resolve/main/"+filename, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	})
	mux.HandleFunc("/api/models/"+repoID, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"tags": []string{}})
	})

	store, dir := newTestStore(t, mux)

	if err := store.doDownload(context.Background(), &downloadEntry{progress: &DownloadProgress{}}, repoID, filename, int64(len(content))); err != nil {
		t.Fatalf("doDownload failed for a file with no LFS checksum to verify against: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, filename)); err != nil {
		t.Fatalf("final file missing: %v", err)
	}
}
