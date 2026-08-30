package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRoleDirectives(t *testing.T) {
	rm := parseRoleDirectives("intro\n<!-- memo:taskloop planlayıcı=claude kodlayıcı=local doğrulayıcı=gemini -->\nmore")
	if rm.Planner != "claude" || rm.Coder != "local" || rm.Verifier != "gemini" {
		t.Fatalf("got %+v", rm)
	}
	if got := parseRoleDirectives("no directive here"); got != (roleModels{}) {
		t.Fatalf("expected empty, got %+v", got)
	}
}

func TestResolveRoleModels_Priority(t *testing.T) {
	a, sm := newSelfDrivingTaskApp(t)
	dir := t.TempDir()
	chat := sm.NewAgentChat(dir)

	// AGENTS.md directive sets all three.
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"),
		[]byte("# rules\n<!-- memo:taskloop planlayıcı=claude kodlayıcı=gemini doğrulayıcı=gemini -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Task.md header overrides the coder.
	md := filepath.Join(dir, "Task.md")
	if err := os.WriteFile(md, []byte("# kodlayıcı: local\n\n- [ ] a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tl, err := a.CreateTaskListFromTaskMd(chat, "", md)
	if err != nil {
		t.Fatalf("CreateTaskListFromTaskMd: %v", err)
	}
	// Settings default fills nothing new here, but exercise the path.
	a.cfg.TaskLoop.VerifierModel = "should-not-win"

	rm := a.resolveRoleModels(tl.ID)
	if rm.Coder != "local" {
		t.Fatalf("Coder = %q, want local (Task.md header wins)", rm.Coder)
	}
	if rm.Planner != "claude" {
		t.Fatalf("Planner = %q, want claude (AGENTS.md directive)", rm.Planner)
	}
	if rm.Verifier != "gemini" {
		t.Fatalf("Verifier = %q, want gemini (AGENTS.md beats Settings)", rm.Verifier)
	}
}

func TestResolveRoleModels_ConfigFallback(t *testing.T) {
	a, sm := newSelfDrivingTaskApp(t)
	dir := t.TempDir()
	chat := sm.NewAgentChat(dir)
	md := filepath.Join(dir, "Task.md")
	os.WriteFile(md, []byte("- [ ] a\n"), 0o644)
	tl, _ := a.CreateTaskListFromTaskMd(chat, "", md)

	a.cfg.TaskLoop.PlannerModel = "cfg-planner"
	rm := a.resolveRoleModels(tl.ID)
	if rm.Planner != "cfg-planner" {
		t.Fatalf("Planner = %q, want cfg-planner", rm.Planner)
	}
	if rm.Coder != "" || rm.Verifier != "" {
		t.Fatalf("unset roles should stay empty, got %+v", rm)
	}
}

func TestPersistRoleChoiceToRules_CreatesAndUpdates(t *testing.T) {
	a, _ := newSelfDrivingTaskApp(t)
	dir := t.TempDir()

	if err := a.persistRoleChoiceToRules(dir, "kodlayıcı", "local"); err != nil {
		t.Fatalf("persist coder: %v", err)
	}
	if err := a.persistRoleChoiceToRules(dir, "planlayıcı", "claude"); err != nil {
		t.Fatalf("persist planner: %v", err)
	}
	// Update the coder choice — the single directive line must be rewritten,
	// not duplicated.
	if err := a.persistRoleChoiceToRules(dir, "kodlayıcı", "gemini"); err != nil {
		t.Fatalf("update coder: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := parseRoleDirectives(string(data))
	if got.Coder != "gemini" || got.Planner != "claude" {
		t.Fatalf("after updates: %+v\nfile:\n%s", got, data)
	}
	// Exactly one directive line.
	if n := roleDirectiveRe.FindAllStringIndex(string(data), -1); len(n) != 1 {
		t.Fatalf("want 1 directive line, found %d", len(n))
	}
}
