package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/sessions"
	"memo/internal/skill"
)

// newTestAppWithActiveSkill creates a real skill.Manager backed by a temp
// dir, with one skill discovered whose SKILL.md body is exactly bodySize
// bytes of filler instructions — big enough to blow past a small token
// budget on purpose — and a real sessions.Manager with that skill activated
// on its active chat. Returns the App and that chat's ID, since
// buildActiveSkillPrompt is now chat-scoped rather than global.
func newTestAppWithActiveSkill(t *testing.T, name string, bodySize int) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(skillsDir, name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	body := strings.Repeat("word ", bodySize/5+1)
	content := "---\nname: " + name + "\ndescription: \"test skill\"\ndanger_level: safe\n---\n" + body
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m := skill.NewManager(dir)
	if err := m.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("sessions.NewManager() error: %v", err)
	}
	chatID := sm.GetActiveID()
	if err := sm.SetActiveSkills(chatID, []string{name}); err != nil {
		t.Fatalf("SetActiveSkills() error: %v", err)
	}

	return &App{skillManager: m, sessions: sm}, chatID
}

func TestBuildActiveSkillPrompt_NilManagerReturnsEmpty(t *testing.T) {
	a := &App{}
	if got := a.buildActiveSkillPrompt(""); got != "" {
		t.Errorf("buildActiveSkillPrompt() = %q, want empty", got)
	}
}

func TestBuildActiveSkillPrompt_NoBudgetIncludesEverything(t *testing.T) {
	a, chatID := newTestAppWithActiveSkill(t, "big-skill", 6000)

	got := a.buildActiveSkillPrompt(chatID)
	if !strings.Contains(got, "big-skill") {
		t.Errorf("expected the skill name in the output with no budget, got: %.200s...", got)
	}
	if strings.Contains(got, "not shown here") {
		t.Errorf("no budget should never omit anything, got: %.200s...", got)
	}
}

// Reported live: 5 simultaneously active Claude-Code-imported skills
// (testsprite-verify's SKILL.md alone ~25KB) injected their full,
// unbounded instructions into every system prompt with no size check at
// all, blowing a 4096-ctx local model's entire budget (~20K estimated
// tokens against a ~3840 budget) on a bare "selam" — confirmed via the
// live CONTEXT log line the user captured (system=20101 budget=3840).
// buildActiveSkillPrompt must now drop a whole skill rather than let an
// oversized one blow past the caller's budget.
func TestBuildActiveSkillPrompt_BudgetOmitsSkillThatDoesNotFit(t *testing.T) {
	a, chatID := newTestAppWithActiveSkill(t, "huge-skill", 6000)

	const budget = 100 // ~300 chars — far smaller than the ~6000-byte skill body
	got := a.buildActiveSkillPrompt(chatID, budget)

	if strings.Contains(got, "huge-skill") {
		t.Errorf("expected the oversized skill to be omitted, got: %.200s...", got)
	}
	if !strings.Contains(got, "not shown here") {
		t.Errorf("expected an omission note, got: %q", got)
	}
	if len(got)/3 > budget*2 {
		t.Errorf("output (%d bytes, ~%d est. tokens) should stay close to budget %d, not balloon", len(got), len(got)/3, budget)
	}
}

func TestBuildActiveSkillPrompt_SkillFittingBudgetIsIncluded(t *testing.T) {
	a, chatID := newTestAppWithActiveSkill(t, "small-skill", 20)

	got := a.buildActiveSkillPrompt(chatID, 2000)
	if !strings.Contains(got, "small-skill") {
		t.Errorf("expected a small skill to fit within a generous budget, got: %q", got)
	}
}

// TestBuildActiveSkillPrompt_ChatScoped is the regression test for the bug
// this whole per-chat rework fixes: a skill active in one chat must not
// leak its instructions into a different chat that never activated it.
func TestBuildActiveSkillPrompt_ChatScoped(t *testing.T) {
	a, activeChatID := newTestAppWithActiveSkill(t, "only-here", 20)

	sm := a.getSessionManager()
	otherChatID := sm.NewBackgroundChat("other chat")

	if got := a.buildActiveSkillPrompt(activeChatID); !strings.Contains(got, "only-here") {
		t.Errorf("expected the skill in its own chat's prompt, got: %q", got)
	}
	if got := a.buildActiveSkillPrompt(otherChatID); got != "" {
		t.Errorf("expected no skill prompt in an unrelated chat, got: %q", got)
	}
}
