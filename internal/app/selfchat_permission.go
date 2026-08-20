package app

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"memo/internal/agent"
	"memo/internal/api"
	"memo/internal/logx"
)

// drainSelfChatReply is a self-chat-surface-aware replacement for
// drainToReply (chat.go): the plain version silently discards every
// "agent_event" chunk, including permission_request — a self-chat message
// that triggers a Medium/Dangerous tool call just stalled for the agent
// pipeline's own 60s permission timeout and returned that timeout error as
// the assistant's "reply", with no chance for the user to ever actually
// grant or deny it (there is no permission dialog reachable from WhatsApp/
// Telegram chat text). This resolves a permission_request itself instead:
// autoApprove skips straight to AllowOnce; otherwise buildQuestion formats
// a y/n prompt, sendQuestion delivers it via the surface's own send API,
// and awaitAnswer blocks on that surface's pending-answer channel (wired up
// by its own message loop — see routeWhatsAppPermissionAnswer/
// routeTelegramPermissionAnswer) for the reply.
func (a *App) drainSelfChatReply(
	ch <-chan api.StreamChunk,
	autoApprove bool,
	buildQuestion func(ev agent.AgentEvent) string,
	sendQuestion func(text string) error,
	awaitAnswer func(ctx context.Context) (string, bool),
) string {
	var reply strings.Builder
	for chunk := range ch {
		if chunk.Error != "" {
			return chunk.Error
		}
		if chunk.FinishReason == "agent_event" {
			var ev agent.AgentEvent
			if err := json.Unmarshal([]byte(chunk.Content), &ev); err == nil && ev.Type == agent.EventPermissionRequest {
				a.resolveSelfChatPermission(ev, autoApprove, buildQuestion, sendQuestion, awaitAnswer)
			}
			continue
		}
		if chunk.FinishReason != "" {
			continue
		}
		reply.WriteString(chunk.Content)
	}
	return reply.String()
}

// resolveSelfChatPermission answers a single permission_request event —
// see drainSelfChatReply's doc comment for the full picture.
func (a *App) resolveSelfChatPermission(
	ev agent.AgentEvent,
	autoApprove bool,
	buildQuestion func(ev agent.AgentEvent) string,
	sendQuestion func(text string) error,
	awaitAnswer func(ctx context.Context) (string, bool),
) {
	if autoApprove {
		_ = a.HandleAgentPermission(ev.RequestID, string(agent.AllowOnce))
		return
	}

	if err := sendQuestion(buildQuestion(ev)); err != nil {
		logx.Printf("self-chat permission: send question error: %v", err)
		_ = a.HandleAgentPermission(ev.RequestID, string(agent.DenyOnce))
		return
	}

	// 45s, not the pipeline's own full 60s: sendQuestion's round-trip and
	// everything spent generating up to this tool call already eat into
	// that same 60s budget, so waiting the full 60 here would race the
	// pipeline's own timer instead of safely beating it to a real answer.
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	answer, ok := awaitAnswer(ctx)
	if !ok {
		// No reply in time — the pipeline's own 60s timer auto-denies and
		// aborts the stream on its own; nothing further to do here.
		return
	}

	policy := agent.DenyOnce
	if isAffirmativeAnswer(answer) {
		policy = agent.AllowOnce
	}
	_ = a.HandleAgentPermission(ev.RequestID, string(policy))
}

// isAffirmativeAnswer recognizes a y/n reply bilingually — a self-chat
// user types in whatever language they're already speaking, independent of
// the app's own UILanguage setting (which only decides which language *we*
// ask the question in, not what a reply is allowed to look like).
func isAffirmativeAnswer(text string) bool {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "y", "yes", "evet", "e", "onay", "onayla", "onaylıyorum", "tamam":
		return true
	default:
		return false
	}
}
