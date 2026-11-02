package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"memo/internal/twin"
)

// Found in the end-to-end run: the twin_contacts CHAT TOOL was backed by the full
// state, so a chat model could read every prepared reply's text and recipient,
// the waiting list and the briefing cards (which carry private meeting titles),
// although the tool promises contacts, working hours and project ids only.
func TestTwinContactsToolReturnsOnlyWhatItPromises(t *testing.T) {
	ctx := context.Background()
	a, s, _ := radarApp(t, 48)
	if _, err := s.SaveContact(ctx, twin.Contact{Name: "Friend", AutonomyPolicy: "draft_only", Aliases: []twin.Alias{{Channel: "mail", Address: "friend@example.com"}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveAction(ctx, twin.Action{ID: "act", Channel: "mail", Account: "owner", Recipient: "friend@example.com", Subject: "SECRET SUBJECT", Body: "SECRET DRAFT BODY", State: "pending", Mode: "awaiting_approval", Reason: "drafted"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordMessage(ctx, twin.Message{Channel: "mail", Account: "owner", EventID: "q", Address: "friend@example.com", Outgoing: true, Body: "WAITING QUESTION TEXT?", SentAt: time.Now(), Subject: "WAITING SUBJECT"}, "Friend"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveBriefing(ctx, twin.Briefing{Key: "meeting:x", Kind: "meeting", Title: "PRIVATE MEETING TITLE", Sections: []twin.BriefingSection{{Heading: "meeting_when", Items: []string{"PRIVATE ITEM"}}}}); err != nil {
		t.Fatal(err)
	}
	raw, err := a.CallTwin(ctx, "twin_contacts", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := json.Marshal(raw)
	var got map[string]json.RawMessage
	if err = json.Unmarshal(blob, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"actions", "commitments", "briefings", "reply_model"} {
		if _, ok := got[key]; ok {
			t.Errorf("twin_contacts exposes %q", key)
		}
	}
	for _, secret := range []string{"SECRET SUBJECT", "SECRET DRAFT BODY", "WAITING QUESTION TEXT", "WAITING SUBJECT", "PRIVATE MEETING TITLE", "PRIVATE ITEM"} {
		if strings.Contains(string(blob), secret) {
			t.Errorf("twin_contacts leaked %q", secret)
		}
	}
	// What it does promise is still there.
	var shape struct {
		Contacts    []twin.Contact `json:"contacts"`
		Preferences struct {
			Timezone  string `json:"timezone"`
			WorkStart string `json:"work_start"`
			WorkEnd   string `json:"work_end"`
		} `json:"preferences"`
		Projects json.RawMessage `json:"projects"`
	}
	if err = json.Unmarshal(blob, &shape); err != nil {
		t.Fatal(err)
	}
	if len(shape.Contacts) == 0 || shape.Preferences.WorkStart != "09:00" || shape.Preferences.WorkEnd != "18:00" || shape.Preferences.Timezone == "" || len(shape.Projects) == 0 {
		t.Fatalf("the promised data is missing: %s", blob)
	}
	// Autonomy, radar and briefing settings are the owner's configuration, not
	// something the chat model needs.
	for _, key := range []string{"autonomy", "radar", "briefing"} {
		if strings.Contains(string(blob), `"`+key+`"`) {
			t.Errorf("twin_contacts exposes the %q settings", key)
		}
	}
}

// The settings dialog still reads everything through "state".
func TestTwinStateStillCarriesEverythingForTheDialog(t *testing.T) {
	ctx := context.Background()
	a, s, _ := radarApp(t, 48)
	if _, err := s.SaveBriefing(ctx, twin.Briefing{Key: "morning:x", Kind: "morning", Title: "2026-11-03"}); err != nil {
		t.Fatal(err)
	}
	raw, err := a.CallTwin(ctx, "state", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := json.Marshal(raw)
	for _, key := range []string{`"actions"`, `"commitments"`, `"briefings"`, `"preferences"`, `"contacts"`} {
		if !strings.Contains(string(blob), key) {
			t.Errorf("state lost %s", key)
		}
	}
}
