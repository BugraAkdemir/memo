package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"memo/internal/mail"
	"memo/internal/telegram"
	"memo/internal/twin"
)

// fakeTelegram answers sendMessage like the Bot API and records what it got.
// The first `fail` sends answer "not ok".
type fakeTelegram struct {
	mu   sync.Mutex
	sent []map[string]any
	fail int
}

func (f *fakeTelegram) messages() []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]any(nil), f.sent...)
}

func attachTelegram(t *testing.T, a *App, owner int64, fail int) *fakeTelegram {
	t.Helper()
	f := &fakeTelegram{fail: fail}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data map[string]any
		json.NewDecoder(r.Body).Decode(&data)
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.fail > 0 {
			f.fail--
			w.Write([]byte(`{"ok":false,"description":"boom"}`))
			return
		}
		f.sent = append(f.sent, data)
		w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv(telegram.EnvAPIBase, server.URL)
	store := telegram.NewStore(filepath.Join(t.TempDir(), "telegram.json"), make([]byte, 32))
	store.Set(telegram.State{Enabled: true, OwnerChatID: owner})
	a.tgStore, a.tgClient = store, telegram.NewClient("fixture")
	return f
}

func setBriefingChannels(t *testing.T, s *twin.Store, mutate func(*twin.BriefingSettings)) {
	t.Helper()
	ctx := context.Background()
	p, _ := s.Preferences(ctx)
	mutate(&p.Briefing)
	if err := s.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
}

func TestMorningBriefingIsSentToTheOwnersTelegramOnceAndOnlyOnce(t *testing.T) {
	ctx := context.Background()
	a, s, _ := briefingApp(t, true, false, false)
	tg := attachTelegram(t, a, 55, 0)
	setBriefingChannels(t, s, func(b *twin.BriefingSettings) {
		b.Channels = []string{"telegram"}
		b.Custom = []twin.BriefingCustom{{Title: "Water", Prompt: "Drink a glass of water"}}
	})
	now := time.Now()
	if _, err := a.runBriefings(ctx, now); err != nil {
		t.Fatal(err)
	}
	got := tg.messages()
	if len(got) != 1 || got[0]["chat_id"] != float64(55) {
		t.Fatalf("one message to the owner chat expected: %v", got)
	}
	text, _ := got[0]["text"].(string)
	if !strings.Contains(text, "Water") || !strings.Contains(text, "Drink a glass of water") {
		t.Fatalf("the chosen content must be in the message: %q", text)
	}
	for i := 1; i <= 3; i++ {
		if _, err := a.runBriefings(ctx, now.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(tg.messages()); n != 1 {
		t.Fatalf("later ticks sent the briefing again: %d messages", n)
	}
}

func TestBriefingDeliveryIsRetriedAfterAKnownFailureNotBefore(t *testing.T) {
	ctx := context.Background()
	a, s, _ := briefingApp(t, true, false, false)
	tg := attachTelegram(t, a, 55, 1)
	setBriefingChannels(t, s, func(b *twin.BriefingSettings) { b.Channels = []string{"telegram"} })
	now := time.Now()
	a.runBriefings(ctx, now)
	if n := len(tg.messages()); n != 0 {
		t.Fatalf("the first send was meant to fail: %d", n)
	}
	a.runBriefings(ctx, now.Add(time.Minute))
	if n := len(tg.messages()); n != 0 {
		t.Fatal("retried before the backoff")
	}
	a.runBriefings(ctx, now.Add(6*time.Minute))
	if n := len(tg.messages()); n != 1 {
		t.Fatalf("expected one retry to succeed: %d", n)
	}
	a.runBriefings(ctx, now.Add(20*time.Minute))
	if n := len(tg.messages()); n != 1 {
		t.Fatalf("a delivered briefing was sent again: %d", n)
	}
}

func TestBriefingWithASecretIsNotSentOutside(t *testing.T) {
	ctx := context.Background()
	a, s, _ := briefingApp(t, true, false, false)
	tg := attachTelegram(t, a, 55, 0)
	setBriefingChannels(t, s, func(b *twin.BriefingSettings) {
		b.Channels = []string{"telegram"}
		b.Custom = []twin.BriefingCustom{{Title: "Note", Prompt: "key sk-abcdefghijklmnopqrstuvwx"}}
	})
	now := time.Now()
	a.runBriefings(ctx, now)
	a.runBriefings(ctx, now.Add(10*time.Minute))
	if n := len(tg.messages()); n != 0 {
		t.Fatalf("a briefing holding an API key left the app: %v", tg.messages())
	}
	list, _ := s.Briefings(ctx, 1)
	if got, _ := s.BriefingDeliveries(ctx, list[0].Key); got["telegram"] != twin.DeliveryBlocked {
		t.Fatalf("a refused briefing must be recorded as blocked, not retried: %v", got)
	}
}

func TestOneUnavailableChannelDoesNotStopTheOthers(t *testing.T) {
	ctx := context.Background()
	a, s, _ := briefingApp(t, true, false, false)
	tg := attachTelegram(t, a, 55, 0)
	setBriefingChannels(t, s, func(b *twin.BriefingSettings) { b.Channels = []string{"whatsapp", "telegram"} })
	a.runBriefings(ctx, time.Now())
	if n := len(tg.messages()); n != 1 {
		t.Fatalf("telegram must still get the briefing: %d", n)
	}
	list, _ := s.Briefings(ctx, 1)
	got, _ := s.BriefingDeliveries(ctx, list[0].Key)
	if got["whatsapp"] != twin.DeliveryFailed || got["telegram"] != twin.DeliverySent {
		t.Fatalf("states: %v", got)
	}
}

func TestBriefingGoesNowhereWithoutChosenChannelsOrWhenSwitchedOff(t *testing.T) {
	ctx := context.Background()
	a, s, _ := briefingApp(t, true, false, false)
	tg := attachTelegram(t, a, 55, 0)
	a.runBriefings(ctx, time.Now()) // no channel chosen: in-app only
	if n := len(tg.messages()); n != 0 {
		t.Fatalf("no channel was chosen: %d", n)
	}
	setBriefingChannels(t, s, func(b *twin.BriefingSettings) { b.Channels = []string{"telegram"}; b.Morning = false })
	a.runBriefings(ctx, time.Now())
	if n := len(tg.messages()); n != 0 {
		t.Fatalf("the briefing is switched off: %d", n)
	}
}

func TestTelegramBriefingNeedsALinkedOwnerChat(t *testing.T) {
	ctx := context.Background()
	a, s, _ := briefingApp(t, true, false, false)
	tg := attachTelegram(t, a, 0, 0) // bot connected, nobody linked as owner
	setBriefingChannels(t, s, func(b *twin.BriefingSettings) { b.Channels = []string{"telegram"} })
	a.runBriefings(ctx, time.Now())
	if n := len(tg.messages()); n != 0 {
		t.Fatalf("a briefing must never go to an unlinked chat: %d", n)
	}
}

func TestBriefingMailAccountChoice(t *testing.T) {
	accounts := []mail.Account{{ID: "a", Email: "a@example.com"}, {ID: "b", Email: "b@example.com"}, {ID: "c"}}
	if acct, err := briefingMailAccount(accounts, ""); err != nil || acct.ID != "a" {
		t.Fatalf("default is the first account: %+v %v", acct, err)
	}
	if acct, err := briefingMailAccount(accounts, "b"); err != nil || acct.Email != "b@example.com" {
		t.Fatalf("named account: %+v %v", acct, err)
	}
	if _, err := briefingMailAccount(accounts, "gone"); err == nil {
		t.Fatal("a named account that is gone must fail, not fall back to another one")
	}
	if _, err := briefingMailAccount(accounts, "c"); err == nil {
		t.Fatal("an account without an address cannot receive its own briefing")
	}
	if _, err := briefingMailAccount(nil, ""); err == nil {
		t.Fatal("no account connected")
	}
}
