package webserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRemoteAuthOK_LocalOnlyAlwaysPasses guards the default, most common
// case: a server bound to 127.0.0.1 (remote access never enabled, or turned
// back off) must never require a token — this is exactly today's behavior
// for the local Flutter/CLI clients, unchanged by BUG-C1's fix.
func TestRemoteAuthOK_LocalOnlyAlwaysPasses(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/wipe", nil)
	if !remoteAuthOK("127.0.0.1", "memo-sometoken", r) {
		t.Fatal("expected local-only bind to pass with no token presented")
	}
}

// TestRemoteAuthOK_RemoteRequiresToken is the actual BUG-C1 regression case:
// once the listener is bound for remote access (LAN mode or ngrok — both
// rebind to 0.0.0.0), a request with no credential at all must be rejected.
// Before the fix, nothing checked the token, so every one of these
// previously-dangerous requests (wipe, agent permission grant, import,
// shutdown) sailed through unauthenticated.
func TestRemoteAuthOK_RemoteRequiresToken(t *testing.T) {
	sensitivePaths := []string{
		"/api/wipe",
		"/api/agent/permission",
		"/api/import",
		"/api/shutdown",
		"/api/uninstall",
		"/api/providers", // BUG-H1: plaintext provider API keys
	}
	for _, path := range sensitivePaths {
		r := httptest.NewRequest(http.MethodPost, path, nil)
		if remoteAuthOK("0.0.0.0", "memo-sometoken", r) {
			t.Errorf("path %s: expected remote request with no token to be rejected", path)
		}
	}
}

func TestRemoteAuthOK_RemoteWithCorrectXMemoTokenPasses(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	r.Header.Set("X-Memo-Token", "memo-sometoken")
	if !remoteAuthOK("0.0.0.0", "memo-sometoken", r) {
		t.Fatal("expected matching X-Memo-Token to be accepted")
	}
}

func TestRemoteAuthOK_RemoteWithCorrectBearerTokenPasses(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	r.Header.Set("Authorization", "Bearer memo-sometoken")
	if !remoteAuthOK("0.0.0.0", "memo-sometoken", r) {
		t.Fatal("expected matching Authorization: Bearer token to be accepted")
	}
}

func TestRemoteAuthOK_RemoteWithWrongTokenFails(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	r.Header.Set("X-Memo-Token", "attacker-guess")
	if remoteAuthOK("0.0.0.0", "memo-sometoken", r) {
		t.Fatal("expected mismatched token to be rejected")
	}
}

// TestRemoteAuthOK_EmptyStoredTokenFailsClosed covers a token that hasn't
// been generated yet (shouldn't normally happen once remote access has been
// enabled — SetRemoteAccess generates one — but must never fail open).
func TestRemoteAuthOK_EmptyStoredTokenFailsClosed(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	// An attacker presenting an empty token too must not match an empty
	// stored token.
	r.Header.Set("X-Memo-Token", "")
	if remoteAuthOK("0.0.0.0", "", r) {
		t.Fatal("expected empty stored token to never authenticate")
	}
}
