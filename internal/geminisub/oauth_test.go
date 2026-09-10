// SPDX-License-Identifier: AGPL-3.0-or-later

package geminisub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// TestMain neutralises the real ~/.gemini/oauth_creds.json fallback so tests
// never pick up an actual gemini-cli login on the dev's machine.
func TestMain(m *testing.M) {
	geminiCLICredsPath = ""
	os.Exit(m.Run())
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return newManager(newTokenStoreAt(filepath.Join(t.TempDir(), "token.enc"), testKey()))
}

// fakeGoogle stands in for Google's OAuth token endpoint.
func fakeGoogle(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var exchanges int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&exchanges, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-new","refresh_token":"rt-new","token_type":"Bearer","expires_in":3600}`))
	}))
	prevAuth := googleAuthEndpoint
	googleAuthEndpoint = oauth2.Endpoint{AuthURL: srv.URL + "/auth", TokenURL: srv.URL + "/token"}
	t.Cleanup(func() {
		googleAuthEndpoint = prevAuth
		srv.Close()
	})
	return srv, &exchanges
}

func redirectPort(t *testing.T, authURL string) string {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse authURL: %v", err)
	}
	ru, err := url.Parse(u.Query().Get("redirect_uri"))
	if err != nil {
		t.Fatalf("parse redirect_uri: %v", err)
	}
	return ru.Host // 127.0.0.1:PORT
}

func TestStartAuth_URLShape(t *testing.T) {
	fakeGoogle(t)
	m := newTestManager(t)

	authURL, err := m.StartAuth()
	if err != nil {
		t.Fatalf("StartAuth: %v", err)
	}
	defer m.finishFlow(m.flow, nil) // release the waitgroup

	u, _ := url.Parse(authURL)
	q := u.Query()
	if q.Get("code_challenge") == "" {
		t.Error("no PKCE code_challenge in auth URL")
	}
	if got := q.Get("code_challenge_method"); got != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", got)
	}
	if got := q.Get("access_type"); got != "offline" {
		t.Errorf("access_type = %q, want offline", got)
	}
	if ru := q.Get("redirect_uri"); ru == "" || !strings.Contains(ru, "127.0.0.1") {
		t.Errorf("redirect_uri = %q, want a 127.0.0.1 loopback", ru)
	}
}

func TestOAuthFlow_Success(t *testing.T) {
	_, exchanges := fakeGoogle(t)
	m := newTestManager(t)

	authURL, err := m.StartAuth()
	if err != nil {
		t.Fatalf("StartAuth: %v", err)
	}
	host := redirectPort(t, authURL)

	resp, err := http.Get("http://" + host + "/oauth2callback?code=fake-code&state=memo-gemini-sub")
	if err != nil {
		t.Fatalf("callback GET: %v", err)
	}
	resp.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.AwaitAuth(ctx); err != nil {
		t.Fatalf("AwaitAuth: %v", err)
	}
	if atomic.LoadInt32(exchanges) != 1 {
		t.Errorf("token exchanges = %d, want 1", *exchanges)
	}
	if !m.Connected() {
		t.Fatal("Connected() = false after successful flow")
	}

	// Token was persisted encrypted and reloads.
	reloaded := newManager(m.tok)
	if !reloaded.Connected() {
		t.Error("token did not persist across Manager reload")
	}
	tok, _ := m.tok.load()
	if tok.AccessToken != "at-new" || tok.RefreshToken != "rt-new" {
		t.Errorf("persisted token = %+v, want at-new/rt-new", tok)
	}
}

func TestOAuthFlow_Denied(t *testing.T) {
	fakeGoogle(t)
	m := newTestManager(t)

	authURL, err := m.StartAuth()
	if err != nil {
		t.Fatalf("StartAuth: %v", err)
	}
	host := redirectPort(t, authURL)

	resp, err := http.Get("http://" + host + "/oauth2callback?error=access_denied")
	if err != nil {
		t.Fatalf("callback GET: %v", err)
	}
	resp.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.AwaitAuth(ctx); err == nil {
		t.Fatal("AwaitAuth returned nil after a denied authorization")
	}
	if m.Connected() {
		t.Error("Connected() = true after a denied authorization")
	}
}

func TestTokenSource_NotConnected(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.TokenSource(context.Background()); err != ErrNotConnected {
		t.Errorf("TokenSource err = %v, want ErrNotConnected", err)
	}
}

func TestAccountInfo(t *testing.T) {
	m := newTestManager(t)
	m.token = &oauth2.Token{AccessToken: "at", Expiry: time.Now().Add(time.Hour)}

	ui := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"Ada Lovelace","email":"ada@example.com"}`))
	}))
	defer ui.Close()
	prev := userinfoEndpoint
	userinfoEndpoint = ui.URL
	defer func() { userinfoEndpoint = prev }()

	name, email, err := m.AccountInfo(context.Background())
	if err != nil {
		t.Fatalf("AccountInfo: %v", err)
	}
	if name != "Ada Lovelace" || email != "ada@example.com" {
		t.Errorf("AccountInfo = %q / %q", name, email)
	}
}

func TestDisconnect_RevokesAndClears(t *testing.T) {
	m := newTestManager(t)
	m.token = &oauth2.Token{AccessToken: "at", RefreshToken: "rt-to-revoke"}
	if err := m.tok.save(m.token); err != nil {
		t.Fatal(err)
	}

	var revoked atomic.Value
	rk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		revoked.Store(r.URL.Query().Get("token"))
	}))
	defer rk.Close()
	prev := revokeEndpoint
	revokeEndpoint = rk.URL
	defer func() { revokeEndpoint = prev }()

	if err := m.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if m.Connected() {
		t.Error("Connected() = true after Disconnect")
	}
	if got, _ := revoked.Load().(string); got != "rt-to-revoke" {
		t.Errorf("revoked token = %q, want rt-to-revoke", got)
	}
	if tok, ok := m.tok.load(); ok || tok != nil {
		t.Error("token file survived Disconnect")
	}
}
