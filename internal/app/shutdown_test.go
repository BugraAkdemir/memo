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
	oldTimeout, oldExit, oldCleanup := shutdownTimeout, shutdownForceExit, shutdownCleanup
	defer func() {
		shutdownTimeout, shutdownForceExit, shutdownCleanup = oldTimeout, oldExit, oldCleanup
	}()

	// Cleanup that genuinely doesn't finish until this test lets it, so the
	// watchdog is the only branch that can win. Shrinking shutdownTimeout to
	// 1ns instead (the previous approach) left both select cases ready at
	// once, which Go resolves at random — that flaked in CI, where the
	// cleanup goroutine finished before the runtime delivered the expired
	// timer. Deferred before the restore above so it runs FIRST (LIFO),
	// releasing the goroutine while the stub is still installed.
	release := make(chan struct{})
	defer close(release)
	shutdownCleanup = func(*App, context.Context) { <-release }
	shutdownTimeout = 50 * time.Millisecond

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
