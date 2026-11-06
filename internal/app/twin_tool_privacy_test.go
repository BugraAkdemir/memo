package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"memo/internal/autonomy"
	"memo/internal/config"
	"memo/internal/mail"
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
	for _, key := range []string{`"actions"`, `"commitments"`, `"briefings"`, `"preferences"`, `"contacts"`, `"audit_log"`} {
		if !strings.Contains(string(blob), key) {
			t.Errorf("state lost %s", key)
		}
	}
}

func TestStepEightCommunicationToolsCommitmentsDevStatusAndAttachments(t *testing.T) {
	t.Setenv("MEMO_DATA_DIR", t.TempDir())
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)
	ctx := context.Background()
	a, s, _ := radarApp(t, 48)
	if err := s.RecordMessage(ctx, twin.Message{Channel: "mail", Account: "owner", EventID: "q1", Address: "ali@example.com", Outgoing: true, Body: "SECRET QUESTION BODY?", SentAt: time.Now(), Subject: "Teklif"}, "Ali"); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordMessage(ctx, twin.Message{Channel: "mail", Account: "owner", EventID: "q2", Address: "veli@example.com", Outgoing: true, Body: "OTHER QUESTION?", SentAt: time.Now(), Subject: "Fatura"}, "Veli"); err != nil {
		t.Fatal(err)
	}
	aliID, err := s.Resolve(ctx, twin.Alias{Channel: "mail", Address: "ali@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddPromise(ctx, twin.Promise{Channel: "mail", Account: "owner", Address: "ali@example.com", Summary: "Sözleşmeyi gönder", SourceRef: "q1", DueAt: time.Now().Add(24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	out, err := a.CallCommunicationTool(ctx, "twin_commitments", json.RawMessage(`{"contact_id":"`+aliID+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Teklif") || !strings.Contains(out, "Sözleşmeyi gönder") || strings.Contains(out, "Fatura") || strings.Contains(out, "SECRET QUESTION BODY") {
		t.Fatalf("unexpected twin_commitments output: %s", out)
	}
	// dev_status must be refused when project inspection is disabled (default).
	if _, err := a.CallCommunicationTool(ctx, "dev_status", json.RawMessage(`{}`)); !errors.Is(err, autonomy.ErrAutonomousCommandForbidden) {
		t.Fatalf("dev_status allowed with AllowProjectInspection=false: %v", err)
	}
	prefs, err := s.Preferences(ctx)
	if err != nil {
		t.Fatal(err)
	}
	prefs.AllowProjectInspection = true
	if err := s.SavePreferences(ctx, prefs); err != nil {
		t.Fatal(err)
	}
	devOut, err := a.CallCommunicationTool(ctx, "dev_status", json.RawMessage(`{}`))
	if err != nil || (!strings.Contains(devOut, `"projects"`) && !strings.Contains(devOut, "&#34;projects&#34;")) {
		t.Fatalf("dev_status failed: %s, %v", devOut, err)
	}
	// mail_read includes attachment metadata without binary data, and saveAndFormatMailAttachment saves safely.
	readOut := formatMailReadResult(&mail.MessageDetail{
		SeqNum:      7,
		UID:         42,
		From:        "ali@example.com",
		Subject:     "Davet",
		Attachments: []mail.AttachmentInfo{{PartID: "2", Filename: "../invite.ics", ContentType: "text/calendar", Size: 24, Data: []byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR")}},
	})
	if !strings.Contains(readOut, "attachments") || !strings.Contains(readOut, "invite.ics") || strings.Contains(readOut, "BEGIN:VCALENDAR") {
		t.Fatalf("unexpected mail_read attachment metadata: %s", readOut)
	}
	attOut, err := saveAndFormatMailAttachment(7, &mail.AttachmentInfo{PartID: "2", Filename: "../invite.ics", ContentType: "text/calendar", Size: 24, Data: []byte("BEGIN:VCALENDAR\nEND:VCALENDAR")})
	if err != nil || !strings.Contains(attOut, "BEGIN:VCALENDAR") || !strings.Contains(attOut, "untrusted_external_message") {
		t.Fatalf("attachment download failed: %s, %v", attOut, err)
	}
	entries, err := os.ReadDir(config.DataPath("mail", "attachments"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "7_invite.ics" {
		t.Fatalf("unexpected saved attachment files: %+v, %v", entries, err)
	}
}
