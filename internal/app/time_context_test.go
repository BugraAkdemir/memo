package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// seedStaleChat writes a session file whose UpdatedAt is ~3 days in the
// past, through the exact on-disk format save() produces (<id>.json,
// "updated_at": "2006-01-02 15:04"), and loads it back via NewManager —
// Manager's own API can only stamp UpdatedAt with "now", and backdating is
// precisely the scenario this feature exists for (a user returning after
// days).
func seedStaleChat(t *testing.T) (*sessions.Manager, string) {
	t.Helper()
	dir := t.TempDir()
	const id = "stale-chat-seed"
	stale := time.Now().Add(-72 * time.Hour)
	body := fmt.Sprintf(`{
  "id": %q,
  "title": "old chat",
  "created_at": %q,
  "updated_at": %q,
  "messages": [
    {"role": "user", "content": "merhaba", "timestamp": "10:00"},
    {"role": "assistant", "content": "selam", "timestamp": "10:00"}
  ]
}`, id, stale.Add(-time.Hour).Format("2006-01-02 15:04"), stale.Format("2006-01-02 15:04"))
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(body), 0644); err != nil {
		t.Fatalf("seed session file: %v", err)
	}
	sm, err := sessions.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return sm, id
}

// The gap sentence must survive the real prompt builder, not only the pure
// function: what the model actually sees comes out of
// buildMessagesForSession, and until now that path was only ever exercised
// with a fresh chat, where the threshold correctly suppresses the clause.
func TestBuildMessagesForSession_MentionsStaleConversationGap(t *testing.T) {
	sm, chatID := seedStaleChat(t)
	id := identity.New("Test", "Memo", "casual", "", false)
	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: false},
			Llama:  config.LlamaConfig{CtxSize: 4096},
		},
		identity: id,
		sessions: sm,
	}

	messages := a.buildMessagesForSession(context.Background(), chatID, "yeni mesaj", nil, nil)

	var all []string
	found := false
	for _, m := range messages {
		if content, ok := m.Content.(string); ok {
			all = append(all, content)
			if strings.Contains(content, "Last message in this conversation was 3 days ago.") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("buildMessagesForSession() did not mention the 3-day gap for a stale chat; prompt contents: %q", all)
	}
}

// LastActivity's design leans on one contract: AddMessageToSession refreshes
// UpdatedAt, so a once-stale chat reads as fresh again after a new turn —
// without this, every subsequent message in an old chat would keep claiming
// "last message was N days ago".
func TestLastActivity_RefreshesAfterNewMessage(t *testing.T) {
	sm, chatID := seedStaleChat(t)

	if age := time.Since(sm.LastActivity(chatID)); age < 71*time.Hour {
		t.Fatalf("seeded chat should read ~3 days stale, got age %v", age)
	}

	sm.AddMessageToSession(chatID, "user", "geri döndüm", "", "")

	if age := time.Since(sm.LastActivity(chatID)); age < -time.Second || age > time.Minute {
		t.Errorf("LastActivity() after AddMessageToSession should be ~now, got age %v", age)
	}
}
