package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const skillsDirName = "skills"

type ToolRegistrar interface {
	RegisterTool(name string, toolDef any) error
	UnregisterTool(name string)
}

type Manager struct {
	mu      sync.RWMutex
	baseDir string

	skills       map[string]*SkillDefinition
	activeSkills map[string]bool

	toolRegistrar ToolRegistrar
}

func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir:      baseDir,
		skills:       make(map[string]*SkillDefinition),
		activeSkills: make(map[string]bool),
	}
}

func (m *Manager) SkillsDir() string {
	return filepath.Join(m.baseDir, skillsDirName)
}

func (m *Manager) Discover() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	skills, err := DiscoverSkills(m.SkillsDir())
	if err != nil {
		return fmt.Errorf("discover skills: %w", err)
	}

	m.skills = make(map[string]*SkillDefinition, len(skills))
	for _, s := range skills {
		m.skills[s.Manifest.Name] = s
	}

	for name := range m.activeSkills {
		if _, ok := m.skills[name]; !ok {
			delete(m.activeSkills, name)
		}
	}

	return nil
}

func (m *Manager) List() []*SkillDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SkillDefinition, 0, len(m.skills))
	for _, s := range m.skills {
		result = append(result, s)
	}
	return result
}

func (m *Manager) Get(name string) (*SkillDefinition, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.skills[name]
	return s, ok
}

func (m *Manager) Install(sourcePath string) (*SkillDefinition, error) {
	def, err := LoadSkill(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("load skill from %q: %w", sourcePath, err)
	}

	// Reject names that could escape the skills directory via path traversal.
	if err := validateSkillName(def.Manifest.Name); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	targetDir := filepath.Join(m.SkillsDir(), def.Manifest.Name)

	// Confirm the resolved path is still inside SkillsDir.
	skillsDir, err := filepath.Abs(m.SkillsDir())
	if err != nil {
		return nil, fmt.Errorf("resolve skills dir: %w", err)
	}
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, fmt.Errorf("resolve target dir: %w", err)
	}
	if !strings.HasPrefix(absTarget+string(filepath.Separator), skillsDir+string(filepath.Separator)) {
		return nil, fmt.Errorf("skill name %q resolves outside skills directory", def.Manifest.Name)
	}

	switch _, statErr := os.Stat(targetDir); {
	case statErr == nil:
		return nil, fmt.Errorf("skill %q already exists", def.Manifest.Name)
	case !os.IsNotExist(statErr):
		return nil, fmt.Errorf("stat target dir: %w", statErr)
	}

	// Roll back on partial failure.
	if err := copyDir(sourcePath, targetDir); err != nil {
		_ = os.RemoveAll(targetDir)
		return nil, fmt.Errorf("copy skill: %w", err)
	}

	def.Path = targetDir
	m.skills[def.Manifest.Name] = def

	return def, nil
}

// validateSkillName rejects names that contain path-traversal sequences.
func validateSkillName(name string) error {
	if name == "" {
		return fmt.Errorf("skill name must not be empty")
	}
	// Disallow any path separator or relative traversal component.
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return fmt.Errorf("skill name %q contains invalid characters", name)
	}
	return nil
}

func (m *Manager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	def, ok := m.skills[name]
	if !ok {
		return fmt.Errorf("skill %q not found", name)
	}

	// Confirm the stored path is still inside SkillsDir before removing.
	skillsDir, err := filepath.Abs(m.SkillsDir())
	if err != nil {
		return fmt.Errorf("resolve skills dir: %w", err)
	}
	absPath, err := filepath.Abs(def.Path)
	if err != nil {
		return fmt.Errorf("resolve skill path: %w", err)
	}
	if !strings.HasPrefix(absPath+string(filepath.Separator), skillsDir+string(filepath.Separator)) {
		return fmt.Errorf("skill path %q is outside skills directory, refusing to remove", def.Path)
	}

	if err := os.RemoveAll(def.Path); err != nil {
		return fmt.Errorf("remove skill dir: %w", err)
	}

	delete(m.skills, name)
	delete(m.activeSkills, name)

	if m.toolRegistrar != nil {
		for _, tool := range def.Manifest.Tools {
			m.toolRegistrar.UnregisterTool(skillToolName(name, tool.Name))
		}
	}

	return nil
}

func (m *Manager) IsActive(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeSkills[name]
}

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

		for name := range newActive {
			if !m.activeSkills[name] {
				if def, ok := m.skills[name]; ok {
					for _, tool := range def.Manifest.Tools {
						m.toolRegistrar.RegisterTool(skillToolName(name, tool.Name), SkillToolRegistration{SkillName: name, Tool: tool})
					}
				}
			}
		}
	}

	m.activeSkills = newActive

	return nil
}

func (m *Manager) GetActiveNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.activeSkills))
	for name := range m.activeSkills {
		names = append(names, name)
	}
	return names
}

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

func (m *Manager) SetToolRegistrar(r ToolRegistrar) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toolRegistrar = r
}

func skillToolName(skillName, toolName string) string {
	return fmt.Sprintf("skill_%s_%s", skillName, toolName)
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	const maxFileSize = 10 * 1024 * 1024 // 10 MB per file

	for _, entry := range entries {
		// Skip symlinks — following them could exfiltrate sensitive files or cause DoS.
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.Size() > maxFileSize {
				return fmt.Errorf("file %q exceeds size limit (%d bytes)", entry.Name(), info.Size())
			}
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
