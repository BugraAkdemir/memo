package app

import (
	"context"
	"strings"
	"testing"

	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/sessions"
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
