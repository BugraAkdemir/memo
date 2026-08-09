// SPDX-License-Identifier: AGPL-3.0-or-later

package remoteauth

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSessionToken_RoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	token, err := IssueSessionToken(key, "admin")
	if err != nil {
		t.Fatalf("IssueSessionToken: %v", err)
	}
	subject, err := ValidateSessionToken(key, token)
	if err != nil {
		t.Fatalf("ValidateSessionToken: %v", err)
	}
	if subject != "admin" {
		t.Fatalf("expected subject %q, got %q", "admin", subject)
	}
}

func TestValidateSessionToken_ExpiredFails(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	// Issued far enough in the past that its 1-minute ttl has elapsed.
	token, err := issueSessionTokenAt(key, "admin", time.Now().Add(-2*time.Hour), time.Minute)
	if err != nil {
		t.Fatalf("issueSessionTokenAt: %v", err)
	}
	if _, err := ValidateSessionToken(key, token); err == nil {
		t.Fatal("expected expired token to fail validation")
	}
}

func TestValidateSessionToken_WrongKeyFails(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	wrongKey := []byte("ffffffffffffffffffffffffffffffff")
	token, err := IssueSessionToken(key, "admin")
	if err != nil {
		t.Fatalf("IssueSessionToken: %v", err)
	}
	if _, err := ValidateSessionToken(wrongKey, token); err == nil {
		t.Fatal("expected token signed with a different key to fail validation")
	}
}

func TestValidateSessionToken_MalformedFails(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	cases := []string{"", "not-a-jwt", "a.b.c", "a.b"}
	for _, c := range cases {
		if _, err := ValidateSessionToken(key, c); err == nil {
			t.Errorf("case %q: expected malformed token to fail validation", c)
		}
	}
}

func TestLoadOrCreateSigningKey_PersistsAndReuses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.key")

	key1, err := LoadOrCreateSigningKey(path)
	if err != nil {
		t.Fatalf("LoadOrCreateSigningKey (create): %v", err)
	}
	if len(key1) != 32 {
		t.Fatalf("expected 32-byte key, got %d bytes", len(key1))
	}

	key2, err := LoadOrCreateSigningKey(path)
	if err != nil {
		t.Fatalf("LoadOrCreateSigningKey (reload): %v", err)
	}
	if string(key1) != string(key2) {
		t.Fatal("expected the same key to be reloaded from disk, not regenerated")
	}
}

func TestLoadOrCreateSigningKey_DifferentPathsGetDifferentKeys(t *testing.T) {
	dir := t.TempDir()
	key1, err := LoadOrCreateSigningKey(filepath.Join(dir, "a.key"))
	if err != nil {
		t.Fatalf("LoadOrCreateSigningKey: %v", err)
	}
	key2, err := LoadOrCreateSigningKey(filepath.Join(dir, "b.key"))
	if err != nil {
		t.Fatalf("LoadOrCreateSigningKey: %v", err)
	}
	if string(key1) == string(key2) {
		t.Fatal("expected freshly generated keys at different paths to differ")
	}
}
