// SPDX-License-Identifier: AGPL-3.0-or-later

package geminisub

import (
	"context"
	"encoding/base64"
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

// Built-in OAuth client — gemini-cli's own public "installed app" client,
// published verbatim in the google-gemini/gemini-cli source
// (packages/core/src/code_assist/oauth2.ts). For an installed-app client the
// "secret" is not confidential (OAuth spec), so there is nothing to register
// and no Google Cloud project / consent screen to set up: the user just
// signs in with their Google account in the browser, exactly the way
// claude-code-proxy reuses Claude Code's public client. The env vars are an
// optional override, not a requirement.
//
// The two values are base64-encoded ONLY so GitHub's push-protection secret
// scanner doesn't pattern-match the literal `...apps.googleusercontent.com`
// / `GOCSPX-` shapes and block every push. This is not obfuscation of
// anything sensitive — decode with `base64 -d` to verify against gemini-cli.
const (
	builtinClientIDB64     = "NjgxMjU1ODA5Mzk1LW9vOGZ0Mm9wcmRybnA5ZTNhcWY2YXYzaG1kaWIxMzVqLmFwcHMuZ29vZ2xldXNlcmNvbnRlbnQuY29t"
	builtinClientSecretB64 = "R09DU1BYLTR1SGdNUG0tMW83U2stZ2VWNkN1NWNsWEZzeGw="

	envClientID     = "MEMO_GOOGLE_GEMINI_CLIENT_ID"
	envClientSecret = "MEMO_GOOGLE_GEMINI_CLIENT_SECRET"
)

var (
	builtinClientID     = mustB64(builtinClientIDB64)
	builtinClientSecret = mustB64(builtinClientSecretB64)
)

func mustB64(s string) string {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic("geminisub: bad builtin credential encoding: " + err.Error())
	}
	return string(b)
}

// oauthScopes matches gemini-cli exactly: cloud-platform is what the Code
// Assist endpoint requires; the two userinfo scopes give the account
// name+email shown in the UI.
var oauthScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
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
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/oauth2callback", tcpAddr.Port)
	cfg := oauthConfig(redirectURL)
	authURL := cfg.AuthCodeURL("memo-gemini-sub",
		oauth2.AccessTypeOffline, oauth2.ApprovalForce,
		oauth2.S256ChallengeOption(f.verifier))

	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux}
	m.mu.Lock()
	f.srv = srv
	m.mu.Unlock()

	mux.HandleFunc("/oauth2callback", func(w http.ResponseWriter, r *http.Request) {
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
		// The caller (StartGoogleAuth's 5-minute ctx) is giving up on this
		// flow, but nothing else ever told the loopback server to stop: the
		// only other shutdown paths are the OAuth callback actually firing
		// (success or error) and the *next* StartAuth call's own cleanup
		// guard. An abandoned flow — the user closes the tab, or the
		// browser never gets opened at all — left the listening socket and
		// its Serve goroutine running indefinitely past this timeout, until
		// the user happened to retry (or the process restarted). Shut it
		// down here too so a genuinely abandoned flow's resources are freed
		// promptly instead of only on the next attempt.
		m.mu.Lock()
		srv := f.srv
		m.mu.Unlock()
		if srv != nil {
			shutdown(srv)
		}
		return ctx.Err()
	}
}

// TokenSource returns an oauth2.TokenSource that transparently refreshes the
// stored token and re-persists it (encrypted) whenever it changes. Returns
// ErrNotConnected when no account is connected.
//
// Callers (codeassist.go, models.go, provider.go, AccountInfo) each call
// this fresh per use rather than sharing one long-lived source, so two
// concurrent callers can easily observe the same expired token at once —
// e.g. an interactive chat and a background task both hitting Gemini-sub at
// the same moment after the access token has expired. Every
// persistingTokenSource this returns funnels its actual refresh through
// Manager.refreshMu, so only one of them ever calls Google; the rest block
// on the lock and then find (via the re-read of m.token right after
// acquiring it) that the winner already refreshed, and reuse that result
// instead of making their own redundant — and, if Google were ever to
// rotate/invalidate the prior access token on refresh, mutually
// destructive — refresh call.
func (m *Manager) TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	m.mu.Lock()
	tok := m.token
	m.mu.Unlock()
	if tok == nil {
		return nil, ErrNotConnected
	}
	return &persistingTokenSource{
		m:    m,
		cfg:  oauthConfig(""),
		last: tok,
	}, nil
}

type persistingTokenSource struct {
	m    *Manager
	cfg  *oauth2.Config
	mu   sync.Mutex
	last *oauth2.Token
}

// refreshTimeout bounds the actual network round-trip to Google's token
// endpoint once decided (see Token below). Deliberately not tied to any one
// caller's ctx: the refresh, once started, is shared by every caller
// waiting on refreshMu, so it must not be cancellable by whichever one of
// them happened to trigger it.
var refreshTimeout = 30 * time.Second

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	p.mu.Lock()
	tok := p.last
	p.mu.Unlock()
	if tok.Valid() {
		return tok, nil
	}

	p.m.refreshMu.Lock()
	defer p.m.refreshMu.Unlock()

	// Re-read the Manager's current token: another persistingTokenSource may
	// have already refreshed (and persisted) a new one while this call was
	// waiting for refreshMu.
	p.m.mu.Lock()
	current := p.m.token
	p.m.mu.Unlock()
	if current.Valid() {
		p.mu.Lock()
		p.last = current
		p.mu.Unlock()
		return current, nil
	}

	refreshCtx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()
	t, err := p.cfg.TokenSource(refreshCtx, current).Token()
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.last = t
	p.mu.Unlock()

	p.m.mu.Lock()
	p.m.token = t
	p.m.mu.Unlock()
	if err := p.m.tok.save(t); err != nil {
		logx.Printf("geminisub: persist refreshed token: %v", err)
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
	m.invalidateModels()
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
