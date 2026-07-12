package skill

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// newManagerWithSkill creates a Manager over dir and writes a test skill
// into its skills/ subdirectory (Manager.SkillsDir(), not dir itself).
func newManagerWithSkill(t *testing.T, dir, skillName, content string) (*Manager, string) {
	t.Helper()
	m := NewManager(dir)
	skillDir := writeTestSkill(t, m.SkillsDir(), skillName, content)
	return m, skillDir
}

// writeExecutableScript writes scripts/hello.sh inside a skill directory,
// mirroring how a real skill would bundle a helper script next to SKILL.md.
func writeExecutableScript(skillDir, content string) error {
	scriptsDir := filepath.Join(skillDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(scriptsDir, "hello.sh"), []byte(content), 0755)
}

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("test relies on a Unix shell (bash -c); PrepareCommand uses cmd /C on Windows")
	}
}

func TestExecuteTool_Success_ReceivesArgsOnStdin(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	m, _ := newManagerWithSkill(t, dir, "echoer", `---
name: echoer
description: "Echoes its stdin"
danger_level: safe
tools:
  - name: echo_args
    description: "Echoes call args back"
    danger_level: safe
    command: "cat"
---
Instructions.
`)
	if err := m.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	args := json.RawMessage(`{"hello":"world"}`)
	out, err := m.ExecuteTool(context.Background(), "echoer", "echo_args", args, "/tmp")
	if err != nil {
		t.Fatalf("ExecuteTool() error: %v", err)
	}
	if !strings.Contains(out, `"hello":"world"`) {
		t.Errorf("output = %q, want it to contain the stdin-delivered args", out)
	}
}

func TestExecuteTool_EnvVarsArePopulated(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	m, _ := newManagerWithSkill(t, dir, "envtest", `---
name: envtest
description: "Reports its env"
danger_level: safe
tools:
  - name: report_env
    description: "Prints skill env vars"
    danger_level: safe
    command: "echo $MEMO_SKILL_NAME:$MEMO_PROJECT_DIR"
---
Instructions.
`)
	if err := m.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	out, err := m.ExecuteTool(context.Background(), "envtest", "report_env", json.RawMessage(`{}`), "/my/project")
	if err != nil {
		t.Fatalf("ExecuteTool() error: %v", err)
	}
	if !strings.Contains(out, "envtest:/my/project") {
		t.Errorf("output = %q, want it to contain envtest:/my/project", out)
	}
}

func TestExecuteTool_MissingCommand(t *testing.T) {
	dir := t.TempDir()
	m, _ := newManagerWithSkill(t, dir, "declarative", `---
name: declarative
description: "A tool listed only for documentation"
danger_level: safe
tools:
  - name: no_op
    description: "Has no command"
    danger_level: safe
---
Instructions.
`)
	if err := m.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	if _, err := m.ExecuteTool(context.Background(), "declarative", "no_op", json.RawMessage(`{}`), "/tmp"); err == nil {
		t.Fatal("expected error for a tool with no command configured")
	}
}

func TestExecuteTool_UnknownSkillOrTool(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	if _, err := m.ExecuteTool(context.Background(), "nonexistent", "whatever", json.RawMessage(`{}`), "/tmp"); err == nil {
		t.Fatal("expected error for unknown skill")
	}

	writeTestSkill(t, m.SkillsDir(), "hasnothing", `---
name: hasnothing
description: "No tools"
danger_level: safe
---
Instructions.
`)
	if err := m.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if _, err := m.ExecuteTool(context.Background(), "hasnothing", "whatever", json.RawMessage(`{}`), "/tmp"); err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestExecuteTool_BlacklistedCommand(t *testing.T) {
	dir := t.TempDir()
	m, _ := newManagerWithSkill(t, dir, "dangerous", `---
name: dangerous
description: "Tries something destructive"
danger_level: dangerous
tools:
  - name: wipe
    description: "Wipes root"
    danger_level: dangerous
    command: "sudo rm -rf /"
---
Instructions.
`)
	if err := m.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	if _, err := m.ExecuteTool(context.Background(), "dangerous", "wipe", json.RawMessage(`{}`), "/tmp"); err == nil {
		t.Fatal("expected the blacklist to reject this command")
	}
}

func TestExecuteTool_ScriptRelativeToSkillDir(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	m, skillDir := newManagerWithSkill(t, dir, "scripted", `---
name: scripted
description: "Runs a bundled script"
danger_level: safe
tools:
  - name: run_script
    description: "Runs scripts/hello.sh"
    danger_level: safe
    command: "bash scripts/hello.sh"
---
Instructions.
`)

	if err := writeExecutableScript(skillDir, `#!/bin/bash
echo "hello from bundled script"
`); err != nil {
		t.Fatalf("writeExecutableScript: %v", err)
	}

	if err := m.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	out, err := m.ExecuteTool(context.Background(), "scripted", "run_script", json.RawMessage(`{}`), "/tmp")
	if err != nil {
		t.Fatalf("ExecuteTool() error: %v", err)
	}
	if !strings.Contains(out, "hello from bundled script") {
		t.Errorf("output = %q, want the bundled script's output", out)
	}
}
