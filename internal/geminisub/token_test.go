// SPDX-License-Identifier: AGPL-3.0-or-later

package geminisub

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func testKey() []byte { return bytes.Repeat([]byte{0x2a}, 32) }

func TestTokenStore_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "token.enc")
	s := newTokenStoreAt(path, testKey())

	want := &oauth2.Token{
		AccessToken:  "access-123",
		RefreshToken: "refresh-456",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour).Round(time.Second),
	}
	if err := s.save(want); err != nil {
		t.Fatalf("save: %v", err)
	}

	// File must exist, be 0600, and not contain the secret in the clear.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("token file perm = %o, want 600", perm)
	}
	raw, _ := os.ReadFile(path)
	if bytes.Contains(raw, []byte("refresh-456")) {
		t.Error("refresh token stored in plaintext")
	}

	got, ok := s.load()
	if !ok {
		t.Fatal("load: ok=false after save")
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken || !got.Expiry.Equal(want.Expiry) {
		t.Errorf("round-trip mismatch: got %+v want %+v", got, want)
	}
}

func TestTokenStore_MissingFile(t *testing.T) {
	s := newTokenStoreAt(filepath.Join(t.TempDir(), "token.enc"), testKey())
	if tok, ok := s.load(); ok || tok != nil {
		t.Errorf("load on missing file = (%v, %v), want (nil, false)", tok, ok)
	}
}

func TestTokenStore_CorruptCiphertext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.enc")
	if err := os.WriteFile(path, []byte("not-hex-and-not-encrypted"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := newTokenStoreAt(path, testKey())
	// Must not panic; must read as "not connected".
	if tok, ok := s.load(); ok || tok != nil {
		t.Errorf("load on corrupt file = (%v, %v), want (nil, false)", tok, ok)
	}
}

func TestTokenStore_WrongKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.enc")
	if err := newTokenStoreAt(path, testKey()).save(&oauth2.Token{AccessToken: "x"}); err != nil {
		t.Fatal(err)
	}
	other := newTokenStoreAt(path, bytes.Repeat([]byte{0x11}, 32))
	if tok, ok := other.load(); ok || tok != nil {
		t.Errorf("load with wrong key = (%v, %v), want (nil, false)", tok, ok)
	}
}

func TestTokenStore_Clear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.enc")
	s := newTokenStoreAt(path, testKey())
	if err := s.save(&oauth2.Token{AccessToken: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := s.clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still present after clear: %v", err)
	}
	// clear on an already-missing file is not an error.
	if err := s.clear(); err != nil {
		t.Errorf("second clear: %v", err)
	}
}

// TestTokenStore_SaveIsAtomic_FailedWriteDoesNotCorruptExistingToken guards
// O6: save() used to write straight to the destination file (os.WriteFile),
// unlike every other credential file in this codebase. A crash or power
// loss partway through that write left a truncated token.enc that fails to
// decrypt on the next load() (see load's doc comment) — read as "not
// connected", silently forcing a fresh Google sign-in for no reason ever
// logged as an actual error. save() now goes through fileutil.AtomicWrite
// (write-to-a-sibling-.tmp, then rename), so a failed write must leave the
// previously-saved good token completely intact rather than overwriting it.
//
// A real mid-write crash can't be simulated in a unit test, so this
// exercises the same guarantee via a write that fails for a different
// reason (a read-only directory refusing to create the .tmp file) —
// exactly the failure branch AtomicWrite's rename-or-nothing design exists
// to protect against either way.
func TestTokenStore_SaveIsAtomic_FailedWriteDoesNotCorruptExistingToken(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits don't work the same way on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permission checks")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "token.enc")
	s := newTokenStoreAt(path, testKey())

	good := &oauth2.Token{AccessToken: "good-token", RefreshToken: "good-refresh"}
	if err := s.save(good); err != nil {
		t.Fatalf("initial save: %v", err)
	}

	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	bad := &oauth2.Token{AccessToken: "should-never-land", RefreshToken: "should-never-land"}
	if err := s.save(bad); err == nil {
		t.Fatal("expected save to fail while its directory is read-only")
	}

	os.Chmod(dir, 0o700) // restore write access before reading back
	got, ok := s.load()
	if !ok {
		t.Fatal("load: ok=false — the original good token was lost after a failed save")
	}
	if got.AccessToken != good.AccessToken {
		t.Errorf("AccessToken = %q after a failed save, want the untouched original %q", got.AccessToken, good.AccessToken)
	}
}
