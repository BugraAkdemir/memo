// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"testing"

	"memo/internal/provider"
)

// TestIsSessionProviderName covers the check reinitProviderAndOrchestra uses
// to stop a previously-active per-session provider (Claude Code CLI / Codex
// CLI / Subscriptions) from being silently restored across an app restart —
// see BUG_REPORT: after using a CLI provider, closing and reopening Memo
// kept routing every new chat through the same CLI subprocess instead of
// defaulting back to the local model / no active provider. Subscriptions is
// deliberately NOT session: the model picked in the top-bar selector must
// survive a restart.
func TestIsSessionProviderName(t *testing.T) {
	configs := []provider.ProviderConfig{
		{Name: "Claude Code", Type: provider.ProviderClaudeCodeCLI},
		{Name: "Codex", Type: provider.ProviderCodexCLI},
		{Name: "My OpenAI", Type: provider.ProviderOpenAI},
		// The Subscriptions provider is a plain custom type, told apart by
		// name — and it is sticky, like any custom provider.
		{Name: subsProviderName, Type: provider.ProviderCustom},
		{Name: "My Own Endpoint", Type: provider.ProviderCustom},
	}

	tests := []struct {
		name string
		want bool
	}{
		{"Claude Code", true},
		{"Codex", true},
		{"My OpenAI", false},
		{subsProviderName, false},
		{"My Own Endpoint", false},
		{"unknown-provider", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isSessionProviderName(tt.name, configs)
		if got != tt.want {
			t.Errorf("isSessionProviderName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
