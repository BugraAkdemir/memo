package webserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestHandleOutboxDownload_ServesStagedFile is the end-to-end check for the
// share_file agent tool's frontend delivery path: App.DeliverFile (see
// internal/app/sendfile.go) stages a file under a token and returns a
// relative "/api/files/outbox/<token>" link; this is the handler that link
// actually resolves to. Confirms the real file bytes come back with a
// Content-Disposition naming the display filename (not the temp file's own
// on-disk name, which for a zipped folder is a random tmp path).
func TestHandleOutboxDownload_ServesStagedFile(t *testing.T) {
	dir := t.TempDir()
	realPath := filepath.Join(dir, "tmp-random-name.zip")
	if err := os.WriteFile(realPath, []byte("zip bytes here"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	bridge := &swarmStubBridge{
		getOutboxFile: func(token string) (string, string, bool) {
			if token == "good-token" {
				return realPath, "notes.zip", true
			}
			return "", "", false
		},
	}
	s := &Server{fullBridge: bridge}

	req := httptest.NewRequest(http.MethodGet, "/api/files/outbox/good-token", nil)
	req.SetPathValue("token", "good-token")
	w := httptest.NewRecorder()

	s.handleOutboxDownload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %q)", w.Code, w.Body.String())
	}
	if w.Body.String() != "zip bytes here" {
		t.Errorf("body = %q, want the staged file's actual contents", w.Body.String())
	}
	disposition := w.Header().Get("Content-Disposition")
	if disposition != `attachment; filename="notes.zip"` {
		t.Errorf("Content-Disposition = %q, want it to name the display filename, not the temp file's own name", disposition)
	}
}

// TestHandleOutboxDownload_UnknownTokenIs404 confirms a token that
// GetOutboxFile doesn't recognize (never issued, or already expired) 404s
// rather than leaking any information about what tokens might be valid.
func TestHandleOutboxDownload_UnknownTokenIs404(t *testing.T) {
	bridge := &swarmStubBridge{
		getOutboxFile: func(token string) (string, string, bool) { return "", "", false },
	}
	s := &Server{fullBridge: bridge}

	req := httptest.NewRequest(http.MethodGet, "/api/files/outbox/nope", nil)
	req.SetPathValue("token", "nope")
	w := httptest.NewRecorder()

	s.handleOutboxDownload(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// TestHandleOutboxDownload_PostRejected confirms only GET is accepted —
// this is a read-only download endpoint, no reason to accept a body.
func TestHandleOutboxDownload_PostRejected(t *testing.T) {
	s := &Server{fullBridge: &swarmStubBridge{}}

	req := httptest.NewRequest(http.MethodPost, "/api/files/outbox/x", nil)
	w := httptest.NewRecorder()

	s.handleOutboxDownload(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}
