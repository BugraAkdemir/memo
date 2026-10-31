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
	"strings"
	"testing"
	"time"

	"memo/internal/agent/tools"
	"memo/internal/autonomy"
	"memo/internal/mail"
	"memo/internal/twin"
)

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
