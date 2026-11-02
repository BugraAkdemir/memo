package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"memo/internal/telegram"
	"memo/internal/twin"
	"memo/internal/whatsapp"
)

func TestApprovalNotificationsAndAuthenticatedTelegramCancellation(t *testing.T) {
	ctx := context.Background()
	var mu sync.Mutex
	var sent []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "getMe"):
			fmt.Fprint(w, `{"ok":true,"result":{"id":1,"username":"fixture"}}`)
		case strings.HasSuffix(r.URL.Path, "getUpdates"):
			select {
			case <-time.After(30 * time.Millisecond):
			case <-r.Context().Done():
				return
			}
			fmt.Fprint(w, `{"ok":true,"result":[]}`)
		case strings.HasSuffix(r.URL.Path, "sendMessage"):
			var data map[string]any
			json.NewDecoder(r.Body).Decode(&data)
			mu.Lock()
			sent = append(sent, data)
			mu.Unlock()
			fmt.Fprint(w, `{"ok":true,"result":{"message_id":1}}`)
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv(telegram.EnvAPIBase, server.URL)
	client := telegram.NewClient("fixture")
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		client.Stop()
		deadline := time.After(time.Second)
		for client.IsRunning() {
			select {
			case <-deadline:
				t.Error("Telegram poll did not stop")
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}()
	store := telegram.NewStore(filepath.Join(t.TempDir(), "telegram.json"), make([]byte, 32))
	store.Set(telegram.State{Enabled: true, OwnerChatID: 55})
	s, err := twin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	id, err := s.SaveContact(ctx, twin.Contact{Name: "Friend", AutonomyPolicy: "draft_only", Aliases: []twin.Alias{{Channel: "mail", Address: "friend@example.com"}}})
	if err != nil {
		t.Fatal(err)
	}
	action := twin.Action{ID: "one", DeliveryVersion: 1, ContactID: id, Channel: "mail", Account: "owner", Recipient: "friend@example.com", Subject: "Private subject", Body: "Review this draft", State: "pending", CreatedAt: time.Now()}
	if err = s.SaveAction(ctx, action); err != nil {
		t.Fatal(err)
	}
	action, _ = s.Action(ctx, action.ID)
	a := &App{twinStore: s, tgStore: store, tgClient: client, events: &eventRing{}}
	a.notifyApproval(ctx, action, "pending")
	a.notifyApproval(ctx, action, "pending")
	mu.Lock()
	count := len(sent)
	var recipient any
	if count > 0 {
		recipient = sent[0]["chat_id"]
	}
	mu.Unlock()
	if count != 1 || recipient != float64(55) {
		t.Fatal(count, recipient)
	}
	events := a.events.snapshot()
	if len(events) != 1 || events[0].Name != "autonomy:action_pending" || strings.Contains(events[0].Data, action.Body) || strings.Contains(events[0].Data, action.Subject) || strings.Contains(events[0].Data, action.Recipient) {
		t.Fatal(events)
	}
	msg := telegram.Message{ID: 100, ChatID: 55, FromID: 55, Text: fmt.Sprintf("#iptal %d", action.ApprovalCode), Timestamp: time.Now()}
	for _, wrong := range []telegram.Message{
		{ID: 101, ChatID: 66, FromID: 66, Text: msg.Text, Timestamp: msg.Timestamp},
		{ID: 102, ChatID: 55, FromID: 66, Text: msg.Text, Timestamp: msg.Timestamp},
		{ID: 103, ChatID: -55, FromID: 55, Text: msg.Text, Timestamp: msg.Timestamp},
	} {
		if a.routeTelegramApproval(ctx, wrong, true) {
			t.Fatal("non-owner authorized", wrong)
		}
	}
	if !a.routeTelegramApproval(ctx, msg, true) {
		t.Fatal("owner command not handled")
	}
	got, _ := s.Action(ctx, action.ID)
	if got.State != "cancelled" {
		t.Fatal(got)
	}
	// Duplicate and stale bare replies cannot advance to a second pending action.
	second := action
	second.ID = "two"
	if err = s.SaveAction(ctx, second); err != nil {
		t.Fatal(err)
	}
	s.ClaimNotification(ctx, second.ID, "pending", "telegram:55")
	if _, handled := a.controlApproval(ctx, "telegram:55", "100", "evet", time.Now()); !handled {
		t.Fatal("duplicate not consumed")
	}
	if _, handled := a.controlApproval(ctx, "telegram:55", "stale", "evet", time.Now().Add(-time.Hour)); !handled {
		t.Fatal("stale not consumed")
	}
	got, _ = s.Action(ctx, second.ID)
	if got.State != "pending" {
		t.Fatal(got)
	}
	if a.routeWhatsAppApproval(ctx, whatsapp.Message{ID: "x", FromMe: true, ChatJID: "66@s.whatsapp.net", Text: msg.Text, Timestamp: time.Now()}, true) {
		t.Fatal("unconnected WhatsApp authorized")
	}
}

func TestApprovalWorkerRestartDisarmsSchedulesAndStopsBeforeClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := twin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(time.Minute)
	if err = s.SaveAction(ctx, twin.Action{ID: "old", Channel: "mail", State: "pending", Mode: "grace_period_auto", ExecuteAt: &at, Body: "old", DeliveryVersion: 1}); err != nil {
		t.Fatal(err)
	}
	a := &App{twinStore: s, lifecycleCtx: ctx}
	a.startApprovalBridge()
	got, _ := s.Action(ctx, "old")
	if got.Mode != "awaiting_approval" || got.ExecuteAt != nil {
		t.Fatal(got)
	}
	if err = a.closeTwin(); err != nil {
		t.Fatal(err)
	}
	if a.approvalCancel != nil || a.approvalDone != nil || a.twinStore != nil {
		t.Fatal("worker/store still active")
	}
}
