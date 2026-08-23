package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/sessions"
)

// fixedNow is a stable "now" for the pure-function tests: it really is a
// Sunday (the test asserts the rendered weekday, so this anchor matters).
var fixedNow = time.Date(2026, 8, 23, 14, 35, 0, 0, time.Local)

func TestTimeContextBlock_AlwaysStatesCurrentTime(t *testing.T) {
	got := timeContextBlock(fixedNow, time.Time{})
	if !strings.Contains(got, "Sunday, 23 August 2026, 14:35") {
		t.Errorf("block should state the current local time, got %q", got)
	}
	if strings.Contains(got, "ago") {
		t.Errorf("no last-activity signal should mean no gap sentence, got %q", got)
	}
}

func TestTimeContextBlock_FreshGapStaysSilent(t *testing.T) {
	// Same-session back-and-forth (and every WhatsApp self-chat turn) lands
	// below the threshold; naming such tiny gaps would be token noise.
	for _, gap := range []time.Duration{0, time.Minute, timeGapMentionThreshold - time.Second} {
		got := timeContextBlock(fixedNow, fixedNow.Add(-gap))
		if strings.Contains(got, "ago") {
			t.Errorf("gap %v (below threshold) should produce no elapsed sentence, got %q", gap, got)
		}
	}
}

func TestTimeContextBlock_GapMentioned(t *testing.T) {
	cases := []struct {
		gap  time.Duration
		want string
	}{
		{timeGapMentionThreshold, "15 minutes ago"},
		{90 * time.Minute, "an hour ago"},
		{26 * time.Hour, "a day ago"},
		{50 * time.Hour, "2 days ago"},
	}
	for _, tc := range cases {
		got := timeContextBlock(fixedNow, fixedNow.Add(-tc.gap))
		want := "Last message in this conversation was " + tc.want
		if !strings.Contains(got, want) {
			t.Errorf("gap %v: block should contain %q, got %q", tc.gap, want, got)
		}
	}
}

func TestTimeContextBlock_FutureTimestampIgnored(t *testing.T) {
	// Clock skew: a last-activity stamp in the future must not yield a
	// negative/nonsense elapsed sentence — treated as no signal at all.
	got := timeContextBlock(fixedNow, fixedNow.Add(time.Hour))
	if strings.Contains(got, "ago") {
		t.Errorf("future last-activity should produce no elapsed sentence, got %q", got)
	}
}

func TestLastActivity_ParsesUpdatedAtAndHandlesUnknownChat(t *testing.T) {
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	id := sm.NewChat()
	la := sm.LastActivity(id)
	if la.IsZero() {
		t.Fatal("LastActivity() = zero for a real chat; newSession seeds UpdatedAt so it must parse")
	}
	if d := time.Since(la); d < -time.Second || d > time.Minute {
		t.Errorf("LastActivity() = %v, want within the last minute of test execution", la)
	}
	if !sm.LastActivity("does-not-exist").IsZero() {
		t.Error("LastActivity() for an unknown chat id should be zero")
	}
}

// The wiring point matters more than the formatting: like the active-skill
// block, the time block must ride inside buildMessagesForSession's
// systemPrompt itself, because routeStream's later role:"system" append finds
// nothing when the local-model branch merges the prompt into a user message.
// And it must survive MinimalMode, which strips persona — not facts.
func TestBuildMessagesForSession_IncludesTimeContextEvenInMinimalMode(t *testing.T) {
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	id := identity.New("Test", "Memo", "casual", "", true) // MinimalMode on
	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: false},
			Llama:  config.LlamaConfig{CtxSize: 4096},
		},
		identity: id,
		sessions: sm,
	}

	chatID := sm.GetActiveID()
	messages := a.buildMessagesForSession(context.Background(), chatID, "hello", nil, nil)

	found := false
	for _, m := range messages {
		if content, ok := m.Content.(string); ok && strings.Contains(content, "[Time context]") {
			found = true
		}
	}
	if !found {
		t.Fatal("buildMessagesForSession() did not include the [Time context] block; MinimalMode strips persona, not temporal grounding")
	}
}
