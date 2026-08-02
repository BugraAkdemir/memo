// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"memo/internal/provider"
)

// CLIStatus reports whether a CLI-backed provider's underlying executable is
// actually installed and reachable — the backing data for Settings > CLI
// Bağlantıları (yapacam.md §8).
type CLIStatus struct {
	Installed bool   `json:"installed"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
	// BinaryName is the executable this checked for — surfaced so the UI can
	// show it in a "add <name> to PATH" hint without hardcoding it a second
	// time on the frontend.
	BinaryName string `json:"binary_name,omitempty"`
}

// cliBinaryName maps a CLI provider type to the executable it shells out to.
// Empty for anything that isn't actually a CLI-backed provider.
func cliBinaryName(cliType string) string {
	switch provider.ProviderType(cliType) {
	case provider.ProviderClaudeCodeCLI:
		return "claude"
	case provider.ProviderCodexCLI:
		return "codex"
	default:
		return ""
	}
}

// GetCLIStatus checks whether cliType's binary is on PATH and, if so, asks
// it for its version. Never fails outright — an unrecognized cliType or a
// missing binary both just come back Installed: false, since "not there" is
// an expected, common state this exists specifically to report to the user,
// not an error condition for the caller to handle separately.
func (a *App) GetCLIStatus(cliType string) interface{} {
	bin := cliBinaryName(cliType)
	if bin == "" {
		return CLIStatus{}
	}

	path, err := exec.LookPath(bin)
	if err != nil {
		return CLIStatus{BinaryName: bin}
	}
	st := CLIStatus{Installed: true, Path: path, BinaryName: bin}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, bin, "--version").Output(); err == nil {
		st.Version = strings.TrimSpace(string(out))
	}
	return st
}
