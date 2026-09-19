package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"memo/internal/agent"
	"memo/internal/skill"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("test relies on a Unix shell (bash -c)")
	}
}

func writeSkillWithTool(t *testing.T, mgr *skill.Manager, name, command string) {
	t.Helper()
	skillDir := filepath.Join(mgr.SkillsDir(), name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\n" +
		"name: " + name + "\n" +
		"description: \"test skill\"\n" +
		"danger_level: safe\n" +
		"tools:\n" +
		"  - name: dothing\n" +
		"    description: \"does a thing\"\n" +
		"    danger_level: safe\n" +
		"    command: \"" + command + "\"\n" +
		"---\n" +
		"Instructions.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestSkillToolRegistrar_RegistersRealAgentTool is the end-to-end
// regression test for TD-1 (BUG_REPORT.md): before this, SetToolRegistrar
// was never called in prod code, so a skill with a `tools:` entry never
// made it callable by the agent. Here we wire the registrar exactly as
// app.go's Startup() does and drive it through RegisterAllTools (tools are
// registered as soon as a skill is known now, no activation step involved
// — see skill.Manager's doc comments), then execute the registered tool
// through the real agent.ToolRegistry the pipeline calls. Whether a given
// chat is actually allowed to use it is enforced separately, per-chat, at
// Pipeline/ExecuteToolCall dispatch time (see internal/agent/pipeline_test.go's
// TestRunStream_SkillToolBlockedWhenSkillNotActive) — not by anything in
// this registry.
func TestSkillToolRegistrar_RegistersRealAgentTool(t *testing.T) {
	skipOnWindows(t)

	dataDir := t.TempDir()
	t.Setenv("MEMO_DATA_DIR", dataDir)

	skillMgr := skill.NewManager(dataDir)
	writeSkillWithTool(t, skillMgr, "greeter", "cat")
	if err := skillMgr.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	exec := agent.NewExecutor(t.TempDir(), nil, nil, nil)
	skillMgr.SetToolRegistrar(newSkillToolRegistrar(exec.Registry(), skillMgr))
	skillMgr.RegisterAllTools()

	toolDef, ok := exec.Registry().Get("skill_greeter_dothing")
	if !ok {
		t.Fatal("expected skill_greeter_dothing to be registered in the agent tool registry")
	}
	if toolDef.DangerLevel != agent.Safe {
		t.Errorf("DangerLevel = %v, want %v", toolDef.DangerLevel, agent.Safe)
	}
	if toolDef.SkillOwner != "greeter" {
		t.Errorf("SkillOwner = %q, want %q", toolDef.SkillOwner, "greeter")
	}

	result, err := exec.Registry().Execute(context.Background(), "skill_greeter_dothing", json.RawMessage(`{"msg":"hi"}`), t.TempDir(), func(string) error { return nil })
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(result, `"msg":"hi"`) {
		t.Errorf("result = %q, want it to contain the args delivered on stdin", result)
	}
}

func TestSkillToolRegistrar_DeclarativeOnlyToolNotRegistered(t *testing.T) {
	dataDir := t.TempDir()
	skillMgr := skill.NewManager(dataDir)

	skillDir := filepath.Join(skillMgr.SkillsDir(), "docs-only")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\n" +
		"name: docs-only\n" +
		"description: \"no executable tools\"\n" +
		"danger_level: safe\n" +
		"tools:\n" +
		"  - name: reference\n" +
		"    description: \"documented, not executable\"\n" +
		"    danger_level: safe\n" +
		"---\n" +
		"Instructions.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := skillMgr.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	exec := agent.NewExecutor(t.TempDir(), nil, nil, nil)
	skillMgr.SetToolRegistrar(newSkillToolRegistrar(exec.Registry(), skillMgr))
	skillMgr.RegisterAllTools()

	if _, ok := exec.Registry().Get("skill_docs-only_reference"); ok {
		t.Error("a tool with no command must not be registered as a callable agent tool")
	}
}

func TestSkillToolRegistrar_RejectsWrongType(t *testing.T) {
	exec := agent.NewExecutor(t.TempDir(), nil, nil, nil)
	skillMgr := skill.NewManager(t.TempDir())
	r := newSkillToolRegistrar(exec.Registry(), skillMgr)

	if err := r.RegisterTool("bogus", "not-a-registration"); err == nil {
		t.Error("expected an error when the registrar receives the wrong toolDef type")
	}
}
