package skill

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeExternalSkill(t *testing.T, root, name, description string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: \"" + description + "\"\n---\nInstructions for " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSyncExternalSkills_ImportsAndActivates(t *testing.T) {
	dataDir := t.TempDir()
	sourceDir := t.TempDir()
	m := NewManager(dataDir)

	writeExternalSkill(t, sourceDir, "claude-only", "A Claude Code skill")

	result, err := SyncExternalSkills(m, []ExternalSource{{ID: "claude-code", Name: "Claude Code", Dirs: []string{sourceDir}}})
	if err != nil {
		t.Fatalf("SyncExternalSkills() error: %v", err)
	}
	if len(result.Imported) != 1 || result.Imported[0] != "claude-only" {
		t.Fatalf("Imported = %v, want [claude-only]", result.Imported)
	}

	def, ok := m.Get("claude-only")
	if !ok {
		t.Fatal("imported skill not found in manager")
	}
	if def.Manifest.Description != "A Claude Code skill" {
		t.Errorf("Description = %q", def.Manifest.Description)
	}
	if !m.IsActive("claude-only") {
		t.Error("imported skill should be auto-activated")
	}
}

// TestSyncExternalSkills_RespectsManualDeactivation is the regression test
// for "an imported skill you turned off comes back on by itself": the
// original auto-activate logic re-scanned every skill the registry had
// *ever* imported on *every* sync and force-activated any that weren't
// currently active — indistinguishable from "never activated yet" once
// activation wasn't persisted across restarts either. Only genuinely new
// imports (result.Imported) should ever be auto-activated.
func TestSyncExternalSkills_RespectsManualDeactivation(t *testing.T) {
	dataDir := t.TempDir()
	sourceDir := t.TempDir()
	m := NewManager(dataDir)
	sources := []ExternalSource{{ID: "claude-code", Name: "Claude Code", Dirs: []string{sourceDir}}}

	writeExternalSkill(t, sourceDir, "opt-out", "Auto-imported skill")

	if _, err := SyncExternalSkills(m, sources); err != nil {
		t.Fatalf("first sync error: %v", err)
	}
	if !m.IsActive("opt-out") {
		t.Fatal("first import should auto-activate")
	}

	if err := m.SetActive(nil); err != nil {
		t.Fatalf("SetActive(nil) error: %v", err)
	}

	result, err := SyncExternalSkills(m, sources)
	if err != nil {
		t.Fatalf("second sync error: %v", err)
	}
	if len(result.Imported) != 0 {
		t.Fatalf("unchanged skill should not be re-imported, got %+v", result.Imported)
	}
	if m.IsActive("opt-out") {
		t.Fatal("sync re-activated a skill the user had manually turned off")
	}
}

func TestSyncExternalSkills_SecondRunIsNoOp(t *testing.T) {
	dataDir := t.TempDir()
	sourceDir := t.TempDir()
	m := NewManager(dataDir)
	sources := []ExternalSource{{ID: "claude-code", Name: "Claude Code", Dirs: []string{sourceDir}}}

	writeExternalSkill(t, sourceDir, "stable", "Unchanged skill")

	if _, err := SyncExternalSkills(m, sources); err != nil {
		t.Fatalf("first sync error: %v", err)
	}

	result, err := SyncExternalSkills(m, sources)
	if err != nil {
		t.Fatalf("second sync error: %v", err)
	}
	if len(result.Imported) != 0 || len(result.Updated) != 0 {
		t.Fatalf("second sync should be a no-op, got %+v", result)
	}
}

func TestSyncExternalSkills_ReimportsOnContentChange(t *testing.T) {
	dataDir := t.TempDir()
	sourceDir := t.TempDir()
	m := NewManager(dataDir)
	sources := []ExternalSource{{ID: "claude-code", Name: "Claude Code", Dirs: []string{sourceDir}}}

	skillDir := writeExternalSkill(t, sourceDir, "evolving", "First version")
	if _, err := SyncExternalSkills(m, sources); err != nil {
		t.Fatalf("first sync error: %v", err)
	}

	// Force a distinct mtime — dirSignature keys off size/mtime, and the
	// two writes could otherwise land in the same second on a fast test run.
	time.Sleep(10 * time.Millisecond)
	content := "---\nname: evolving\ndescription: \"Second version\"\n---\nUpdated instructions\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(skillDir, "SKILL.md"), time.Now().Add(time.Minute), time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	result, err := SyncExternalSkills(m, sources)
	if err != nil {
		t.Fatalf("second sync error: %v", err)
	}
	if len(result.Updated) != 1 || result.Updated[0] != "evolving" {
		t.Fatalf("Updated = %v, want [evolving]", result.Updated)
	}

	def, _ := m.Get("evolving")
	if def.Manifest.Description != "Second version" {
		t.Errorf("Description = %q, want %q", def.Manifest.Description, "Second version")
	}
}

func TestSyncExternalSkills_SkipsNameCollisionWithManualSkill(t *testing.T) {
	dataDir := t.TempDir()
	sourceDir := t.TempDir()
	m := NewManager(dataDir)

	// A skill the user installed/authored themselves, not via import.
	manualSourceDir := writeExternalSkill(t, t.TempDir(), "shared-name", "Manually authored")
	if _, err := m.Install(manualSourceDir); err != nil {
		t.Fatalf("manual Install() error: %v", err)
	}

	writeExternalSkill(t, sourceDir, "shared-name", "From Claude Code")

	result, err := SyncExternalSkills(m, []ExternalSource{{ID: "claude-code", Name: "Claude Code", Dirs: []string{sourceDir}}})
	if err != nil {
		t.Fatalf("SyncExternalSkills() error: %v", err)
	}
	if len(result.Skipped) != 1 || result.Skipped[0] != "shared-name" {
		t.Fatalf("Skipped = %v, want [shared-name]", result.Skipped)
	}

	def, _ := m.Get("shared-name")
	if def.Manifest.Description != "Manually authored" {
		t.Errorf("manual skill was overwritten: Description = %q", def.Manifest.Description)
	}
}

func TestSyncExternalSkills_MissingSourceDirIsNotAnError(t *testing.T) {
	dataDir := t.TempDir()
	m := NewManager(dataDir)

	result, err := SyncExternalSkills(m, []ExternalSource{{ID: "claude-code", Name: "Claude Code", Dirs: []string{filepath.Join(t.TempDir(), "does-not-exist")}}})
	if err != nil {
		t.Fatalf("SyncExternalSkills() error: %v", err)
	}
	if len(result.Imported) != 0 || len(result.Updated) != 0 || len(result.Skipped) != 0 {
		t.Fatalf("expected empty result for missing source dir, got %+v", result)
	}
}

func TestKnownExternalSources_IncludesClaudeCode(t *testing.T) {
	sources := KnownExternalSources()
	found := false
	for _, s := range sources {
		if s.ID == "claude-code" {
			found = true
			if len(s.Dirs) != 1 || filepath.Base(s.Dirs[0]) != "skills" {
				t.Errorf("claude-code Dirs = %v", s.Dirs)
			}
		}
	}
	if !found {
		t.Error("KnownExternalSources() missing claude-code entry")
	}
}
