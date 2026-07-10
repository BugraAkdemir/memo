package whatsapp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestHasRegisteredDeviceHardensSessionDBPermissions guards BUG-H1:
// whatsmeow's sqlstore creates session.db itself (Signal/Noise session
// keys) with no Go-level perm parameter, landing at whatever the process
// umask allows (typically 0644, world-readable). HasRegisteredDevice is
// what may create this file on a fresh install (first startup probe before
// any pairing), so it must chmod it to 0600.
func TestHasRegisteredDeviceHardensSessionDBPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.db")

	// Return value doesn't matter here (a fresh file has no registered
	// device) — only that the probe creates the file and hardens it.
	HasRegisteredDevice(context.Background(), path)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v (session.db should have been created)", err)
	}
	if mode := info.Mode() & os.ModePerm; mode != 0o600 {
		t.Errorf("mode = %o, want 0600", mode)
	}
}
