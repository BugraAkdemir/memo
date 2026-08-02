// SPDX-License-Identifier: AGPL-3.0-or-later

package agentcli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"memo/internal/provider"
)

func init() {
	provider.RegisterConstructor(provider.ProviderCodexCLI, func(cfg provider.ProviderConfig) (provider.Provider, error) {
		return NewCodexCLI(cfg)
	})
}

// CodexCLI shells out to the user's locally installed `codex` binary, one
// subprocess per message — same shape as ClaudeCodeCLI, see that type's doc
// comment for the shared design rationale (per-chat session via --resume,
// not resent history).
type CodexCLI struct {
	// binaryPath mirrors ClaudeCodeCLI.binaryPath — reuses
	// ProviderConfig.BaseURL as a manual override for the same reason.
	binaryPath string
	model      string
}

// NewCodexCLI builds a CodexCLI from cfg. Never fails on its own — see
// NewClaudeCodeCLI's doc comment.
func NewCodexCLI(cfg provider.ProviderConfig) (*CodexCLI, error) {
	bin := strings.TrimSpace(cfg.BaseURL)
	if bin == "" {
		bin = "codex"
	}
	model := cfg.Model
	if model == "" {
		model = "codex"
	}
	return &CodexCLI{binaryPath: bin, model: model}, nil
}

func (c *CodexCLI) Name() provider.ProviderType { return provider.ProviderCodexCLI }
func (c *CodexCLI) DisplayName() string         { return "Codex (CLI)" }

// ListModels returns a single synthetic entry — same reasoning as
// ClaudeCodeCLI.ListModels.
func (c *CodexCLI) ListModels(ctx context.Context) ([]string, error) {
	return []string{"codex"}, nil
}

// ChatCompletion drains ChatCompletionStream into a single response.
func (c *CodexCLI) ChatCompletion(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
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

// ChatCompletionStream runs `codex exec --json
// --dangerously-bypass-approvals-and-sandbox [-C <workdir>] "<message>"` for
// a fresh chat, or `codex exec resume <thread-id> --json
// --dangerously-bypass-approvals-and-sandbox "<message>"` when
// req.ResumeSessionID is set (resume carries its own working directory from
// the original session — codex rejects -C there, unlike the fresh-run form).
//
// --dangerously-bypass-approvals-and-sandbox is the codex equivalent of
// ClaudeCodeCLI's --dangerously-skip-permissions — same rationale: headless
// mode has no terminal to approve a prompt, so without it every file/command
// action would just block forever.
func (c *CodexCLI) ChatCompletionStream(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	userMsg := lastUserMessage(req.Messages)
	if userMsg == "" {
		return nil, fmt.Errorf("codex-cli: no user message to send")
	}
	// Unlike Claude Code, `codex exec` does not resolve slash commands
	// itself — see ExpandCommand's doc comment for the verification. Expand
	// here so a codex chat's "/" commands do the same thing they do in
	// codex's own TUI; an unknown command falls through unchanged rather
	// than being swallowed.
	if expanded, ok := ExpandCommand(provider.ProviderCodexCLI, req.WorkDir, userMsg); ok {
		userMsg = expanded
	}

	var args []string
	if req.ResumeSessionID != "" {
		args = []string{"exec", "resume", req.ResumeSessionID, "--json", "--dangerously-bypass-approvals-and-sandbox", userMsg}
	} else {
		args = []string{"exec", "--json", "--dangerously-bypass-approvals-and-sandbox"}
		if req.WorkDir != "" {
			args = append(args, "-C", req.WorkDir)
		}
		args = append(args, userMsg)
	}

	cmd := execCommandContext(ctx, c.binaryPath, args...)
	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("codex-cli: stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("codex-cli: could not start %q — is the Codex CLI installed and on PATH? (%w)", c.binaryPath, err)
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
			ev, ok := parseCodexJSONLine(line)
			if !ok {
				continue
			}
			if ev.sessionID != "" {
				sessionID = ev.sessionID
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

		// Every branch below must send a terminal chunk (Done or Error) — see
		// ClaudeCodeCLI.ChatCompletionStream's identical comment.
		if waitErr := cmd.Wait(); waitErr != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = waitErr.Error()
			}
			send(ctx, ch, provider.StreamChunk{Error: "codex CLI: " + msg, Done: true, CLISessionID: sessionID})
			return
		}
		send(ctx, ch, provider.StreamChunk{Done: true, FinishReason: "stop", CLISessionID: sessionID})
	}()

	return ch, nil
}

// parseCodexJSONLine interprets one line of `codex exec --json` output, per
// codex-cli 0.146.0's actual event schema (verified 2026-08-02 against a
// real `codex` binary):
// {"type":"thread.started","thread_id":"..."} once at the start,
// {"type":"item.completed","item":{"type":"agent_message","text":"..."}}
// with the model's full reply text (codex emits this as one complete chunk
// per turn, not incremental deltas the way Claude Code's assistant events
// do — the frontend's existing "still working" cue, built for Claude's own
// bursty gaps, already covers this),
// {"type":"turn.completed",...} on success, and
// {"type":"turn.failed","error":{"message":"..."}} on failure. Other item
// types (command_execution, reasoning, the top-level "item.started"/"error"
// events) are silently ignored — opaque to Memo by design, same as
// parseStreamJSONLine's unhandled Claude Code event types.
func parseCodexJSONLine(line []byte) (streamEvent, bool) {
	var raw struct {
		Type     string `json:"type"`
		ThreadID string `json:"thread_id"`
		Item     struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"item"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return streamEvent{}, false
	}

	switch raw.Type {
	case "thread.started":
		return streamEvent{sessionID: raw.ThreadID}, true
	case "item.completed":
		if raw.Item.Type != "agent_message" {
			return streamEvent{}, false
		}
		return streamEvent{textDelta: raw.Item.Text}, true
	case "turn.completed":
		return streamEvent{isFinal: true}, true
	case "turn.failed":
		return streamEvent{isFinal: true, errText: raw.Error.Message}, true
	default:
		return streamEvent{}, false
	}
}
