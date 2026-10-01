// SPDX-License-Identifier: AGPL-3.0-or-later

package claudesub

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// randomState returns a CSRF state token for the PKCE flow. 32 bytes, the same
// width claude-code-proxy uses, base64url so it survives a round trip through
// a query string untouched.
func randomState() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing means the OS entropy source is broken; a weak
		// state would be worse than an obvious failure.
		panic("claudesub: no entropy for OAuth state: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// challengeFor is the PKCE S256 challenge for a verifier.
func challengeFor(verifier string) string {
	return oauth2.S256ChallengeFromVerifier(verifier)
}

// modelsCache holds the live model list fetched from Anthropic.
//
// Unlike internal/geminisub's equivalent this is deliberately simple: the
// subscription exposes the same GET /v1/models as the API key path (verified
// against OAuth bearer tokens in anthropics/claude-code#35269 — "the
// GET /v1/models endpoint works fine with the same token"), so there is no
// separate discovery protocol to speak and no bootstrap handshake.
//
// One TTL cache for the whole process. A user opening the model dropdown is
// not a load-bearing event, and the list changes on Anthropic's release
// cadence, not per minute.
type modelsCache struct {
	mu      sync.Mutex
	models  []string
	fetched time.Time
}

// modelsTTL is how long a fetched list is trusted. Deliberately short enough
// that "is there a new model yet" is answered by waiting out a coffee, and
// long enough that redrawing the settings dialog ten times makes one request.
const modelsTTL = 15 * time.Minute

// fallbackModels is used ONLY when the live list cannot be fetched at all
// (offline, DNS failure, a rotated token) AND the account has never been
// listed successfully in this process. It is a last resort so the provider is
// never left with an empty dropdown; the moment GET /v1/models answers, its
// result replaces this wholesale and the live list is what the user sees.
//
// This is not a hardcoded catalogue: nothing here filters or restricts what
// the account may use, and the first successful fetch discards it.
var fallbackModels = []string{"claude-haiku-4-5-20251001"}

// CachedModels returns the live list if one has been fetched this process, with
// no TTL check — this is what ListGatewayModels wants, since it is showing
// models to an external tool on a UI refresh rather than answering "is this
// dropdown stale?". Returns nil when nothing has been fetched yet.
func (m *Manager) CachedModels() []string {
	m.models.mu.Lock()
	defer m.models.mu.Unlock()
	if len(m.models.models) == 0 {
		return nil
	}
	return append([]string(nil), m.models.models...)
}

// cachedModels returns the live list if it has been fetched within modelsTTL.
func (m *Manager) cachedModels() ([]string, bool) {
	m.models.mu.Lock()
	defer m.models.mu.Unlock()
	if len(m.models.models) == 0 || time.Since(m.models.fetched) >= modelsTTL {
		return nil, false
	}
	return append([]string(nil), m.models.models...), true
}

// storeModels caches a freshly fetched list.
func (m *Manager) storeModels(list []string) {
	if len(list) == 0 {
		return
	}
	m.models.mu.Lock()
	m.models.models, m.models.fetched = append([]string(nil), list...), time.Now()
	m.models.mu.Unlock()
}

// invalidateModels drops the cache. Called on connect and disconnect so a
// newly-connected account is never answered from the previous one's list.
func (m *Manager) invalidateModels() {
	m.models.mu.Lock()
	m.models.models, m.models.fetched = nil, time.Time{}
	m.models.mu.Unlock()
}
