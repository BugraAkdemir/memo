// SPDX-License-Identifier: AGPL-3.0-or-later

package agentcli

import (
	"encoding/json"
	"os"
	"path/filepath"

	"memo/internal/provider"
)

// claudeModelAliases are the aliases `claude --help` itself documents for
// --model ("Provide an alias for the latest model (e.g. 'fable', 'opus', or
// 'sonnet')"). A short, hand-verified list rather than a guess: Claude Code
// has no local file listing every valid model id the way Codex does (see
// codexCachedModels below), and offering an unconfirmed alias that turns out
// invalid would fail confusingly deep inside a chat instead of up front.
var claudeModelAliases = []string{"opus", "sonnet", "fable"}

// AvailableModels returns the model ids a CLI-backed chat can be switched to
// via its per-chat model override (sessions.Session.CLIModel), for the top
// bar's model picker when that chat is CLI mode. Empty means "no override
// available" — the CLI's own configured default is the only option, which
// is a legitimate, common state, not an error.
func AvailableModels(cliType provider.ProviderType) []string {
	switch cliType {
	case provider.ProviderClaudeCodeCLI:
		out := make([]string, len(claudeModelAliases))
		copy(out, claudeModelAliases)
		return out
	case provider.ProviderCodexCLI:
		return codexCachedModels()
	default:
		return nil
	}
}

// codexCachedModels best-effort reads ~/.codex/models_cache.json, the file
// codex itself populates by querying its own backend — the one place a
// current, accurate model id list actually exists for Codex (there is no
// claude-style documented alias set, and codex's model names shift often
// enough — "gpt-5.6-terra" as of this writing — that hardcoding one would go
// stale fast). Missing/unreadable/malformed all return nil rather than a
// guessed fallback list: an empty picker is honest, a wrong one isn't.
func codexCachedModels() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(home, ".codex", "models_cache.json"))
	if err != nil {
		return nil
	}
	var parsed struct {
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil
	}
	out := make([]string, 0, len(parsed.Models))
	for _, m := range parsed.Models {
		if m.ID != "" {
			out = append(out, m.ID)
		}
	}
	return out
}
