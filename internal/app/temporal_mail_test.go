package app

import (
	"context"
	"memo/internal/calendar"
	"memo/internal/config"
	"memo/internal/intent"
	"memo/internal/mail"
	"memo/internal/sessions"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadingOldMailPreservesSentDateAndDoesNotRebaseTomorrow(t *testing.T) {
	sent := time.Date(2026, 10, 31, 23, 50, 0, 0, time.FixedZone("TRT", 3*3600))
	detail := &mail.MessageDetail{From: "friend@example.com", Subject: "Acil toplantı", TextBody: "Yarın saat 12:00", Date: sent.Format(time.RFC1123Z), Timestamp: sent}
	read := formatMailReadResult(detail)
	memory, _ := mail.FormatMailForMemory(mail.Account{}, detail, mail.TriageDecision{})
	for _, s := range []string{read, memory} {
		if !strings.Contains(s, "2026-10-31T23:50:00+03:00") || !strings.Contains(s, "Yarın saat 12:00") {
			t.Fatal(s)
		}
	}
	detail.Timestamp = time.Time{}
	detail.Date = ""
	if got := formatMailReadResult(detail); strings.Contains(got, "sent_at") || strings.Contains(got, "0001-") {
		t.Fatal("invented mail date", got)
	}
	tomorrow := time.Date(2026, 11, 1, 9, 0, 0, 0, sent.Location())
	if block := timeContextBlock(tomorrow, sent); !strings.Contains(block, "2026") || !strings.Contains(block, "source date") || !strings.Contains(block, "+03:00") {
		t.Fatal(block)
	}
}

func TestPlainChatReceivesHistoricalSourceDateAlongsideFreshClock(t *testing.T) {
	reqs := &capturedRequests{}
	a := newSendMessageTestApp(t, reqs)
	dir := t.TempDir()
	// Synthetic persisted history, not the user's data and not a guessed migration.
	payload := `{"id":"dated","title":"Dated history","created_at":"2020-12-31 23:50","updated_at":"2020-12-31 23:51","messages":[{"role":"user","content":"Yarın 12:00 toplantı var","timestamp":"23:50","sent_at":"2020-12-31T23:50:00+03:00"},{"role":"assistant","content":"Yarın toplantın var","timestamp":"23:51","sent_at":"2020-12-31T23:51:00+03:00"}]}`
	if err := os.WriteFile(filepath.Join(dir, "dated.json"), []byte(payload), 0600); err != nil {
		t.Fatal(err)
	}
	sm, err := sessions.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	a.sessions = sm
	drainBackground(t, a) // registered after the last TempDir, as cleanup ordering requires
	for range a.SendMessageStreamTo(context.Background(), "dated", "Bugünkü toplantım hangisi?") {
	}
	got := reqs.String()
	if !strings.Contains(got, "sent_at=2020-12-31T23:50:00+03:00") || !strings.Contains(got, "Current local time:") || !strings.Contains(got, "source date") {
		t.Fatalf("provider did not receive historical and current anchors: %s", got)
	}
}

func TestMailCalendarUsesOriginalDateAndDoesNotGuessAnUnknownOrigin(t *testing.T) {
	cs, err := calendar.NewStore(filepath.Join(t.TempDir(), "calendar"))
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	a := &App{cfg: &config.AppConfig{}, lifecycleCtx: context.Background(), calendarStore: cs}
	var reference string
	a.intentExtractor = intent.NewExtractor(func(_ context.Context, _ string, user string) (string, error) {
		reference = user
		return `{"has_intent":false}`, nil
	})
	a.extractMailStructuredToCalendar(context.Background(), &mail.MessageDetail{Subject: "Yarın toplantı", TextBody: "Tomorrow at 12:00"})
	if reference != "" {
		t.Fatal("undated mail was resolved against processing time")
	}
	sent := time.Date(2026, 10, 31, 23, 50, 0, 0, time.FixedZone("TRT", 3*3600))
	a.extractMailStructuredToCalendar(context.Background(), &mail.MessageDetail{Subject: "Yarın toplantı", TextBody: "Tomorrow at 12:00", Timestamp: sent})
	if !strings.Contains(reference, "Reference datetime: 2026-10-31 23:50") {
		t.Fatal(reference)
	}
}
