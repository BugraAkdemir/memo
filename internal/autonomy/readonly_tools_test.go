package autonomy

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memo/internal/calendar"
	"memo/internal/twin"
)

func TestReadOnlyExecutorScopeAndMaskedOverlaps(t *testing.T) {
	ctx := context.Background()
	s, e := twin.NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	a, _ := s.SaveContact(ctx, twin.Contact{Name: "A", Aliases: []twin.Alias{{Channel: "mail", Address: "a@example.com"}, {Channel: "whatsapp", Address: "90555"}}})
	b, _ := s.SaveContact(ctx, twin.Contact{Name: "B", Aliases: []twin.Alias{{Channel: "mail", Address: "b@example.com"}}})
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	for _, address := range []string{"a@example.com", "b@example.com"} {
		if e = s.RecordMessage(ctx, twin.Message{Channel: "mail", Account: "inbox", Address: address, EventID: address, Body: address, SentAt: at}, ""); e != nil {
			t.Fatal(e)
		}
	}
	if e = s.RecordMessage(ctx, twin.Message{Channel: "whatsapp", Account: "device", Address: "90555", EventID: "wa", Body: "wa a", SentAt: at}, ""); e != nil {
		t.Fatal(e)
	}
	cal, e := calendar.NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer cal.Close()
	end := at.Add(time.Hour)
	if _, e = cal.Add(ctx, calendar.Event{Title: "PRIVATE MEDICAL TITLE", Description: "PRIVATE DESCRIPTION", StartTime: at.Add(-time.Hour), EndTime: &end}); e != nil {
		t.Fatal(e)
	}
	ex := ExternalAutonomousExecutor{Store: s, Calendar: cal, Sender: twin.Alias{Channel: "mail", Address: "a@example.com"}}
	for _, name := range []string{"run_command", "write_file", "edit_file", "mail_send", "use_skill"} {
		if _, e = ex.Execute(ctx, name, json.RawMessage(`{}`)); !errors.Is(e, ErrAutonomousCommandForbidden) {
			t.Fatalf("%s %v", name, e)
		}
	}
	raw, _ := json.Marshal(map[string]string{"contact_id": b})
	if _, e = ex.Execute(ctx, "contact_timeline", raw); !errors.Is(e, ErrContactScope) {
		t.Fatal(e)
	}
	got, e := ex.Execute(ctx, "contact_timeline", json.RawMessage(`{}`))
	if e != nil {
		t.Fatal(e)
	}
	messages := got.([]twin.Message)
	if len(messages) != 2 {
		t.Fatal(messages)
	}
	for _, m := range messages {
		if strings.Contains(m.Body, "b@example.com") {
			t.Fatal("scope leak")
		}
	}
	_ = a
	raw, _ = json.Marshal(map[string]time.Time{"from": at, "to": at.Add(time.Hour)})
	got, e = ex.Execute(ctx, "check_availability", raw)
	if e != nil {
		t.Fatal(e)
	}
	data, _ := json.Marshal(got)
	if strings.Contains(string(data), "PRIVATE") || !strings.Contains(string(data), "start") {
		t.Fatal(string(data))
	}
	ex.Sender.Address = "unknown@example.com"
	if _, e = ex.Execute(ctx, "check_availability", raw); !errors.Is(e, ErrContactScope) {
		t.Fatal(e)
	}
}
func TestGitStatusRealMergeAndNoConfiguredCommands(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("git %v: %s %v", args, b, e)
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-b", "main")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test")
	if e := os.WriteFile(filepath.Join(root, "file"), []byte("first"), 0600); e != nil {
		t.Fatal(e)
	}
	git("add", "file")
	git("commit", "-m", "initial")
	git("checkout", "-b", "merged-feature")
	if e := os.WriteFile(filepath.Join(root, "file"), []byte("feature"), 0600); e != nil {
		t.Fatal(e)
	}
	git("commit", "-am", "feature change")
	git("checkout", "main")
	git("merge", "--no-ff", "merged-feature", "-m", "merge feature")
	git("checkout", "-b", "unmerged-feature")
	if e := os.WriteFile(filepath.Join(root, "file"), []byte("unmerged"), 0600); e != nil {
		t.Fatal(e)
	}
	git("commit", "-am", "unmerged")
	git("checkout", "main")
	if e := os.WriteFile(filepath.Join(root, "dirty"), []byte("dirty"), 0600); e != nil {
		t.Fatal(e)
	}
	// If status executed core.fsmonitor, this sentinel would be created.
	sentinel := filepath.Join(root, "executed")
	script := filepath.Join(t.TempDir(), "monitor.sh")
	if e := os.WriteFile(script, []byte("#!/bin/sh\ntouch '"+sentinel+"'\n"), 0700); e != nil {
		t.Fatal(e)
	}
	git("config", "core.fsmonitor", script)
	git("config", "filter.evil.clean", script)
	git("config", "filter.evil.process", script)
	git("config", "filter.evil.required", "true")
	if e := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("file filter=evil\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(root, "file"), []byte("dirty tracked content"), 0600); e != nil {
		t.Fatal(e)
	}
	indexBefore, e := os.ReadFile(filepath.Join(root, ".git", "index"))
	if e != nil {
		t.Fatal(e)
	}

	s, e := twin.NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ex := ExternalAutonomousExecutor{Store: s, Projects: func() []RegisteredProject { return []RegisteredProject{{ID: "registered", Root: root}} }}
	raw := json.RawMessage(`{"project_id":"registered"}`)
	if _, e = ex.ExecuteForOwner(ctx, "git_project_status", raw); !errors.Is(e, ErrAutonomousCommandForbidden) {
		t.Fatal("inspection should default off", e)
	}
	p, _ := s.Preferences(ctx)
	p.AllowProjectInspection = true
	if e = s.SavePreferences(ctx, p); e != nil {
		t.Fatal(e)
	}
	// Inherited overrides must not redirect evidence to another repository.
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.fsmonitor")
	t.Setenv("GIT_CONFIG_VALUE_0", script)
	got, e := ex.ExecuteForOwner(ctx, "git_project_status", raw)
	if e != nil {
		t.Fatal(e)
	}
	status := got.(GitStatus)
	if !strings.Contains(status.Status, "dirty") || !strings.Contains(status.MergedBranches, "merged-feature") || strings.Contains(status.MergedBranches, "unmerged-feature") || !strings.Contains(status.RecentCommits, "merge feature") {
		t.Fatal(status)
	}
	indexAfter, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil || string(indexBefore) != string(indexAfter) {
		t.Fatal("read-only inspection modified the git index", err)
	}
	if _, e = os.Stat(sentinel); !os.IsNotExist(e) {
		t.Fatal("executed configured script")
	}
	for _, bad := range []json.RawMessage{json.RawMessage(`{"project_id":"registered","command":"touch x"}`), json.RawMessage(`{"project_id":"../../elsewhere"}`)} {
		if _, e = ex.ExecuteForOwner(ctx, "git_project_status", bad); e == nil {
			t.Fatal("accepted unsafe input")
		}
	}
}
