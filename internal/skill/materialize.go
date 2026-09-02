package skill

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// MaterializeEmbedded copies an embedded skill tree rooted at name into the
// manager's on-disk skills directory, but only when that skill is not already
// present. This lets a built-in skill ship inside the binary yet live on disk
// like any user skill: it is discovered normally, can be inspected, and — if
// the user deletes it — reappears on the next start. src must contain
// name/SKILL.md (typically an embed.FS declared with //go:embed <name>).
//
// Returns true if files were written, false if the skill was already there.
func MaterializeEmbedded(mgr *Manager, name string, src fs.FS) (bool, error) {
	if err := validateSkillName(name); err != nil {
		return false, err
	}

	target := filepath.Join(mgr.SkillsDir(), name)
	switch _, err := os.Stat(filepath.Join(target, "SKILL.md")); {
	case err == nil:
		return false, nil // already materialized (or user-provided) — leave it alone
	case !os.IsNotExist(err):
		return false, fmt.Errorf("stat %s: %w", target, err)
	}

	walkErr := fs.WalkDir(src, name, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(name, p)
		if err != nil {
			return err
		}
		dst := filepath.Join(target, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0755)
		}
		data, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0644)
	})
	if walkErr != nil {
		_ = os.RemoveAll(target) // don't leave a half-written skill dir behind
		return false, fmt.Errorf("materialize embedded skill %q: %w", name, walkErr)
	}
	return true, nil
}
