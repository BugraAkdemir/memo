package app

import (
	"context"
	"testing"
	"time"
)

// newTestApp builds a minimal App safe to call shutdownSync on: every field
// shutdownSync touches is either nil-checked already or, like
// memorySaveCh, must be non-nil to avoid a panic on close().
func newTestApp() *App {
	return &App{memorySaveCh: make(chan saveTask, 1)}
}

func TestShutdown_NormalPathDoesNotForceExit(t *testing.T) {
	oldTimeout, oldExit := shutdownTimeout, shutdownForceExit
	defer func() { shutdownTimeout, shutdownForceExit = oldTimeout, oldExit }()

	shutdownTimeout = time.Second
	exitCalled := false
	shutdownForceExit = func(code int) { exitCalled = true }

	a := newTestApp()
	a.Shutdown(context.Background())

	if exitCalled {
		t.Error("shutdownForceExit should not be called when cleanup finishes well within the timeout")
	}
}

func TestShutdown_WatchdogForcesExitWhenCleanupIsSlow(t *testing.T) {
	oldTimeout, oldExit := shutdownTimeout, shutdownForceExit
	defer func() { shutdownTimeout, shutdownForceExit = oldTimeout, oldExit }()

	// A timeout far shorter than any real cleanup path guarantees the
	// watchdog branch wins the select race deterministically, without
	// needing an actually-hanging subsystem to reproduce it.
	shutdownTimeout = 1 * time.Nanosecond
	exitCode := make(chan int, 1)
	shutdownForceExit = func(code int) { exitCode <- code }

	a := newTestApp()
	a.Shutdown(context.Background())

	select {
	case code := <-exitCode:
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watchdog did not fire within 2s")
	}
}

func TestShutdown_OnlyRunsOnce(t *testing.T) {
	oldTimeout, oldExit := shutdownTimeout, shutdownForceExit
	defer func() { shutdownTimeout, shutdownForceExit = oldTimeout, oldExit }()

	shutdownTimeout = time.Second
	shutdownForceExit = func(code int) {}

	a := newTestApp()
	a.Shutdown(context.Background())
	a.Shutdown(context.Background()) // must not panic (e.g. double-close memorySaveCh)
}
