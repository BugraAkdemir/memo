package app

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"

	"memo/internal/api"
	"memo/internal/logx"
	"memo/internal/truncate"
)

// convSummary is the cached result of one maybeCompactHistory summarization.
type convSummary struct {
	coveredSig string // signature of the message slice this summary condensed
	text       string
}

// maybeCompactHistory condenses the oldest part of a long conversation into a
// single summary system message, keeping the recent tail verbatim, instead of
// letting the token-aware fetch silently drop early turns. Interactive chat
// and agent mode had no equivalent of the task loop's compactPlanState.
//
// It fires only when history already occupies >= CompactThresholdPct of the
// turn's context budget, and the summarization LLM call is skipped whenever
// the region being condensed is unchanged from the last call for this chat
// (the common case: a long session growing one turn at a time). On any
// failure it returns the history untouched.
func (a *App) maybeCompactHistory(ctx context.Context, chatID string, history []api.Message, tokenBudget int) []api.Message {
	a.cfgMu.RLock()
	enabled := a.cfg.AgentMode.ConversationCompactEnabled
	thresholdPct := a.cfg.AgentMode.CompactThresholdPct
	a.cfgMu.RUnlock()

	if !enabled || chatID == "" || tokenBudget <= 0 || len(history) < 8 {
		return history
	}
	if thresholdPct <= 0 || thresholdPct > 95 {
		thresholdPct = 60
	}

	used := 0
	for _, h := range history {
		used += truncate.EstimateTokens(h.GetTextContent())
	}
	if used*100 < thresholdPct*tokenBudget {
		return history
	}

	// Condense the oldest ~60% on a message boundary, keep the newest ~40%.
	cut := len(history) * 6 / 10
	if cut < 2 || len(history)-cut < 2 {
		return history
	}
	old, recent := history[:cut], history[cut:]

	sig := conversationSig(old)
	a.convSummaryMu.Lock()
	cached := a.convSummaries[chatID]
	a.convSummaryMu.Unlock()

	summary := ""
	if cached != nil && cached.coveredSig == sig {
		summary = cached.text
	} else {
		summary = a.summarizeHistorySlice(ctx, old)
		if summary == "" {
			logx.Printf("CONTEXT: conversation compaction produced nothing, keeping raw history (%d msgs)", len(history))
			return history
		}
		a.convSummaryMu.Lock()
		if a.convSummaries == nil {
			a.convSummaries = map[string]*convSummary{}
		}
		a.convSummaries[chatID] = &convSummary{coveredSig: sig, text: summary}
		a.convSummaryMu.Unlock()
	}

	logx.Printf("CONTEXT: compacted %d old messages into a %d-token summary, %d recent kept verbatim",
		len(old), truncate.EstimateTokens(summary), len(recent))

	out := make([]api.Message, 0, len(recent)+1)
	out = append(out, api.NewTextMessage("system",
		"[Earlier conversation summary — the turns before this point were condensed to save context; treat it as background, not as something the user just said]\n"+summary))
	out = append(out, recent...)
	return out
}

// summarizeHistorySlice runs the single compaction LLM call. Reuses the task
// loop's single-completion helper and its "compaction" usage category.
func (a *App) summarizeHistorySlice(ctx context.Context, msgs []api.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		role := m.Role
		if role == "" {
			role = "user"
		}
		fmt.Fprintf(&b, "%s: %s\n", role, m.GetTextContent())
	}
	req := []api.Message{
		api.NewTextMessage("system", "Condense this earlier stretch of a conversation into a compact briefing for continuing it. Keep concrete facts, decisions, open questions, file/name/number details, and anything the user asked for that isn't done yet. Drop pleasantries and repetition. Write terse bullet points, no preamble."),
		api.NewTextMessage("user", b.String()),
	}
	raw := a.callLLMForReviewWith(ctx, req, categoryCompaction, nil)
	if strings.HasPrefix(raw, "⚠") {
		return ""
	}
	return strings.TrimSpace(raw)
}

// conversationSig is a cheap content signature of a message slice, used to
// tell whether the region being summarized has changed since last time.
func conversationSig(msgs []api.Message) string {
	h := sha1.New()
	fmt.Fprintf(h, "%d\n", len(msgs))
	for _, m := range msgs {
		fmt.Fprintf(h, "%s\x00%s\x00", m.Role, m.GetTextContent())
	}
	return hex.EncodeToString(h.Sum(nil))
}
