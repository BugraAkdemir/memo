package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/sessions"
	"memo/internal/skill"
)

func newCodeModeApp(t *testing.T) (*App, *sessions.Manager) {
	t.Helper()
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	a := &App{
		cfg: &config.AppConfig{
			// memory ENABLED on purpose — Code Mode must suppress it anyway
			Memory: config.MemoryConfig{MemoryEnabled: true},
			Llama:  config.LlamaConfig{CtxSize: 4096},
			AgentMode: config.AgentModeConfig{
				WorkingSetEnabled: true, WorkingSetMaxTokens: 600,
			},
		},
		identity: identity.New("Tester", "Memo", "casual", "", false),
		sessions: sm,
	}
	return a, sm
}

func TestResolveCodeMode(t *testing.T) {
	a, sm := newCodeModeApp(t)

	plain := sm.NewChat()
	if a.resolveCodeMode(plain) {
		t.Error("plain chat should default to Code Mode OFF")
	}

	project := sm.NewAgentChat("/tmp/proj")
	if !a.resolveCodeMode(project) {
		t.Error("a project/agent chat should default to Code Mode ON")
	}

	// explicit override wins both ways
	off := false
	if err := sm.SetCodeMode(project, &off); err != nil {
		t.Fatal(err)
	}
	if a.resolveCodeMode(project) {
		t.Error("explicit &false must override the project-chat default")
	}
	on := true
	if err := sm.SetCodeMode(plain, &on); err != nil {
		t.Fatal(err)
	}
	if !a.resolveCodeMode(plain) {
		t.Error("explicit &true must override the plain-chat default")
	}

	if a.resolveCodeMode("") {
		t.Error("empty chatID should be OFF")
	}
}

func TestResolveCodeSubMode(t *testing.T) {
	a, sm := newCodeModeApp(t)

	chat := sm.NewAgentChat("/tmp/proj")
	if got := a.resolveCodeSubMode(chat); got != "auto" {
		t.Errorf("unpinned chat resolveCodeSubMode = %q, want \"auto\"", got)
	}

	for _, mode := range []string{"plan", "build", "auto"} {
		if err := sm.SetCodeSubMode(chat, mode); err != nil {
			t.Fatal(err)
		}
		if got := a.resolveCodeSubMode(chat); got != mode {
			t.Errorf("pinned resolveCodeSubMode = %q, want %q", got, mode)
		}
	}

	if err := sm.SetCodeSubMode(chat, ""); err != nil {
		t.Fatal(err)
	}
	if got := a.resolveCodeSubMode(chat); got != "auto" {
		t.Errorf("cleared pin resolveCodeSubMode = %q, want \"auto\"", got)
	}

	if got := a.resolveCodeSubMode(""); got != "auto" {
		t.Errorf("empty chatID resolveCodeSubMode = %q, want \"auto\"", got)
	}
}

func TestBuildMessagesForSession_CodeMode(t *testing.T) {
	a, sm := newCodeModeApp(t)
	chat := sm.NewAgentChat("/tmp/proj") // Code Mode on by default

	ctx := withCodeMode(context.Background())
	msgs := a.buildMessagesForSession(ctx, chat, "add a --verbose flag", nil, nil)

	var whole strings.Builder
	sawSystem := false
	for _, m := range msgs {
		if s, ok := m.Content.(string); ok {
			whole.WriteString(s)
			whole.WriteByte('\n')
			if m.Role == "system" {
				sawSystem = true
				if !strings.HasPrefix(s, "You are a coding agent") {
					t.Errorf("Code Mode system prompt should be the coding directive, got: %q", s)
				}
			}
		}
	}
	all := whole.String()
	if !sawSystem {
		t.Fatal("expected a system message carrying the coding directive")
	}
	for _, banned := range []string{"You are Memo", "AI friend", "[Time context]", "RELEVANT MEMORIES", "Communication Style"} {
		if strings.Contains(all, banned) {
			t.Errorf("Code Mode leaked chat-mode content: %q", banned)
		}
	}
}

// installGreeterSkill discovers a single-instruction "greeter" skill into a
// fresh manager and returns it, for tests that just need one active skill
// with a distinctive, greppable instruction string.
func installGreeterSkill(t *testing.T) *skill.Manager {
	t.Helper()
	skillMgr := skill.NewManager(t.TempDir())
	skillDir := filepath.Join(skillMgr.SkillsDir(), "greeter")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: greeter\ndescription: \"test skill\"\n---\n" +
		"Always greet the user by name before answering.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := skillMgr.Discover(); err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	return skillMgr
}

func TestBuildMessagesForSession_CodeModeIncludesActiveSkillInstructions(t *testing.T) {
	a, sm := newCodeModeApp(t)
	chat := sm.NewAgentChat("/tmp/proj") // Code Mode on by default

	a.skillManager = installGreeterSkill(t)
	if err := sm.SetActiveSkills(chat, []string{"greeter"}); err != nil {
		t.Fatalf("SetActiveSkills() error = %v", err)
	}

	ctx := withCodeMode(context.Background())
	msgs := a.buildMessagesForSession(ctx, chat, "add a --verbose flag", nil, nil)

	var found bool
	for _, m := range msgs {
		if s, ok := m.Content.(string); ok &&
			strings.Contains(s, "Always greet the user by name before answering.") {
			found = true
		}
	}
	if !found {
		t.Fatal("Code Mode must still inject an explicitly activated skill's instructions — the tool already dispatches in this chat, the model needs to know how to use it")
	}
}

func TestBuildMessagesForSession_CodeModeMinimalModeStillOmitsSkill(t *testing.T) {
	a, sm := newCodeModeApp(t)
	chat := sm.NewAgentChat("/tmp/proj")

	a.skillManager = installGreeterSkill(t)
	if err := sm.SetActiveSkills(chat, []string{"greeter"}); err != nil {
		t.Fatalf("SetActiveSkills() error = %v", err)
	}
	a.identity.SetMinimalMode(true)

	ctx := withCodeMode(context.Background())
	msgs := a.buildMessagesForSession(ctx, chat, "add a --verbose flag", nil, nil)

	for _, m := range msgs {
		if s, ok := m.Content.(string); ok &&
			strings.Contains(s, "Always greet the user by name before answering.") {
			t.Error("Minimal Mode must still strip skill instructions even in Code Mode")
		}
	}
}

func TestBuildMessagesForSession_CodeModeOffKeepsPersona(t *testing.T) {
	a, sm := newCodeModeApp(t)
	chat := sm.NewChat() // plain chat, Code Mode off

	msgs := a.buildMessagesForSession(context.Background(), chat, "hi", nil, nil)
	found := false
	for _, m := range msgs {
		if s, ok := m.Content.(string); ok && strings.Contains(s, "You are Memo") {
			found = true
		}
	}
	if !found {
		t.Error("a plain chat must still get the normal persona")
	}
}
