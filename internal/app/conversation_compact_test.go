package app

import (
	"context"
	"strings"
	"testing"

	"memo/internal/api"
	"memo/internal/config"
)

func longHistory(n int) []api.Message {
	msgs := make([]api.Message, 0, n)
	for i := 0; i < n; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		// ~30 tokens each so a modest count clears a small budget.
		msgs = append(msgs, api.NewTextMessage(role, strings.Repeat("word ", 30)+"#"))
	}
	return msgs
}

func TestConversationSig_StableAndSensitive(t *testing.T) {
	a := longHistory(6)
	if conversationSig(a) != conversationSig(a) {
		t.Fatal("sig not stable for the same slice")
	}
	b := append([]api.Message{}, a...)
	b[2] = api.NewTextMessage(b[2].Role, "changed content")
	if conversationSig(a) == conversationSig(b) {
		t.Fatal("sig did not change when a message changed")
	}
	if conversationSig(a[:4]) == conversationSig(a[:5]) {
		t.Fatal("sig did not change with slice length")
	}
}

func TestMaybeCompactHistory_BelowThresholdIsNoop(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	a.cfg.AgentMode.ConversationCompactEnabled = true
	a.cfg.AgentMode.CompactThresholdPct = 60

	h := longHistory(10)
	got := a.maybeCompactHistory(context.Background(), "c1", h, 1_000_000, 0) // huge window
	if len(got) != len(h) {
		t.Fatalf("expected history untouched below threshold, got %d want %d", len(got), len(h))
	}
}

func TestMaybeCompactHistory_DisabledIsNoop(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	a.cfg.AgentMode.ConversationCompactEnabled = false
	h := longHistory(20)
	got := a.maybeCompactHistory(context.Background(), "c1", h, 100, 0)
	if len(got) != len(h) {
		t.Fatalf("disabled: history should be untouched, got %d want %d", len(got), len(h))
	}
}

// TestMaybeCompactHistory_UsesCachedSummary drives the real compaction
// assembly (over-threshold, split, prepend summary system message) without a
// provider by pre-seeding the cache with a matching signature.
func TestMaybeCompactHistory_UsesCachedSummary(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	a.cfg.AgentMode.ConversationCompactEnabled = true
	a.cfg.AgentMode.CompactThresholdPct = 60

	h := longHistory(20)
	cut := len(h) * 6 / 10
	a.convSummaries = map[string]*convSummary{
		"c1": {coveredCount: cut, prefixSig: conversationSig(h[:cut]), text: "- did X\n- decided Y"},
	}

	// Budget small enough that the ~20*~31-token history is well over 60%.
	got := a.maybeCompactHistory(context.Background(), "c1", h, 400, 0)

	if len(got) != (len(h)-cut)+1 {
		t.Fatalf("compacted length = %d, want %d (recent tail + 1 summary)", len(got), (len(h)-cut)+1)
	}
	if got[0].Role != "system" || !strings.Contains(got[0].GetTextContent(), "did X") {
		t.Fatalf("first message should be the cached summary as a system message, got %+v", got[0])
	}
	if got[0].GetTextContent() == "" || !strings.Contains(got[0].GetTextContent(), "Earlier conversation summary") {
		t.Fatalf("summary header missing: %q", got[0].GetTextContent())
	}
	// The recent tail must be preserved verbatim and in order.
	for i, m := range got[1:] {
		if m.GetTextContent() != h[cut+i].GetTextContent() {
			t.Fatalf("recent tail message %d altered", i)
		}
	}
}

// TestCompactSummaryHeader_StatesCurrentInfoWins guards a live report: the
// Telegram/WhatsApp self-chat sessions (handleTelegramMessage/
// handleWhatsAppSelfChatMessage) reuse ONE session forever, unlike the
// Flutter UI where a user naturally starts fresh chats — so they're far
// more likely to actually be carrying a cached compacted-history summary
// that describes something (a preference, a fact) which has since been
// superseded by a newly pinned fact. Without an explicit precedence
// sentence, nothing ever told the model that current instructions/memory
// beat an old "earlier in this conversation" summary if the two conflict.
func TestCompactSummaryHeader_StatesCurrentInfoWins(t *testing.T) {
	if !strings.Contains(strings.ToLower(compactSummaryHeader), "outdated") {
		t.Fatalf("compactSummaryHeader must warn the model this summary can be outdated, got: %q", compactSummaryHeader)
	}
	if !strings.Contains(strings.ToLower(compactSummaryHeader), "current") {
		t.Fatalf("compactSummaryHeader must tell the model current info wins on conflict, got: %q", compactSummaryHeader)
	}
}

// TestMaybeCompactHistory_CachedSummaryKeepsMinTail guards BUG-SCAN4: the
// cached-summary reuse path overwrote `cut` with cached.coveredCount
// without re-checking the min-tail guard the fresh path applies. If the
// history shrank back to (or past) coveredCount — user edited/deleted/
// branched recent turns — reuse spliced the verbatim tail down to nothing
// and returned only the summary.
func TestMaybeCompactHistory_CachedSummaryKeepsMinTail(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	a.cfg.AgentMode.ConversationCompactEnabled = true
	a.cfg.AgentMode.CompactThresholdPct = 60

	// A 30-message history compacted with coveredCount=18...
	full := longHistory(30)
	a.convSummaries = map[string]*convSummary{
		"c1": {coveredCount: 18, prefixSig: conversationSig(full[:18]), text: "- did X"},
	}

	// ...then the user trims recent turns so only those first 18 remain.
	shrunk := full[:18]
	got := a.maybeCompactHistory(context.Background(), "c1", shrunk, 400, 0)

	if len(got) != len(shrunk) {
		t.Fatalf("history collapsed to %d messages (summary=%v); the whole visible "+
			"conversation was replaced by the cached summary", len(got),
			len(got) > 0 && got[0].Role == "system")
	}
}

// cachedSummaryFor seeds the summary cache so a compaction that fires can be
// observed without a provider: the result starts with the cached summary.
func cachedSummaryFor(a *App, h []api.Message) {
	cut := len(h) * 6 / 10
	a.convSummaries = map[string]*convSummary{
		"c1": {coveredCount: cut, prefixSig: conversationSig(h[:cut]), text: "- did X"},
	}
}

func compacted(got []api.Message) bool {
	return len(got) > 0 && got[0].Role == "system" && strings.Contains(got[0].GetTextContent(), "did X")
}

// longHistory(20) is 1000 estimated tokens (20 × 151 chars / 3). The threshold
// is a share of the WINDOW, over the whole prompt.
func TestMaybeCompactHistory_NinetyPercentOfTheWindow(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	a.cfg.AgentMode.ConversationCompactEnabled = true
	a.cfg.AgentMode.CompactThresholdPct = 90
	h := longHistory(20)
	cachedSummaryFor(a, h)

	if got := a.maybeCompactHistory(context.Background(), "c1", h, 1300, 0); compacted(got) {
		t.Fatal("history at ~77% of the window must not be compacted at a 90% threshold")
	}
	if got := a.maybeCompactHistory(context.Background(), "c1", h, 1050, 0); !compacted(got) {
		t.Fatal("history at ~95% of the window must be compacted")
	}
}

// The system prompt, the tool schema and the new message take context too: a
// history that would fit alone still has to be condensed when they push the
// whole prompt past the threshold.
func TestMaybeCompactHistory_CountsTheFixedPartOfThePrompt(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	a.cfg.AgentMode.ConversationCompactEnabled = true
	a.cfg.AgentMode.CompactThresholdPct = 90
	h := longHistory(20)
	cachedSummaryFor(a, h)

	if got := a.maybeCompactHistory(context.Background(), "c1", h, 1500, 0); compacted(got) {
		t.Fatal("history alone (~67%) is under the threshold")
	}
	if got := a.maybeCompactHistory(context.Background(), "c1", h, 1500, 400); !compacted(got) {
		t.Fatal("history + 400 tokens of system/tools/message (~93%) must be compacted")
	}
}

// The provider's own count from the previous turn catches what len/3
// under-counts: the estimate says 50%, the provider said the conversation
// already stood at 90%.
func TestMaybeCompactHistory_TrustsTheProvidersLastCount(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	a.cfg.AgentMode.ConversationCompactEnabled = true
	a.cfg.AgentMode.CompactThresholdPct = 90
	h := longHistory(20)
	cachedSummaryFor(a, h)
	providerName, model := a.activeModelIdentity()

	a.ctxState.usage = map[string]turnUsage{"c1": {prompt: 1700, completion: 100, real: true, provider: providerName, model: model, window: 2000}}
	if got := a.maybeCompactHistory(context.Background(), "c1", h, 2000, 0); !compacted(got) {
		t.Fatal("a real 1800-token conversation in a 2000-token window must be compacted")
	}

	// A count the provider did not report is only an estimate: not trusted.
	a.ctxState.usage["c1"] = turnUsage{prompt: 1700, completion: 100, real: false, provider: providerName, model: model, window: 2000}
	if got := a.maybeCompactHistory(context.Background(), "c1", h, 2000, 0); compacted(got) {
		t.Fatal("an estimated count must not trigger compaction by itself")
	}
	// Another model's tokens say nothing about this one.
	a.ctxState.usage["c1"] = turnUsage{prompt: 1700, completion: 100, real: true, provider: "other", model: "x", window: 2000}
	if got := a.maybeCompactHistory(context.Background(), "c1", h, 2000, 0); compacted(got) {
		t.Fatal("a count from a different model must be ignored")
	}
}
