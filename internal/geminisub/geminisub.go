// SPDX-License-Identifier: AGPL-3.0-or-later

// Package geminisub connects a user's personal Google account (native OAuth,
// no CLI dependency) and uses it to reach Gemini on their Google AI Pro/Ultra
// subscription quota through Google's Code Assist endpoint.
//
// It is deliberately a separate package, not part of internal/provider or
// internal/agentcli:
//
//   - Unlike internal/provider's stateless HTTP providers, this one owns an
//     OAuth flow, a refresh token persisted (encrypted) to disk, and a
//     Code Assist project/tier bootstrap handshake.
//   - Unlike internal/agentcli, there is no subprocess — it is a plain HTTPS
//     client, just with a Bearer token instead of an API key.
//
// The whole feature lives under this one directory so it can be debugged (or
// deleted) without touching shared code. It is wired into the rest of Memo
// through exactly three seams: a ProviderType constant in internal/provider,
// a RegisterConstructor call from this package's init(), and a blank import
// in internal/app. See the plan in .claude/plans/ and AGENTS.md.
//
// Phase 1 (this commit) is the OAuth module only: the loopback flow, the
// encrypted token store, refresh, revoke, and account identity. The Code
// Assist bootstrap and the provider.Provider implementation land in later
// phases.
package geminisub

import (
	"errors"
	"sync"

	"golang.org/x/oauth2"
)

// ErrNotConnected is returned by every Manager method that needs a token when
// no Google account has been connected yet. The provider.Provider built for
// ProviderGeminiSub surfaces this verbatim on every call rather than failing
// construction, so a disconnected account never breaks router setup.
var ErrNotConnected = errors.New("no Google account connected — connect it in Settings › Developer")

// Manager owns the Google account connection: the OAuth flow, the encrypted
// token at rest, token refresh, revoke, account identity, and (later phases)
// the Code Assist project/tier bootstrap.
//
// Use Default() for the process-wide instance. Both internal/app (the
// connect/disconnect surface) and the provider constructor (per-call token)
// resolve to the same Manager so they share one view of the connection.
type Manager struct {
	mu    sync.Mutex
	tok   *tokenStore
	token *oauth2.Token // nil == not connected
	flow  *authFlow     // in-progress OAuth flow, if any
}

var (
	defaultMu  sync.Mutex
	defaultMgr *Manager
)

// Default returns the process-wide Manager, creating it on first use. It
// reads any previously stored token from config.DataDir()/geminisub/.
func Default() *Manager {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultMgr == nil {
		defaultMgr = newManager(newTokenStore())
	}
	return defaultMgr
}

func newManager(ts *tokenStore) *Manager {
	m := &Manager{tok: ts}
	if t, ok := ts.load(); ok {
		m.token = t
	}
	return m
}

// Connected reports whether a token is present. It does not verify the token
// is still valid with Google — that happens lazily on first use via the
// refreshing TokenSource.
func (m *Manager) Connected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.token != nil
}
