package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/mail"
	"memo/internal/provider"
	"memo/internal/twin"
)

func radarApp(t *testing.T, hours int) (*App, *twin.Store, *atomic.Int32) {
	t.Helper()
	ctx := context.Background()
	s, err := twin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	p := twin.Preferences{Timezone: "UTC", WorkStart: "09:00", WorkEnd: "18:00", Radar: twin.RadarSettings{Enabled: true, FollowUpHours: hours}}
	if err = s.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveReplyModel(ctx, twin.ReplyModel{Provider: "writer", Model: "reply-writer"}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var req struct {
			Messages []provider.Message `json:"messages"`
			Tools    []any              `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if len(req.Tools) != 0 || len(req.Messages) == 0 || !strings.Contains(fmt.Sprint(req.Messages[0].Content), "follow-up message") {
			t.Errorf("not an isolated follow-up call: tools=%d", len(req.Tools))
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": `{"body":"Merhaba, bir geri dönüşün olursa sevinirim."}`}, "finish_reason": "stop"}}})
	}))
	t.Cleanup(server.Close)
	mgr := provider.NewConfigManager(t.TempDir()+"/providers.json", make([]byte, 32))
	if err = mgr.Set(provider.ProviderConfig{Name: "writer", Type: provider.ProviderCustom, BaseURL: server.URL, Model: "reply-writer", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	mailMgr, err := mail.NewManager(t.TempDir() + "/accounts.json")
	if err != nil {
		t.Fatal(err)
	}
	return &App{mailManager: mailMgr, twinStore: s, providerCfgMgr: mgr, cfg: &config.AppConfig{}}, s, &calls
}

func TestSentMailThatAsksAQuestionOpensACommitment(t *testing.T) {
	ctx := context.Background()
	a, s, _ := radarApp(t, 48)
	a.recordTwinSentMail(ctx, mail.SendRequest{AccountID: "owner", To: []string{"Ali <ali@example.com>"}, Subject: "Teklif", Body: "Teklifi inceledin mi?", InReplyTo: "<m1@example.com>"})
	open, err := s.Commitments(ctx)
	if err != nil || len(open) != 1 || open[0].Address != "ali@example.com" || open[0].Subject != "Teklif" || open[0].InReplyTo != "<m1@example.com>" {
		t.Fatalf("open=%+v err=%v", open, err)
	}
}

func TestSentMailToSeveralRecipientsNeverOpensACommitment(t *testing.T) {
	ctx := context.Background()
	a, s, _ := radarApp(t, 48)
	a.recordTwinSentMail(ctx, mail.SendRequest{AccountID: "owner", To: []string{"ali@example.com"}, Cc: []string{"veli@example.com"}, Subject: "Toplantı", Body: "Hangi gün uygun?"})
	a.recordTwinSentMail(ctx, mail.SendRequest{AccountID: "owner", To: []string{"ali@example.com", "veli@example.com"}, Subject: "Toplantı 2", Body: "Hangi gün uygun?"})
	if open, _ := s.Commitments(ctx); len(open) != 0 {
		t.Fatalf("a group or CC thread must not be nudged: %+v", open)
	}
}

func TestFollowUpScanUsesTheReplyModelOnceAndReportsAVisibleResult(t *testing.T) {
	ctx := context.Background()
	a, s, calls := radarApp(t, 1)
	sent := time.Now().Add(-10 * time.Minute)
	if err := s.RecordMessage(ctx, twin.Message{Channel: "mail", Account: "owner", EventID: "o1", Address: "ali@example.com", Outgoing: true, Body: "Teklifi inceledin mi?", SentAt: sent, Subject: "Teklif"}, "Ali"); err != nil {
		t.Fatal(err)
	}
	// Move the clock instead of waiting an hour.
	n, err := a.runFollowUpScan(ctx, time.Now().Add(2*time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("model calls=%d, want exactly one isolated follow-up call", calls.Load())
	}
	actions, err := s.Actions(ctx)
	if err != nil || len(actions) != 1 {
		t.Fatalf("actions=%+v err=%v", actions, err)
	}
	// No mail account is connected here, so the Drafts upload fails visibly;
	// nothing is ever sent.
	if actions[0].Reason != "draft_upload_failed" || actions[0].State != "failed" || actions[0].Body == "" {
		t.Fatalf("unexpected result: %+v", actions[0])
	}
	if n, _ = a.runFollowUpScan(ctx, time.Now().Add(3*time.Hour)); n != 0 || calls.Load() != 1 {
		t.Fatalf("second scan repeated the reminder: n=%d calls=%d", n, calls.Load())
	}
}

func TestFollowUpScanWaitsUntilAReplyModelIsChosen(t *testing.T) {
	ctx := context.Background()
	a, s, calls := radarApp(t, 1)
	if err := s.SaveReplyModel(ctx, twin.ReplyModel{}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordMessage(ctx, twin.Message{Channel: "mail", Account: "owner", EventID: "o1", Address: "ali@example.com", Outgoing: true, Body: "Uygun mu?", SentAt: time.Now().Add(-5 * time.Minute)}, "Ali"); err != nil {
		t.Fatal(err)
	}
	if n, err := a.runFollowUpScan(ctx, time.Now().Add(2*time.Hour)); err != nil || n != 0 || calls.Load() != 0 {
		t.Fatalf("n=%d err=%v calls=%d", n, err, calls.Load())
	}
	if open, _ := s.Commitments(ctx); len(open) != 1 || open[0].Status != "open" {
		t.Fatalf("waiting reminder was consumed: %+v", open)
	}
}

func TestActivityShowsRadarSettingsAndWaitingListWithoutMessageBodies(t *testing.T) {
	ctx := context.Background()
	a, s, _ := radarApp(t, 48)
	if err := s.RecordMessage(ctx, twin.Message{Channel: "mail", Account: "owner", EventID: "o1", Address: "ali@example.com", Outgoing: true, Body: "GİZLİ PROJE ADI hakkında ne düşünüyorsun?", SentAt: time.Now(), Subject: "Teklif"}, "Ali"); err != nil {
		t.Fatal(err)
	}
	raw, err := a.CallTwin(ctx, "activity", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := json.Marshal(raw)
	var got struct {
		Radar       twin.RadarSettings `json:"radar"`
		Commitments []struct {
			ID      string `json:"id"`
			Channel string `json:"channel"`
			Address string `json:"address"`
			Subject string `json:"subject"`
			Status  string `json:"status"`
			DueAt   string `json:"due_at"`
		} `json:"commitments"`
	}
	if err = json.Unmarshal(blob, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Radar.Enabled || got.Radar.FollowUpHours != 48 || len(got.Commitments) != 1 || got.Commitments[0].Address != "ali@example.com" || got.Commitments[0].Subject != "Teklif" || got.Commitments[0].Status != "open" || got.Commitments[0].DueAt == "" {
		t.Fatalf("activity=%s", blob)
	}
	if strings.Contains(string(blob), "GİZLİ PROJE ADI") {
		t.Fatal("the waiting list leaked the owner's message body")
	}
}

func TestTwinStateCarriesTheWaitingListForTheSettingsDialog(t *testing.T) {
	ctx := context.Background()
	a, s, _ := radarApp(t, 48)
	if err := s.RecordMessage(ctx, twin.Message{Channel: "mail", Account: "owner", EventID: "o1", Address: "ali@example.com", Outgoing: true, Body: "GİZLİ PROJE ADI ne olacak?", SentAt: time.Now(), Subject: "Teklif"}, "Ali"); err != nil {
		t.Fatal(err)
	}
	raw, err := a.CallTwin(ctx, "state", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := json.Marshal(raw)
	var got struct {
		Preferences twin.Preferences `json:"preferences"`
		Commitments []struct {
			Address string `json:"address"`
			Status  string `json:"status"`
		} `json:"commitments"`
	}
	if err = json.Unmarshal(blob, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Preferences.Radar.Enabled || len(got.Commitments) != 1 || got.Commitments[0].Address != "ali@example.com" || got.Commitments[0].Status != "open" {
		t.Fatalf("state=%s", blob)
	}
	if strings.Contains(string(blob), "GİZLİ PROJE ADI") {
		t.Fatal("state leaked the owner's message body")
	}
}

// Saving preferences through the API must keep the radar settings round-trip.
func TestPreferencesSaveRoundTripsRadar(t *testing.T) {
	ctx := context.Background()
	a, s, _ := radarApp(t, 72)
	body := `{"timezone":"UTC","work_start":"09:00","work_end":"18:00","allow_project_inspection":false,"autonomy":{},"radar":{"enabled":true,"follow_up_hours":24}}`
	if _, err := a.CallTwin(ctx, "preferences", json.RawMessage(body)); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.Preferences(ctx); !p.Radar.Enabled || p.Radar.FollowUpHours != 24 {
		t.Fatalf("radar not saved: %+v", p.Radar)
	}
}
