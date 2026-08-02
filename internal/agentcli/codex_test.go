// SPDX-License-Identifier: AGPL-3.0-or-later

package agentcli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/provider"
)

func TestParseCodexJSONLine_ThreadStarted(t *testing.T) {
	ev, ok := parseCodexJSONLine([]byte(`{"type":"thread.started","thread_id":"abc-123"}`))
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if ev.sessionID != "abc-123" {
		t.Errorf("sessionID = %q, want abc-123", ev.sessionID)
	}
	if ev.textDelta != "" || ev.isFinal {
		t.Errorf("thread.started should carry no text/final: %+v", ev)
	}
}

func TestParseCodexJSONLine_AgentMessage(t *testing.T) {
	ev, ok := parseCodexJSONLine([]byte(`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"merhaba"}}`))
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if ev.textDelta != "merhaba" {
		t.Errorf("textDelta = %q, want merhaba", ev.textDelta)
	}
}

func TestParseCodexJSONLine_NonAgentMessageItemIgnored(t *testing.T) {
	if _, ok := parseCodexJSONLine([]byte(`{"type":"item.completed","item":{"type":"command_execution","command":"ls"}}`)); ok {
		t.Errorf("expected command_execution item to be ignored (ok=false)")
	}
}

func TestParseCodexJSONLine_TurnCompleted(t *testing.T) {
	ev, ok := parseCodexJSONLine([]byte(`{"type":"turn.completed","usage":{"input_tokens":1}}`))
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if !ev.isFinal || ev.errText != "" {
		t.Errorf("got %+v, want isFinal=true errText=empty", ev)
	}
}

func TestParseCodexJSONLine_TurnFailed(t *testing.T) {
	ev, ok := parseCodexJSONLine([]byte(`{"type":"turn.failed","error":{"message":"boom"}}`))
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if !ev.isFinal || ev.errText != "boom" {
		t.Errorf("got %+v, want isFinal=true errText=boom", ev)
	}
}

func TestParseCodexJSONLine_UnknownTypeIgnored(t *testing.T) {
	if _, ok := parseCodexJSONLine([]byte(`{"type":"item.started","item":{"type":"reasoning"}}`)); ok {
		t.Errorf("expected unknown event type to be ignored (ok=false)")
	}
}

func TestParseCodexJSONLine_InvalidJSON(t *testing.T) {
	if _, ok := parseCodexJSONLine([]byte(`not json`)); ok {
		t.Errorf("expected invalid JSON to be ignored (ok=false)")
	}
}

func TestNewCodexCLI_Defaults(t *testing.T) {
	c, err := NewCodexCLI(provider.ProviderConfig{Type: provider.ProviderCodexCLI})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.binaryPath != "codex" {
		t.Errorf("binaryPath = %q, want codex", c.binaryPath)
	}
	if c.model != "codex" {
		t.Errorf("model = %q, want codex", c.model)
	}
	if c.Name() != provider.ProviderCodexCLI {
		t.Errorf("Name() = %q", c.Name())
	}
}

func TestNewCodexCLI_BaseURLOverridesBinaryPath(t *testing.T) {
	c, err := NewCodexCLI(provider.ProviderConfig{BaseURL: "/opt/custom/codex", Model: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.binaryPath != "/opt/custom/codex" {
		t.Errorf("binaryPath = %q, want /opt/custom/codex", c.binaryPath)
	}
}

func TestCodexChatCompletionStream_HappyPath(t *testing.T) {
	script := `
printf '%s\n' '{"type":"thread.started","thread_id":"thread-1"}'
printf '%s\n' '{"type":"turn.started"}'
printf '%s\n' '{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"Merhaba dunya"}}'
printf '%s\n' '{"type":"turn.completed","usage":{}}'
`
	var args []string
	fakeScript(t, script, &args)

	c, _ := NewCodexCLI(provider.ProviderConfig{Model: "x"})
	ch, err := c.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{provider.TextMessage("user", "selam")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var content strings.Builder
	var gotDone bool
	var gotSessionID string
	for chunk := range drainWithTimeout(t, ch) {
		if chunk.Error != "" {
			t.Fatalf("unexpected error chunk: %s", chunk.Error)
		}
		content.WriteString(chunk.Content)
		if chunk.Done {
			gotDone = true
			gotSessionID = chunk.CLISessionID
		}
	}
	if content.String() != "Merhaba dunya" {
		t.Errorf("content = %q", content.String())
	}
	if !gotDone {
		t.Errorf("expected a Done chunk")
	}
	if gotSessionID != "thread-1" {
		t.Errorf("CLISessionID = %q, want thread-1", gotSessionID)
	}
}

func TestCodexChatCompletionStream_ResumePassesResumeArgsNotDashC(t *testing.T) {
	script := `printf '%s\n' '{"type":"turn.completed","usage":{}}'`
	var args []string
	fakeScript(t, script, &args)

	c, _ := NewCodexCLI(provider.ProviderConfig{Model: "x"})
	ch, err := c.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Messages:        []provider.Message{provider.TextMessage("user", "devam et")},
		ResumeSessionID: "thread-1",
		WorkDir:         t.TempDir(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range drainWithTimeout(t, ch) {
	}

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "exec resume thread-1") {
		t.Errorf("args %v missing exec resume thread-1", args)
	}
	if !strings.Contains(joined, "--dangerously-bypass-approvals-and-sandbox") {
		t.Errorf("args %v missing --dangerously-bypass-approvals-and-sandbox", args)
	}
	if strings.Contains(joined, "-C ") {
		t.Errorf("args %v should not pass -C on resume (codex rejects it there), got: %s", args, joined)
	}
}

func TestCodexChatCompletionStream_FreshRunPassesWorkDirFlag(t *testing.T) {
	script := `printf '%s\n' '{"type":"turn.completed","usage":{}}'`
	var args []string
	fakeScript(t, script, &args)

	workdir := t.TempDir()
	c, _ := NewCodexCLI(provider.ProviderConfig{Model: "x"})
	ch, err := c.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{provider.TextMessage("user", "selam")},
		WorkDir:  workdir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range drainWithTimeout(t, ch) {
	}

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-C "+workdir) {
		t.Errorf("args %v missing -C %s", args, workdir)
	}
}

// TestCodexChatCompletionStream_ExpandsSlashCommand covers the behavior
// difference that makes this necessary at all: `codex exec` passes "/foo"
// through to the model as literal text instead of resolving the prompt file
// (verified against the real binary — it improvised an answer rather than
// running the prompt), so the provider has to expand it before sending.
func TestCodexChatCompletionStream_ExpandsSlashCommand(t *testing.T) {
	home := isolatedHome(t)
	writeFile(t, filepath.Join(home, ".codex", "prompts", "audit.md"),
		"---\ndescription: Audit\n---\nAudit this: $ARGUMENTS\n")

	script := `printf '%s\n' '{"type":"turn.completed","usage":{}}'`
	var args []string
	fakeScript(t, script, &args)

	c, _ := NewCodexCLI(provider.ProviderConfig{Model: "x"})
	ch, err := c.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{provider.TextMessage("user", "/audit the login flow")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range drainWithTimeout(t, ch) {
	}

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "Audit this: the login flow") {
		t.Errorf("codex should receive the expanded prompt, got args: %v", args)
	}
	if strings.Contains(joined, "/audit the login flow") {
		t.Errorf("the raw slash command should have been replaced, got args: %v", args)
	}
}

func TestCodexChatCompletionStream_UnknownSlashCommandSentVerbatim(t *testing.T) {
	isolatedHome(t)
	script := `printf '%s\n' '{"type":"turn.completed","usage":{}}'`
	var args []string
	fakeScript(t, script, &args)

	c, _ := NewCodexCLI(provider.ProviderConfig{Model: "x"})
	ch, err := c.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{provider.TextMessage("user", "/nope still send me")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range drainWithTimeout(t, ch) {
	}

	if !strings.Contains(strings.Join(args, " "), "/nope still send me") {
		t.Errorf("an unknown command must still reach the CLI unchanged, got args: %v", args)
	}
}

// TestCodexChatCompletionStream_PassesModelFlag is the regression test for
// the same gap as Claude Code's equivalent test: ChatRequest.Model was never
// actually passed to the codex subprocess, so every CLI chat silently used
// codex's own config.toml default regardless of what a caller set here.
func TestCodexChatCompletionStream_PassesModelFlag(t *testing.T) {
	script := `printf '%s\n' '{"type":"turn.completed","usage":{}}'`
	var args []string
	fakeScript(t, script, &args)

	c, _ := NewCodexCLI(provider.ProviderConfig{Model: "x"})
	ch, err := c.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{provider.TextMessage("user", "selam")},
		Model:    "gpt-5.6-terra",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range drainWithTimeout(t, ch) {
	}

	if !strings.Contains(strings.Join(args, " "), "-m gpt-5.6-terra") {
		t.Errorf("args %v missing -m gpt-5.6-terra", args)
	}
}

func TestCodexChatCompletionStream_ResumePassesModelFlagToo(t *testing.T) {
	script := `printf '%s\n' '{"type":"turn.completed","usage":{}}'`
	var args []string
	fakeScript(t, script, &args)

	c, _ := NewCodexCLI(provider.ProviderConfig{Model: "x"})
	ch, err := c.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Messages:        []provider.Message{provider.TextMessage("user", "devam")},
		ResumeSessionID: "thread-1",
		Model:           "gpt-5.5",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range drainWithTimeout(t, ch) {
	}

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-m gpt-5.5") {
		t.Errorf("args %v missing -m gpt-5.5 on resume", args)
	}
	if strings.Contains(joined, "-C ") {
		t.Errorf("args %v should still not pass -C on resume, got: %s", args, joined)
	}
}

func TestCodexChatCompletionStream_EmptyModelOmitsFlag(t *testing.T) {
	script := `printf '%s\n' '{"type":"turn.completed","usage":{}}'`
	var args []string
	fakeScript(t, script, &args)

	c, _ := NewCodexCLI(provider.ProviderConfig{Model: "x"})
	ch, err := c.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{provider.TextMessage("user", "selam")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range drainWithTimeout(t, ch) {
	}

	if strings.Contains(strings.Join(args, " "), "-m ") {
		t.Errorf("args %v should not pass -m when unset, letting codex use its own default", args)
	}
}

func TestCodexChatCompletionStream_TurnFailedSendsErrorChunk(t *testing.T) {
	script := `printf '%s\n' '{"type":"turn.failed","error":{"message":"model not supported"}}'`
	fakeScript(t, script, nil)

	c, _ := NewCodexCLI(provider.ProviderConfig{Model: "x"})
	ch, err := c.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{provider.TextMessage("user", "selam")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var gotErrChunk bool
	for chunk := range drainWithTimeout(t, ch) {
		if chunk.Error != "" && chunk.Done {
			gotErrChunk = true
			if !strings.Contains(chunk.Error, "model not supported") {
				t.Errorf("error chunk = %q", chunk.Error)
			}
		}
	}
	if !gotErrChunk {
		t.Errorf("expected a terminal error chunk")
	}
}

func TestCodexChatCompletionStream_ProcessExitsWithErrorSendsTerminalChunk(t *testing.T) {
	script := `echo "boom on stderr" 1>&2; exit 1`
	fakeScript(t, script, nil)

	c, _ := NewCodexCLI(provider.ProviderConfig{Model: "x"})
	ch, err := c.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{provider.TextMessage("user", "selam")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var gotErrChunk bool
	for chunk := range drainWithTimeout(t, ch) {
		if chunk.Error != "" && chunk.Done {
			gotErrChunk = true
			if !strings.Contains(chunk.Error, "boom on stderr") {
				t.Errorf("error chunk = %q, want it to include stderr", chunk.Error)
			}
		}
	}
	if !gotErrChunk {
		t.Errorf("expected a terminal error chunk — every branch must send one (AGENTS.md streaming gotcha)")
	}
}

func TestCodexChatCompletionStream_NoUserMessageErrors(t *testing.T) {
	c, _ := NewCodexCLI(provider.ProviderConfig{Model: "x"})
	_, err := c.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{provider.TextMessage("assistant", "no user turn here")},
	})
	if err == nil {
		t.Fatalf("expected an error when there's no user message")
	}
}

func TestCodexChatCompletion_AssemblesFullContent(t *testing.T) {
	script := `
printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"tam cevap"}}'
printf '%s\n' '{"type":"turn.completed","usage":{}}'
`
	fakeScript(t, script, nil)

	c, _ := NewCodexCLI(provider.ProviderConfig{Model: "x"})
	resp, err := c.ChatCompletion(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{provider.TextMessage("user", "selam")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "tam cevap" {
		t.Errorf("Content = %q", resp.Content)
	}
}

func TestCodexListModels(t *testing.T) {
	c, _ := NewCodexCLI(provider.ProviderConfig{Model: "x"})
	models, err := c.ListModels(context.Background())
	if err != nil || len(models) == 0 {
		t.Fatalf("ListModels() = %v, %v", models, err)
	}
}

func TestRegisterConstructor_WiresCodexIntoProviderNewProvider(t *testing.T) {
	p, err := provider.NewProvider(provider.ProviderConfig{
		Type:  provider.ProviderCodexCLI,
		Model: "codex",
	})
	if err != nil {
		t.Fatalf("provider.NewProvider did not resolve codex-cli via RegisterConstructor: %v", err)
	}
	if p.Name() != provider.ProviderCodexCLI {
		t.Errorf("Name() = %q", p.Name())
	}
}
