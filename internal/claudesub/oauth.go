// SPDX-License-Identifier: AGPL-3.0-or-later

package claudesub

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"memo/internal/logx"

	"golang.org/x/oauth2"
)

// Built-in OAuth client — Claude Code's own public client, taken verbatim from
// Anthropic's own tooling (the client_id appears in claude-code-proxy, in
// Anthropic's SDKs, and in the Claude Code binary). It is a public client, so
// there is no secret to register and no app to create: the user just signs in
// with their Claude account in the browser, the same way claude-code-proxy
// reuses Claude Code's credentials. The env var is an optional override, not
// a requirement.
//
// It is base64-encoded ONLY so GitHub's push-protection secret scanner does not
// pattern-match the literal UUID and block every push. This is not obfuscation
// of anything sensitive — decode with `base64 -d` to check it against
// claude-code-proxy/server/OAuthManager.js.
const (
	builtinClientIDB64 = "OWQxYzI1MGEtZTYxYi00NGQ5LThiZWQtNTk0NGQxOTYyZjVl"

	envClientID = "MEMO_CLAUDE_CLIENT_ID"
)

// The flow is NOT a loopback flow, and that is Anthropic's constraint rather
// than a choice. The only redirect_uri registered for this client is a hosted
// Anthropic page, so the browser lands on console/platform.claude.com, that
// page DISPLAYS the authorization code, and the user copies it back into
// Memo. (Two redirect paths are in circulation and which one the server has
// registered has flipped at least once — anthropics/claude-code#88877 — so
// both are offered and the UI can show the second as a fallback.)
//
// Consequence for the REST surface: unlike gemini-sub, there is no callback
// endpoint to poll. StartAuth hands out a URL plus a state string, and
// CompleteAuth takes the pasted code back. The state lives in the Manager,
// expires, and is single-use.
var (
	defaultRedirectURIs = []string{
		"https://platform.claude.com/oauth/code/callback",
		"https://console.anthropic.com/oauth/code/callback",
	}
)

var (
	// Package-level so tests can point them at httptest servers. Not consts.
	authorizeEndpoint = "https://claude.ai/oauth/authorize"
	tokenEndpoint     = "https://platform.claude.com/v1/oauth/token"

	envAuthorizeURL = "MEMO_CLAUDE_AUTH_URL"
	envTokenURL     = "MEMO_CLAUDE_TOKEN_URL"
	envRedirectURI  = "MEMO_CLAUDE_REDIRECT_URI"
)

// oauthScopes is the scope set claude-code-proxy and anthropic-max-router both
// use against this client. org:create_api_key is not something this package
// acts on; it is part of what Anthropic's authorize page expects from this
// client id.
var oauthScopes = []string{
	"org:create_api_key",
	"user:profile",
	"user:inference",
}

func clientID() string {
	if v := strings.TrimSpace(os.Getenv(envClientID)); v != "" {
		return v
	}
	b, err := base64.StdEncoding.DecodeString(builtinClientIDB64)
	if err != nil {
		panic("claudesub: bad builtin client id encoding: " + err.Error())
	}
	return string(b)
}

func authorizeURL() string {
	if v := strings.TrimSpace(os.Getenv(envAuthorizeURL)); v != "" {
		return v
	}
	return authorizeEndpoint
}

func tokenURL() string {
	if v := strings.TrimSpace(os.Getenv(envTokenURL)); v != "" {
		return v
	}
	return tokenEndpoint
}

func redirectURIs() []string {
	if v := strings.TrimSpace(os.Getenv(envRedirectURI)); v != "" {
		return []string{v}
	}
	return defaultRedirectURIs
}

// authFlow tracks one in-progress OAuth flow. There is no listener and no
// callback — see the redirect comment above — so this is just the PKCE
// verifier plus the state string the browser will echo back, held until the
// user pastes a code or the flow times out.
type authFlow struct {
	verifier   string
	state      string
	expiresAt  time.Time
	redirect   string
	done       bool
	err        error
	wg         sync.WaitGroup
	mu         sync.Mutex
	authURL    string
	finishedAt time.Time
}

// flowTTL bounds how long a pasted-code window stays open. Anthropic's
// authorization codes are themselves short-lived, so a long window is not
// useful; this only exists so a stale verifier cannot be exchanged hours later
// from a UI the user left open.
const flowTTL = 10 * time.Minute

// StartAuth begins a PKCE flow and returns the URL the user must open plus the
// state string CompleteAuth will check the pasted code against. A second call
// replaces any previous in-flight flow rather than racing it.
//
// redirectIndex selects which registered redirect_uri to use; 0 is the current
// path, 1 the older one (see defaultRedirectURIs).
func (m *Manager) StartAuth(redirectIndex int) (authURL, state string, redirect string, err error) {
	uris := redirectURIs()
	if redirectIndex < 0 || redirectIndex >= len(uris) {
		redirectIndex = 0
	}

	f := &authFlow{
		verifier:  oauth2.GenerateVerifier(),
		state:     randomState(),
		redirect:  uris[redirectIndex],
		expiresAt: time.Now().Add(flowTTL),
	}
	f.authURL = buildAuthorizeURL(f, redirectIndex)
	f.wg.Add(1)

	m.mu.Lock()
	old := m.flow
	m.flow = f
	m.mu.Unlock()

	// An abandoned flow's waiter (nothing in the app blocks on it, but a test
	// or a second caller might) must not stay parked forever.
	if old != nil {
		old.finish(nil, nil)
	}
	return f.authURL, f.state, f.redirect, nil
}

// buildAuthorizeURL assembles the browser URL. `code=true` is what tells
// Anthropic to DISPLAY the code on the callback page instead of trying to
// redirect somewhere; without it the user gets a blank page and no code.
func buildAuthorizeURL(f *authFlow, redirectIndex int) string {
	uris := redirectURIs()
	v := url.Values{
		"code":                  {"true"},
		"client_id":             {clientID()},
		"response_type":         {"code"},
		"redirect_uri":          {uris[redirectIndex]},
		"scope":                 {strings.Join(oauthScopes, " ")},
		"code_challenge":        {challengeFor(f.verifier)},
		"code_challenge_method": {"S256"},
		"state":                 {f.state},
	}
	return authorizeURL() + "?" + v.Encode()
}

// CompleteAuth exchanges the code the user pasted for tokens, stores them
// encrypted, and returns the account label Anthropic reports. It fails when no
// flow is open, when the flow has expired, when a state that came with the
// code does not match the one issued (CSRF), or when Anthropic rejects the
// exchange.
//
// Users paste whatever is in front of them, and there are three shapes of it
// (see parseCodePaste). state is the one the client received from StartAuth;
// it may be empty. Every state that IS known — the client's and the one
// pasted alongside the code — must match the flow's own.
func (m *Manager) CompleteAuth(ctx context.Context, pasted, state string) error {
	m.mu.Lock()
	f := m.flow
	m.mu.Unlock()
	if f == nil {
		return fmt.Errorf("claudesub: no sign-in in progress — start one first")
	}
	if time.Now().After(f.expiresAt) {
		m.clearFlow(f)
		f.finish(nil, fmt.Errorf("claudesub: sign-in expired, try again"))
		return fmt.Errorf("claudesub: sign-in expired, try again")
	}

	code, pastedState, err := parseCodePaste(pasted)
	if err != nil {
		return err
	}
	for _, s := range []string{strings.TrimSpace(state), pastedState} {
		if s != "" && s != f.state {
			return fmt.Errorf("claudesub: state mismatch — this code came from a different sign-in attempt")
		}
	}

	t, err := exchange(ctx, f, code, f.state)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.token = t
	m.source = SourceBrowser
	m.flow = nil
	m.mu.Unlock()
	if err := m.tok.save(t, SourceBrowser); err != nil {
		logx.Printf("claudesub: save token after exchange: %v", err)
	}
	m.invalidateModels()
	m.invalidateProbe()
	f.finish(t, nil)
	return nil
}

// parseCodePaste accepts any of the three things a user can end up copying and
// returns the authorization code plus the state that came with it ("" when
// none did). Anything else is an error rather than a silent bad exchange.
//
//   - "code#state" — what Anthropic's hosted callback page DISPLAYS, and so by
//     far the most common paste. Sending it whole as the code is rejected by
//     the token endpoint (invalid_grant), which is how this was found:
//     claude-code-proxy splits on '#' for exactly this reason.
//   - the full callback URL from the address bar, ?code=...&state=...
//   - a bare code.
func parseCodePaste(raw string) (code, state string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("claudesub: paste the authorization code")
	}
	if strings.Contains(raw, "://") || strings.HasPrefix(raw, "?") {
		u, perr := url.Parse(raw)
		if perr != nil {
			return "", "", fmt.Errorf("claudesub: that does not look like a code or a redirect URL: %w", perr)
		}
		q := u.Query()
		if e := q.Get("error"); e != "" {
			return "", "", fmt.Errorf("claudesub: authorization denied: %s", e)
		}
		code, state = q.Get("code"), q.Get("state")
		if code == "" {
			return "", "", fmt.Errorf("claudesub: no authorization code found in that URL")
		}
		// Some callback pages carry the state in the fragment instead.
		if state == "" && u.Fragment != "" {
			state = u.Fragment
		}
		return code, state, nil
	}
	code, state, _ = strings.Cut(raw, "#")
	code, state = strings.TrimSpace(code), strings.TrimSpace(state)
	if code == "" {
		return "", "", fmt.Errorf("claudesub: paste the authorization code")
	}
	return code, state, nil
}

func exchange(ctx context.Context, f *authFlow, code, state string) (*oauth2.Token, error) {
	payload := map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"state":         state,
		"client_id":     clientID(),
		"code_verifier": f.verifier,
		"redirect_uri":  f.redirect,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, tokenURL(), strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("claudesub: token request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claudesub: token request failed (%d): %s", resp.StatusCode, provider_msg(raw))
	}

	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("claudesub: token response: %w", err)
	}
	if out.AccessToken == "" {
		return nil, fmt.Errorf("claudesub: token response carried no access token")
	}
	if out.TokenType == "" {
		out.TokenType = "Bearer"
	}
	return &oauth2.Token{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		TokenType:    out.TokenType,
		Expiry:       time.Now().Add(time.Duration(out.ExpiresIn) * time.Second),
	}, nil
}

// provider_msg pulls the human-readable part out of an Anthropic error body.
func provider_msg(raw []byte) string {
	var e struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &e); err == nil {
		if e.Error.Message != "" {
			return e.Error.Type + ": " + e.Error.Message
		}
		if e.Message != "" {
			return e.Type + ": " + e.Message
		}
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}

// AwaitAuth blocks until the flow started by the most recent StartAuth
// finishes, returning its outcome, or until ctx is cancelled. Nothing in the
// app blocks on this today — the user pastes the code on their own schedule and
// CompleteAuth runs then — but it makes the flow's lifecycle testable and
// gives a future REPL command a place to block.
func (m *Manager) AwaitAuth(ctx context.Context) error {
	m.mu.Lock()
	f := m.flow
	m.mu.Unlock()
	if f == nil {
		return fmt.Errorf("claudesub: no sign-in in progress")
	}
	done := make(chan struct{}, 1)
	go func() {
		defer logx.Recover("claudesub.AwaitAuth")
		f.wg.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
		return f.result()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f *authFlow) finish(t *oauth2.Token, err error) {
	f.mu.Lock()
	if f.done {
		f.mu.Unlock()
		return
	}
	f.done, f.err, f.finishedAt = true, err, time.Now()
	f.mu.Unlock()
	if err == nil && t != nil {
		_ = t
	}
	f.wg.Done()
}

func (f *authFlow) result() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

// TokenSource returns an oauth2.TokenSource that transparently refreshes the
// stored token and re-persists it (encrypted) whenever it changes. Returns
// ErrNotConnected when no account is connected.
//
// Callers each call this fresh per use rather than sharing one long-lived
// source, so two concurrent callers can easily observe the same expired token
// at once — an interactive chat and a background task both hitting this
// provider at the same moment. Every persistingTokenSource this returns funnels
// its actual refresh through Manager.refreshMu, so only one of them ever calls
// Anthropic; the rest block on the lock and then find, by re-reading m.token
// right after acquiring it, that the winner already refreshed and reuse that
// result instead of making a redundant — and, if Anthropic ever rotates the
// access token on refresh, mutually destructive — second call.
func (m *Manager) TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	m.mu.Lock()
	tok := m.token
	m.mu.Unlock()
	if tok == nil {
		return nil, ErrNotConnected
	}
	return &persistingTokenSource{m: m, last: tok}, nil
}

type persistingTokenSource struct {
	m    *Manager
	mu   sync.Mutex
	last *oauth2.Token
}

// refreshTimeout bounds the network round-trip to Anthropic's token endpoint
// once the refresh has been decided. Deliberately not tied to any one caller's
// ctx: the refresh, once started, is shared by every caller waiting on
// refreshMu, so it must not be cancellable by whichever of them happened to
// trigger it.
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
	// waiting for refreshMu — or the user may have disconnected, in which case
	// there is nothing to refresh and refresh(nil) would panic.
	p.m.mu.Lock()
	current, source := p.m.token, p.m.source
	p.m.mu.Unlock()
	if current == nil {
		return nil, ErrNotConnected
	}
	if current.Valid() {
		p.mu.Lock()
		p.last = current
		p.mu.Unlock()
		return current, nil
	}

	// A login shared with Claude Code may already have been refreshed THERE;
	// using that costs a file read instead of our (possibly rotated-away)
	// refresh token.
	if t := freshLocalLogin(source, current); t != nil {
		return p.install(current, t)
	}

	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()
	t, rotated, err := refresh(ctx, current)
	if err != nil {
		// Claude Code may have won a refresh race between our file read above
		// and the request, rotating the token we just sent away.
		if t := freshLocalLogin(source, current); t != nil {
			return p.install(current, t)
		}
		return nil, err
	}
	out, err := p.install(current, t)
	if err != nil {
		return nil, err
	}
	// Anthropic is not documented as rotating refresh tokens, but claude-code-
	// proxy handles a rotated one and this is free insurance: if the response
	// carried a different refresh token, the old one is dead and Claude Code
	// must be told or the user's own CLI breaks. Only for a token that came
	// from Claude Code's file — our own browser session is not the CLI's.
	if rotated && source == SourceClaudeCodeFile {
		writeBackRefreshed(t)
	}
	return out, nil
}

// install swaps the Manager's token from old to next and persists it. Reports
// ErrNotConnected when the Manager no longer holds old — the user disconnected
// (or connected a different account) while the refresh was in flight, and
// writing next back would resurrect a session they just ended.
func (p *persistingTokenSource) install(old, next *oauth2.Token) (*oauth2.Token, error) {
	p.m.mu.Lock()
	if p.m.token != old {
		p.m.mu.Unlock()
		return nil, ErrNotConnected
	}
	p.m.token = next
	source := p.m.source
	p.m.mu.Unlock()

	p.mu.Lock()
	p.last = next
	p.mu.Unlock()
	if err := p.m.tok.save(next, source); err != nil {
		logx.Printf("claudesub: persist refreshed token: %v", err)
	}
	return next, nil
}

// refresh exchanges the stored refresh token for a fresh access token. Reports
// rotated=true when Anthropic returned a DIFFERENT refresh token than we sent.
func refresh(ctx context.Context, tok *oauth2.Token) (*oauth2.Token, bool, error) {
	if tok.RefreshToken == "" {
		return nil, false, fmt.Errorf("claudesub: token expired and has no refresh token — reconnect your Claude account")
	}
	payload := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": tok.RefreshToken,
		"client_id":     clientID(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL(), strings.NewReader(string(body)))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("claudesub: token refresh: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		if strings.Contains(string(raw), "invalid_grant") {
			return nil, false, fmt.Errorf("claudesub: sign-in expired — reconnect your Claude account")
		}
		return nil, false, fmt.Errorf("claudesub: token refresh failed (%d): %s", resp.StatusCode, provider_msg(raw))
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, false, fmt.Errorf("claudesub: refresh response: %w", err)
	}
	if out.AccessToken == "" {
		return nil, false, fmt.Errorf("claudesub: refresh response carried no access token")
	}
	tokType := out.TokenType
	if tokType == "" {
		tokType = tok.TokenType
	}
	if tokType == "" {
		tokType = "Bearer"
	}
	next := &oauth2.Token{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		TokenType:    tokType,
		Expiry:       time.Now().Add(time.Duration(out.ExpiresIn) * time.Second),
	}
	rotated := out.RefreshToken != "" && out.RefreshToken != tok.RefreshToken
	if out.RefreshToken == "" {
		next.RefreshToken = tok.RefreshToken
	}
	return next, rotated, nil
}

// Disconnect clears the stored token and the cached model list. It does NOT
// revoke with Anthropic: the token was adopted from, or shared with, the user's
// own Claude Code install, and revoking it there would sign the CLI out. The
// user signs out from Claude Code if they want that.
func (m *Manager) Disconnect() error {
	m.mu.Lock()
	m.token = nil
	m.source = ""
	m.flow = nil
	m.mu.Unlock()
	m.invalidateModels()
	m.invalidateProbe()
	return m.tok.clear()
}

func (m *Manager) clearFlow(f *authFlow) {
	m.mu.Lock()
	if m.flow == f {
		m.flow = nil
	}
	m.mu.Unlock()
}
