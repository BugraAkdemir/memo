package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/agent/tools"
	"memo/internal/taskloop"
)

func TestCreateTaskMd_WritesSchemaValidFile(t *testing.T) {
	a, sm := newSelfDrivingTaskApp(t)
	dir := t.TempDir()
	chat := sm.NewAgentChat(dir)
	ctx := withCurrentChatID(context.Background(), chat)

	out, err := a.createTaskMd(ctx, tools.CreateTaskMdRequest{
		Intro:      "Build a landing page.",
		Items:      []string{"index.html", "styles.css", ""},
		Notify:     "her-şey",
		Mode:       "planlayıcı",
		CoderModel: "local",
	})
	if err != nil {
		t.Fatalf("createTaskMd: %v", err)
	}
	if !strings.Contains(out, "2 items") && !strings.Contains(out, "2 madde") {
		t.Fatalf("reply %q should report 2 items", out)
	}

	path := filepath.Join(dir, "Task.md")
	parsed, err := taskloop.ParseTaskMd(path)
	if err != nil {
		t.Fatalf("ParseTaskMd: %v", err)
	}
	if parsed.NotifyLevel != taskloop.NotifyEverything {
		t.Fatalf("NotifyLevel = %q", parsed.NotifyLevel)
	}
	if parsed.Headers["mod"] != "planlayıcı" || parsed.Headers["kodlayıcı"] != "local" {
		t.Fatalf("headers = %#v", parsed.Headers)
	}
	if len(parsed.Items) != 2 {
		t.Fatalf("items = %d, want 2 (empty one dropped)", len(parsed.Items))
	}

	// A second create must not clobber.
	if _, err := a.createTaskMd(ctx, tools.CreateTaskMdRequest{Items: []string{"x"}}); err == nil {
		t.Fatal("expected an error creating over an existing Task.md")
	}
}

func TestCreateTaskMd_NoPathNoProjectErrors(t *testing.T) {
	a, sm := newSelfDrivingTaskApp(t)
	plain := sm.NewChat()
	ctx := withCurrentChatID(context.Background(), plain)
	if _, err := a.createTaskMd(ctx, tools.CreateTaskMdRequest{Items: []string{"x"}}); err == nil {
		t.Fatal("expected an error when no path and no project folder")
	}
}

func TestEditTaskMd_Operations(t *testing.T) {
	a, sm := newSelfDrivingTaskApp(t)
	dir := t.TempDir()
	chat := sm.NewAgentChat(dir)
	ctx := withCurrentChatID(context.Background(), chat)
	path := filepath.Join(dir, "Task.md")

	if err := os.WriteFile(path, []byte("# bildirim: önemli\n\n- [ ] one\n- [ ] two\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// add_item
	if _, err := a.editTaskMd(ctx, tools.EditTaskMdRequest{Op: "add_item", Text: "three"}); err != nil {
		t.Fatalf("add_item: %v", err)
	}
	// set_header
	if _, err := a.editTaskMd(ctx, tools.EditTaskMdRequest{Op: "set_header", HeaderKey: "mod", HeaderValue: "planlayıcı"}); err != nil {
		t.Fatalf("set_header: %v", err)
	}
	// check_item (item 1)
	if _, err := a.editTaskMd(ctx, tools.EditTaskMdRequest{Op: "check_item", ItemIndex: 1}); err != nil {
		t.Fatalf("check_item: %v", err)
	}
	// split_item (item 2 == "two")
	if _, err := a.editTaskMd(ctx, tools.EditTaskMdRequest{Op: "split_item", ItemIndex: 2, SubItems: []string{"two-a", "two-b"}}); err != nil {
		t.Fatalf("split_item: %v", err)
	}

	parsed, err := taskloop.ParseTaskMd(path)
	if err != nil {
		t.Fatalf("ParseTaskMd: %v", err)
	}
	if parsed.Headers["mod"] != "planlayıcı" {
		t.Fatalf("set_header didn't take: %#v", parsed.Headers)
	}
	// one(done) + two(+[parallel]) + two-a + two-b + three = 5 checkbox lines
	if len(parsed.Items) != 5 {
		t.Fatalf("items = %d, want 5: %+v", len(parsed.Items), parsed.Items)
	}
	if parsed.Items[0].Text != "one" || parsed.Items[0].Status != "done" {
		t.Fatalf("Items[0] = %+v (check_item failed)", parsed.Items[0])
	}
	if !strings.Contains(parsed.Items[1].Text, "[parallel]") {
		t.Fatalf("split_item didn't tag parent: %+v", parsed.Items[1])
	}
	if parsed.Items[2].Text != "two-a" || parsed.Items[2].Indent == 0 {
		t.Fatalf("sub-item not nested: %+v", parsed.Items[2])
	}
	if parsed.Items[4].Text != "three" {
		t.Fatalf("Items[4] = %+v, want 'three' (add_item order)", parsed.Items[4])
	}
}
