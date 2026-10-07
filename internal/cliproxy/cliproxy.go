// SPDX-License-Identifier: AGPL-3.0-or-later

// Package cliproxy runs CLIProxyAPI (github.com/router-for-me/CLIProxyAPI, MIT)
// as a local sidecar so Memo can use an Antigravity / Claude / Codex login —
// the same account and quota the vendors' own CLIs use — without Memo itself
// carrying any OAuth client.
//
// CLIProxyAPI owns the hard, fast-moving part: the vendors' OAuth clients, the
// request shapes and the client-version fingerprints they check. Memo owns only
// the lifecycle:
//
//   - Binary   finds the bundled executable and verifies it against the SHA-256
//     shipped beside it. Memo NEVER downloads it at run time (the project's
//     zero-external-dependency promise): release builds pull it from R2 the way
//     they pull llama.cpp, and scripts/vendor_cliproxy.sh is the one place the
//     network is touched, pinned by internal/cliproxy/PINNED.txt.
//   - Start    writes a loopback-only config with a random API key and runs it.
//   - Login    runs the binary's own `-<provider>-login`, hands the vendor URL
//     to the UI, and reports when the credential file appears.
//   - Models   reads the sidecar's OpenAI-style /v1/models, which for a signed-in
//     account is that account's real, current model list.
//
// From the rest of Memo's point of view the sidecar is an ordinary
// OpenAI-compatible endpoint at BaseURL() with APIKey(), registered as a
// `custom` provider — no new wire format, no new provider type.
//
// Everything mutable lives under one directory (Manager.dir, normally
// config.DataPath("cliproxy")); nothing here touches the user's own
// ~/.cli-proxy-api, ~/.claude or ~/.gemini state.
package cliproxy

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// Provider names accepted by StartLogin / Logout. They are also the `type`
// CLIProxyAPI writes into each credential file.
const (
	ProviderAntigravity = "antigravity"
	ProviderClaude      = "claude"
	ProviderCodex       = "codex"
)

// Providers lists every login the sidecar can perform, in UI order.
var Providers = []string{ProviderAntigravity, ProviderClaude, ProviderCodex}

var (
	// ErrNotBundled means no CLIProxyAPI executable was found next to Memo.
	ErrNotBundled = errors.New("cliproxy: this build does not include the CLIProxyAPI sidecar")
	// ErrUnverified means one was found but could not be checked against its
	// shipped SHA-256 — refused rather than run.
	ErrUnverified = errors.New("cliproxy: bundled CLIProxyAPI failed its checksum")
	// ErrUnknownProvider is returned for a provider outside Providers.
	ErrUnknownProvider = errors.New("cliproxy: unknown provider")
)

// Manager owns the sidecar's files and process. Build one with New.
type Manager struct {
	dir string

	mu       sync.Mutex
	st       state
	proc     *process
	login    *loginSession
	restarts []int64 // unix-second timestamps of recent automatic restarts

	// binary resolution is cached: hashing ~70 MB on every call is wasteful,
	// and the answer only changes if the files do.
	binMu     sync.Mutex
	binCache  *binInfo
	roots     []string // extra roots to search (tests); nil = the defaults
	goos      string   // overridable for tests
	goarch    string
	modelsMu  sync.Mutex
	modelsMem *modelsMemo
	quotaMem  *quotaMemo
}

// state is what must survive a restart: the API key the provider entry carries
// and the port it points at. Both are local-only values; the key authorises
// nothing but this loopback listener.
type state struct {
	APIKey string `json:"api_key"`
	Port   int    `json:"port"`
}

// New returns a Manager rooted at dir. It creates nothing until asked to.
//
// dir is made absolute here: the sidecar is started with its working directory
// set to dir, so a relative dir (Memo's default data dir is just "data") would
// make the config path it is handed resolve a second time inside itself
// ("data/cliproxy/data/cliproxy/config.yaml"), and the login would fail with
// "failed to read config file" before printing any URL.
func New(dir string) *Manager {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	m := &Manager{dir: dir}
	m.loadState()
	return m
}

func (m *Manager) authDir() string    { return filepath.Join(m.dir, "auth") }
func (m *Manager) configPath() string { return filepath.Join(m.dir, "config.yaml") }
func (m *Manager) statePath() string  { return filepath.Join(m.dir, "state.json") }

func (m *Manager) loadState() {
	b, err := os.ReadFile(m.statePath())
	if err != nil {
		return
	}
	var s state
	if json.Unmarshal(b, &s) == nil {
		m.st = s
	}
}

func (m *Manager) saveStateLocked() error {
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(m.st)
	if err != nil {
		return err
	}
	return os.WriteFile(m.statePath(), b, 0o600)
}

// ensureIdentityLocked makes sure there is an API key and a usable loopback
// port, generating and persisting them on first use. A remembered port that
// something else has since taken is replaced — callers learn about the change
// through BaseURL().
func (m *Manager) ensureIdentityLocked() error {
	changed := false
	if m.st.APIKey == "" {
		buf := make([]byte, 24)
		if _, err := rand.Read(buf); err != nil {
			return fmt.Errorf("cliproxy: no entropy for API key: %w", err)
		}
		m.st.APIKey = "memo-" + hex.EncodeToString(buf)
		changed = true
	}
	// Our own running sidecar legitimately holds its port; only a port nothing
	// of ours is using and something else has taken needs replacing.
	running := m.proc != nil && m.proc.alive()
	if m.st.Port == 0 || (!running && !portFree(m.st.Port)) {
		p, err := freePort()
		if err != nil {
			return err
		}
		m.st.Port = p
		changed = true
	}
	if changed {
		return m.saveStateLocked()
	}
	return nil
}

// BaseURL is the OpenAI-compatible root, e.g. http://127.0.0.1:41233/v1.
// Empty until the sidecar has been started once.
func (m *Manager) BaseURL() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.st.Port == 0 {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d/v1", m.st.Port)
}

// APIKey is the key the sidecar requires from clients. Empty until first start.
func (m *Manager) APIKey() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.st.APIKey
}

func portFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("cliproxy: no free loopback port: %w", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
