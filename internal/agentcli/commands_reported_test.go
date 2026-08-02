// SPDX-License-Identifier: AGPL-3.0-or-later

package agentcli

import (
	"context"
	"path/filepath"
	"testing"

	"memo/internal/provider"
)

// resetReportedCommands clears the process-wide cache and restores it after
// the test, so these never leak into each other or into ListCommands tests
// that assert on an otherwise-empty environment.
func resetReportedCommands(t *testing.T) {
	t.Helper()
	reportedMu.Lock()
	saved := reportedCommands
	reportedCommands = map[provider.ProviderType][]string{}
	reportedMu.Unlock()

	t.Cleanup(func() {
		reportedMu.Lock()
		reportedCommands = saved
		reportedMu.Unlock()
	})
}

func TestParseStreamJSONLine_CapturesSlashCommandsFromInit(t *testing.T) {
	ev, ok := parseStreamJSONLine([]byte(
		`{"type":"system","subtype":"init","session_id":"s1","slash_commands":["review","clear","dataviz"]}`))
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(ev.slashCommands) != 3 {
		t.Fatalf("slashCommands = %v", ev.slashCommands)
	}
}

func TestRememberReportedCommands_FiltersSessionOnlyAndInternal(t *testing.T) {
	resetReportedCommands(t)

	rememberReportedCommands(provider.ProviderClaudeCodeCLI, []string{
		"dataviz", "code-review", "/review",
		// Session-only and internal — must not reach the dropdown.
		"clear", "model", "compact", "__remote-workflow",
		"workflow-launch-exec", "",
	})

	got := reportedCommandNames(provider.ProviderClaudeCodeCLI)
	want := map[string]bool{"dataviz": true, "code-review": true, "review": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want exactly %v", got, want)
	}
	for _, name := range got {
		if !want[name] {
			t.Errorf("unexpected command %q survived filtering (got %v)", name, got)
		}
	}
}

// TestRememberReportedCommands_UsageAndCostSurvive is the regression test
// for the reported bug: /usage, /usage-credits, /extra-usage, and /cost all
// return a real, local, zero-cost answer against the real binary (verified
// 2026-08-02 — their result carries total_cost_usd:0/duration_api_ms:0, a
// synthetic local read, not an API call) despite sounding like session-only
// commands. They were wrongly on the blocklist and must stay off it.
func TestRememberReportedCommands_UsageAndCostSurvive(t *testing.T) {
	resetReportedCommands(t)

	rememberReportedCommands(provider.ProviderClaudeCodeCLI, []string{
		"usage", "usage-credits", "extra-usage", "cost",
	})

	got := reportedCommandNames(provider.ProviderClaudeCodeCLI)
	if len(got) != 4 {
		t.Fatalf("got %v, want usage/usage-credits/extra-usage/cost to all survive", got)
	}
}

func TestReportedCommandNames_EmptyBeforeAnyRun(t *testing.T) {
	resetReportedCommands(t)
	if got := reportedCommandNames(provider.ProviderCodexCLI); len(got) != 0 {
		t.Errorf("expected no cached commands before a run, got %v", got)
	}
}

func TestReportedCommandNames_ReturnsACopyNotTheCachedSlice(t *testing.T) {
	resetReportedCommands(t)
	rememberReportedCommands(provider.ProviderClaudeCodeCLI, []string{"review"})

	got := reportedCommandNames(provider.ProviderClaudeCodeCLI)
	got[0] = "MUTATED"

	if again := reportedCommandNames(provider.ProviderClaudeCodeCLI); again[0] != "review" {
		t.Errorf("caller mutated the cache through the returned slice: %v", again)
	}
}

// TestListCommands_IncludesCLIReportedCommands is the regression test for the
// reported bug: a directory scan alone finds only user/project files, missing
// every skill bundled inside the CLI's own package — so "/" showed a handful
// of commands when the CLI actually offers dozens.
func TestListCommands_IncludesCLIReportedCommands(t *testing.T) {
	isolatedHome(t)
	resetReportedCommands(t)

	rememberReportedCommands(provider.ProviderClaudeCodeCLI, []string{"dataviz", "debug", "clear"})

	cmds := ListCommands(provider.ProviderClaudeCodeCLI, "")
	if findCommand(cmds, "dataviz") == nil || findCommand(cmds, "debug") == nil {
		t.Errorf("CLI-reported commands missing from %+v", cmds)
	}
	if findCommand(cmds, "clear") != nil {
		t.Errorf("session-only command should never be offered: %+v", cmds)
	}
}

func TestListCommands_ScannedFileKeepsItsDescriptionOverReportedName(t *testing.T) {
	home := isolatedHome(t)
	resetReportedCommands(t)

	writeFile(t, filepath.Join(home, ".claude", "commands", "deploy.md"),
		"---\ndescription: Ship the branch\n---\nbody\n")
	// The CLI reports the same command by bare name; the scanned file's
	// richer entry (description + real source) must win.
	rememberReportedCommands(provider.ProviderClaudeCodeCLI, []string{"deploy"})

	deploy := findCommand(ListCommands(provider.ProviderClaudeCodeCLI, ""), "deploy")
	if deploy == nil {
		t.Fatal("deploy missing")
	}
	if deploy.Description != "Ship the branch" || deploy.Source != "user" {
		t.Errorf("got %+v, want the scanned file's description and source", *deploy)
	}
}

// TestChatCompletionStream_RecordsReportedCommands proves the capture is
// wired into the real stream path, not just the parser — this is what makes
// the list fill in for free during ordinary use, with no extra subprocess
// and no extra tokens.
func TestChatCompletionStream_RecordsReportedCommands(t *testing.T) {
	isolatedHome(t)
	resetReportedCommands(t)

	script := `
printf '%s\n' '{"type":"system","subtype":"init","session_id":"s1","slash_commands":["dataviz","verify","clear"]}'
printf '%s\n' '{"type":"result","session_id":"s1","is_error":false,"result":"ok"}'
`
	fakeScript(t, script, nil)

	c, _ := NewClaudeCodeCLI(provider.ProviderConfig{Model: "x"})
	ch, err := c.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{provider.TextMessage("user", "selam")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range drainWithTimeout(t, ch) {
	}

	got := reportedCommandNames(provider.ProviderClaudeCodeCLI)
	if len(got) != 2 {
		t.Fatalf("got %v, want dataviz+verify (clear filtered out)", got)
	}
}
