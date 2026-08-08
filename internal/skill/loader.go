package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const frontMatterDelim = "---"

// topLevelScalarKey matches an unindented `key: value` line for one of the
// manifest's plain string fields — never `tools:`/`metadata:` block starters
// or their nested `- name:`/`description:` entries, which are always
// indented and so never match this anchored-at-column-0 pattern.
var topLevelScalarKey = regexp.MustCompile(`^(name|description|version|author|license|instructions):\s+(.*\S)\s*$`)

// sanitizeFrontMatterYAML quotes plain-scalar values that contain a bare
// "key: value"-shaped colon, which yaml.v3 otherwise rejects as an
// unexpected nested mapping. Claude Code's own SKILL.md files routinely
// write descriptions like `description: Use X. Triggers on: doing Y` —
// valid enough for whatever lightweight extraction Claude Code itself uses,
// but not valid YAML, so real-world skills from that ecosystem would
// otherwise fail to import here.
func sanitizeFrontMatterYAML(yamlPart string) string {
	lines := strings.Split(yamlPart, "\n")
	for i, line := range lines {
		m := topLevelScalarKey.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key, value := m[1], m[2]
		if strings.ContainsAny(value[:1], `"'|>{[`) {
			continue // already quoted or a flow/block scalar — leave as-is
		}
		if !strings.Contains(value, ": ") && !strings.HasSuffix(value, ":") {
			continue // no embedded colon, nothing to fix
		}
		escaped := strings.ReplaceAll(value, `"`, `\"`)
		lines[i] = key + `: "` + escaped + `"`
	}
	return strings.Join(lines, "\n")
}

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
	if err := yaml.Unmarshal([]byte(sanitizeFrontMatterYAML(yamlPart)), &manifest); err != nil {
		return nil, "", fmt.Errorf("parse YAML front matter: %w", err)
	}

	return &manifest, bodyPart, nil
}
