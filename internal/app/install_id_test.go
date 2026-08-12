// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"os"
	"testing"

	"memo/internal/config"
)

// stashInstallID clears the on-disk install id for the duration of a test
// and restores whatever was there afterwards, so these tests can exercise
// the "fresh install" and "data wiped" paths against the process-global
// data dir without corrupting a real one.
func stashInstallID(t *testing.T) {
	t.Helper()
	path := config.DataPath(installIDFile)
	original, readErr := os.ReadFile(path)
	hadOriginal := readErr == nil
	_ = os.Remove(path)
	t.Cleanup(func() {
		if hadOriginal {
			_ = os.WriteFile(path, original, 0600)
		} else {
			_ = os.Remove(path)
		}
	})
}

// TestInstallID_StableAcrossRestarts is the property clients rely on to NOT
// wipe their saved sign-in on every launch: the same install must keep
// answering with the same id, including across a process restart (modelled
// here as a second App reading the same data dir).
func TestInstallID_StableAcrossRestarts(t *testing.T) {
	stashInstallID(t)

	first := (&App{}).InstallID()
	if first == "" {
		t.Fatal("expected a fresh install to mint an install id, got empty")
	}
	if again := (&App{}).InstallID(); again != first {
		t.Errorf("install id changed across restarts: %q then %q", first, again)
	}
}

// TestInstallID_CachedWithinOneApp guards the reason the value is cached at
// all: /api/setup/status is polled by every connected client every 30s, and
// must not read the disk each time.
func TestInstallID_CachedWithinOneApp(t *testing.T) {
	stashInstallID(t)

	a := &App{}
	first := a.InstallID()
	if first == "" {
		t.Fatal("expected an install id, got empty")
	}
	// Delete the file out from under the App: a cached value must still
	// be returned rather than silently minting a second, different id.
	_ = os.Remove(config.DataPath(installIDFile))
	if again := a.InstallID(); again != first {
		t.Errorf("cached install id not reused: %q then %q", first, again)
	}
}

// TestInstallID_ChangesAfterDataWipe is the whole point of this file, and
// the exact scenario reported from a Raspberry Pi on 2026-08-13: the user
// ran uninstall-selfhosted.sh and reinstalled, so the backend was brand new
// while the browser still held its old auth state against the same origin.
// A wiped data dir must read as a DIFFERENT install so clients can tell.
func TestInstallID_ChangesAfterDataWipe(t *testing.T) {
	stashInstallID(t)

	before := (&App{}).InstallID()
	if before == "" {
		t.Fatal("expected an install id, got empty")
	}

	// Wipe — exactly what `rm -rf ~/.memo` does to this file.
	if err := os.Remove(config.DataPath(installIDFile)); err != nil {
		t.Fatalf("remove install id: %v", err)
	}

	after := (&App{}).InstallID()
	if after == "" {
		t.Fatal("expected a reinstall to mint a new install id, got empty")
	}
	if after == before {
		t.Errorf("install id survived a data wipe (%q) — clients cannot detect the reinstall", after)
	}
}
