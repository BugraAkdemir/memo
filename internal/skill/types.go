package skill

import (
	"time"
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

type SkillActivation struct {
	Name         string
	Description  string
	Instructions string
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
