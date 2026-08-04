// SPDX-License-Identifier: AGPL-3.0-or-later

package agentcli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memo/internal/provider"
)

func TestParseStreamJSONLine_System(t *testing.T) {
	ev, ok := parseStreamJSONLine([]byte(`{"type":"system","subtype":"init","session_id":"abc-123"}`))
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if ev.sessionID != "abc-123" {
		t.Errorf("sessionID = %q, want abc-123", ev.sessionID)
	}
	if ev.textDelta != "" || ev.isFinal {
		t.Errorf("system event should carry no text/final: %+v", ev)
	}
}

func TestParseStreamJSONLine_AssistantTextDelta(t *testing.T) {
	ev, ok := parseStreamJSONLine([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"merhaba"}]}}`))
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if ev.textDelta != "merhaba" {
		t.Errorf("textDelta = %q, want merhaba", ev.textDelta)
	}
}

func TestParseStreamJSONLine_ResultSuccess(t *testing.T) {
	ev, ok := parseStreamJSONLine([]byte(`{"type":"result","session_id":"abc-123","is_error":false,"result":"done"}`))
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if !ev.isFinal {
		t.Errorf("expected isFinal=true")
	}
	if ev.errText != "" {
		t.Errorf("errText = %q, want empty on success", ev.errText)
	}
}

func TestParseStreamJSONLine_ResultError(t *testing.T) {
	ev, ok := parseStreamJSONLine([]byte(`{"type":"result","session_id":"abc-123","is_error":true,"result":"boom"}`))
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if !ev.isFinal || ev.errText != "boom" {
		t.Errorf("got %+v, want isFinal=true errText=boom", ev)
	}
}

func TestParseStreamJSONLine_UnknownTypeIgnored(t *testing.T) {
	if _, ok := parseStreamJSONLine([]byte(`{"type":"tool_use","name":"whatever"}`)); ok {
		t.Errorf("expected unknown event type to be ignored (ok=false)")
	}
}

func TestParseStreamJSONLine_InvalidJSON(t *testing.T) {
	if _, ok := parseStreamJSONLine([]byte(`not json`)); ok {
		t.Errorf("expected invalid JSON to be ignored (ok=false)")
	}
}

func TestNewClaudeCodeCLI_Defaults(t *testing.T) {
	c, err := NewClaudeCodeCLI(provider.ProviderConfig{Type: provider.ProviderClaudeCodeCLI})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.binaryPath != "claude" {
		t.Errorf("binaryPath = %q, want claude", c.binaryPath)
	}
	if c.model != "claude-code" {
		t.Errorf("model = %q, want claude-code", c.model)
	}
	if c.Name() != provider.ProviderClaudeCodeCLI {
		t.Errorf("Name() = %q", c.Name())
	}
}

func TestNewClaudeCodeCLI_BaseURLOverridesBinaryPath(t *testing.T) {
	c, err := NewClaudeCodeCLI(provider.ProviderConfig{BaseURL: "/opt/custom/claude", Model: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.binaryPath != "/opt/custom/claude" {
		t.Errorf("binaryPath = %q, want /opt/custom/claude", c.binaryPath)
	}
}

// waitForFile polls for path to exist, up to timeout — used instead of a
// fixed sleep to know a background shell job has actually started before
// the test proceeds, so the test's timing doesn't depend on how fast the
// machine running it happens to be.
func waitForFile(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s to appear", timeout, path)
}

// fakeScript builds an execCommandContext replacement that runs sh -c
// <script> instead of the real claude binary, so these tests never require
// claude to actually be installed. It also records the args claude would
// have been called with, into capturedArgs.
func fakeScript(t *testing.T, script string, capturedArgs *[]string) {
	t.Helper()
	orig := execCommandContext
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if capturedArgs != nil {
			*capturedArgs = append([]string{name}, args...)
		}
		return exec.CommandContext(ctx, "sh", "-c", script)
	}
	t.Cleanup(func() { execCommandContext = orig })
}

func TestChatCompletionStream_HappyPath(t *testing.T) {
	script := `
printf '%s\n' '{"type":"system","subtype":"init","session_id":"sess-1"}'
printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"text","text":"Merhaba"}]}}'
printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"text","text":", dunya"}]}}'
printf '%s\n' '{"type":"result","session_id":"sess-1","is_error":false,"result":"ok"}'
`
	var args []string
	fakeScript(t, script, &args)

	c, _ := NewClaudeCodeCLI(provider.ProviderConfig{Model: "x"})
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
	if content.String() != "Merhaba, dunya" {
		t.Errorf("content = %q", content.String())
	}
	if !gotDone {
		t.Errorf("expected a Done chunk")
	}
	if gotSessionID != "sess-1" {
		t.Errorf("CLISessionID = %q, want sess-1", gotSessionID)
	}
}

// TestChatCompletionStream_GrandchildHoldingPipeOpen_WaitDelayUnblocksScan
// is the regression test for BUG_REPORT.md's LK-1: exec.CommandContext only
// SIGKILLs the *direct* child (claude/codex) on cancellation. With
// --dangerously-skip-permissions, that process can spawn its own children
// to carry out a tool call — if ctx is cancelled before a "final" JSON line
// arrives and one of those grandchildren outlives the direct process, it
// keeps the stdout pipe's write end open independently, and
// scanner.Scan() (reading that pipe) blocks on EOF that never comes: the
// goroutine never returns, ch never closes, and ChatCompletion's `for
// chunk := range ch` hangs forever too.
//
// The fix is two-layered — cmd.Cancel now kills the whole process group
// (newSysProcAttr/killProcessGroup, sysproc_unix.go/sysproc_windows.go),
// which already reaps an ordinary tool-call child since it inherits the
// same group by default; cliProcessWaitDelay is the backstop for a
// grandchild that somehow escapes the group anyway (a code-review finding
// on this exact fix — process-group kill alone doesn't cover every case,
// e.g. a child that calls setsid itself). This test exercises the
// end-to-end outcome both layers exist for — cancellation must never hang
// — rather than trying to isolate one mechanism from the other, since a
// portable way to force a *guaranteed* group-escaping grandchild (setsid
// isn't available on every platform this ships for) isn't worth the
// fragility.
//
// The script backgrounds `sleep 3` inside a subshell that exits
// immediately (`(sleep 3 &)`), then writes readyMarker so the test can wait
// for the grandchild to actually exist before cancelling instead of
// guessing with a fixed sleep (a prior version of this test did exactly
// that and was flagged by review as capable of silently testing nothing
// under CI load). The direct process itself just sleeps too, deliberately
// emitting no result line, so cancellation — not a clean exit — is what
// ends it.
func TestChatCompletionStream_GrandchildHoldingPipeOpen_WaitDelayUnblocksScan(t *testing.T) {
	orig := cliProcessWaitDelay
	cliProcessWaitDelay = 100 * time.Millisecond
	t.Cleanup(func() { cliProcessWaitDelay = orig })

	readyMarker := filepath.Join(t.TempDir(), "grandchild-ready")
	script := `(sleep 3 &); touch ` + readyMarker + `; sleep 3`
	fakeScript(t, script, nil)

	c, _ := NewClaudeCodeCLI(provider.ProviderConfig{Model: "x"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := c.ChatCompletionStream(ctx, provider.ChatRequest{
		Messages: []provider.Message{provider.TextMessage("user", "selam")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForFile(t, readyMarker, 2*time.Second)
	cancel()

	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	select {
	case <-done:
		// ch closed promptly instead of blocking on the grandchild's
		// remaining ~3s sleep.
	case <-time.After(1 * time.Second):
		t.Fatal("ChatCompletionStream's channel never closed within 1s of cancellation — scanner.Scan() is stuck reading from a pipe a grandchild process is still holding open (LK-1)")
	}
}

func TestChatCompletionStream_ResumePassesResumeFlag(t *testing.T) {
	script := `printf '%s\n' '{"type":"result","session_id":"sess-1","is_error":false,"result":"ok"}'`
	var args []string
	fakeScript(t, script, &args)

	c, _ := NewClaudeCodeCLI(provider.ProviderConfig{Model: "x"})
	ch, err := c.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Messages:        []provider.Message{provider.TextMessage("user", "devam et")},
		ResumeSessionID: "sess-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range drainWithTimeout(t, ch) {
	}

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--resume sess-1") {
		t.Errorf("args %v missing --resume sess-1", args)
	}
	if !strings.Contains(joined, "--dangerously-skip-permissions") {
		t.Errorf("args %v missing --dangerously-skip-permissions", args)
	}
}

// TestChatCompletionStream_PassesModelFlag is the regression test for a real
// gap: ChatRequest.Model was accepted since this struct's first version but
// never actually reached the subprocess — every CLI chat silently used
// claude's own default model no matter what a caller set here.
func TestChatCompletionStream_PassesModelFlag(t *testing.T) {
	script := `printf '%s\n' '{"type":"result","session_id":"sess-1","is_error":false,"result":"ok"}'`
	var args []string
	fakeScript(t, script, &args)

	c, _ := NewClaudeCodeCLI(provider.ProviderConfig{Model: "x"})
	ch, err := c.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{provider.TextMessage("user", "selam")},
		Model:    "opus",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range drainWithTimeout(t, ch) {
	}

	if !strings.Contains(strings.Join(args, " "), "--model opus") {
		t.Errorf("args %v missing --model opus", args)
	}
}

func TestChatCompletionStream_EmptyModelOmitsFlag(t *testing.T) {
	script := `printf '%s\n' '{"type":"result","session_id":"sess-1","is_error":false,"result":"ok"}'`
	var args []string
	fakeScript(t, script, &args)

	c, _ := NewClaudeCodeCLI(provider.ProviderConfig{Model: "x"})
	ch, err := c.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{provider.TextMessage("user", "selam")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range drainWithTimeout(t, ch) {
	}

	if strings.Contains(strings.Join(args, " "), "--model") {
		t.Errorf("args %v should not pass --model when unset, letting claude use its own default", args)
	}
}

func TestChatCompletionStream_ProcessExitsWithErrorSendsTerminalChunk(t *testing.T) {
	script := `echo "boom on stderr" 1>&2; exit 1`
	fakeScript(t, script, nil)

	c, _ := NewClaudeCodeCLI(provider.ProviderConfig{Model: "x"})
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

func TestChatCompletionStream_NoUserMessageErrors(t *testing.T) {
	c, _ := NewClaudeCodeCLI(provider.ProviderConfig{Model: "x"})
	_, err := c.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{provider.TextMessage("assistant", "no user turn here")},
	})
	if err == nil {
		t.Fatalf("expected an error when there's no user message")
	}
}

func TestChatCompletion_AssemblesFullContent(t *testing.T) {
	script := `
printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"text","text":"parca1"}]}}'
printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"text","text":"parca2"}]}}'
printf '%s\n' '{"type":"result","session_id":"sess-2","is_error":false,"result":"ok"}'
`
	fakeScript(t, script, nil)

	c, _ := NewClaudeCodeCLI(provider.ProviderConfig{Model: "x"})
	resp, err := c.ChatCompletion(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{provider.TextMessage("user", "selam")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "parca1parca2" {
		t.Errorf("Content = %q", resp.Content)
	}
}

func TestListModels(t *testing.T) {
	c, _ := NewClaudeCodeCLI(provider.ProviderConfig{Model: "x"})
	models, err := c.ListModels(context.Background())
	if err != nil || len(models) == 0 {
		t.Fatalf("ListModels() = %v, %v", models, err)
	}
}

func TestRegisterConstructor_WiresIntoProviderNewProvider(t *testing.T) {
	p, err := provider.NewProvider(provider.ProviderConfig{
		Type:  provider.ProviderClaudeCodeCLI,
		Model: "claude-code",
	})
	if err != nil {
		t.Fatalf("provider.NewProvider did not resolve claude-code-cli via RegisterConstructor: %v", err)
	}
	if p.Name() != provider.ProviderClaudeCodeCLI {
		t.Errorf("Name() = %q", p.Name())
	}
}

// drainWithTimeout re-emits every chunk from ch onto a new channel, failing
// the test instead of hanging forever if the source goroutine has a bug
// that leaves it un-closed.
func drainWithTimeout(t *testing.T, ch <-chan provider.StreamChunk) <-chan provider.StreamChunk {
	t.Helper()
	out := make(chan provider.StreamChunk)
	go func() {
		defer close(out)
		timeout := time.After(5 * time.Second)
		for {
			select {
			case chunk, ok := <-ch:
				if !ok {
					return
				}
				out <- chunk
			case <-timeout:
				t.Error("timed out waiting for ChatCompletionStream to close its channel")
				return
			}
		}
	}()
	return out
}
