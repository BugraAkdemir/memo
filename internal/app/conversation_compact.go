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
	coveredCount int    // how many leading history messages this summary condenses
	prefixSig    string // signature of history[:coveredCount] — to detect edits
	text         string
}

// compactRegionGrowthSlack is how many messages the to-be-condensed region
// may grow past a cached summary's coverage before we re-summarize. Without
// it the region (len*0.6) shifts every turn and the LLM call never gets
// cached; with it a 30-turn session re-summarizes a handful of times, not
// every other turn.
const compactRegionGrowthSlack = 8

// compactSummaryHeader's precedence sentence exists for a real, reported
// failure mode specific to the long-lived Telegram/WhatsApp self-chat
// sessions (handleTelegramMessage/handleWhatsAppSelfChatMessage): unlike
// the Flutter UI, where a user naturally starts a fresh chat now and then,
// those two reuse ONE session forever, so they're far more likely to have
// actually hit CompactThresholdPct and be carrying a cached summary. That
// summary is an accurate record of what was said AT THE TIME — if the
// user has since pinned a fact that supersedes something stated back then
// (a favorite color, a job, whatever), the fresh pinned-fact/RAG block in
// systemPrompt (assembled earlier in this same message list, in
// buildMessagesForSession) is just as easy for the model to weigh equally
// against, or even under, an "earlier in this very conversation" summary
// with no explicit signal that the summary can be stale. Without this
// sentence, nothing ever told the model which one wins.
const compactSummaryHeader = "[Earlier conversation summary — the turns before this point were condensed to save context; treat it as background, not as something the user just said. This summary reflects what was true AT THAT TIME and may be outdated — if anything here conflicts with your current instructions or memory above, or with what the user says now, treat the current information as correct.]\n"

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

	a.convSummaryMu.Lock()
	cached := a.convSummaries[chatID]
	a.convSummaryMu.Unlock()

	// Reuse the cached summary when it still condenses an unchanged prefix of
	// this history and the ideal cut hasn't outgrown that prefix by more than
	// the slack — the newer turns just go into the verbatim tail. The
	// len(history)-coveredCount >= 2 check is the same min-tail guard the
	// fresh path applies above: without it, a history that shrank back to
	// (or just past) coveredCount — user edited/deleted/branched recent
	// turns — would splice history[coveredCount:] down to zero or one
	// message and hand the model nothing but the summary (BUG-SCAN4).
	if cached != nil && cached.coveredCount <= len(history)-2 &&
		cut <= cached.coveredCount+compactRegionGrowthSlack &&
		conversationSig(history[:cached.coveredCount]) == cached.prefixSig {
		cut = cached.coveredCount
		summary := cached.text
		out := make([]api.Message, 0, len(history)-cut+1)
		out = append(out, api.NewTextMessage("system", compactSummaryHeader+summary))
		out = append(out, history[cut:]...)
		return out
	}

	old, recent := history[:cut], history[cut:]
	summary := a.summarizeHistorySlice(ctx, old)
	if summary == "" {
		logx.Printf("CONTEXT: conversation compaction produced nothing, keeping raw history (%d msgs)", len(history))
		return history
	}
	a.convSummaryMu.Lock()
	if a.convSummaries == nil {
		a.convSummaries = map[string]*convSummary{}
	}
	a.convSummaries[chatID] = &convSummary{
		coveredCount: cut,
		prefixSig:    conversationSig(old),
		text:         summary,
	}
	a.convSummaryMu.Unlock()

	logx.Printf("CONTEXT: compacted %d old messages into a %d-token summary, %d recent kept verbatim",
		len(old), truncate.EstimateTokens(summary), len(recent))

	out := make([]api.Message, 0, len(recent)+1)
	out = append(out, api.NewTextMessage("system", compactSummaryHeader+summary))
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
