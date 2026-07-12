package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"memo/internal/config"
	"memo/internal/llama"
)

func fakeEmbeddingServerPort(t *testing.T, rawURL string) int {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse %q: %v", rawURL, err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse port from %q: %v", rawURL, err)
	}
	return port
}

// TestReconnectEmbeddingIfAlreadyRunning_WiresUpExternalServer is a
// regression test for a real user-reported bug: a fresh backend process
// (e.g. one the CLI's spawnDetachedBackend started) whose embedding port
// already has a live server on it — left running by an earlier backend
// process, or started manually before this one's Startup() ran — never
// actually got its memory store wired to that server. GetStatus()'s own
// pingPort() fallback already reported it as "running" (misleading the
// CLI's welcome banner into showing embedding as on), but a.embeddingClient
// stayed nil, wired to nothing — memory kept silently embedding through
// the Startup()-time placeholder (the main chat client) until something
// explicitly called StartEmbeddingModel. That's exactly why running
// /embedding "fixed" it: it's the only thing that ever called
// reinitMemoryStore with the real embedding client.
func TestReconnectEmbeddingIfAlreadyRunning_WiresUpExternalServer(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer fake.Close()

	a := &App{
		cfg: &config.AppConfig{
			API:    config.APIConfig{TimeoutSeconds: 5, EmbeddingModel: "test-model"},
			Memory: config.MemoryConfig{PersistDir: t.TempDir(), EmbeddingDimension: 3},
		},
		llamaEmbedServer: llama.NewServer(fakeEmbeddingServerPort(t, fake.URL), 512),
	}

	a.reconnectEmbeddingIfAlreadyRunning()

	if a.embeddingClient == nil {
		t.Fatal("reconnectEmbeddingIfAlreadyRunning() left embeddingClient nil — did not reconnect to the already-running server")
	}
	if a.store == nil {
		t.Fatal("reconnectEmbeddingIfAlreadyRunning() did not reinitialize the memory store")
	}
}

// TestReconnectEmbeddingIfAlreadyRunning_NoopWhenPortIsEmpty confirms the
// function doesn't blindly wire up a client when nothing is actually
// listening on the configured embedding port.
func TestReconnectEmbeddingIfAlreadyRunning_NoopWhenPortIsEmpty(t *testing.T) {
	a := &App{
		cfg: &config.AppConfig{
			API:    config.APIConfig{TimeoutSeconds: 5, EmbeddingModel: "test-model"},
			Memory: config.MemoryConfig{PersistDir: t.TempDir(), EmbeddingDimension: 3},
		},
		// Port 1 is a privileged port nothing listens on in a test sandbox —
		// connecting fails fast and reliably, no server to accidentally hit.
		llamaEmbedServer: llama.NewServer(1, 512),
	}

	a.reconnectEmbeddingIfAlreadyRunning()

	if a.embeddingClient != nil {
		t.Fatal("reconnectEmbeddingIfAlreadyRunning() wired a client even though nothing is listening on the port")
	}
}
