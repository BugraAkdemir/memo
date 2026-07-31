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

// An empty AuthKey is no longer rejected upfront — it's the (default)
// interactive-login path, which genuinely tries to reach Tailscale's control
// servers and blocks for up to interactiveLoginTimeout waiting on a human to
// approve a browser prompt. That's not something a unit test should exercise
// end-to-end (real network I/O, minutes-long by design); instead, test the
// fast, deterministic part of that path directly: the pending-auth-URL
// plumbing (setPendingAuthURL/AuthURL) that Start()/connect() drive.
func TestSetPendingAuthURL_UpdatesAuthURL(t *testing.T) {
	ts := NewTailscale()
	if url := ts.AuthURL(); url != "" {
		t.Fatalf("expected empty AuthURL initially, got %q", url)
	}

	const u = "https://login.tailscale.com/a/testtoken"
	ts.setPendingAuthURL(u, false)
	if url := ts.AuthURL(); url != u {
		t.Errorf("expected AuthURL %q, got %q", u, url)
	}

	ts.setPendingAuthURL("", false)
	if url := ts.AuthURL(); url != "" {
		t.Errorf("expected AuthURL cleared, got %q", url)
	}
}

// autoOpen only gates whether setPendingAuthURL attempts to launch a
// browser — the URL itself must still be recorded either way, so
// GetRemoteAccessStatus/the Settings UI can surface a "log in again" link
// for a boot-time (non-interactive) attempt that turned out to need a fresh
// login. Actually verifying the browser launch is skipped would need
// injecting a fake browseropen.OpenURL, which isn't wired up for testing;
// this only covers the state side.
func TestSetPendingAuthURL_NoAutoOpenStillRecordsURL(t *testing.T) {
	ts := NewTailscale()
	const u = "https://login.tailscale.com/a/testtoken2"
	ts.setPendingAuthURL(u, false)
	if url := ts.AuthURL(); url != u {
		t.Errorf("expected AuthURL %q, got %q", u, url)
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
	// The early rejection must set lastErr too — otherwise a caller that
	// only polls LastError()/GetRemoteAccessStatus (rather than the return
	// value of this specific Start() call) sees no error at all.
	if got := ts.LastError(); got == "" {
		t.Error("expected LastError() to be set after an already-running rejection")
	}
}

func TestStart_AlreadyConnecting(t *testing.T) {
	ts := NewTailscale()
	ts.mu.Lock()
	ts.connecting = true
	ts.mu.Unlock()

	err := ts.Start(TailscaleConfig{AuthKey: "tskey-auth-xxxx"})
	if err == nil {
		t.Error("expected error when a connect() is already in flight")
	}
	if got := ts.LastError(); got == "" {
		t.Error("expected LastError() to be set after an already-connecting rejection")
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
