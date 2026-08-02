// SPDX-License-Identifier: AGPL-3.0-or-later

// Package agentcli wires external coding-agent CLIs into Memo's
// provider.Provider interface by shelling out to them, instead of making an
// HTTP call the way every type in internal/provider does. Deliberately not
// internal/provider itself — these are stateful, side-effecting local
// processes (they can edit files and run shell commands) with their own
// session/auth model, not stateless chat-completion APIs, and mixing that
// into internal/provider would mislead anyone reading it. Also not
// internal/replcli (Memo's own terminal client) or internal/agent (Memo's
// own tool-execution sandbox) — unrelated packages this one must not be
// confused with.
package agentcli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"memo/internal/provider"
)

func init() {
	provider.RegisterConstructor(provider.ProviderClaudeCodeCLI, func(cfg provider.ProviderConfig) (provider.Provider, error) {
		return NewClaudeCodeCLI(cfg)
	})
}

// execCommandContext is a seam over exec.CommandContext so tests can run a
// stand-in script instead of the real `claude` binary.
var execCommandContext = exec.CommandContext

// ClaudeCodeCLI shells out to the user's locally installed `claude` binary
// in non-interactive mode, one subprocess per message. Unlike every HTTP
// provider in internal/provider, multi-turn continuity comes from the CLI's
// own --resume <id> mechanism (provider.ChatRequest.ResumeSessionID / the
// session id echoed back on provider.StreamChunk.CLISessionID) rather than
// resending the full message history on every call.
type ClaudeCodeCLI struct {
	// binaryPath is the `claude` executable — plain "claude" (resolved via
	// PATH at exec time) unless the user set a manual override. Reuses
	// ProviderConfig.BaseURL for that override rather than adding a new
	// field to a struct shared by every provider type, for just the one
	// case (this provider family) that needs a filesystem path instead of a
	// URL there.
	binaryPath string
	model      string
}

// NewClaudeCodeCLI builds a ClaudeCodeCLI from cfg. Never fails on its own —
// a missing/bad binary is only discovered when a chat is actually attempted
// (see the "CLI Bağlantıları" Settings tab in yapacam.md §8 for the
// separate up-front exec.LookPath check).
func NewClaudeCodeCLI(cfg provider.ProviderConfig) (*ClaudeCodeCLI, error) {
	bin := strings.TrimSpace(cfg.BaseURL)
	if bin == "" {
		bin = "claude"
	}
	model := cfg.Model
	if model == "" {
		model = "claude-code"
	}
	return &ClaudeCodeCLI{binaryPath: bin, model: model}, nil
}

func (c *ClaudeCodeCLI) Name() provider.ProviderType { return provider.ProviderClaudeCodeCLI }
func (c *ClaudeCodeCLI) DisplayName() string         { return "Claude Code (CLI)" }

// ListModels returns a single synthetic entry — the CLI manages its own
// model selection internally, there is nothing to list dynamically the way
// OpenAI-compatible providers do.
func (c *ClaudeCodeCLI) ListModels(ctx context.Context) ([]string, error) {
	return []string{"claude-code"}, nil
}

// ChatCompletion drains ChatCompletionStream into a single response.
func (c *ClaudeCodeCLI) ChatCompletion(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	ch, err := c.ChatCompletionStream(ctx, req)
	if err != nil {
		return nil, err
	}
	var content strings.Builder
	for chunk := range ch {
		if chunk.Error != "" {
			return nil, fmt.Errorf("%s", chunk.Error)
		}
		content.WriteString(chunk.Content)
	}
	return &provider.ChatResponse{Content: content.String(), Model: c.model}, nil
}

// ChatCompletionStream runs `claude -p --output-format stream-json
// --dangerously-skip-permissions [--resume <id>] "<message>"` and translates
// its stdout, line by line, into provider.StreamChunk. Only the latest user
// message in req.Messages is sent — see ChatRequest.ResumeSessionID's doc
// comment for why the rest of the history isn't resent.
//
// --dangerously-skip-permissions is deliberate, not an oversight: headless
// mode has no terminal to approve a permission prompt, so without it every
// file/command action would just block forever. Selecting this provider at
// all means granting it that authority — decided explicitly in yapacam.md
// §6, not something this function second-guesses per call.
func (c *ClaudeCodeCLI) ChatCompletionStream(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	userMsg := lastUserMessage(req.Messages)
	if userMsg == "" {
		return nil, fmt.Errorf("claude-code-cli: no user message to send")
	}

	args := []string{"-p", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"}
	if req.ResumeSessionID != "" {
		args = append(args, "--resume", req.ResumeSessionID)
	}
	// req.Model was accepted by this struct from the start but never actually
	// passed to the subprocess — every CLI chat silently used claude's own
	// configured default model regardless of what a caller set here. Accepts
	// either an alias ("opus", "sonnet", "fable" — the ones claude --help
	// itself documents) or a full model name; unvalidated here, same as
	// every other pass-through flag in this file, since claude itself is the
	// authority on what's valid.
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	args = append(args, userMsg)

	cmd := execCommandContext(ctx, c.binaryPath, args...)
	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("claude-code-cli: stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("claude-code-cli: could not start %q — is the Claude Code CLI installed and on PATH? (%w)", c.binaryPath, err)
	}

	ch := make(chan provider.StreamChunk, 128)
	go func() {
		defer close(ch)

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

		var sessionID string
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			ev, ok := parseStreamJSONLine(line)
			if !ok {
				continue
			}
			if ev.sessionID != "" {
				sessionID = ev.sessionID
			}
			if len(ev.slashCommands) > 0 {
				rememberReportedCommands(provider.ProviderClaudeCodeCLI, ev.slashCommands)
			}
			if ev.textDelta != "" {
				send(ctx, ch, provider.StreamChunk{Content: ev.textDelta})
			}
			if ev.isFinal {
				if ev.errText != "" {
					send(ctx, ch, provider.StreamChunk{Error: ev.errText, Done: true, CLISessionID: sessionID})
				} else {
					send(ctx, ch, provider.StreamChunk{Done: true, FinishReason: "stop", CLISessionID: sessionID})
				}
				_ = cmd.Wait()
				return
			}
		}

		// Every branch below must send a terminal chunk (Done or Error) —
		// same rule already established for internal/app/llm.go's streaming
		// loops (AGENTS.md "Streaming / SSE" gotcha): a goroutine that just
		// returns without one leaves the frontend waiting forever.
		if waitErr := cmd.Wait(); waitErr != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = waitErr.Error()
			}
			send(ctx, ch, provider.StreamChunk{Error: "claude CLI: " + msg, Done: true, CLISessionID: sessionID})
			return
		}
		// Process exited cleanly but the stream never carried an explicit
		// "result" event — shouldn't happen against a real binary (verified
		// shape, see parseStreamJSONLine's doc comment) but still needs a
		// terminal chunk sent regardless, per this file's own rule above.
		send(ctx, ch, provider.StreamChunk{Done: true, FinishReason: "stop", CLISessionID: sessionID})
	}()

	return ch, nil
}

// send delivers chunk to ch, preferring the send over ctx cancellation —
// local copy of the same non-blocking-then-ctx-aware pattern used by
// provider.trySend/internal/app/llm.go's trySend (unexported in both, so
// duplicated here rather than exported just for this).
func send(ctx context.Context, ch chan<- provider.StreamChunk, chunk provider.StreamChunk) {
	select {
	case ch <- chunk:
		return
	default:
	}
	select {
	case ch <- chunk:
	case <-ctx.Done():
	}
}

// lastUserMessage returns the most recent user-role message's text content,
// or "" if there isn't one.
func lastUserMessage(messages []provider.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		return messageText(messages[i].Content)
	}
	return ""
}

// messageText extracts plain text from a provider.Message.Content value,
// which is either a plain string or []provider.ContentPart (multimodal).
// Image parts are dropped — the claude CLI is invoked as a plain positional
// argument, there's no multipart request to put them in.
func messageText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []provider.ContentPart:
		var b strings.Builder
		for _, p := range v {
			if p.Type == "text" {
				b.WriteString(p.Text)
			}
		}
		return b.String()
	default:
		return ""
	}
}

// streamEvent is the subset of one `claude --output-format stream-json`
// line this package acts on.
type streamEvent struct {
	sessionID string
	textDelta string
	isFinal   bool
	errText   string
	// slashCommands is the full command list the CLI reports about itself on
	// its init event — the only place it is exposed (there is no --list flag,
	// checked). Recorded so the composer's "/" dropdown can offer everything
	// the CLI actually has, including skills bundled inside the CLI package
	// that no directory scan on Memo's side could find.
	slashCommands []string
}

// parseStreamJSONLine interprets one line of `claude --output-format
// stream-json` output, per Claude Code's own documented event schema:
// {"type":"system","subtype":"init","session_id":"..."} on start,
// {"type":"assistant","message":{"content":[{"type":"text","text":"..."}]}}
// per text delta, and a terminal {"type":"result",...,"session_id":"...",
// "is_error":bool,"result":"..."}. Anything else (tool-call events, etc.) is
// silently ignored — opaque to Memo by design, see yapacam.md §5.
//
// Verified 2026-08-02 against a real `claude --version` 2.1.220 binary's
// actual --dangerously-skip-permissions output (also confirmed that flag
// itself works: the init event's "permissionMode" came back
// "bypassPermissions") — every field this function reads matched exactly,
// plus several unhandled event types (hook_started/hook_response,
// rate_limit_event) that fall through the switch's default case harmlessly,
// as intended.
func parseStreamJSONLine(line []byte) (streamEvent, bool) {
	var raw struct {
		Type          string   `json:"type"`
		SessionID     string   `json:"session_id"`
		SlashCommands []string `json:"slash_commands"`
		IsError       bool     `json:"is_error"`
		Result        string   `json:"result"`
		Message       struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return streamEvent{}, false
	}

	switch raw.Type {
	case "system":
		return streamEvent{sessionID: raw.SessionID, slashCommands: raw.SlashCommands}, true
	case "assistant":
		var b strings.Builder
		for _, part := range raw.Message.Content {
			if part.Type == "text" {
				b.WriteString(part.Text)
			}
		}
		return streamEvent{sessionID: raw.SessionID, textDelta: b.String()}, true
	case "result":
		ev := streamEvent{sessionID: raw.SessionID, isFinal: true}
		if raw.IsError {
			ev.errText = raw.Result
		}
		return ev, true
	default:
		return streamEvent{}, false
	}
}
