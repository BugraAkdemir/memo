package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
}

func TestManagerDiscover(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	skillsDir := filepath.Join(dir, skillsDirName)
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeTestSkill(t, skillsDir, "discover-test", `---
name: discover-test
description: "Discovery test"
danger_level: safe
---
Discovery instructions
`)

	if err := m.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	skills := m.List()
	if len(skills) != 1 {
		t.Fatalf("len(List()) = %d, want 1", len(skills))
	}
	if skills[0].Manifest.Name != "discover-test" {
		t.Errorf("Name = %q", skills[0].Manifest.Name)
	}
}

func TestManagerGet(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	skillsDir := filepath.Join(dir, skillsDirName)
	os.MkdirAll(skillsDir, 0755)
	writeTestSkill(t, skillsDir, "get-test", `---
name: get-test
description: "Get test"
danger_level: safe
---
Get instructions
`)
	m.Discover()

	def, ok := m.Get("get-test")
	if !ok {
		t.Fatal("Get() returned false")
	}
	if def.Manifest.Name != "get-test" {
		t.Errorf("Name = %q", def.Manifest.Name)
	}

	_, ok = m.Get("nonexistent")
	if ok {
		t.Fatal("Get('nonexistent') should return false")
	}
}

// fakeToolRegistrar is a minimal ToolRegistrar recording every
// register/unregister call, used to prove tools get wired up without any
// "activate" step now that registration happens at install/discover time.
type fakeToolRegistrar struct {
	registered map[string]bool
}

func newFakeToolRegistrar() *fakeToolRegistrar {
	return &fakeToolRegistrar{registered: make(map[string]bool)}
}

func (f *fakeToolRegistrar) RegisterTool(name string, toolDef any) error {
	f.registered[name] = true
	return nil
}

func (f *fakeToolRegistrar) UnregisterTool(name string) {
	delete(f.registered, name)
}

// TestManagerRegisterAllTools_RegistersWithoutActivation is the regression
// test for the old model where a skill's tools only reached the agent's
// ToolRegistry once something called SetActive — that concept is gone now;
// a skill's tools must be registered as soon as the skill is known,
// regardless of whether any chat has turned it on (per-chat visibility is
// enforced later, at dispatch time, via agent.ToolDef.SkillOwner).
func TestManagerRegisterAllTools_RegistersWithoutActivation(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	skillsDir := filepath.Join(dir, skillsDirName)
	os.MkdirAll(skillsDir, 0755)
	writeTestSkill(t, skillsDir, "tooled", `---
name: tooled
description: "Has a tool"
danger_level: safe
tools:
  - name: dothing
    description: "does a thing"
    danger_level: safe
    command: "cat"
---
Instructions
`)
	if err := m.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	reg := newFakeToolRegistrar()
	m.SetToolRegistrar(reg)
	m.RegisterAllTools()

	if !reg.registered["skill_tooled_dothing"] {
		t.Fatal("expected skill_tooled_dothing to be registered by RegisterAllTools with no activation step")
	}
}

// TestManagerInstall_RegistersToolsImmediately covers the runtime-install
// path (as opposed to startup's Discover+RegisterAllTools): a skill
// installed after SetToolRegistrar must get its tools wired up right away.
func TestManagerInstall_RegistersToolsImmediately(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	reg := newFakeToolRegistrar()
	m.SetToolRegistrar(reg)

	sourceDir := filepath.Join(dir, "source-tooled")
	skillSourceDir := writeTestSkill(t, sourceDir, "runtime-tooled", `---
name: runtime-tooled
description: "Installed at runtime"
danger_level: safe
tools:
  - name: dothing
    description: "does a thing"
    danger_level: safe
    command: "cat"
---
Instructions
`)

	if _, err := m.Install(skillSourceDir); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if !reg.registered["skill_runtime-tooled_dothing"] {
		t.Fatal("expected Install() to register the skill's tool immediately")
	}
}

func TestManagerInstall(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	sourceDir := filepath.Join(dir, "source-skill")
	skillSourceDir := writeTestSkill(t, sourceDir, "installed-skill", `---
name: installed-skill
description: "Installed from path"
danger_level: safe
---
Installed instructions
`)

	def, err := m.Install(skillSourceDir)
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if def.Manifest.Name != "installed-skill" {
		t.Errorf("Name = %q", def.Manifest.Name)
	}

	skills := m.List()
	if len(skills) != 1 {
		t.Fatalf("len(List()) = %d after install", len(skills))
	}

	skillDir := filepath.Join(dir, skillsDirName, "installed-skill")
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not found at target: %v", err)
	}

	_, err = m.Install(skillSourceDir)
	if err == nil {
		t.Fatal("duplicate install should error")
	}
}

func TestManagerRemove(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	skillsDir := filepath.Join(dir, skillsDirName)
	os.MkdirAll(skillsDir, 0755)
	writeTestSkill(t, skillsDir, "removable", `---
name: removable
description: "Removable skill"
danger_level: safe
---
Remove instructions
`)
	m.Discover()

	if err := m.Remove("removable"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if len(m.List()) != 0 {
		t.Fatal("skill not removed")
	}

	if err := m.Remove("nonexistent"); err == nil {
		t.Fatal("expected error for non-existent skill")
	}
}

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.MkdirAll(filepath.Join(src, "subdir"), 0755)
	os.WriteFile(filepath.Join(src, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(src, "subdir", "file2.txt"), []byte("content2"), 0644)

	if err := copyDir(src, filepath.Join(dst, "copied")); err != nil {
		t.Fatalf("copyDir() error: %v", err)
	}

	data1, _ := os.ReadFile(filepath.Join(dst, "copied", "file1.txt"))
	if string(data1) != "content1" {
		t.Errorf("file1 content = %q", string(data1))
	}

	data2, _ := os.ReadFile(filepath.Join(dst, "copied", "subdir", "file2.txt"))
	if string(data2) != "content2" {
		t.Errorf("file2 content = %q", string(data2))
	}
}
