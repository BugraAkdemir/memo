package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestSkill(t *testing.T, dir, skillName, content string) string {
	t.Helper()
	skillDir := filepath.Join(dir, skillName)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return skillDir
}

func TestParseSkill_OpenCodeFormat(t *testing.T) {
	content := `---
name: brainstorming
description: "Creative brainstorming skill"
danger_level: safe
---

# Brainstorming

Help turn ideas into designs.

## Usage

Use this when exploring ideas.
`
	def, err := ParseSkill([]byte(content), "/skills/test-skill")
	if err != nil {
		t.Fatalf("ParseSkill() error: %v", err)
	}

	if def.Manifest.Name != "brainstorming" {
		t.Errorf("Name = %q, want %q", def.Manifest.Name, "brainstorming")
	}
	if def.Manifest.Description != "Creative brainstorming skill" {
		t.Errorf("Description = %q, want %q", def.Manifest.Description, "Creative brainstorming skill")
	}
	if def.Manifest.DangerLevel != DangerLevelSafe {
		t.Errorf("DangerLevel = %q, want %q", def.Manifest.DangerLevel, DangerLevelSafe)
	}
	if !containsStr(def.Instructions, "Help turn ideas into designs") {
		t.Errorf("Instructions missing expected content: %s", def.Instructions)
	}
}

func TestParseSkill_InlineInstructions(t *testing.T) {
	content := `---
name: greeter
description: "Greets users"
danger_level: safe
instructions: "Always greet the user warmly and ask about their day."
---
`
	def, err := ParseSkill([]byte(content), "/skills/greeter")
	if err != nil {
		t.Fatalf("ParseSkill() error: %v", err)
	}
	if def.Instructions != "Always greet the user warmly and ask about their day." {
		t.Errorf("Instructions = %q, want greeting text", def.Instructions)
	}
}

func TestParseSkill_MissingFrontMatter(t *testing.T) {
	content := `# Just a markdown file
No front matter here.
`
	_, err := ParseSkill([]byte(content), "/skills/bad")
	if err == nil {
		t.Fatal("expected error for missing front matter")
	}
}

func TestParseSkill_MissingName(t *testing.T) {
	content := `---
description: "No name"
danger_level: safe
---
Some instructions
`
	_, err := ParseSkill([]byte(content), "/skills/bad")
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseSkill_WithTools(t *testing.T) {
	content := `---
name: coder
description: "Code assistant"
danger_level: medium
tools:
  - name: format_code
    description: "Formats code according to style guide"
    parameters:
      type: object
      properties:
        code:
          type: string
    danger_level: safe
---

# Coder Skill

Helps with coding tasks.
`
	def, err := ParseSkill([]byte(content), "/skills/coder")
	if err != nil {
		t.Fatalf("ParseSkill() error: %v", err)
	}
	if len(def.Manifest.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(def.Manifest.Tools))
	}
	if def.Manifest.Tools[0].Name != "format_code" {
		t.Errorf("Tool.Name = %q, want %q", def.Manifest.Tools[0].Name, "format_code")
	}
}

func TestParseSkill_UnquotedColonInDescription(t *testing.T) {
	// Reproduces a real ~/.claude/skills SKILL.md that fails strict YAML
	// parsing verbatim: an unquoted description containing "Triggers on: ...".
	content := `---
name: codebase-memory
description: Use the codebase knowledge graph for structural queries. Triggers on: explore the codebase, trace the call chain, find callers of.
---
Instructions body
`
	def, err := ParseSkill([]byte(content), "/skills/codebase-memory")
	if err != nil {
		t.Fatalf("ParseSkill() error: %v", err)
	}
	if def.Manifest.Name != "codebase-memory" {
		t.Errorf("Name = %q", def.Manifest.Name)
	}
	want := "Use the codebase knowledge graph for structural queries. Triggers on: explore the codebase, trace the call chain, find callers of."
	if def.Manifest.Description != want {
		t.Errorf("Description = %q, want %q", def.Manifest.Description, want)
	}
}

func TestParseSkill_ToolsBlockUnaffectedBySanitizer(t *testing.T) {
	// The nested `- name:`/`description:` lines inside `tools:` must not be
	// touched by the top-level-only sanitizer, even though they share key
	// names with the manifest's own top-level fields.
	content := `---
name: coder
description: "Code assistant"
tools:
  - name: format_code
    description: "Formats: applies style rules"
    danger_level: safe
---
Coder instructions
`
	def, err := ParseSkill([]byte(content), "/skills/coder")
	if err != nil {
		t.Fatalf("ParseSkill() error: %v", err)
	}
	if len(def.Manifest.Tools) != 1 || def.Manifest.Tools[0].Name != "format_code" {
		t.Fatalf("Tools = %+v", def.Manifest.Tools)
	}
	if def.Manifest.Tools[0].Description != "Formats: applies style rules" {
		t.Errorf("Tool.Description = %q", def.Manifest.Tools[0].Description)
	}
}

func TestDiscoverSkills(t *testing.T) {
	dir := t.TempDir()

	writeTestSkill(t, dir, "skill-a", `---
name: skill-a
description: "First skill"
danger_level: safe
---
Instructions A
`)

	writeTestSkill(t, dir, "skill-b", `---
name: skill-b
description: "Second skill"
danger_level: medium
---
Instructions B
`)

	skills, err := DiscoverSkills(dir)
	if err != nil {
		t.Fatalf("DiscoverSkills() error: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("len(skills) = %d, want 2", len(skills))
	}
}

func TestDiscoverSkills_SkipsInvalid(t *testing.T) {
	dir := t.TempDir()

	writeTestSkill(t, dir, "valid", `---
name: valid
description: "Valid skill"
danger_level: safe
---
Instructions
`)

	invalidDir := filepath.Join(dir, "invalid-skill")
	if err := os.MkdirAll(invalidDir, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(invalidDir, "SKILL.md"), []byte(`---
description: "No name"
danger_level: safe
---
Instructions
`), 0644)

	skills, err := DiscoverSkills(dir)
	if err != nil {
		t.Fatalf("DiscoverSkills() error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1 (invalid should be skipped)", len(skills))
	}
}

func TestLoadSkill(t *testing.T) {
	dir := t.TempDir()
	skillDir := writeTestSkill(t, dir, "loadtest", `---
name: loadtest
description: "Load test skill"
danger_level: safe
---
Load test instructions
`)

	def, err := LoadSkill(skillDir)
	if err != nil {
		t.Fatalf("LoadSkill() error: %v", err)
	}
	if def.Manifest.Name != "loadtest" {
		t.Errorf("Name = %q", def.Manifest.Name)
	}
	if def.Path != skillDir {
		t.Errorf("Path = %q, want %q", def.Path, skillDir)
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
