package webserver

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

// freePort asks the OS for an unused TCP port, then releases it immediately
// so StartHTTPWithAddr can bind it — the same pattern used elsewhere in this
// codebase for real-listener tests (see internal/llama/process_test.go).
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// waitForListening polls port until something accepts connections, or fails
// the test — Serve() runs in its own goroutine inside StartHTTPWithAddr, so
// the caller can't assume the listener is ready the instant it returns.
func waitForListening(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server on port %d never started listening", port)
}

// TestAPIVersioning_V1AliasesMirrorUnversionedRoutes is the regression test
// for TD-2 (BUG_REPORT.md): every "/api/..." route registered in
// StartHTTPWithAddr must also answer under the mirrored "/api/v1/..."
// path — this is the actual "versioning strategy" that was missing, added
// without moving or renaming a single existing route (zero risk to the
// three existing clients: Flutter desktop, mobile, the Go CLI).
func TestAPIVersioning_V1AliasesMirrorUnversionedRoutes(t *testing.T) {
	port := freePort(t)
	s := New(&mockBridge{})
	if err := s.StartHTTPWithAddr(port, "127.0.0.1"); err != nil {
		t.Fatalf("StartHTTPWithAddr() error = %v", err)
	}
	defer s.Stop()
	waitForListening(t, port)

	base := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Plain route, nil-fullBridge-safe: /api/version.
	for _, path := range []string{"/api/version", "/api/v1/version"} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status = %d, want %d", path, resp.StatusCode, http.StatusOK)
		}
	}

	// Go 1.22+ wildcard-pattern route: /api/skills/remove/{name}. A
	// nil fullBridge answers 503, not 404 — proves the pattern matched.
	for _, path := range []string{"/api/skills/remove/foo", "/api/v1/skills/remove/foo"} {
		req, _ := http.NewRequest(http.MethodDelete, base+path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("DELETE %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			t.Errorf("DELETE %s: status = 404, route was not registered", path)
		}
	}

	// Trailing-slash prefix route: /api/tasklists/. A nil fullBridge
	// answers 405, not 404 — proves the prefix pattern matched.
	for _, path := range []string{"/api/tasklists/abc", "/api/v1/tasklists/abc"} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			t.Errorf("GET %s: status = 404, route was not registered", path)
		}
	}

	// A path that was never a real route must stay a genuine 404 under
	// both prefixes — the alias must not turn into a catch-all.
	for _, path := range []string{"/api/does-not-exist", "/api/v1/does-not-exist"} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404", path, resp.StatusCode)
		}
	}
}
