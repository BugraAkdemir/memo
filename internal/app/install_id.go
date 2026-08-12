// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"

	"memo/internal/config"
	"memo/internal/logx"
)

// installIDFile is the data-dir file holding this install's identity.
const installIDFile = "install_id"

// InstallID returns a stable, opaque identifier for THIS Memo install —
// random on first boot, persisted in the data directory, and gone the
// moment the data directory is wiped.
//
// Why it exists: a browser client keeps its auth state in localStorage,
// keyed by origin. Wiping the server and reinstalling gives the same
// origin (http://<lan-ip>:8090) a completely new backend, but the browser
// has no reason to drop anything — so a stale "I already made my
// first-run choice" flag (authSetupDoneKey) kept the setup screen from
// ever appearing again, while every API call 401'd against the fresh
// backend's new credentials. Reported live from a Raspberry Pi
// (2026-08-13); the only way out was clearing site data by hand in
// DevTools, since Ctrl+Shift+R only bypasses the HTTP cache.
//
// Clients compare this value against the one they saw last and drop their
// server-coupled state when it changes — which covers a wipe+reinstall
// and "same browser pointed at a different Memo" alike.
//
// Deliberately NOT derived from machine.key: that is the AES key for
// providers.json's encrypted API keys, and this value is served over an
// unauthenticated endpoint. A dedicated random file has no such coupling.
// It is also deliberately absent from ExportData's archive (see
// backup.go) — like sync_token.json and tailscale/, it is machine state,
// not user content, so restoring a backup onto a fresh install correctly
// reads as a new install to every client.
//
// Never fatal: on any I/O failure this returns "" and the endpoint simply
// omits the field, leaving clients on their fallback path (an
// unauthorized probe) rather than breaking the bootstrap route.
func (a *App) InstallID() string {
	a.installIDMu.Lock()
	defer a.installIDMu.Unlock()
	if a.installIDVal != "" {
		return a.installIDVal
	}

	path := config.DataPath(installIDFile)
	if data, err := os.ReadFile(path); err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			a.installIDVal = id
			return a.installIDVal
		}
	}

	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		logx.Printf("install_id: crypto/rand failed: %v", err)
		return ""
	}
	id := hex.EncodeToString(raw)

	if err := os.MkdirAll(config.DataDir(), 0700); err != nil {
		logx.Printf("install_id: cannot create data dir: %v", err)
		return ""
	}
	if err := os.WriteFile(path, []byte(id), 0600); err != nil {
		logx.Printf("install_id: cannot persist install id: %v", err)
		return ""
	}
	a.installIDVal = id
	return a.installIDVal
}
