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
	got := a.maybeCompactHistory(context.Background(), "c1", h, 1_000_000) // huge budget
	if len(got) != len(h) {
		t.Fatalf("expected history untouched below threshold, got %d want %d", len(got), len(h))
	}
}

func TestMaybeCompactHistory_DisabledIsNoop(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	a.cfg.AgentMode.ConversationCompactEnabled = false
	h := longHistory(20)
	got := a.maybeCompactHistory(context.Background(), "c1", h, 100)
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
	got := a.maybeCompactHistory(context.Background(), "c1", h, 400)

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
