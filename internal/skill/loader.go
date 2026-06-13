package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const frontMatterDelim = "---"

func LoadSkill(dir string) (*SkillDefinition, error) {
	path := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md: %w", err)
	}

	return ParseSkill(data, dir)
}

func ParseSkill(data []byte, baseDir string) (*SkillDefinition, error) {
	content := string(data)

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
			fmt.Fprintf(os.Stderr, "skill: skip %q: %v\n", entry.Name(), err)
			continue
		}
		skills = append(skills, def)
	}
	return skills, nil
}

func hasFrontMatter(content string) bool {
	trimmed := strings.TrimLeft(content, "\n\r\t ")
	return strings.HasPrefix(trimmed, frontMatterDelim)
}

func extractFrontMatter(content string) (*SkillManifest, string, error) {
	first := strings.Index(content, frontMatterDelim)
	if first < 0 {
		return nil, "", fmt.Errorf("no opening ---")
	}

	rest := content[first+len(frontMatterDelim):]

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
