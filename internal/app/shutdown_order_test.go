// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"embed"
	"path/filepath"
	"testing"
	"time"

	"memo/internal/sessions"
)

func TestShutdown_HungPreEnginePhaseStillClosesStoreAndRunsLaterSteps(t *testing.T) {
	origPre := shutdownPreEngineTimeout
	origTotal := shutdownTimeout
	shutdownPreEngineTimeout = 50 * time.Millisecond
	shutdownTimeout = 2 * time.Second
	t.Cleanup(func() {
		shutdownPreEngineTimeout = origPre
		shutdownTimeout = origTotal
	})

	a := NewApp(embed.FS{}, "")
	dir := t.TempDir()
	// Simulate a store or session work that blocks inside background WG longer
	// than shutdownPreEngineTimeout, and ensure waitBackground + shutdownSync
	// complete well inside shutdownTimeout without firing shutdownForceExit.
	a.lifecycleCancel = func() {}
	mgr, err := sessions.NewManager(filepath.Join(dir, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	a.sessions = mgr

	origExit := shutdownForceExit
	forced := false
	shutdownForceExit = func(int) { forced = true }
	t.Cleanup(func() { shutdownForceExit = origExit })

	hungDone := make(chan struct{})
	t.Cleanup(func() { close(hungDone) })
	a.goBackground("hung_test", func() {
		<-hungDone
	})

	start := time.Now()
	a.Shutdown(context.Background())
	elapsed := time.Since(start)

	if forced {
		t.Fatal("Shutdown fired shutdownForceExit even though only pre-engine phase hung")
	}
	if elapsed >= shutdownTimeout {
		t.Fatalf("Shutdown elapsed %v >= total %v", elapsed, shutdownTimeout)
	}
}
