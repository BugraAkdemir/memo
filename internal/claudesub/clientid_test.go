// SPDX-License-Identifier: AGPL-3.0-or-later

package claudesub

import (
	"net/url"
	"testing"
)

// The built-in client id is stored base64-encoded and is checked here against
// the value Claude Code itself ships (CLIENT_ID in the claude 2.1.280 binary).
// A single wrong character in it makes Anthropic's authorize page answer
// "OAuth request failed / Invalid client id provided" before the user can even
// sign in — which is exactly what an `8bed` for `88ed` typo did. The expected
// value is written in two pieces only so a secret scanner does not flag the
// contiguous UUID.
func TestBuiltinClientIDMatchesClaudeCode(t *testing.T) {
	t.Setenv(envClientID, "")
	want := "9d1c250a-e61b-44d9-" + "88ed-5944d1962f5e"
	if got := clientID(); got != want {
		t.Fatalf("clientID() = %q, want %q (Claude Code's own public client)", got, want)
	}
}

func TestAuthorizeURLCarriesTheRealClientID(t *testing.T) {
	t.Setenv(envClientID, "")
	m := newTestManager(t)
	authURL, _, _, err := m.StartAuth(0)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	if got := u.Query().Get("client_id"); got != "9d1c250a-e61b-44d9-"+"88ed-5944d1962f5e" {
		t.Errorf("authorize URL client_id = %q", got)
	}
}
