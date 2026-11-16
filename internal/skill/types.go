package skill

import (
	"time"

	"gopkg.in/yaml.v3"
)

type DangerLevel string

const (
	DangerLevelSafe      DangerLevel = "safe"
	DangerLevelMedium    DangerLevel = "medium"
	DangerLevelDangerous DangerLevel = "dangerous"
)

type SkillTool struct {
	Name        string      `yaml:"name" json:"name"`
	Description string      `yaml:"description" json:"description"`
	Parameters  any         `yaml:"parameters" json:"parameters"`
	DangerLevel DangerLevel `yaml:"danger_level" json:"danger_level"`
	// Command is the shell command run to execute this tool, resolved
	// relative to the skill's own install directory (so a manifest can
	// reference its own bundled scripts, e.g. "python3 scripts/format.py").
	// The LLM's call arguments are delivered on the process's stdin as raw
	// JSON, never interpolated into the command string. A tool with no
	// Command is registered for documentation purposes only (it appears in
	// the skill's manifest) but is never wired up as a callable agent tool.
	Command string `yaml:"command" json:"command,omitempty"`
}

// UnmarshalYAML accepts both a full SkillTool mapping (`name: ...`, `command: ...`)
// and a plain scalar string (`- browser` or `tools: [browser, bash]`, as used in
// Claude Code / Agent Skills frontmatter for documentation-only tool allowlists).
func (t *SkillTool) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		var name string
		if err := value.Decode(&name); err != nil {
			return err
		}
		*t = SkillTool{Name: name}
		return nil
	}
	type rawSkillTool SkillTool
	var raw rawSkillTool
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*t = SkillTool(raw)
	return nil
}

type SkillManifest struct {
	Name         string         `yaml:"name" json:"name"`
	Description  string         `yaml:"description" json:"description"`
	Version      string         `yaml:"version" json:"version,omitempty"`
	Author       string         `yaml:"author" json:"author,omitempty"`
	DangerLevel  DangerLevel    `yaml:"danger_level" json:"danger_level"`
	License      string         `yaml:"license" json:"license,omitempty"`
	Metadata     map[string]any `yaml:"metadata" json:"metadata,omitempty"`
	Tools        []SkillTool    `yaml:"tools" json:"tools,omitempty"`
	Instructions string         `yaml:"instructions" json:"instructions,omitempty"`
}

type SkillDefinition struct {
	Manifest     SkillManifest
	Instructions string
	Path         string
	LoadedAt     time.Time
}

// SkillToolRegistration is the value passed to ToolRegistrar.RegisterTool.
// It carries the owning skill's name alongside the tool spec because a
// registrar's ExecuteFn needs both to call back into Manager.ExecuteTool —
// skillToolName()'s composed "skill_<skill>_<tool>" string isn't reliably
// splittable back into its two parts when either name contains an underscore.
type SkillToolRegistration struct {
	SkillName string
	Tool      SkillTool
}
