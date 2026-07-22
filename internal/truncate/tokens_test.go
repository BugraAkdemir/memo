package truncate

import "testing"

func TestText(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		// Multi-byte UTF-8 (Turkish, CJK): must cut on rune boundaries, not
		// byte offsets — a byte-slice truncation would corrupt these.
		{"日本語", 2, "日本..."},
		{"ığüşöç", 3, "ığü..."},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Text(tt.input, tt.n)
			if got != tt.want {
				t.Errorf("Text(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input string
		want  int // approximate
	}{
		{"", 0},
		{"hello world", 3},  // 11 chars / 3 = 3
		{"Merhaba dünya", 4}, // 13 chars / 3 = 4
	}
	for _, tc := range tests {
		got := EstimateTokens(tc.input)
		if got != tc.want {
			t.Errorf("EstimateTokens(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestTruncateMessages_Empty(t *testing.T) {
	got := TruncateMessages(nil, 1000)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestTruncateMessages_NoBudget(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello"},
	}
	got := TruncateMessages(msgs, 0)
	if len(got) != 2 {
		t.Errorf("expected 2 messages (no truncation), got %d", len(got))
	}
}

func TestTruncateMessages_PreservesSystem(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
		{Role: "user", Content: "c"},
	}
	got := TruncateMessages(msgs, 5) // tight budget
	// System must be preserved
	if len(got) < 1 || got[0].Role != "system" {
		t.Errorf("system prompt not preserved: %+v", got)
	}
}

func TestTruncateMessages_DropsOldestFirst(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "old message that should be dropped first because it is very long"},
		{Role: "assistant", Content: "middle"},
		{Role: "user", Content: "newest"},
	}
	got := TruncateMessages(msgs, 10)
	// Budget 10: system ~1 token → 9 left for history
	// From end: "newest" ~2, "middle" ~2, "old..." ~25
	// So keeps: sys + middle + newest = 3
	if len(got) == 4 {
		t.Errorf("should have dropped oldest, got all 4: %+v", got)
	}
	if got[0].Role != "system" {
		t.Errorf("system prompt should be first: %+v", got)
	}
}

func TestTruncateMessages_LargeBudget(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
	}
	got := TruncateMessages(msgs, 100000)
	if len(got) != 3 {
		t.Errorf("expected all 3 messages to fit, got %d", len(got))
	}
}

func TestTruncateMessages_TightBudget(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Can you help me with something?"},
		{Role: "assistant", Content: "Sure, what do you need?"},
	}
	got := TruncateMessages(msgs, 10)
	// Budget 10: system is ~7 tokens, so only ~3 tokens left for history
	// "Can you help..." is ~10 tokens alone, so it won't fit
	if len(got) != 1 {
		t.Errorf("expected only system prompt, got %d messages", len(got))
	}
}

// TestTruncateMessages_KeepsAssistantWithItsToolResults is the regression
// test for BUG-M6: pipeline.go's own doc comment claims this drops "oldest
// assistant+tool message pairs," but TruncateMessages has no concept of
// pairing at all — it's a plain backward token-budget scan over messages —
// so a cutoff could land between an assistant's tool_calls and its own
// tool-result messages, keeping a lone "tool"-role message with no
// preceding assistant declaring its tool_call_id. That's an invalid
// message sequence for a provider's ChatCompletion API.
//
// EstimateTokens is len(text)/3. Content lengths below are chosen so the
// backward scan (budget = maxTokens - systemTokens = 5 - 1 = 4) keeps only
// the final "tool" message (9 chars = 3 tokens, fits) and then breaks
// before the second tool message (3+3=6 > 4) — without the fix, that
// leaves the kept slice starting on a lone tool message. The old user
// message is padded long enough that it's never a candidate either way.
func TestTruncateMessages_KeepsAssistantWithItsToolResults(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "sys"},                                        // 1 token
		{Role: "user", Content: "an old unrelated user turn, long padding here"}, // ~15 tokens
		{Role: "assistant", Content: "call tool"},                               // 3 tokens
		{Role: "tool", Content: "result aa"},                                    // 3 tokens
		{Role: "tool", Content: "result bb"},                                    // 3 tokens
	}
	got := TruncateMessages(msgs, 5)

	if len(got) == 0 {
		t.Fatal("expected at least the system message")
	}
	for i, m := range got {
		if m.Role == "tool" {
			hasPrecedingAssistant := false
			for j := i - 1; j >= 0; j-- {
				if got[j].Role == "assistant" {
					hasPrecedingAssistant = true
					break
				}
				if got[j].Role != "tool" {
					break
				}
			}
			if !hasPrecedingAssistant {
				t.Fatalf("got[%d] is a \"tool\" message with no preceding assistant message in the kept slice — invalid message sequence: %+v", i, got)
			}
		}
	}
	// The whole assistant+tool-results group must survive together, not
	// just avoid being invalid — confirms the fix keeps the group instead
	// of e.g. dropping tool messages down to nothing.
	if got[len(got)-1].Content != "result bb" {
		t.Errorf("expected the last kept message to still be the final tool result, got %+v", got)
	}
	foundAssistant := false
	for _, m := range got {
		if m.Role == "assistant" {
			foundAssistant = true
		}
	}
	if !foundAssistant {
		t.Errorf("expected the assistant message to be kept alongside its tool results, got %+v", got)
	}
}
