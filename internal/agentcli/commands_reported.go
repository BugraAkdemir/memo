// SPDX-License-Identifier: AGPL-3.0-or-later

package agentcli

import (
	"sort"
	"strings"
	"sync"

	"memo/internal/provider"
)

// A CLI knows its own command list far better than any directory scan can:
// Claude Code ships a set of skills inside its own package (dataviz, debug,
// verify, code-review, run, ...) that live nowhere under ~/.claude, plus
// whatever plugins are loaded. Scanning alone therefore finds only a
// fraction of what "/" actually offers — the reported bug was exactly that,
// "a few commands show up, most don't".
//
// The CLI exposes the full list in one place only: the `slash_commands`
// array on its stream-json init event (verified 2026-08-02; `claude --help`
// has no listing flag, and `claude -p ""` exits before emitting init, so
// there is no cheap standalone way to ask). Every real chat turn already
// parses that event, so the list is captured there for free and cached
// process-wide per CLI type — no extra subprocess, no extra tokens, and it
// self-updates when the CLI is upgraded.
//
// Cached in memory rather than on disk deliberately: a stale list that
// survives a CLI downgrade/uninstall would offer commands that no longer
// exist, and one chat turn is enough to refill it.
var (
	reportedMu       sync.RWMutex
	reportedCommands = map[provider.ProviderType][]string{}
)

// rememberReportedCommands records the command names a CLI reported about
// itself. Called from the stream parser on every init event.
func rememberReportedCommands(cliType provider.ProviderType, names []string) {
	cleaned := make([]string, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(strings.TrimPrefix(n, "/"))
		if n == "" || isSessionOnlyCommand(n) {
			continue
		}
		cleaned = append(cleaned, n)
	}
	sort.Strings(cleaned)

	reportedMu.Lock()
	defer reportedMu.Unlock()
	reportedCommands[cliType] = cleaned
}

// reportedCommandNames returns the cached list for cliType, or nil if the
// CLI hasn't run yet in this process.
func reportedCommandNames(cliType provider.ProviderType) []string {
	reportedMu.RLock()
	defer reportedMu.RUnlock()
	names := reportedCommands[cliType]
	out := make([]string, len(names))
	copy(out, names)
	return out
}

// sessionOnlyCommands are commands that only manipulate an interactive
// terminal session's own state — its model, theme, context window, login.
// They do nothing meaningful sent through Memo's chat, which has no such
// session to act on, so they're hidden rather than offered and left to fail
// confusingly.
//
// NOT everything that sounds session-related belongs here — verified
// individually against the real `claude` binary in exec mode (2026-08-02)
// rather than assumed: /usage, /usage-credits, /extra-usage, and /cost all
// return a real, local, zero-cost answer in `-p` mode (their result carries
// "total_cost_usd":0 / "duration_api_ms":0 — a synthetic local read, not an
// API call), so they're genuinely useful sent through Memo and must stay off
// this list. Re-verify before adding anything here on the strength of its
// name alone.
//
// A blocklist rather than an allowlist on purpose: the useful set changes
// with every CLI release (bundled skills come and go), and silently hiding a
// new genuinely-useful command is worse than showing one extra that turns
// out to be a no-op.
var sessionOnlyCommands = map[string]bool{
	"add-dir": true, "agents": true, "bug": true, "clear": true, "color": true,
	"compact": true, "config": true, "context": true,
	"design-consent": true, "design-revoke": true, "effort": true,
	"exit": true, "export": true, "fast": true,
	"heapdump": true, "help": true, "hooks": true, "ide": true,
	"insights": true, "install-github-app": true, "login": true,
	"logout": true, "mcp": true, "memory": true, "migrate-installer": true,
	"model": true, "output-style": true, "permissions": true,
	"privacy-settings": true, "quit": true, "recap": true,
	"release-notes": true, "reload-skills": true, "rename": true,
	"resume": true, "status": true, "statusline": true, "team-onboarding": true,
	"terminal-setup": true, "theme": true, "todos": true, "upgrade": true,
	"vim": true,
}

// isSessionOnlyCommand also filters the CLI's internal, double-underscore
// commands (__remote-workflow, workflow-launch-exec), which are wiring
// rather than anything a user would invoke.
func isSessionOnlyCommand(name string) bool {
	if strings.HasPrefix(name, "__") {
		return true
	}
	if strings.HasPrefix(name, "workflow-") {
		return true
	}
	return sessionOnlyCommands[name]
}
