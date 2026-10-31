package sessions

import (
	"strings"
	"testing"
	"time"
)

func TestDatedHistoryKeepsSourceDatesAfterTruncationAndDoesNotGuessLegacyDates(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := m.NewChat()
	m.mu.Lock()
	m.sessions[id].Messages = []ChatMessage{
		{Role: "user", Content: "Yarın toplantı var", SentAt: "2026-10-31T23:50:00+03:00", Timestamp: "23:50"},
		{Role: "assistant", Content: "Yarın saat 12'de", SentAt: "2026-10-31T23:51:00+03:00"},
		{Role: "user", Content: "Bugün ne var?", SentAt: "2026-11-01T00:05:00+03:00"},
	}
	m.mu.Unlock()
	readAt := time.Date(2026, 11, 2, 10, 0, 0, 0, time.FixedZone("TRT", 3*3600))
	history := m.GetDatedHistoryForAPIForSession(id, 4096, 20, readAt)
	if !strings.Contains(history[0]["content"], "2026-10-31T23:50:00+03:00") || !strings.Contains(history[2]["content"], "2026-11-01T00:05:00+03:00") {
		t.Fatalf("missing days: %v", history)
	}
	if !strings.HasSuffix(history[1]["content"], "Yarın saat 12'de") {
		t.Fatal("content rewritten")
	}
	sameDay := m.GetDatedHistoryForAPIForSession(id, 4096, 20, time.Date(2026, 11, 1, 10, 0, 0, 0, readAt.Location()))
	if sameDay[2]["content"] != "Bugün ne var?" {
		t.Fatal("duplicated today's date")
	}
	// Keep only a suffix from the first day: the first surviving row must re-anchor.
	m.mu.Lock()
	m.sessions[id].Messages = m.sessions[id].Messages[:2]
	m.sessions[id].Messages[0].Content = strings.Repeat("long ", 500)
	m.mu.Unlock()
	short := m.GetDatedHistoryForAPIForSession(id, 40, 20, readAt)
	if len(short) != 1 || !strings.Contains(short[0]["content"], "2026-10-31T23:51:00+03:00") {
		t.Fatalf("truncation dropped day: %v", short)
	}
	// Existing messages had only HH:MM; do not invent a date from session UpdatedAt.
	m.mu.Lock()
	m.sessions[id].Messages = []ChatMessage{{Role: "user", Content: "Yarın gel", Timestamp: "12:00"}}
	m.mu.Unlock()
	legacy := m.GetDatedHistoryForAPIForSession(id, 4096, 20)
	if !strings.Contains(legacy[0]["content"], "sent_at=unknown") {
		t.Fatal(legacy)
	}
	raw := m.GetHistoryForAPIForSession(id, 20)
	if raw[0]["content"] != "Yarın gel" {
		t.Fatal("rewrote original stored/UI content")
	}
}

func TestFullMessageTimestampSurvivesReloadAndBothAppendPaths(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	id := m.NewChat()
	m.AddMessage("user", "first", "", "")
	m.AddMessageToSession(id, "assistant", "second", "", "")
	reloaded, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	msgs := reloaded.GetActiveMessagesForSession(id)
	if len(msgs) != 2 {
		t.Fatalf("lost messages on reload: %d", len(msgs))
	}
	for _, msg := range msgs {
		if _, err := time.Parse(time.RFC3339, msg.SentAt); err != nil {
			t.Fatalf("full timestamp lost: %+v", msg)
		}
		if len(msg.Timestamp) != 5 {
			t.Fatal("changed compact UI clock")
		}
	}
}
