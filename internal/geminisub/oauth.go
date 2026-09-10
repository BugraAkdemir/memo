// SPDX-License-Identifier: AGPL-3.0-or-later

package geminisub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"memo/internal/logx"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Built-in OAuth client — Google "Desktop app" type. A desktop client secret
// is not confidential (it ships in gemini-cli's public source the same way);
// PKCE is what actually protects the loopback exchange. Override both with
// the env vars below.
//
// TODO(setup): replace these placeholders with the real values from the Memo
// Google Cloud project — Code Assist API enabled, OAuth consent screen
// configured, a "Desktop app" OAuth client created. Until then the connect
// flow will fail at Google's consent screen.
const (
	builtinClientID     = "REPLACE_ME.apps.googleusercontent.com"
	builtinClientSecret = "REPLACE_ME"

	envClientID     = "MEMO_GOOGLE_GEMINI_CLIENT_ID"
	envClientSecret = "MEMO_GOOGLE_GEMINI_CLIENT_SECRET"
)

// oauthScopes: cloud-platform is what the Code Assist endpoint requires; the
// openid/userinfo scopes give the account name+email shown in the UI.
var oauthScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"openid",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
}

// Package-level so tests can point them at httptest servers. Not consts.
var (
	googleAuthEndpoint = google.Endpoint
	userinfoEndpoint   = "https://www.googleapis.com/oauth2/v2/userinfo"
	revokeEndpoint     = "https://oauth2.googleapis.com/revoke"
)

func clientID() string {
	if v := strings.TrimSpace(os.Getenv(envClientID)); v != "" {
		return v
	}
	return builtinClientID
}

func clientSecret() string {
	if v := strings.TrimSpace(os.Getenv(envClientSecret)); v != "" {
		return v
	}
	return builtinClientSecret
}

func oauthConfig(redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID(),
		ClientSecret: clientSecret(),
		Scopes:       oauthScopes,
		Endpoint:     googleAuthEndpoint,
		RedirectURL:  redirectURL,
	}
}

// authFlow tracks one in-progress loopback OAuth flow.
type authFlow struct {
	verifier string
	srv      *http.Server
	wg       sync.WaitGroup
	done     bool
	err      error
}

// StartAuth starts a loopback HTTP server on 127.0.0.1 and returns the URL
// the user must open in a browser. Call AwaitAuth to block until the flow
// finishes. A second call cancels any previous in-flight flow.
func (m *Manager) StartAuth() (string, error) {
	m.mu.Lock()
	if m.flow != nil && m.flow.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = m.flow.srv.Shutdown(ctx)
		cancel()
	}
	f := &authFlow{verifier: oauth2.GenerateVerifier()}
	f.wg.Add(1)
	m.flow = f
	m.mu.Unlock()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("geminisub: oauth listener: %w", err)
	}
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		ln.Close()
		return "", fmt.Errorf("geminisub: oauth listener is not TCP")
	}
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/callback", tcpAddr.Port)
	cfg := oauthConfig(redirectURL)
	authURL := cfg.AuthCodeURL("memo-gemini-sub",
		oauth2.AccessTypeOffline, oauth2.ApprovalForce,
		oauth2.S256ChallengeOption(f.verifier))

	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux}
	m.mu.Lock()
	f.srv = srv
	m.mu.Unlock()

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if e := r.URL.Query().Get("error"); e != "" {
			http.Error(w, "authorization denied", http.StatusBadRequest)
			m.finishFlow(f, fmt.Errorf("geminisub: authorization denied: %s", e))
			shutdown(srv)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, successHTML)

		// Exchange in the background so the HTTP response can flush first.
		go func() {
			defer logx.Recover("geminisub/oauth exchange")
			defer shutdown(srv)

			t, err := cfg.Exchange(context.Background(), code, oauth2.VerifierOption(f.verifier))
			if err != nil {
				m.finishFlow(f, fmt.Errorf("geminisub: token exchange: %w", err))
				return
			}
			m.mu.Lock()
			m.token = t
			m.mu.Unlock()
			if err := m.tok.save(t); err != nil {
				logx.Printf("geminisub: save token after exchange: %v", err)
			}
			m.finishFlow(f, nil)
		}()
	})

	go func() {
		defer logx.Recover("geminisub/auth server")
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			logx.Printf("geminisub: auth server: %v", err)
		}
	}()

	return authURL, nil
}

func shutdown(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func (m *Manager) finishFlow(f *authFlow, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if f.done {
		return
	}
	f.done = true
	f.err = err
	f.wg.Done()
}

// AwaitAuth blocks until the in-progress OAuth flow completes, returning its
// outcome, or until ctx is cancelled.
func (m *Manager) AwaitAuth(ctx context.Context) error {
	m.mu.Lock()
	f := m.flow
	m.mu.Unlock()
	if f == nil {
		return fmt.Errorf("geminisub: no auth flow in progress")
	}

	done := make(chan struct{}, 1)
	go func() {
		defer logx.Recover("geminisub.AwaitAuth")
		f.wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-done:
		m.mu.Lock()
		err := f.err
		m.mu.Unlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TokenSource returns an oauth2.TokenSource that transparently refreshes the
// stored token and re-persists it (encrypted) whenever it changes. Returns
// ErrNotConnected when no account is connected.
func (m *Manager) TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	m.mu.Lock()
	tok := m.token
	m.mu.Unlock()
	if tok == nil {
		return nil, ErrNotConnected
	}
	return &persistingTokenSource{
		m:    m,
		src:  oauthConfig("").TokenSource(ctx, tok),
		last: tok,
	}, nil
}

type persistingTokenSource struct {
	m    *Manager
	src  oauth2.TokenSource
	mu   sync.Mutex
	last *oauth2.Token
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	t, err := p.src.Token()
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	changed := p.last == nil ||
		t.AccessToken != p.last.AccessToken ||
		t.RefreshToken != p.last.RefreshToken ||
		!t.Expiry.Equal(p.last.Expiry)
	p.last = t
	p.mu.Unlock()

	if changed {
		p.m.mu.Lock()
		p.m.token = t
		p.m.mu.Unlock()
		if err := p.m.tok.save(t); err != nil {
			logx.Printf("geminisub: persist refreshed token: %v", err)
		}
	}
	return t, nil
}

// AccountInfo returns the connected Google account's display name and email.
func (m *Manager) AccountInfo(ctx context.Context) (name, email string, err error) {
	ts, err := m.TokenSource(ctx)
	if err != nil {
		return "", "", err
	}
	client := oauth2.NewClient(ctx, ts)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userinfoEndpoint, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", "", fmt.Errorf("geminisub: userinfo status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var u struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return "", "", err
	}
	return u.Name, u.Email, nil
}

// Disconnect revokes the token with Google (best-effort) and deletes it from
// disk. It always clears local state even if the revoke call fails.
func (m *Manager) Disconnect(ctx context.Context) error {
	m.mu.Lock()
	tok := m.token
	m.token = nil
	m.mu.Unlock()

	if tok != nil {
		revokeToken(ctx, tok)
	}
	m.invalidateBootstrap()
	return m.tok.clear()
}

func revokeToken(ctx context.Context, tok *oauth2.Token) {
	val := tok.RefreshToken
	if val == "" {
		val = tok.AccessToken
	}
	if val == "" {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		revokeEndpoint+"?token="+url.QueryEscape(val), nil)
	if err != nil {
		logx.Printf("geminisub: build revoke request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logx.Printf("geminisub: revoke call: %v", err)
		return
	}
	_ = resp.Body.Close()
}

const successHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Memo — Connected</title>
<style>
  *{margin:0;padding:0;box-sizing:border-box}
  body{min-height:100vh;display:flex;align-items:center;justify-content:center;background:#0a0a0a;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;color:#e5e5e5}
  .card{background:#141414;border:1px solid #2a2a2a;border-radius:16px;padding:48px 40px;max-width:400px;width:90%;text-align:center;box-shadow:0 24px 64px rgba(0,0,0,.6)}
  .icon{width:64px;height:64px;background:linear-gradient(135deg,#a78bfa,#7c3aed);border-radius:50%;display:flex;align-items:center;justify-content:center;margin:0 auto 24px}
  .icon svg{width:32px;height:32px;stroke:#fff;fill:none;stroke-width:2.5;stroke-linecap:round;stroke-linejoin:round}
  h1{font-size:20px;font-weight:600;color:#f5f5f5;margin-bottom:8px}
  p{font-size:14px;color:#888;line-height:1.6}
  .badge{display:inline-block;margin-top:24px;padding:6px 16px;background:#1e1e1e;border:1px solid #333;border-radius:999px;font-size:12px;color:#666;letter-spacing:.04em}
</style>
</head>
<body>
<div class="card">
  <div class="icon">
    <svg viewBox="0 0 24 24"><polyline points="20 6 9 17 4 12"/></svg>
  </div>
  <h1>Google Account Connected</h1>
  <p>Authorization complete. You can close this tab and return to Memo.</p>
  <span class="badge">MEMO · LOCAL AI</span>
</div>
</body>
</html>`
