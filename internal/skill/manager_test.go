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

func TestManagerActivateDeactivate(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	skillsDir := filepath.Join(dir, skillsDirName)
	os.MkdirAll(skillsDir, 0755)
	writeTestSkill(t, skillsDir, "active-test", `---
name: active-test
description: "Active test"
danger_level: safe
---
Active instructions
`)
	m.Discover()

	if m.IsActive("active-test") {
		t.Fatal("should not be active yet")
	}

	if err := m.SetActive([]string{"active-test"}); err != nil {
		t.Fatalf("SetActive() error: %v", err)
	}

	if !m.IsActive("active-test") {
		t.Fatal("should be active")
	}

	active := m.GetActiveNames()
	if len(active) != 1 || active[0] != "active-test" {
		t.Errorf("GetActiveNames() = %v", active)
	}

	if err := m.SetActive(nil); err != nil {
		t.Fatalf("SetActive(nil) error: %v", err)
	}
	if m.IsActive("active-test") {
		t.Fatal("should be inactive after SetActive(nil)")
	}
}

func TestManagerActiveInstructions(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	skillsDir := filepath.Join(dir, skillsDirName)
	os.MkdirAll(skillsDir, 0755)
	writeTestSkill(t, skillsDir, "test-skill", `---
name: test-skill
description: "Test instructions"
danger_level: safe
---
Instructions content here
`)
	m.Discover()
	m.SetActive([]string{"test-skill"})

	activations := m.ActiveInstructions()
	if len(activations) != 1 {
		t.Fatalf("len(ActiveInstructions()) = %d", len(activations))
	}
	if activations[0].Name != "test-skill" {
		t.Errorf("Name = %q", activations[0].Name)
	}
	if activations[0].Instructions != "Instructions content here" {
		t.Errorf("Instructions mismatch")
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

// TestManagerSetActive_PersistsAcrossRestart is the regression test for
// "activation only lived in memory": a fresh Manager instance pointed at
// the same baseDir (simulating an app restart) used to always start with
// every skill inactive, no matter what the user had turned on before.
func TestManagerSetActive_PersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, skillsDirName)
	os.MkdirAll(skillsDir, 0755)
	writeTestSkill(t, skillsDir, "persistent", `---
name: persistent
description: "Persistence test"
danger_level: safe
---
Persistent instructions
`)

	m1 := NewManager(dir)
	if err := m1.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if err := m1.SetActive([]string{"persistent"}); err != nil {
		t.Fatalf("SetActive() error: %v", err)
	}

	// A brand new Manager, same baseDir — this is what app.go's Startup()
	// actually constructs on every launch.
	m2 := NewManager(dir)
	if err := m2.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if m2.IsActive("persistent") {
		t.Fatal("m2 should start inactive before LoadActiveSkills() runs")
	}
	if err := m2.LoadActiveSkills(); err != nil {
		t.Fatalf("LoadActiveSkills() error: %v", err)
	}
	if !m2.IsActive("persistent") {
		t.Fatal("LoadActiveSkills() did not restore activation from the previous Manager instance")
	}
}

// TestManagerLoadActiveSkills_DropsRemovedSkillsSilently covers a name in
// the persisted file that no longer corresponds to an installed skill
// (removed since last session) — must not error and must not block
// restoring the other, still-valid names.
func TestManagerLoadActiveSkills_DropsRemovedSkillsSilently(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, skillsDirName)
	os.MkdirAll(skillsDir, 0755)
	writeTestSkill(t, skillsDir, "still-here", `---
name: still-here
description: "Still installed"
danger_level: safe
---
Instructions
`)

	m := NewManager(dir)
	if err := m.Discover(); err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	data := `["still-here", "long-since-removed"]`
	if err := os.WriteFile(m.ActiveSkillsPath(), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	if err := m.LoadActiveSkills(); err != nil {
		t.Fatalf("LoadActiveSkills() error: %v", err)
	}
	if !m.IsActive("still-here") {
		t.Fatal("still-installed skill should be restored active")
	}
}

func TestManagerLoadActiveSkills_NoFileIsNotAnError(t *testing.T) {
	m := NewManager(t.TempDir())
	if err := m.LoadActiveSkills(); err != nil {
		t.Fatalf("LoadActiveSkills() with no persisted file should be a no-op, got error: %v", err)
	}
}

func TestManagerSetActiveInvalid(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	if err := m.SetActive([]string{"nonexistent"}); err == nil {
		t.Fatal("expected error for non-existent skill activation")
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
