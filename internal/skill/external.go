package skill

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const importRegistryFileName = "skills_imported.json"

// ExternalSource describes a third-party tool whose on-disk skill directories
// Memo knows how to read. Each entry under a Dir is expected to be a
// subdirectory containing its own SKILL.md, same layout Memo's own
// DiscoverSkills already scans.
type ExternalSource struct {
	ID   string
	Name string
	Dirs []string
}

// KnownExternalSources returns the external tools Memo currently knows how
// to import skills from. Claude Code's skill directory
// (~/.claude/skills/<name>/SKILL.md) uses the same YAML-frontmatter-plus-body
// shape as Memo's own manifest (ParseSkill only requires a `name` field and a
// non-empty body), so no format translation is needed.
//
// Other tools (OpenCode, Codex, ...) were deliberately left out: their
// on-disk skill layout wasn't confirmed in this environment, and guessing a
// path would silently do nothing (best case) or import the wrong files
// (worst case). Add them here once verified.
func KnownExternalSources() []ExternalSource {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []ExternalSource{
		{
			ID:   "claude-code",
			Name: "Claude Code",
			Dirs: []string{filepath.Join(home, ".claude", "skills")},
		},
	}
}

// importRecord tracks a single skill Memo previously copied in from an
// external source, so a later sync can tell "unchanged", "source content
// changed, re-copy" and "this name belongs to a skill we never imported,
// leave it alone" apart.
type importRecord struct {
	Name       string    `json:"name"`
	SourceID   string    `json:"source_id"`
	SourcePath string    `json:"source_path"`
	Signature  string    `json:"signature"`
	ImportedAt time.Time `json:"imported_at"`
}

// SyncResult reports what a SyncExternalSkills call actually did.
type SyncResult struct {
	Imported []string // brand new, copied in for the first time
	Updated  []string // previously imported, source content changed, re-copied
	Skipped  []string // name collides with a skill Memo did not import; left untouched
}

// ImportRegistryPath is where the import bookkeeping (which skills came from
// which external source, and their last-seen content signature) is
// persisted, so a later restart's sync doesn't blindly redo every copy.
func (m *Manager) ImportRegistryPath() string {
	return filepath.Join(m.baseDir, importRegistryFileName)
}

// SyncExternalSkills scans every source's directories for skills, installs
// any that are new, re-installs any whose source content changed since the
// last sync, and auto-activates everything it has ever imported. A name
// that already exists in the manager but was never imported by this
// function (a hand-authored or manually `/skill install`ed skill) is left
// alone rather than overwritten.
//
// Skills are never auto-removed just because their source directory
// disappeared (unmounted drive, renamed folder) — that's a deliberate,
// conservative default; the import registry keeps no "seen this run"
// eviction logic.
func SyncExternalSkills(m *Manager, sources []ExternalSource) (SyncResult, error) {
	var result SyncResult

	registryPath := m.ImportRegistryPath()
	registry, err := loadImportRegistry(registryPath)
	if err != nil {
		return result, fmt.Errorf("load import registry: %w", err)
	}

	for _, src := range sources {
		for _, root := range src.Dirs {
			entries, err := os.ReadDir(root)
			if err != nil {
				continue // missing/unreadable source dir is not an error — tool likely isn't installed
			}

			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				skillDir := filepath.Join(root, entry.Name())
				if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
					continue
				}

				def, err := LoadSkill(skillDir)
				if err != nil {
					fmt.Fprintf(os.Stderr, "skill: skip external %q: %v\n", skillDir, err)
					continue
				}
				name := def.Manifest.Name

				sig, err := dirSignature(skillDir)
				if err != nil {
					fmt.Fprintf(os.Stderr, "skill: signature %q: %v\n", skillDir, err)
					continue
				}

				rec, tracked := registry[name]
				_, installed := m.Get(name)

				switch {
				case tracked && installed && rec.Signature == sig:
					continue // unchanged since last sync

				case tracked && installed:
					if err := m.Remove(name); err != nil {
						fmt.Fprintf(os.Stderr, "skill: reimport %q: %v\n", name, err)
						continue
					}
					if _, err := m.Install(skillDir); err != nil {
						fmt.Fprintf(os.Stderr, "skill: reimport %q: %v\n", name, err)
						continue
					}
					result.Updated = append(result.Updated, name)

				case !installed:
					if _, err := m.Install(skillDir); err != nil {
						fmt.Fprintf(os.Stderr, "skill: import %q: %v\n", name, err)
						continue
					}
					result.Imported = append(result.Imported, name)

				default:
					// Name collides with a skill this function never imported.
					result.Skipped = append(result.Skipped, name)
					continue
				}

				registry[name] = importRecord{
					Name:       name,
					SourceID:   src.ID,
					SourcePath: skillDir,
					Signature:  sig,
					ImportedAt: time.Now(),
				}
			}
		}
	}

	if len(result.Imported) > 0 || len(result.Updated) > 0 {
		active := m.GetActiveNames()
		activeSet := make(map[string]bool, len(active))
		for _, n := range active {
			activeSet[n] = true
		}
		for name := range registry {
			if _, ok := m.Get(name); ok && !activeSet[name] {
				active = append(active, name)
				activeSet[name] = true
			}
		}
		if err := m.SetActive(active); err != nil {
			return result, fmt.Errorf("activate imported skills: %w", err)
		}
	}

	if err := saveImportRegistry(registryPath, registry); err != nil {
		return result, fmt.Errorf("save import registry: %w", err)
	}

	return result, nil
}

// dirSignature is a cheap change-detector for a skill directory: it hashes
// each file's relative path, size and mtime rather than its content, so a
// sync doesn't need to read every byte of every skill just to tell whether
// anything changed.
func dirSignature(dir string) (string, error) {
	var lines []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		lines = append(lines, fmt.Sprintf("%s|%d|%d", rel, info.Size(), info.ModTime().UnixNano()))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

func loadImportRegistry(path string) (map[string]importRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]importRecord), nil
		}
		return nil, err
	}
	registry := make(map[string]importRecord)
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, err
	}
	return registry, nil
}

func saveImportRegistry(path string, registry map[string]importRecord) error {
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
