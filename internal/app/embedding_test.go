package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

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

// Exercise real process death and readiness cancellation, without a real GGUF,
// GPU, network download or language model.
func TestEmbeddingLifecycle(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "embedding-server")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", binary, "./testdata/embedding-server")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v\n%s", err, out)
	}

	newApp := func(t *testing.T, filename string) *App {
		t.Helper()
		root := t.TempDir()
		config.ResetForTests()
		t.Cleanup(config.ResetForTests)
		cfg, err := config.Load(filepath.Join(root, "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		cfg.Llama.BinaryPath, cfg.Llama.EngineMode, cfg.Llama.EmbeddingPort = binary, "cpu", port
		cfg.Llama.ModelsDir = root
		cfg.Memory.PersistDir, cfg.Memory.EmbeddingDimension = filepath.Join(root, "memory"), 3
		cfg.Memory.EmbeddingModelFile = filename
		cfg.Memory.MemoryEnabled, cfg.Memory.EmbeddingAutoStart = true, true
		if err := os.WriteFile(filepath.Join(root, filename), []byte("fake model"), 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		a := &App{cfg: cfg, lifecycleCtx: ctx, lifecycleCancel: cancel, llamaEmbedServer: llama.NewIsolatedServer(port, 512), llamaServer: llama.NewIsolatedServer(port+1, 512)}
		t.Cleanup(func() {
			cancel()
			a.embeddingMu.Lock()
			defer a.embeddingMu.Unlock()
			_ = a.llamaEmbedServer.Stop()
			if a.store != nil {
				_ = a.store.Close()
			}
		})
		return a
	}
	starts := func(a *App) int {
		data, _ := os.ReadFile(filepath.Join(a.cfg.Llama.ModelsDir, a.cfg.Memory.EmbeddingModelFile) + ".starts")
		return len(strings.Fields(string(data)))
	}
	t.Run("boot and crash recover, manual stop persists and suppresses every hook", func(t *testing.T) {
		a := newApp(t, "embedding.gguf")
		a.startupEmbeddingModel()
		if !a.llamaEmbedServer.IsRunning() || a.embeddingClient == nil || a.store == nil {
			t.Fatal("boot did not wire embedding")
		}
		// The supervisor must restart the same model after an actual unexpected exit.
		done := make(chan struct{})
		go func() { defer close(done); a.superviseEmbedding(a.lifecycleCtx, 20*time.Millisecond) }()
		t.Cleanup(func() { a.lifecycleCancel(); <-done })
		client := &http.Client{Timeout: time.Second}
		resp, _ := client.Post(strings.TrimSuffix(a.llamaEmbedServer.GetBaseURL(), "/v1")+"/crash", "", nil)
		if resp != nil {
			resp.Body.Close()
		}
		deadline := time.Now().Add(10 * time.Second)
		for starts(a) < 2 || !a.llamaEmbedServer.IsRunning() {
			if time.Now().After(deadline) {
				t.Fatal("crashed process was not restarted")
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err := a.StopEmbeddingModel(); err != nil {
			t.Fatal(err)
		}
		count := starts(a)
		a.startupEmbeddingModel()
		a.autoStartEmbeddingModel()
		if starts(a) != count || a.llamaEmbedServer.IsRunning() {
			t.Fatal("manual stop was undone")
		}
		saved, err := config.Load(filepath.Join(a.cfg.Llama.ModelsDir, "config.yaml"))
		if err != nil || saved.Memory.EmbeddingAutoStart {
			t.Fatalf("manual stop was not persisted: %v", err)
		}
		if err := a.SetMemoryEnabled(false); err != nil {
			t.Fatal(err)
		}
		if err := a.SetMemoryEnabled(true); err != nil {
			t.Fatal(err)
		}
		a.startupEmbeddingModel()
		if starts(a) != count {
			t.Fatal("memory toggle erased manual stop")
		}
		if err := a.StartEmbeddingModel(filepath.Join(a.cfg.Llama.ModelsDir, "embedding.gguf"), 0); err != nil {
			t.Fatal(err)
		}
		saved, err = config.Load(filepath.Join(a.cfg.Llama.ModelsDir, "config.yaml"))
		if err != nil || !saved.Memory.EmbeddingAutoStart {
			t.Fatalf("manual start did not persist auto-start: %v", err)
		}
	})
	t.Run("memory off cancels readiness and suppresses startup", func(t *testing.T) {
		a := newApp(t, "loading.gguf")
		done := make(chan struct{})
		go func() { defer close(done); a.startupEmbeddingModel() }()
		deadline := time.Now().Add(5 * time.Second)
		for starts(a) == 0 {
			if time.Now().After(deadline) {
				t.Fatal("helper never launched")
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err := a.SetMemoryEnabled(false); err != nil {
			t.Fatal(err)
		}
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("disabled memory did not cancel loading")
		}
		count := starts(a)
		a.startupEmbeddingModel()
		a.autoStartEmbeddingModel()
		if starts(a) != count || a.llamaEmbedServer.IsRunning() {
			t.Fatal("disabled memory was revived")
		}
		if !a.cfg.Memory.EmbeddingAutoStart {
			t.Fatal("memory-off erased startup preference")
		}
	})
	t.Run("manual stop cancels loading and survives supervisor checks", func(t *testing.T) {
		a := newApp(t, "loading.gguf")
		done := make(chan struct{})
		go func() { defer close(done); a.startupEmbeddingModel() }()
		deadline := time.Now().Add(5 * time.Second)
		for starts(a) == 0 {
			if time.Now().After(deadline) {
				t.Fatal("helper never launched")
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err := a.StopEmbeddingModel(); err != nil {
			t.Fatal(err)
		}
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("manual stop did not cancel loading")
		}
		count := starts(a)
		a.startupEmbeddingModel()
		if starts(a) != count || a.llamaEmbedServer.IsRunning() {
			t.Fatal("stopped loading process was revived")
		}
	})
	t.Run("manual start waits for initial memory store and shutdown cancels wait", func(t *testing.T) {
		a := newApp(t, "embedding.gguf")
		a.memoryStoreReady = make(chan struct{})
		done := make(chan error, 1)
		go func() { done <- a.StartEmbeddingModel(filepath.Join(a.cfg.Llama.ModelsDir, "embedding.gguf"), 0) }()
		deadline := time.Now().Add(5 * time.Second)
		for {
			a.cfgMu.RLock()
			waiting := a.embeddingCancel != nil
			a.cfgMu.RUnlock()
			if waiting {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("start never reached readiness gate")
			}
			time.Sleep(10 * time.Millisecond)
		}
		if starts(a) != 0 {
			t.Fatal("embedding started before placeholder store was ready")
		}
		a.lifecycleCancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("start error=%v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("shutdown did not cancel waiting start")
		}
		if starts(a) != 0 {
			t.Fatal("embedding started after shutdown")
		}
	})
	t.Run("memory off stops a running engine and reenable restores it", func(t *testing.T) {
		a := newApp(t, "embedding.gguf")
		a.startupEmbeddingModel()
		if !a.llamaEmbedServer.IsRunning() {
			t.Fatal("initial embedding not running")
		}
		if err := a.SetMemoryEnabled(false); err != nil {
			t.Fatal(err)
		}
		count := starts(a)
		a.startupEmbeddingModel()
		if a.llamaEmbedServer.IsRunning() || starts(a) != count {
			t.Fatal("memory off was ignored")
		}
		if err := a.SetMemoryEnabled(true); err != nil {
			t.Fatal(err)
		}
		a.startupEmbeddingModel()
		if !a.llamaEmbedServer.IsRunning() {
			t.Fatal("memory reenable failed to restore engine")
		}
	})
	t.Run("engine update suspends supervisor without changing user preference", func(t *testing.T) {
		a := newApp(t, "embedding.gguf")
		a.startupEmbeddingModel()
		resume, err := a.quiesceLlama()
		if err != nil {
			t.Fatal(err)
		}
		count := starts(a)
		a.startupEmbeddingModel()
		if starts(a) != count || a.llamaEmbedServer.IsRunning() {
			t.Fatal("supervisor restarted during engine update")
		}
		if err := resume(); err != nil {
			t.Fatal(err)
		}
		if !a.llamaEmbedServer.IsRunning() || !a.cfg.Memory.EmbeddingAutoStart {
			t.Fatal("engine resume lost startup preference")
		}
	})
}
