// SPDX-License-Identifier: AGPL-3.0-or-later

// Package claudesub connects a user's Claude subscription (Pro or Max) so
// Memo can run Claude on their existing plan quota instead of a pay-per-token
// API key. It authenticates with Anthropic's own public Claude Code OAuth
// client — the same client_id claude-code-proxy and Anthropic's own SDKs use
// — so there is nothing to register.
//
// What this package owns: the OAuth flow, a refreshable token persisted
// encrypted at rest, automatic adoption of a Claude Code login that already
// exists on the machine, the live model list, and the provider.Provider that
// plugs into Memo's router.
//
// What it deliberately does NOT own: the Anthropic wire format. The
// subscription endpoint IS api.anthropic.com's Messages API, so this package
// asks internal/provider for a claude.go-derived provider configured through
// provider.ClaudeOverrides (see claude_overrides.go) rather than copying
// 950 lines of translation. internal/geminisub is the opposite case — Google's
// Code Assist wraps the payload in its own envelope — and its copy is correct.
//
// Same three integration seams as geminisub: a ProviderType constant in
// internal/provider, a RegisterConstructor call from this package's init(),
// and a blank import in internal/app.
package claudesub

import (
	"errors"
	"fmt"
	"sync"

	"memo/internal/logx"

	"golang.org/x/oauth2"
)

// ErrNotConnected is returned by every Manager method that needs a token when
// no Claude account has been connected yet. The provider.Provider built for
// ProviderClaudeSub surfaces this verbatim on every call rather than failing
// construction, so a disconnected account never breaks router setup or the
// provider subsystem — the same contract internal/geminisub settled on.
var ErrNotConnected = errors.New("no Claude subscription connected — connect it in Settings › Claude Subscription")

// Manager owns the Claude account connection: the OAuth flow, the encrypted
// token at rest, refresh, revoke, and account identity.
//
// Use Default() for the process-wide instance. internal/app (the
// connect/disconnect surface) and the provider constructor (per-call token)
// resolve to the same Manager so they share one view of the connection, which
// is what lets the user disconnect and have the very next turn say
// ErrNotConnected without rebuilding a provider.
type Manager struct {
	mu    sync.Mutex
	tok   *tokenStore
	token *oauth2.Token // nil == not connected
	// source records how the current token was obtained — one of the
	// AdoptResult.Source values, or "browser" for our own OAuth flow. It is
	// NOT cosmetic: the connect surface reports it, and a user who is
	// connected because we adopted their Claude Code login needs to be told
	// that, not shown a "signed in through the browser" story that never
	// happened. Empty only before any token exists.
	source string
	flow   *authFlow // in-progress OAuth flow, if any

	// refreshMu serializes the actual "refresh the access token with Anthropic"
	// step across every persistingTokenSource — see TokenSource's doc comment.
	// A separate lock from mu so a slow refresh call doesn't block unrelated
	// Manager methods (Connected, Disconnect, ...) that only need the current
	// token snapshot.
	refreshMu sync.Mutex

	models modelsCache // cached live model list (see models.go)
	probe  probeCache  // last capability measurement (see probe.go)
}

var (
	defaultMu  sync.Mutex
	defaultMgr *Manager
)

// Default returns the process-wide Manager, creating it on first use. It reads
// any previously stored token from config.DataDir()/claudesub/, and otherwise
// adopts a Claude Code login already present on this machine (see adopt.go)
// so the feature is usable with no extra click.
func Default() *Manager {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultMgr == nil {
		m := newManager(newTokenStore())
		// Nothing of our own yet: fall back to a login the official Claude Code
		// CLI already has. Same OAuth client, so our refresh works against that
		// refresh token exactly as it would one of our own. We do not write back
		// to Claude Code's file on adopt — only when a refresh rotates the token
		// (adopt.go's writeBackRefreshed).
		if res := AdoptLocal(); res.Token != nil {
			m.token, m.source = res.Token, res.Source
			logx.Printf("claudesub: adopted an existing Claude Code login from %s", res.Source)
		}
		defaultMgr = m
	}
	return defaultMgr
}

// newManager loads the stored token only. Deliberately does NOT adopt a local
// login: adoption is a process-startup decision that reads the user's home
// directory, and tests must not silently pick up whatever Claude Code session
// the developer happens to have.
func newManager(ts *tokenStore) *Manager {
	m := &Manager{tok: ts}
	if t, ok := ts.load(); ok {
		m.token = t
		// A token we persisted earlier came from our own OAuth flow; adoption
		// always writes through Adopt, which sets the real source.
		m.source = "browser"
	}
	return m
}

// ResetForTests drops the process-wide Manager so the next Default() rebuilds
// it. Exists for the same reason config.ResetForTests does: Default() caches a
// singleton, so one test's adopted token would otherwise leak into the next
// test and make an unrelated assertion pass or fail for the wrong reason.
// Not a production API — nothing outside tests should call it.
func ResetForTests() {
	defaultMu.Lock()
	defaultMgr = nil
	defaultMu.Unlock()
}

// Source reports how the current token was obtained: an AdoptResult.Source
// value ("env", "claude-code-file", "macos-keychain") or "browser" for our own
// OAuth flow. Empty when nothing is connected.
func (m *Manager) Source() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.source
}

// Connected reports whether a token is present. It does not verify the token
// is still accepted by Anthropic — that happens lazily on first use via the
// refreshing TokenSource.
func (m *Manager) Connected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.token != nil
}

// Adopt installs a token that came from somewhere other than our own OAuth
// flow — a Claude Code login found on this machine, or a setup token the user
// pasted in. It replaces whatever was there and persists encrypted, so the
// refresh path works exactly as it does for a token we obtained ourselves.
//
// source is recorded so the UI can report what actually happened; it is
// informational only.
func (m *Manager) Adopt(res AdoptResult) error {
	if res.Token == nil || res.Token.AccessToken == "" {
		return fmt.Errorf("claudesub: nothing to adopt")
	}
	m.mu.Lock()
	m.token = res.Token
	m.source = res.Source
	m.mu.Unlock()
	if err := m.tok.save(res.Token); err != nil {
		// Not fatal: the token works for this session either way, and losing
		// it on restart is a re-adoption the user can trigger by clicking
		// Connect again.
		logx.Printf("claudesub: persist adopted token: %v", err)
	}
	m.invalidateModels()
	m.invalidateProbe()
	return nil
}
