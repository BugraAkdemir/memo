package truncate

import "testing"

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
