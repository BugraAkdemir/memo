package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"memo/internal/config"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"memo/internal/agent/tools"
	"memo/internal/autonomy"
	"memo/internal/mail"
	"memo/internal/twin"
)

func TestTwinActivityExplainsMailPolicyWithoutPrivateContactData(t *testing.T) {
	ctx := context.Background()
	store, err := twin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	prefs := twin.Preferences{Timezone: "UTC", WorkStart: "09:00", WorkEnd: "18:00", Autonomy: twin.AutonomySettings{Enabled: true, Mail: true, Mode: "full_auto", GraceSeconds: 60}}
	if err := store.SavePreferences(ctx, prefs); err != nil {
		t.Fatal(err)
	}
	model := twin.ReplyModel{Provider: "writer", Model: "reply-model"}
	if err := store.SaveReplyModel(ctx, model); err != nil {
		t.Fatal(err)
	}
	id, err := store.SaveContact(ctx, twin.Contact{Name: "Mail friend", AutonomyPolicy: "never", Aliases: []twin.Alias{{Channel: "mail", Address: "friend@example.com"}, {Channel: "whatsapp", Address: "905551234567@s.whatsapp.net"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveContact(ctx, twin.Contact{Name: "Private WhatsApp friend", Aliases: []twin.Alias{{Channel: "whatsapp", Address: "905559876543@s.whatsapp.net"}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordMessage(ctx, twin.Message{Channel: "mail", Account: "owner", Address: "friend@example.com", EventID: "private-history", Outgoing: true, Body: "PRIVATE TIMELINE TEXT", SentAt: time.Now()}, "Mail friend"); err != nil {
		t.Fatal(err)
	}
	a := &App{twinStore: store}
	result, err := a.CallTwin(ctx, "activity", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	blob, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Settings twin.AutonomySettings `json:"settings"`
		Model    twin.ReplyModel       `json:"reply_model"`
		Contacts []struct {
			ID        string   `json:"id"`
			Name      string   `json:"name"`
			Policy    string   `json:"policy"`
			Addresses []string `json:"addresses"`
		} `json:"mail_contacts"`
	}
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Settings, prefs.Autonomy) || got.Model != model {
		t.Fatalf("activity omitted saved settings/model: %s", blob)
	}
	if len(got.Contacts) != 1 || got.Contacts[0].ID != id || got.Contacts[0].Name != "Mail friend" || got.Contacts[0].Policy != "never" || !reflect.DeepEqual(got.Contacts[0].Addresses, []string{"friend@example.com"}) {
		t.Fatalf("wrong mail policy summary: %s", blob)
	}
	for _, forbidden := range []string{"905551234567", "905559876543", "Private WhatsApp friend", "PRIVATE TIMELINE TEXT", `"aliases"`, `"style"`, `"projects"`, `"timezone"`} {
		if strings.Contains(string(blob), forbidden) {
			t.Fatalf("activity exposed unrelated contact/preferences data %q", forbidden)
		}
	}
	if _, err := a.CallTwin(tools.WithCommunicationAccess(ctx, false), "activity", json.RawMessage(`{}`)); !errors.Is(err, autonomy.ErrContactScope) {
		t.Fatalf("restricted caller received activity: %v", err)
	}
}

func TestTwinMailIngressDatesAndSecurity(t *testing.T) {
	store, err := twin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a := &App{twinStore: store}
	ctx := context.Background()
	at := time.Date(2026, 10, 31, 23, 0, 0, 0, time.FixedZone("TR", 10800))
	d := &mail.MessageDetail{From: "Friend <friend@example.com>", Subject: "Tomorrow", TextBody: "Meet tomorrow", Timestamp: at, UID: 13}
	a.onIncomingMailEvent(ctx, mail.Account{ID: "a"}, d, mail.TriageDecision{SecurityBlocked: true})
	a.onIncomingMailEvent(ctx, mail.Account{ID: "a"}, d, mail.TriageDecision{DropFromRAG: true})
	list, _ := store.Contacts(ctx)
	if len(list) != 0 {
		t.Fatal("blocked/spam entered timeline")
	}
	a.onIncomingMailEvent(ctx, mail.Account{ID: "a"}, d, mail.TriageDecision{})
	a.onIncomingMailEvent(ctx, mail.Account{ID: "a"}, d, mail.TriageDecision{})
	id, err := store.Resolve(ctx, twin.Alias{Channel: "mail", Address: "friend@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	messages, err := store.Timeline(ctx, id, 20)
	if err != nil || len(messages) != 1 || !messages[0].SentAt.Equal(at) {
		t.Fatal(messages, err)
	}
	d.From = "unknown@example.com"
	d.Timestamp = time.Time{}
	a.recordTwinMail(ctx, mail.Account{ID: "a"}, d)
	list, _ = store.Contacts(ctx)
	if len(list) != 1 {
		t.Fatal("invented source date", list)
	}
	denied := tools.WithCommunicationAccess(ctx, false)
	if _, err = a.CallTwin(denied, "state", json.RawMessage(`{}`)); !errors.Is(err, autonomy.ErrContactScope) {
		t.Fatal(err)
	}
	if _, err = a.CallCommunicationTool(denied, "twin_contacts", json.RawMessage(`{}`)); err == nil {
		t.Fatal("restricted chat entered twin")
	}
	raw := json.RawMessage(`{"contact_id":"` + id + `"}`)
	result, err := a.CallCommunicationTool(ctx, "contact_timeline", raw)
	if err != nil || !strings.Contains(result, "untrusted_external_message") || !strings.Contains(result, "2026-10-31") {
		t.Fatal(result, err)
	}
	for _, raw := range []json.RawMessage{json.RawMessage(`{} {}`), json.RawMessage(`{"command":"x"}`)} {
		if _, err = a.CallTwin(ctx, "preferences", raw); err == nil {
			t.Fatal("accepted invalid payload")
		}
	}
}

func TestTwinExportContainsCommittedIdentities(t *testing.T) {
	t.Setenv("MEMO_DATA_DIR", t.TempDir())
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)
	store, err := twin.NewStore(config.DataPath("twin"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.SaveContact(context.Background(), twin.Contact{Name: "Portable Friend", Aliases: []twin.Alias{{Channel: "mail", Address: "portable@example.com"}}}); err != nil {
		t.Fatal(err)
	}
	a := &App{twinStore: store, cfg: config.Default()}
	blob, err := a.ExportData(false)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(blob), int64(len(blob)))
	if err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	found := false
	for _, f := range zr.File {
		if f.Name == "data/twin/twin.db" {
			r, e := f.Open()
			if e != nil {
				t.Fatal(e)
			}
			data, e := io.ReadAll(r)
			r.Close()
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(dest, "twin.db"), data, 0600); e != nil {
				t.Fatal(e)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("twin store omitted from backup")
	}
	restored, err := twin.NewStore(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	contacts, err := restored.Contacts(context.Background())
	if err != nil || len(contacts) != 1 || contacts[0].Name != "Portable Friend" {
		t.Fatal(contacts, err)
	}
}
