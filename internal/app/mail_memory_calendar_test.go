package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memo/internal/calendar"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/intent"
	"memo/internal/mail"
	"memo/internal/memory"
)

func TestMailMemoryKeepsDistinctMessagesWithIdenticalEmbeddings(t *testing.T) {
	store, err := memory.NewStore(memory.StoreConfig{Dir: t.TempDir(), Dimension: 3, EmbeddingFunc: func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil }})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a := &App{cfg: &config.AppConfig{Memory: config.MemoryConfig{MemoryEnabled: true}}, store: store}
	for _, uid := range []uint32{42, 43} {
		detail := &mail.MessageDetail{UID: uid, From: "same@example.com", Subject: "Repeated invitation", TextBody: fmt.Sprintf("Meet at 14:00. Reference %d", uid)}
		if err := a.indexMailInMemory(context.Background(), mail.Account{ID: "acct"}, detail, mail.TriageDecision{}); err != nil {
			t.Fatal(err)
		}
	}
	if store.Count() != 2 {
		t.Fatalf("got %d memories, want both mails despite identical embeddings", store.Count())
	}
	rows, err := store.RecentSince(context.Background(), memory.Scope{}, time.Now().Add(-time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	id := identity.New("Test", "Memo", "casual", "", false)
	block := id.BuildMemoryBlock(rows, true, 4096)
	for _, want := range []string{"UID: 42", "UID: 43", "Reference 42", "Reference 43"} {
		if !strings.Contains(block, want) {
			t.Errorf("stripAssistant removed mail content: missing %q in %s", want, block)
		}
	}
	a.store = nil
	if err := a.indexMailInMemory(context.Background(), mail.Account{}, &mail.MessageDetail{}, mail.TriageDecision{}); err == nil {
		t.Fatal("missing store must not report success")
	}
}

func TestPlainChatUpcomingCalendarContext(t *testing.T) {
	cs, err := calendar.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	now := time.Now()
	for _, e := range []calendar.Event{{Title: "Tomorrow meeting", StartTime: now.Add(24 * time.Hour)}, {Title: "Past event", StartTime: now.Add(-time.Hour)}, {Title: "Next month", StartTime: now.AddDate(0, 0, 8)}} {
		if _, err := cs.Add(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	a := &App{cfg: &config.AppConfig{Llama: config.LlamaConfig{CtxSize: 4096}}, identity: identity.New("Test", "Memo", "casual", "", false), calendarStore: cs}
	// Agent mode defaults off; the summary must still reach the current user message.
	msgs := a.buildMessagesForSession(context.Background(), "", "What's coming up?", nil, nil)
	last := msgs[len(msgs)-1].GetTextContent()
	if !strings.Contains(last, "[Upcoming calendar events]") || !strings.Contains(last, "Tomorrow meeting") {
		t.Fatalf("calendar missing from plain chat: %s", last)
	}
	for _, bad := range []string{"Past event", "Next month"} {
		if strings.Contains(last, bad) {
			t.Errorf("out-of-range event included: %s", bad)
		}
	}
	if strings.Contains(msgs[0].GetTextContent(), "Tomorrow meeting") {
		t.Fatal("calendar must not change system prefix")
	}
	a.identity.SetMinimalMode(true)
	msgs = a.buildMessagesForSession(context.Background(), "", "hello", nil, nil)
	if strings.Contains(msgs[len(msgs)-1].GetTextContent(), "Tomorrow meeting") {
		t.Fatal("minimal mode leaked calendar")
	}
	a.identity.SetMinimalMode(false)
	msgs = a.buildMessagesForSession(withCodeMode(context.Background()), "", "hello", nil, nil)
	if strings.Contains(msgs[len(msgs)-1].GetTextContent(), "Tomorrow meeting") {
		t.Fatal("code mode leaked calendar")
	}
}

func TestMailCalendarCounterTracksCommittedStructuredAndTextEvents(t *testing.T) {
	cs, err := calendar.NewStore(filepath.Join(t.TempDir(), "calendar"))
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	watcher := mail.NewWatcher(nil, nil, mail.WatcherConfig{})
	now := time.Now()
	a := &App{cfg: &config.AppConfig{}, lifecycleCtx: context.Background(), calendarStore: cs, mailWatcher: watcher}
	a.extractMailStructuredToCalendar(context.Background(), &mail.MessageDetail{Structured: mail.StructuredExtraction{CalendarInvites: []mail.CalendarInvite{{Summary: "ICS meeting", StartTime: now.Add(time.Hour)}}}})
	if watcher.Stats().CalendarExtracted != 1 {
		t.Fatal("structured event not counted")
	}
	a.intentExtractor = intent.NewExtractor(func(context.Context, string, string) (string, error) {
		return fmt.Sprintf(`{"has_intent":true,"is_calendar_event":true,"summary":"Text meeting","event_title":"Text meeting","event_time_iso":%q,"time_explicit":true}`, now.Add(2*time.Hour).Format(time.RFC3339)), nil
	})
	a.extractMailStructuredToCalendar(context.Background(), &mail.MessageDetail{Subject: "14:00 meeting", TextBody: "Tomorrow at 14:00", Timestamp: now})
	if watcher.Stats().CalendarExtracted != 2 {
		t.Fatal("plain-text mail event not counted")
	}
	a.processMessageIntent("Tomorrow at 14:00", intent.Source("chat"), "", now)
	if watcher.Stats().CalendarExtracted != 2 {
		t.Fatal("non-mail event counted as mail")
	}
	cs.Close()
	a.extractMailStructuredToCalendar(context.Background(), &mail.MessageDetail{Structured: mail.StructuredExtraction{CalendarInvites: []mail.CalendarInvite{{Summary: "Failed", StartTime: now.Add(time.Hour)}}}})
	if watcher.Stats().CalendarExtracted != 2 {
		t.Fatal("failed insert must not increment counter")
	}
}
