package app

import (
	"testing"
	"time"

	"memo/internal/shutdown"
)

// withStubbedSelfShutdown swaps selfShutdownSignal for a channel-backed stub
// for the duration of the test, so exercising the "last client left" path
// never delivers a real SIGINT to the test binary (see shutdown_test.go's
// shutdownForceExit for the same technique).
func withStubbedSelfShutdown(t *testing.T) chan struct{} {
	t.Helper()
	fired := make(chan struct{}, 8)
	old := selfShutdownSignal
	selfShutdownSignal = func() { fired <- struct{}{} }
	t.Cleanup(func() { selfShutdownSignal = old })
	return fired
}

func expectNoShutdown(t *testing.T, fired chan struct{}) {
	t.Helper()
	select {
	case <-fired:
		t.Fatal("selfShutdownSignal fired unexpectedly")
	case <-time.After(100 * time.Millisecond):
	}
}

func expectShutdown(t *testing.T, fired chan struct{}) {
	t.Helper()
	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("selfShutdownSignal did not fire")
	}
}

func TestClientRegistry_AutoShutdownOff_NeverSelfShutsDown(t *testing.T) {
	// A plain --headless run (standalone service) never calls
	// EnableAutoShutdown — registering and then unregistering the only
	// client must never trigger a shutdown, no matter how many clients
	// come and go.
	fired := withStubbedSelfShutdown(t)
	a := &App{}

	id := a.RegisterClient()
	a.UnregisterClient(id)

	expectNoShutdown(t, fired)
}

func TestClientRegistry_AutoShutdownOn_LastUnregisterShutsDown(t *testing.T) {
	fired := withStubbedSelfShutdown(t)
	a := &App{}
	a.EnableAutoShutdown()

	id := a.RegisterClient()
	a.UnregisterClient(id)

	expectShutdown(t, fired)
}

func TestClientRegistry_AutoShutdownOn_OtherClientsRemain_NoShutdown(t *testing.T) {
	// The exact scenario this feature exists for: CLI spawns the backend,
	// GUI attaches via /gui, CLI exits — the GUI's registration must keep
	// the backend alive.
	fired := withStubbedSelfShutdown(t)
	a := &App{}
	a.EnableAutoShutdown()

	cli := a.RegisterClient()
	gui := a.RegisterClient()
	a.UnregisterClient(cli)
	expectNoShutdown(t, fired)

	a.UnregisterClient(gui)
	expectShutdown(t, fired)
}

func TestClientRegistry_AutoShutdownOn_NoClientEverRegistered_NoShutdown(t *testing.T) {
	// Sweeping an empty, never-populated registry must not arm shutdown —
	// there's nothing to have "lost".
	fired := withStubbedSelfShutdown(t)
	a := &App{}
	a.EnableAutoShutdown()

	a.sweepStaleClients()

	expectNoShutdown(t, fired)
}

func TestClientRegistry_HeartbeatKeepsClientAlive(t *testing.T) {
	fired := withStubbedSelfShutdown(t)
	a := &App{}
	a.EnableAutoShutdown()

	id := a.RegisterClient()
	if err := a.HeartbeatClient(id); err != nil {
		t.Fatalf("HeartbeatClient() error = %v", err)
	}

	a.sweepStaleClients()
	expectNoShutdown(t, fired)
}

func TestClientRegistry_HeartbeatUnknownID_ReturnsError(t *testing.T) {
	a := &App{}
	if err := a.HeartbeatClient("does-not-exist"); err == nil {
		t.Error("HeartbeatClient() with an unknown ID should error")
	}
}

func TestClientRegistry_SweepPrunesStaleClient(t *testing.T) {
	fired := withStubbedSelfShutdown(t)
	a := &App{}
	a.EnableAutoShutdown()

	id := a.RegisterClient()
	// Backdate the heartbeat past clientStaleAfter instead of sleeping for
	// real — same "force the deadline" approach as the shutdown watchdog
	// test above.
	a.clients.mu.Lock()
	a.clients.clients[id] = time.Now().Add(-clientStaleAfter - time.Second)
	a.clients.mu.Unlock()

	a.sweepStaleClients()

	expectShutdown(t, fired)
}

// TestClientRegistry_HasActiveClients is a regression test for BUG-L3: the
// shutdown decision (registry was empty) and the self-signal actually being
// handled in main.go are separated by a real time window in which a new
// client can register. main.go's signal-wait loop re-checks
// HasActiveClients() right before committing to shut down, so it needs to
// reliably reflect "is anything registered right now" at any point —
// including immediately after the exact register/unregister calls that
// drive the shutdown decision itself.
func TestClientRegistry_HasActiveClients(t *testing.T) {
	a := &App{}

	if a.HasActiveClients() {
		t.Fatal("HasActiveClients() = true on a fresh registry, want false")
	}

	id := a.RegisterClient()
	if !a.HasActiveClients() {
		t.Fatal("HasActiveClients() = false right after RegisterClient(), want true")
	}

	// The stale-signal scenario BUG-L3 describes: a second client registers
	// (e.g. /gui) after the first unregisters but before the pending
	// self-shutdown signal is handled.
	withStubbedSelfShutdown(t)
	a.EnableAutoShutdown()
	other := a.RegisterClient()
	a.UnregisterClient(id)
	if !a.HasActiveClients() {
		t.Fatal("HasActiveClients() = false with a client still registered, want true")
	}

	a.UnregisterClient(other)
	if a.HasActiveClients() {
		t.Fatal("HasActiveClients() = true after the last client unregistered, want false")
	}
}

// TestSelfShutdownSignal_RequestsProcessWideShutdown is a regression test
// for BUG-H3: the real (unstubbed) selfShutdownSignal used to self-deliver
// os.Interrupt via os.Process.Signal, which is a silent no-op on Windows
// (Go only implements Process.Signal for os.Kill there) — the backend never
// actually stopped. It must now go through internal/shutdown, which has no
// such platform gap.
func TestSelfShutdownSignal_RequestsProcessWideShutdown(t *testing.T) {
	// shutdown.ch is a process-wide package var — drain any stray pending
	// request before asserting on it.
	select {
	case <-shutdown.Requested():
	default:
	}

	selfShutdownSignal()

	select {
	case <-shutdown.Requested():
	case <-time.After(time.Second):
		t.Fatal("selfShutdownSignal() did not request a shutdown via internal/shutdown")
	}
}

func TestClientRegistry_RegisterReturnsUniqueIDs(t *testing.T) {
	a := &App{}
	seen := make(map[string]bool)
	for range 20 {
		id := a.RegisterClient()
		if seen[id] {
			t.Fatalf("RegisterClient() returned a duplicate ID: %q", id)
		}
		seen[id] = true
	}
}
