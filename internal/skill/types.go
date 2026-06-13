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
