package app

import "memo/internal/truncate"

// compactWindowLimit is the context window (tokens) below which a local model
// runs in the compact profile: a short system prompt with nothing personal in
// it, a reserved share for the reply, smaller memory and skill shares, no
// per-turn calendar or sub-agent blocks, and no extra LLM calls of Memo's own
// (title, fact extraction, history summary). Measured on 2026-11-10: the full
// system prompt alone is ~1.1k tokens and the agent tool schema ~10k, so a
// 4k–8k window had no room left for the conversation or the answer.
//
// Windows at or above the limit, and every cloud model, are untouched.
const compactWindowLimit = 10000

// isCompactWindow reports whether a model with this real context window gets
// the compact profile. 0 means "unknown" and never counts as small.
func isCompactWindow(window int) bool {
	return window > 0 && window < compactWindowLimit
}

// compactReplyReserve is the part of a compact window kept free for the reply.
// llama.max_tokens defaults to 0 (no limit) and the budget's own safety margin
// is only ~5%, so without this a filled 4k prompt left ~256 tokens to answer in.
func compactReplyReserve(window int) int {
	return window / 4
}

// compactMemoryShare and compactSkillShare cap the memory block and the
// active-skill instructions on a compact window (the regular local budget
// gives memory up to 2/5 of the prompt and skills 1/5).
func compactMemoryShare(window int) int { return window / 10 }
func compactSkillShare(window int) int  { return window * 7 / 100 }

// compactWorkingSetTokens caps the agent working-set digest on a compact
// window (the regular cap is agent_mode.working_set_max_tokens, default 600).
const compactWorkingSetTokens = 200

// clipToTokens shortens s to roughly maxTokens (EstimateTokens is len/3).
func clipToTokens(s string, maxTokens int) string {
	if truncate.EstimateTokens(s) <= maxTokens {
		return s
	}
	return truncate.Text(s, maxTokens*3)
}

// localCompactActive reports whether turns right now go to a running local
// model with a compact window and no cloud provider in front of it. Used by
// the work that happens outside buildMessagesForSession — the chat title and
// fact extraction — which would otherwise queue an extra request on the one
// local inference slot, minutes long on a CPU.
func (a *App) localCompactActive() bool {
	if a.llamaServer == nil || !a.llamaServer.IsRunning() || a.GetActiveProvider() != "" {
		return false
	}
	return isCompactWindow(a.localContextWindow())
}
