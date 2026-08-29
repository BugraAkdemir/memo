package skill

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func embeddedFixture() fstest.MapFS {
	return fstest.MapFS{
		"memo-system/SKILL.md": &fstest.MapFile{Data: []byte(`---
name: memo-system
description: "Memo self-management guidance"
danger_level: safe
---

# memo-system

Engine-only guidance.
`)},
		"memo-system/reference/notes.md": &fstest.MapFile{Data: []byte("extra reference\n")},
	}
}

func TestMaterializeEmbedded_CreatesThenDiscovers(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	created, err := MaterializeEmbedded(m, "memo-system", embeddedFixture())
	if err != nil {
		t.Fatalf("MaterializeEmbedded: %v", err)
	}
	if !created {
		t.Fatal("created = false on first run, want true")
	}

	// The whole tree landed on disk.
	if _, err := os.Stat(filepath.Join(m.SkillsDir(), "memo-system", "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md not materialized: %v", err)
	}
	if _, err := os.Stat(filepath.Join(m.SkillsDir(), "memo-system", "reference", "notes.md")); err != nil {
		t.Fatalf("nested file not materialized: %v", err)
	}

	if err := m.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	def, ok := m.Get("memo-system")
	if !ok {
		t.Fatal("memo-system not discovered after materialize")
	}
	if def.Instructions == "" {
		t.Fatal("discovered skill has empty instructions")
	}
}

func TestMaterializeEmbedded_NoopWhenPresent(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	if _, err := MaterializeEmbedded(m, "memo-system", embeddedFixture()); err != nil {
		t.Fatalf("first MaterializeEmbedded: %v", err)
	}

	// Hand-edit the on-disk copy, then re-run: it must be left untouched.
	skillMd := filepath.Join(m.SkillsDir(), "memo-system", "SKILL.md")
	edited := []byte("user edited this\n")
	if err := os.WriteFile(skillMd, edited, 0644); err != nil {
		t.Fatal(err)
	}

	created, err := MaterializeEmbedded(m, "memo-system", embeddedFixture())
	if err != nil {
		t.Fatalf("second MaterializeEmbedded: %v", err)
	}
	if created {
		t.Fatal("created = true on second run, want false (already present)")
	}
	got, _ := os.ReadFile(skillMd)
	if string(got) != string(edited) {
		t.Fatalf("on-disk SKILL.md was overwritten: %q", string(got))
	}
}
