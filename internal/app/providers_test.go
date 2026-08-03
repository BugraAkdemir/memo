// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"testing"

	"memo/internal/provider"
)

// TestIsCLIProviderName covers the check reinitProviderAndOrchestra uses to
// stop a previously-active Claude Code CLI / Codex CLI provider from being
// silently restored across an app restart — see BUG_REPORT: after using a
// CLI provider, closing and reopening Memo kept routing every new chat
// through the same CLI subprocess instead of defaulting back to the local
// model / no active provider.
func TestIsCLIProviderName(t *testing.T) {
	configs := []provider.ProviderConfig{
		{Name: "Claude Code", Type: provider.ProviderClaudeCodeCLI},
		{Name: "Codex", Type: provider.ProviderCodexCLI},
		{Name: "My OpenAI", Type: provider.ProviderOpenAI},
	}

	tests := []struct {
		name string
		want bool
	}{
		{"Claude Code", true},
		{"Codex", true},
		{"My OpenAI", false},
		{"unknown-provider", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isCLIProviderName(tt.name, configs)
		if got != tt.want {
			t.Errorf("isCLIProviderName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
