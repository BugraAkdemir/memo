package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/config"
	"memo/internal/sessions"
	"memo/internal/taskloop"
	"memo/internal/twin"
)

type bridgeFixture struct {
	a     *App
	store *twin.Store
	tasks *taskloop.Store
	chat  string
}

func newBridge(t *testing.T) *bridgeFixture {
	t.Helper()
	ctx := context.Background()
	s, err := twin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := taskloop.NewStore(filepath.Join(t.TempDir(), "tasklists"))
	if err != nil {
		t.Fatal(err)
	}
	a := &App{cfg: &config.AppConfig{}, twinStore: s, sessions: sm, taskloopStore: tasks}
	act := twin.Action{ID: "act1", Channel: "mail", Account: "owner", Recipient: "ali@example.com", Subject: "Hata düzeltmesi", Body: "taslak", State: "pending", Mode: "awaiting_approval", Reason: "drafted"}
	if err = s.SaveAction(ctx, act); err != nil {
		t.Fatal(err)
	}
	return &bridgeFixture{a: a, store: s, tasks: tasks, chat: sm.NewAgentChat(t.TempDir())}
}

func (f *bridgeFixture) call(t *testing.T, payload map[string]any) (any, error) {
	t.Helper()
	raw, _ := json.Marshal(payload)
	return f.a.CallTwin(context.Background(), "task_from_action", raw)
}

func good(f *bridgeFixture) map[string]any {
	return map[string]any{"id": "act1", "chat_id": f.chat, "title": "Hata düzeltmesi", "items": []string{"Hatayı yeniden üret", "Testi yaz", "Düzelt"}}
}

func TestTaskFromActionCreatesAnIdleListWithExactlyTheOwnersText(t *testing.T) {
	f := newBridge(t)
	res, err := f.call(t, good(f))
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := json.Marshal(res)
	var out struct {
		TaskID string `json:"task_id"`
	}
	if err = json.Unmarshal(blob, &out); err != nil || out.TaskID == "" {
		t.Fatalf("result=%s err=%v", blob, err)
	}
	tl, err := f.tasks.Get(out.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if tl.Status != "idle" || tl.ChatID != f.chat || tl.Title != "Hata düzeltmesi" || len(tl.Items) != 3 || tl.Items[1].Text != "Testi yaz" {
		t.Fatalf("list=%+v", tl)
	}
	if lists := f.a.ListTaskLists(); len(lists) != 1 {
		t.Fatalf("lists=%d", len(lists))
	}
}

// One message can become one task: a second tap or a replayed request must not
// spawn another autonomous job.
func TestTaskFromActionHappensOncePerAction(t *testing.T) {
	f := newBridge(t)
	if _, err := f.call(t, good(f)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.call(t, good(f)); err == nil {
		t.Fatal("a second task was created for the same message")
	}
	if lists := f.a.ListTaskLists(); len(lists) != 1 {
		t.Fatalf("lists=%d, want 1", len(lists))
	}
}

func TestTaskFromActionRejectsAnythingUnsafeAndCreatesNothing(t *testing.T) {
	long := strings.Repeat("a", 501)
	many := make([]string, 11)
	for i := range many {
		many[i] = "adım"
	}
	cases := map[string]func(map[string]any){
		"empty title":       func(p map[string]any) { p["title"] = "  " },
		"multi-line title":  func(p map[string]any) { p["title"] = "a\nb" },
		"long title":        func(p map[string]any) { p["title"] = strings.Repeat("t", 201) },
		"no items":          func(p map[string]any) { p["items"] = []string{} },
		"blank item":        func(p map[string]any) { p["items"] = []string{"ok", "   "} },
		"too many items":    func(p map[string]any) { p["items"] = many },
		"long item":         func(p map[string]any) { p["items"] = []string{long} },
		"control character": func(p map[string]any) { p["items"] = []string{"a\x00b"} },
		"unknown action":    func(p map[string]any) { p["id"] = "nope" },
		"not an agent chat": func(p map[string]any) { p["chat_id"] = "no-such-chat" },
		"missing chat":      func(p map[string]any) { delete(p, "chat_id") },
		"unexpected field":  func(p map[string]any) { p["start"] = true },
		"missing action id": func(p map[string]any) { delete(p, "id") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newBridge(t)
			p := good(f)
			mutate(p)
			if _, err := f.call(t, p); err == nil {
				t.Fatal("accepted")
			}
			if lists := f.a.ListTaskLists(); len(lists) != 0 {
				t.Fatalf("a rejected request still created %d list(s)", len(lists))
			}
			// A rejected request must not use up the one allowed conversion.
			if _, err := f.call(t, good(f)); err != nil {
				t.Fatalf("the valid request after a rejected one failed: %v", err)
			}
		})
	}
}

func TestTaskFromActionNeverStartsTheList(t *testing.T) {
	f := newBridge(t)
	res, err := f.call(t, good(f))
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := json.Marshal(res)
	var out struct {
		TaskID string `json:"task_id"`
	}
	_ = json.Unmarshal(blob, &out)
	if f.a.taskloopEngine != nil {
		t.Fatal("fixture should have no engine: the bridge must not need one")
	}
	if running := f.a.ListRunningTasks(); len(running) != 0 {
		t.Fatalf("something is running: %+v", running)
	}
	tl, _ := f.tasks.Get(out.TaskID)
	for _, it := range tl.Items {
		if it.Status != "pending" {
			t.Fatalf("an item already moved: %+v", it)
		}
	}
}
