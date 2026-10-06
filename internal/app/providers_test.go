// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"testing"

	"memo/internal/provider"
)

// TestIsSessionProviderName covers the check reinitProviderAndOrchestra uses
// to stop a previously-active per-session provider (Claude Code CLI / Codex
// CLI / gemini-sub) from being silently restored across an app restart —
// see BUG_REPORT: after using a CLI provider, closing and reopening Memo
// kept routing every new chat through the same CLI subprocess instead of
// defaulting back to the local model / no active provider. gemini-sub has
// the same "per session, not sticky" semantics.
func TestIsSessionProviderName(t *testing.T) {
	configs := []provider.ProviderConfig{
		{Name: "Claude Code", Type: provider.ProviderClaudeCodeCLI},
		{Name: "Codex", Type: provider.ProviderCodexCLI},
		{Name: "Google — Gemini (subscription)", Type: provider.ProviderGeminiSub},
		{Name: "My OpenAI", Type: provider.ProviderOpenAI},
		// The Subscriptions provider is a plain custom type, told apart by
		// name; a user's own custom provider with another name is not session.
		{Name: subsProviderName, Type: provider.ProviderCustom},
		{Name: "My Own Endpoint", Type: provider.ProviderCustom},
	}

	tests := []struct {
		name string
		want bool
	}{
		{"Claude Code", true},
		{"Codex", true},
		{"Google — Gemini (subscription)", true},
		{"My OpenAI", false},
		{subsProviderName, true},
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
