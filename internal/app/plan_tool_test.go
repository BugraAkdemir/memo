package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/config"
)

func TestProjectSlug(t *testing.T) {
	cases := []struct {
		name        string
		projectPath string
		fallback    string
		want        string
	}{
		{"normal name", "/home/user/Documents/my-project", "fallback-id", "my-project"},
		{"empty project path falls back", "", "fallback-id", "fallback-id"},
		{"trailing slash", "/home/user/Documents/my-project/", "fallback-id", "my-project"},
		{"dot-dot component resolves to a safe last element", "/home/user/../../etc/passwd", "fallback-id", "passwd"},
		{"bare dot-dot falls back (sanitizes to empty)", "/home/user/..", "fallback-id", "fallback-id"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := projectSlug(c.projectPath, c.fallback)
			if got != c.want {
				t.Errorf("projectSlug(%q, %q) = %q, want %q", c.projectPath, c.fallback, got, c.want)
			}
			// Whatever comes out must always be safe to use as a single path
			// component: no separators, no "..".
			if strings.ContainsAny(got, "/\\") || got == ".." || got == "." {
				t.Errorf("projectSlug(%q, %q) = %q is not a safe path component", c.projectPath, c.fallback, got)
			}
		})
	}

	// Spaces/unicode: not asserting an exact transliteration, just that the
	// result is a safe, non-empty path component.
	got := projectSlug("/home/user/Belgeler/Proje Adı Öğüş", "fallback-id")
	if got == "" || strings.ContainsAny(got, " /\\") {
		t.Errorf("projectSlug with spaces/unicode = %q, want a safe non-empty slug", got)
	}
}

func TestSaveCodePlan_WritesUnderDataPlans(t *testing.T) {
	a, sm := newCodeModeApp(t)

	projectDir := t.TempDir() // unique per test run, doubles as a collision-proof slug source
	chat := sm.NewAgentChat(projectDir)
	slug := projectSlug(projectDir, chat)
	dir := config.DataPath("plans", slug)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	ctx := withCurrentChatID(context.Background(), chat)
	msg, err := a.saveCodePlan(ctx, "# My Plan\n\n- do the thing")
	if err != nil {
		t.Fatalf("saveCodePlan: %v", err)
	}
	wantPath := filepath.Join(dir, "plan.md")
	if !strings.Contains(msg, wantPath) {
		t.Errorf("saveCodePlan message = %q, want it to mention %q", msg, wantPath)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("plan.md not written: %v", err)
	}
	if string(data) != "# My Plan\n\n- do the thing" {
		t.Errorf("plan.md content = %q", data)
	}
}

func TestSaveCodePlan_EmptyContentRejected(t *testing.T) {
	a, _ := newCodeModeApp(t)
	if _, err := a.saveCodePlan(context.Background(), "   \n  "); err == nil {
		t.Fatal("expected an error for an empty/whitespace-only plan")
	}
}

func TestSaveCodePlan_NoProjectPathFallsBackToChatID(t *testing.T) {
	a, sm := newCodeModeApp(t)
	chat := sm.NewChat() // no ProjectPath at all

	slug := projectSlug("", chat)
	dir := config.DataPath("plans", slug)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	ctx := withCurrentChatID(context.Background(), chat)
	if _, err := a.saveCodePlan(ctx, "# Plan without a project dir"); err != nil {
		t.Fatalf("saveCodePlan: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "plan.md")); err != nil {
		t.Fatalf("plan.md not written under the chat-ID fallback slug: %v", err)
	}
}
