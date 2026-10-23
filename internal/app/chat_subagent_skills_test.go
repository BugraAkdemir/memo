package app

import (
	"memo/internal/agent"
	"testing"
)

func TestChatSubAgentSelectedSkillToolsUseRegisteredNamesAndScope(t *testing.T) {
	exec := agent.NewExecutor(t.TempDir(), nil, nil, nil)
	exec.Registry().Register(agent.ToolDef{Name: "skill_alpha_formatter", SkillOwner: "alpha"})
	exec.Registry().Register(agent.ToolDef{Name: "skill_beta_formatter", SkillOwner: "beta"})
	a := &App{agentExecutor: exec}
	child, set := a.chatSubAgentToolRegistry(true, []string{"alpha"})
	if _, ok := child.Get("skill_alpha_formatter"); !ok || !set["alpha"] {
		t.Fatal("selected namespaced skill tool missing")
	}
	if _, ok := child.Get("skill_beta_formatter"); ok {
		t.Fatal("unselected skill leaked into child")
	}
	reader, _ := a.chatSubAgentToolRegistry(false, []string{"alpha"})
	if _, ok := reader.Get("skill_alpha_formatter"); ok {
		t.Fatal("read-only child acquired a custom shell tool")
	}
}
