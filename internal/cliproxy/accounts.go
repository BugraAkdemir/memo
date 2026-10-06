// SPDX-License-Identifier: AGPL-3.0-or-later

package cliproxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Account is one signed-in credential. It carries identity only: the token
// stays in the credential file on disk and never reaches the API, the logs or
// the UI.
type Account struct {
	Provider string `json:"provider"`
	Email    string `json:"email"`
	Project  string `json:"project,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
	file     string
}

// Accounts lists every credential CLIProxyAPI has written under auth/.
func (m *Manager) Accounts() []Account {
	entries, err := os.ReadDir(m.authDir())
	if err != nil {
		return nil
	}
	var out []Account
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		path := filepath.Join(m.authDir(), e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var raw struct {
			Type      string `json:"type"`
			Email     string `json:"email"`
			ProjectID string `json:"project_id"`
			Disabled  bool   `json:"disabled"`
		}
		if json.Unmarshal(b, &raw) != nil || raw.Type == "" {
			continue
		}
		out = append(out, Account{
			Provider: strings.ToLower(raw.Type),
			Email:    raw.Email,
			Project:  raw.ProjectID,
			Disabled: raw.Disabled,
			file:     path,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		return out[i].Email < out[j].Email
	})
	return out
}

// HasAccounts reports whether any provider is signed in.
func (m *Manager) HasAccounts() bool { return len(m.Accounts()) > 0 }

// Logout removes the stored credential(s) for a provider. It deliberately does
// NOT revoke the grant with the vendor: for Claude and Codex the same login may
// be shared with the user's own CLI, and revoking it there signs that CLI out
// because of a click in Memo. Removing the file is what stops Memo using it.
// The running sidecar notices the deletion on its own.
func (m *Manager) Logout(provider string) error {
	if !validProvider(provider) {
		return ErrUnknownProvider
	}
	var firstErr error
	for _, a := range m.Accounts() {
		if a.Provider != provider {
			continue
		}
		if err := os.Remove(a.file); err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = err
		}
	}
	m.invalidateModels()
	return firstErr
}

func validProvider(p string) bool {
	for _, v := range Providers {
		if v == p {
			return true
		}
	}
	return false
}
