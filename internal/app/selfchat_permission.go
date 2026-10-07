package app

import (
	"context"
	"strings"
	"time"

	"memo/internal/agent"
	"memo/internal/logx"
)

// resolveSelfChatPermission answers a single permission_request event —
// see drainSelfChatTurn's doc comment for the full picture.
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
