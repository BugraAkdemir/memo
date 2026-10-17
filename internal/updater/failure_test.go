// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFailureNote_RoundTripAndWording(t *testing.T) {
	p := filepath.Join(t.TempDir(), FailureFile)
	if got := ReadFailure(p); got != "" {
		t.Fatalf("no file should read as empty, got %q", got)
	}
	cause := fmt.Errorf("%w (the new version exited before it answered)", ErrRolledBack)
	if err := WriteFailure(p, "V4.7.0", cause); err != nil {
		t.Fatal(err)
	}
	got := ReadFailure(p)
	if !strings.Contains(got, "V4.7.0") || !strings.Contains(got, "rolled back") || !strings.Contains(got, "exited before it answered") {
		t.Fatalf("note = %q", got)
	}
	if strings.ContainsAny(got, "\n\r") {
		t.Fatalf("note must be one line: %q", got)
	}
	// A failure that is not a rollback does not claim one.
	if err := WriteFailure(p, "V4.7.0", errors.New("disk full")); err != nil {
		t.Fatal(err)
	}
	if got := ReadFailure(p); strings.Contains(got, "rolled back") || !strings.Contains(got, "disk full") {
		t.Fatalf("note = %q", got)
	}
}

func TestFailureNote_IsBoundedAndAgesOut(t *testing.T) {
	p := filepath.Join(t.TempDir(), FailureFile)
	if err := WriteFailure(p, "V4.7.0", errors.New(strings.Repeat("x\n", 5000))); err != nil {
		t.Fatal(err)
	}
	if got := ReadFailure(p); len(got) > 450 || strings.Contains(got, "\n") {
		t.Fatalf("a huge cause must be cut to one short line, got %d bytes", len(got))
	}
	old := time.Now().Add(-failureTTL - time.Hour)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	if got := ReadFailure(p); got != "" {
		t.Fatalf("a note older than two weeks must not nag forever, got %q", got)
	}
}

func TestFailureNote_RefusesNonFilesAndGarbage(t *testing.T) {
	dir := t.TempDir()
	if got := ReadFailure(dir); got != "" {
		t.Errorf("a directory read as %q", got)
	}
	p := filepath.Join(dir, FailureFile)
	if err := os.WriteFile(p, []byte{0xff, 0xfe, 0xfd, 'a'}, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ReadFailure(p); got != "" {
		t.Errorf("invalid UTF-8 shown: %q", got)
	}
	if err := os.Symlink(p, filepath.Join(dir, "link")); err == nil {
		// A symlink is followed by Stat, but a regular file behind it is fine; what
		// must not happen is reading a FIFO/device: covered by IsRegular above.
		_ = os.Remove(filepath.Join(dir, "link"))
	}
}
