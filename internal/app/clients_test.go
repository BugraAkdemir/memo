package app

import (
	"testing"
	"time"
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
