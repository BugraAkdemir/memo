package taskloop

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestEngine_MaxConcurrentLists_Gate is the v4.6.0 Faz C check: lists no
// longer serialise against each other, but MaxConcurrentLists (0 = unlimited)
// is still honoured as a safety valve.
func TestEngine_MaxConcurrentLists_Gate(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tlA, _ := store.Create("c1", "A", []string{"x"})
	tlB, _ := store.Create("c2", "B", []string{"y"})
	a, b := tlA.ID, tlB.ID

	block := make(chan struct{})
	worker := func(ctx context.Context, chatID, prompt string) (string, error) {
		<-block // hold the list "running" until the test releases it
		return "ok", nil
	}
	chief := func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil }

	eng := NewEngine(store, worker, chief, func(bool) {}, func(string, string) {},
		WithMaxConcurrentLists(1))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := eng.Start(ctx, a); err != nil {
		t.Fatalf("Start(A) with limit 1 and nothing running: %v", err)
	}
	// A is now occupying the only slot.
	waitFor(t, func() bool { return eng.IsRunning(a) }, 2*time.Second, "A to be running")

	if err := eng.Start(ctx, b); err == nil {
		t.Fatal("Start(B) should be rejected while A holds the only concurrency slot")
	} else if !strings.Contains(err.Error(), "aynı anda") {
		t.Fatalf("Start(B) error = %q, want the concurrency-limit message", err)
	}

	close(block) // let A finish, freeing the slot
	waitFor(t, func() bool { return !eng.IsRunning(a) }, 3*time.Second, "A to finish")

	// Fresh engine, unlimited: both start.
	block2 := make(chan struct{})
	eng2 := NewEngine(store,
		func(ctx context.Context, chatID, prompt string) (string, error) { <-block2; return "ok", nil },
		chief, func(bool) {}, func(string, string) {},
		WithMaxConcurrentLists(0))
	if err := eng2.Start(ctx, a); err != nil {
		t.Fatalf("unlimited Start(A): %v", err)
	}
	if err := eng2.Start(ctx, b); err != nil {
		t.Fatalf("unlimited Start(B) should not be gated: %v", err)
	}
	// Drain both eng2 goroutines before the test's TempDir is torn down, so
	// they don't try to persist state to a deleted directory.
	close(block2)
	waitFor(t, func() bool { return !eng2.IsRunning(a) && !eng2.IsRunning(b) },
		3*time.Second, "eng2 lists to finish")
}

func waitFor(t *testing.T, cond func() bool, d time.Duration, what string) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
