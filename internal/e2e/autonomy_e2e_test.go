package e2e

import (
	"context"
	"memo/internal/config"
	"memo/internal/provider"
	"memo/internal/twin"
	"net/http"
	"testing"
	"time"
)

// Real application startup, HTTP admin API and the same persisted queue the
// incoming-event engine writes. No real account or external delivery is used.
func TestAutonomyOwnerQueueSettingsAndCancellationHTTP(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(false)
	s, err := twin.NewStore(config.DataPath("twin"))
	if err != nil {
		t.Fatal(err)
	}
	err = s.SaveAction(context.Background(), twin.Action{ID: "prepared", Channel: "mail", Account: "a", Recipient: "friend@example.com", Body: "Draft for review", Mode: "awaiting_approval", State: "pending", Reason: "drafted", CreatedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	var state struct {
		Actions     []twin.Action    `json:"actions"`
		Preferences twin.Preferences `json:"preferences"`
		ReplyModel  twin.ReplyModel  `json:"reply_model"`
	}
	decodeInto(t, h.getJSON("/api/v1/twin/state"), &state)
	if len(state.Actions) != 1 || state.Actions[0].Body != "Draft for review" {
		t.Fatal(state)
	}
	state.Preferences.Autonomy = twin.AutonomySettings{Enabled: true, Mail: true, Mode: "full_auto", GraceSeconds: 120}
	r := h.postJSON("/api/twin/preferences", state.Preferences)
	if r.StatusCode != http.StatusOK {
		t.Fatal(r.StatusCode)
	}
	r.Body.Close()
	decodeInto(t, h.getJSON("/api/twin/state"), &state)
	if !state.Preferences.Autonomy.Enabled || state.Preferences.Autonomy.GraceSeconds != 120 {
		t.Fatal(state)
	}
	var providers []provider.ProviderConfig
	decodeInto(t, h.getJSON("/api/providers"), &providers)
	if len(providers) == 0 {
		t.Fatal("no fake provider")
	}
	selection := twin.ReplyModel{Provider: providers[0].Name, Model: providers[0].Model}
	r = h.postJSON("/api/twin/reply_model", selection)
	if r.StatusCode != 200 {
		t.Fatal(r.StatusCode)
	}
	r.Body.Close()
	decodeInto(t, h.getJSON("/api/twin/state"), &state)
	if state.ReplyModel != selection {
		t.Fatal(state.ReplyModel, selection)
	}

	r = h.postJSON("/api/v1/twin/cancel_action", map[string]string{"id": "prepared"})
	if r.StatusCode != 200 {
		t.Fatal(r.StatusCode)
	}
	r.Body.Close()
	decodeInto(t, h.getJSON("/api/twin/state"), &state)
	if state.Actions[0].State != "cancelled" || h.App.GetAgentEnabled() {
		t.Fatal(state)
	}
	r = h.postJSON("/api/twin/cancel_action", map[string]string{"id": "prepared"})
	if r.StatusCode != 404 {
		t.Fatal(r.StatusCode)
	}
	r.Body.Close()
}
