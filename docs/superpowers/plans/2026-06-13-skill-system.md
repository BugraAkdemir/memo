# Skill System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans.

**Goal:** Add opencode-style skill support to Memo's Agent and Orchestra modes.

**Architecture:** New `internal/skill/` package with `Manager` + `Loader`. Skills are `SKILL.md` files (YAML front matter + Markdown instructions). Active skills inject instructions into system prompt and optionally register tools into agent's `ToolRegistry`.

**Tech Stack:** Go 1.25, existing agent/orchestra systems

---

### Task 1: Create `internal/skill/types.go`

**Files:**
- Create: `internal/skill/types.go`
- Test: `internal/skill/types_test.go`

- [ ] **Step 1: Write types.go**

```go
package skill

import (
	"encoding/json"
	"time"
)

type DangerLevel string

const (
	DangerLevelSafe      DangerLevel = "safe"
	DangerLevelMedium    DangerLevel = "medium"
	DangerLevelDangerous DangerLevel = "dangerous"
)

type SkillTool struct {
	Name        string          `yaml:"name" json:"name"`
	Description string          `yaml:"description" json:"description"`
	Parameters  json.RawMessage `yaml:"parameters" json:"parameters"`
	DangerLevel DangerLevel     `yaml:"danger_level" json:"danger_level"`
}

type SkillManifest struct {
	Name        string         `yaml:"name" json:"name"`
	Description string         `yaml:"description" json:"description"`
	Version     string         `yaml:"version" json:"version,omitempty"`
	Author      string         `yaml:"author" json:"author,omitempty"`
	DangerLevel DangerLevel    `yaml:"danger_level" json:"danger_level"`
	License     string         `yaml:"license" json:"license,omitempty"`
	Metadata    map[string]any `yaml:"metadata" json:"metadata,omitempty"`
	Tools       []SkillTool    `yaml:"tools" json:"tools,omitempty"`
	Instructions string        `yaml:"instructions" json:"instructions,omitempty"`
}

type SkillDefinition struct {
	Manifest     SkillManifest
	Instructions string
	Path         string
	LoadedAt     time.Time
}
```

- [ ] **Step 2: Write types_test.go with constants test**

```go
package skill

import (
	"testing"
)

func TestDangerLevelConstants(t *testing.T) {
	tests := []struct {
		level DangerLevel
		want  string
	}{
		{DangerLevelSafe, "safe"},
		{DangerLevelMedium, "medium"},
		{DangerLevelDangerous, "dangerous"},
	}
	for _, tt := range tests {
		if string(tt.level) != tt.want {
			t.Errorf("DangerLevel(%s) = %q, want %q", tt.want, string(tt.level), tt.want)
		}
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /home/bugra/Belgeler/memo && go test ./internal/skill/ -v -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

---

### Task 2: Create `internal/skill/loader.go`

**Files:**
- Create: `internal/skill/loader.go`
- Create: `internal/skill/loader_test.go`
- Create: `testdata/skills/test-skill/SKILL.md` (or use `t.TempDir()`)

- [ ] **Step 1: Write loader.go**

```go
package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// frontMatterDelim is the YAML front matter delimiter.
const frontMatterDelim = "---"

// LoadSkill reads and parses a skill from a directory path.
// It looks for SKILL.md in the given directory.
func LoadSkill(dir string) (*SkillDefinition, error) {
	path := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md: %w", err)
	}

	return ParseSkill(data, dir)
}

// ParseSkill parses SKILL.md content (YAML front matter + Markdown body).
// Returns a SkillDefinition or an error.
func ParseSkill(data []byte, baseDir string) (*SkillDefinition, error) {
	content := string(data)

	// Detect YAML front matter (opencode-compatible format)
	if !hasFrontMatter(content) {
		return nil, fmt.Errorf("missing YAML front matter delimiters (---)")
	}

	manifest, body, err := extractFrontMatter(content)
	if err != nil {
		return nil, fmt.Errorf("extract front matter: %w", err)
	}

	def := &SkillDefinition{
		Manifest: *manifest,
		Path:     baseDir,
		LoadedAt: time.Now(),
	}

	// Determine instructions:
	// 1. If manifest has an `instructions` field, use it directly (single-file format)
	// 2. Otherwise, use the body after front matter (opencode format)
	if manifest.Instructions != "" {
		def.Instructions = manifest.Instructions
	} else {
		def.Instructions = strings.TrimSpace(body)
	}

	if def.Instructions == "" {
		return nil, fmt.Errorf("skill %q has no instructions", manifest.Name)
	}

	if manifest.Name == "" {
		return nil, fmt.Errorf("skill is missing required 'name' field")
	}

	return def, nil
}

// DiscoverSkills scans a directory for skill subdirectories (each containing SKILL.md).
func DiscoverSkills(skillsDir string) ([]*SkillDefinition, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read skills dir: %w", err)
	}

	var skills []*SkillDefinition
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(skillsDir, entry.Name())
		def, err := LoadSkill(skillDir)
		if err != nil {
			// Log but skip invalid skills
			fmt.Fprintf(os.Stderr, "skill: skip %q: %v\n", entry.Name(), err)
			continue
		}
		skills = append(skills, def)
	}
	return skills, nil
}

// hasFrontMatter checks if the content starts with ---.
func hasFrontMatter(content string) bool {
	trimmed := strings.TrimLeft(content, "\n\r\t ")
	return strings.HasPrefix(trimmed, frontMatterDelim)
}

// extractFrontMatter parses YAML front matter and returns the manifest + body text.
func extractFrontMatter(content string) (*SkillManifest, string, error) {
	// Find first ---
	first := strings.Index(content, frontMatterDelim)
	if first < 0 {
		return nil, "", fmt.Errorf("no opening ---")
	}

	rest := content[first+len(frontMatterDelim):]

	// Find closing ---
	second := strings.Index(rest, frontMatterDelim)
	if second < 0 {
		return nil, "", fmt.Errorf("no closing ---")
	}

	yamlPart := strings.TrimSpace(rest[:second])
	bodyPart := strings.TrimSpace(rest[second+len(frontMatterDelim):])

	var manifest SkillManifest
	if err := yaml.Unmarshal([]byte(yamlPart), &manifest); err != nil {
		return nil, "", fmt.Errorf("parse YAML front matter: %w", err)
	}

	return &manifest, bodyPart, nil
}
```

- [ ] **Step 2: Write loader_test.go**

```go
package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestSkill(t *testing.T, dir, content string) string {
	t.Helper()
	skillDir := filepath.Join(dir, "test-skill")
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
	if !contains(def.Instructions, "Help turn ideas into designs") {
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

func TestDiscoverSkills(t *testing.T) {
	dir := t.TempDir()

	// Create two valid skills
	writeTestSkill(t, dir, `---
name: skill-a
description: "First skill"
danger_level: safe
---
Instructions A
`)

	writeTestSkill(t, dir, `---
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

	// Valid
	writeTestSkill(t, dir, `---
name: valid
description: "Valid skill"
danger_level: safe
---
Instructions
`)

	// Invalid (missing name)
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
	skillDir := writeTestSkill(t, dir, `---
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Run tests**

Run: `cd /home/bugra/Belgeler/memo && go test ./internal/skill/ -v -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

---

### Task 3: Create `internal/skill/manager.go`

**Files:**
- Create: `internal/skill/manager.go`
- Create: `internal/skill/manager_test.go`

- [ ] **Step 1: Write manager.go**

```go
package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	skillsDirName = "skills"
)

type Manager struct {
	mu      sync.RWMutex
	baseDir string // data/ directory

	skills       map[string]*SkillDefinition
	activeSkills map[string]bool

	toolRegistrar ToolRegistrar
}

// ToolRegistrar allows skills to register their tools into the agent pipeline.
type ToolRegistrar interface {
	RegisterTool(name string, def any) error
	UnregisterTool(name string)
}

func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir:      filepath.Join(baseDir, skillsDirName),
		skills:       make(map[string]*SkillDefinition),
		activeSkills: make(map[string]bool),
	}
}

// Discover loads all skills from the skills directory.
func (m *Manager) Discover() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	skills, err := DiscoverSkills(m.baseDir)
	if err != nil {
		return fmt.Errorf("discover skills: %w", err)
	}

	m.skills = make(map[string]*SkillDefinition, len(skills))
	for _, s := range skills {
		m.skills[s.Manifest.Name] = s
	}

	// Remove active flags for skills that no longer exist
	for name := range m.activeSkills {
		if _, ok := m.skills[name]; !ok {
			delete(m.activeSkills, name)
		}
	}

	return nil
}

// List returns all installed skills.
func (m *Manager) List() []*SkillDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SkillDefinition, 0, len(m.skills))
	for _, s := range m.skills {
		result = append(result, s)
	}
	return result
}

// Get returns a skill by name.
func (m *Manager) Get(name string) (*SkillDefinition, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.skills[name]
	return s, ok
}

// Install copies a skill directory into the skills directory.
func (m *Manager) Install(sourcePath string) (*SkillDefinition, error) {
	// Load from source to validate
	def, err := LoadSkill(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("load skill from %q: %w", sourcePath, err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	targetDir := filepath.Join(m.baseDir, def.Manifest.Name)

	// Check if already exists
	if _, err := os.Stat(targetDir); err == nil {
		return nil, fmt.Errorf("skill %q already exists", def.Manifest.Name)
	}

	// Copy skill directory
	if err := copyDir(sourcePath, targetDir); err != nil {
		return nil, fmt.Errorf("copy skill: %w", err)
	}

	// Update in-memory state
	def.Path = targetDir
	m.skills[def.Manifest.Name] = def

	return def, nil
}

// Remove deletes a skill by name.
func (m *Manager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	def, ok := m.skills[name]
	if !ok {
		return fmt.Errorf("skill %q not found", name)
	}

	if err := os.RemoveAll(def.Path); err != nil {
		return fmt.Errorf("remove skill dir: %w", err)
	}

	delete(m.skills, name)
	delete(m.activeSkills, name)

	// Unregister tools if registrar is set
	if m.toolRegistrar != nil {
		for _, tool := range def.Manifest.Tools {
			m.toolRegistrar.UnregisterTool(skillToolName(name, tool.Name))
		}
	}

	return nil
}

// IsActive checks if a skill is currently active.
func (m *Manager) IsActive(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeSkills[name]
}

// SetActive sets which skills are active.
func (m *Manager) SetActive(names []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	newActive := make(map[string]bool)
	for _, name := range names {
		if _, ok := m.skills[name]; !ok {
			return fmt.Errorf("skill %q not found", name)
		}
		newActive[name] = true
	}

	// Unregister tools from previously active skills
	if m.toolRegistrar != nil {
		for name := range m.activeSkills {
			if !newActive[name] {
				if def, ok := m.skills[name]; ok {
					for _, tool := range def.Manifest.Tools {
						m.toolRegistrar.UnregisterTool(skillToolName(name, tool.Name))
					}
				}
			}
		}
	}

	m.activeSkills = newActive

	// Register tools for newly active skills
	if m.toolRegistrar != nil {
		for name := range newActive {
			if def, ok := m.skills[name]; ok {
				for _, tool := range def.Manifest.Tools {
					// TODO: Register tool with proper ToolDef type from agent package
					_ = tool
				}
			}
		}
	}

	return nil
}

// GetActiveNames returns the names of currently active skills.
func (m *Manager) GetActiveNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.activeSkills))
	for name := range m.activeSkills {
		names = append(names, name)
	}
	return names
}

// ActiveInstructions returns concatenated instructions from all active skills.
func (m *Manager) ActiveInstructions() []SkillActivation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var activations []SkillActivation
	for name := range m.activeSkills {
		if def, ok := m.skills[name]; ok {
			activations = append(activations, SkillActivation{
				Name:         name,
				Description:  def.Manifest.Description,
				Instructions: def.Instructions,
			})
		}
	}
	return activations
}

// SetToolRegistrar sets the tool registrar for registering/unregistering skill tools.
func (m *Manager) SetToolRegistrar(r ToolRegistrar) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toolRegistrar = r
}

// skillToolName creates a unique tool name for a skill's tool.
func skillToolName(skillName, toolName string) string {
	return fmt.Sprintf("skill_%s_%s", skillName, toolName)
}

// SkillActivation represents an active skill's instructions for injection.
type SkillActivation struct {
	Name         string
	Description  string
	Instructions string
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}
```

- [ ] **Step 2: Write manager_test.go**

```go
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

	// Create skills dir and add a skill
	skillsDir := filepath.Join(dir, skillsDirName)
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeTestSkill(t, skillsDir, `---
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
	writeTestSkill(t, skillsDir, `---
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
	writeTestSkill(t, skillsDir, `---
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

	// Deactivate
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
	writeTestSkill(t, skillsDir, `---
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

	// Create a skill outside the manager's scope
	sourceDir := filepath.Join(dir, "source-skill")
	writeTestSkill(t, sourceDir, `---
name: installed-skill
description: "Installed from path"
danger_level: safe
---
Installed instructions
`)

	def, err := m.Install(sourceDir)
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if def.Manifest.Name != "installed-skill" {
		t.Errorf("Name = %q", def.Manifest.Name)
	}

	// Verify it's in the manager
	skills := m.List()
	if len(skills) != 1 {
		t.Fatalf("len(List()) = %d after install", len(skills))
	}

	// Verify directory was copied
	skillDir := filepath.Join(dir, skillsDirName, "installed-skill")
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not found at target: %v", err)
	}

	// Duplicate install should fail
	_, err = m.Install(sourceDir)
	if err == nil {
		t.Fatal("duplicate install should error")
	}
}

func TestManagerRemove(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	skillsDir := filepath.Join(dir, skillsDirName)
	os.MkdirAll(skillsDir, 0755)
	writeTestSkill(t, skillsDir, `---
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

	// Remove non-existent
	if err := m.Remove("nonexistent"); err == nil {
		t.Fatal("expected error for non-existent skill")
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

	// Create nested structure
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
```

- [ ] **Step 3: Run tests**

Run: `cd /home/bugra/Belgeler/memo && go test ./internal/skill/ -v -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

---

### Task 4: Integrate into App — `app_skill.go`

**Files:**
- Create: `app_skill.go`
- Modify: `app.go` (add skill manager field + init)
- Modify: `internal/webserver/bridge.go` (add skill methods to FullBridge)

- [ ] **Step 1: Add skill manager field to App struct in app.go**

Find the `type App struct` declaration and add:
```go
skillManager *skill.Manager
```

Find startup() or equivalent init and add:
```go
a.skillManager = skill.NewManager("data")
if err := a.skillManager.Discover(); err != nil {
    log.Printf("skill: discover error: %v", err)
}
```

- [ ] **Step 2: Create app_skill.go**

```go
package main

import (
	"github.com/bugraakdemir/memo/internal/skill"
)

func (a *App) ListSkills() []skill.SkillDefinition {
	if a.skillManager == nil {
		return nil
	}
	defs := a.skillManager.List()
	result := make([]skill.SkillDefinition, len(defs))
	for i, d := range defs {
		result[i] = *d
	}
	return result
}

func (a *App) InstallSkill(path string) (*skill.SkillDefinition, error) {
	return a.skillManager.Install(path)
}

func (a *App) RemoveSkill(name string) error {
	return a.skillManager.Remove(name)
}

func (a *App) GetSkill(name string) (*skill.SkillDefinition, error) {
	def, ok := a.skillManager.Get(name)
	if !ok {
		return nil, fmt.Errorf("skill %q not found", name)
	}
	return def, nil
}

func (a *App) SetActiveSkills(names []string) error {
	return a.skillManager.SetActive(names)
}

func (a *App) GetActiveSkills() []string {
	if a.skillManager == nil {
		return nil
	}
	return a.skillManager.GetActiveNames()
}

// activeSkillInstructions returns the formatted instructions block for active skills.
func (a *App) activeSkillInstructions() string {
	if a.skillManager == nil {
		return ""
	}
	activations := a.skillManager.ActiveInstructions()
	if len(activations) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n## Active Skills\n\n")
	b.WriteString("The following skills are active. Follow their instructions carefully:\n\n")
	for _, act := range activations {
		b.WriteString(fmt.Sprintf("### Skill: %s\n", act.Name))
		b.WriteString(fmt.Sprintf("_%s_\n\n", act.Description))
		b.WriteString(act.Instructions)
		b.WriteString("\n\n---\n\n")
	}
	return b.String()
}
```

- [ ] **Step 3: Add skill methods to FullBridge interface in bridge.go**

Add these methods to the `FullBridge` interface in `internal/webserver/bridge.go`:
```go
// Skills
ListSkills() []skill.SkillDefinition
InstallSkill(path string) error
RemoveSkill(name string) error
GetSkill(name string) (*skill.SkillDefinition, error)
SetActiveSkills(names []string) error
GetActiveSkills() []string
```

- [ ] **Step 4: Add imports and full bridge method in app_skill.go**

Add the FullBridge interface implementation methods.

- [ ] **Step 5: Inject skill instructions in callAgentStream and callLLMStream**

In `callAgentStream` and `callLLMStream`, before building messages, append active skill instructions:
```go
if skillInstr := a.activeSkillInstructions(); skillInstr != "" {
    // Append to system prompt
}
```

- [ ] **Step 6: Build and test**

Run: `go build ./...` — verify it compiles
Run: `go vet ./...` — verify no issues

- [ ] **Step 7: Commit**

---

### Task 5: Add API endpoints

**Files:**
- Modify: `internal/webserver/handlers_flutter.go`
- Modify: `internal/webserver/server.go`

- [ ] **Step 1: Add handler functions in handlers_flutter.go**

```go
func (h *FlutterHandlers) handleListSkills(w http.ResponseWriter, r *http.Request) {
    skills := h.bridge.ListSkills()
    writeJSON(w, http.StatusOK, skills)
}

func (h *FlutterHandlers) handleInstallSkill(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Path string `json:"path"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
        return
    }
    def, err := h.bridge.InstallSkill(req.Path)
    if err != nil {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
        return
    }
    writeJSON(w, http.StatusOK, def)
}

// ... etc for RemoveSkill, GetSkill, SetActiveSkills, GetActiveSkills
```

- [ ] **Step 2: Register routes in server.go**

```go
mux.HandleFunc("GET /api/skills", h.handleListSkills)
mux.HandleFunc("POST /api/skills/install", h.handleInstallSkill)
mux.HandleFunc("DELETE /api/skills/{name}", h.handleRemoveSkill)
mux.HandleFunc("GET /api/skills/{name}", h.handleGetSkill)
mux.HandleFunc("PUT /api/skills/active", h.handleSetActiveSkills)
mux.HandleFunc("GET /api/skills/active", h.handleGetActiveSkills)
```

- [ ] **Step 3: Add /skill command parsing in message handler**

In the send/stream handler, check for /skill prefix:
```go
if strings.HasPrefix(userMsg, "/skill") {
    // Parse command variant and delegate to skill manager
}
```

- [ ] **Step 4: Build and test**

Run: `go build ./...` — verify it compiles

- [ ] **Step 5: Commit**

---

### Task 6: Verify end-to-end

- [ ] **Step 1: Run all tests**

Run: `cd /home/bugra/Belgeler/memo && go test ./internal/skill/... ./... -count=1`
Expected: PASS

- [ ] **Step 2: Run vet**

Run: `cd /home/bugra/Belgeler/memo && go vet ./...`
Expected: no errors

- [ ] **Step 3: Run with race detector**

Run: `cd /home/bugra/Belgeler/memo && go test -race ./internal/skill/... -count=1`
Expected: PASS (no race conditions)
