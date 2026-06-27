package tunnel

import (
	"testing"
)

func TestNewTailscale(t *testing.T) {
	ts := NewTailscale()
	if ts == nil {
		t.Fatal("NewTailscale returned nil")
	}
}

func TestIsRunning_New(t *testing.T) {
	ts := NewTailscale()
	if ts.IsRunning() {
		t.Error("new tailscale should not be running")
	}
}

func TestPublicURL_New(t *testing.T) {
	ts := NewTailscale()
	if url := ts.PublicURL(); url != "" {
		t.Errorf("expected empty URL, got %q", url)
	}
}

func TestIPURL_New(t *testing.T) {
	ts := NewTailscale()
	if url := ts.IPURL(); url != "" {
		t.Errorf("expected empty URL, got %q", url)
	}
}

func TestLastError_New(t *testing.T) {
	ts := NewTailscale()
	if err := ts.LastError(); err != "" {
		t.Errorf("expected empty error, got %q", err)
	}
}

func TestStart_EmptyAuthKey(t *testing.T) {
	ts := NewTailscale()
	err := ts.Start(TailscaleConfig{
		Hostname:  "test",
		AuthKey:   "",
		Funnel:    false,
		LocalPort: 8090,
		StateDir:  t.TempDir(),
	})
	if err == nil {
		t.Error("expected error for empty auth key")
	}
}

func TestStart_AlreadyRunning(t *testing.T) {
	ts := NewTailscale()
	ts.mu.Lock()
	ts.running = true
	ts.mu.Unlock()

	err := ts.Start(TailscaleConfig{AuthKey: "tskey-auth-xxxx"})
	if err == nil {
		t.Error("expected error when already running")
	}
}

func TestStop_Idempotent(t *testing.T) {
	ts := NewTailscale()
	// Should not panic or error when called on a non-running tunnel
	ts.Stop()
}

func TestConcurrentAccess(t *testing.T) {
	ts := NewTailscale()
	done := make(chan bool, 4)

	fns := []func(){
		func() { ts.IsRunning() },
		func() { ts.PublicURL() },
		func() { ts.IPURL() },
		func() { ts.LastError() },
	}

	for _, fn := range fns {
		go func(f func()) {
			f()
			done <- true
		}(fn)
	}

	for i := 0; i < 4; i++ {
		<-done
	}
}
