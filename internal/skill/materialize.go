package skill

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"memo/internal/fileutil"
)

// materializedSignatureFile is a small sidecar Memo writes into a skill
// directory it materialized itself, recording a content signature of the
// embedded tree at the time it was written. Its mere presence, with a value
// that still matches what's on disk right now, is proof this directory is
// exactly what Memo wrote and the user hasn't touched it since — the only
// case in which a later materialize call is allowed to replace it.
const materializedSignatureFile = ".materialized-signature"

// MaterializeEmbedded copies an embedded skill tree rooted at name into the
// manager's on-disk skills directory. On first run this simply copies the
// tree in, so it lives on disk like any user skill: it is discovered
// normally, can be inspected, and — if the user deletes it — reappears on
// the next start. src must contain name/SKILL.md (typically an embed.FS
// declared with //go:embed <name>).
//
// On later runs, if the directory is already there, it is left untouched
// unless BOTH of the following hold: (1) Memo itself materialized it and
// nothing has modified it since (proven by re-hashing the on-disk tree and
// comparing to the signature recorded at write time), and (2) the embedded
// source's content has changed since then — in which case it is refreshed
// in place, so a fix shipped to a built-in skill actually reaches installs
// that never touched it. Any directory without a matching signature —
// because it predates this check, or because it's genuinely user-provided,
// or because the user hand-edited a file inside it — is left alone, exactly
// as before.
//
// Returns true if files were written, false if the skill was already there
// and up to date (or left alone).
func MaterializeEmbedded(mgr *Manager, name string, src fs.FS) (bool, error) {
	if err := validateSkillName(name); err != nil {
		return false, err
	}

	target := filepath.Join(mgr.SkillsDir(), name)
	embeddedSig, err := hashTree(src, name, nil)
	if err != nil {
		return false, fmt.Errorf("hash embedded skill %q: %w", name, err)
	}

	switch _, statErr := os.Stat(filepath.Join(target, "SKILL.md")); {
	case statErr == nil:
		recorded, readErr := os.ReadFile(filepath.Join(target, materializedSignatureFile))
		if readErr != nil {
			return false, nil // no record we wrote this — unknown provenance, leave it alone
		}
		onDiskSig, hashErr := hashTree(os.DirFS(target), ".", func(rel string) bool {
			return rel == materializedSignatureFile
		})
		if hashErr != nil || onDiskSig != string(recorded) {
			return false, nil // modified since we last wrote it — leave it alone
		}
		if onDiskSig == embeddedSig {
			return false, nil // already up to date
		}
		// Untouched since we wrote it, and the embedded content has since
		// changed: safe to refresh below.
	case !os.IsNotExist(statErr):
		return false, fmt.Errorf("stat %s: %w", target, statErr)
	}

	if err := os.RemoveAll(target); err != nil {
		return false, fmt.Errorf("clear stale materialized skill %q: %w", name, err)
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
		return fileutil.AtomicWrite(dst, data, 0644)
	})
	if walkErr != nil {
		_ = os.RemoveAll(target) // don't leave a half-written skill dir behind
		return false, fmt.Errorf("materialize embedded skill %q: %w", name, walkErr)
	}

	if err := fileutil.AtomicWrite(filepath.Join(target, materializedSignatureFile), []byte(embeddedSig), 0644); err != nil {
		return false, fmt.Errorf("record materialization signature for %q: %w", name, err)
	}

	return true, nil
}

// hashTree computes a single content signature over every regular file
// under root in fsys (relative path plus a hash of its bytes, sorted for
// reproducibility), skipping any relative path for which skip returns true.
// Used both on the embedded source tree and on a real on-disk directory
// (via os.DirFS) so the two are directly comparable.
func hashTree(fsys fs.FS, root string, skip func(rel string) bool) (string, error) {
	var entries []string
	walkErr := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if skip != nil && skip(rel) {
			return nil
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		entries = append(entries, rel+"\x00"+hex.EncodeToString(sum[:]))
		return nil
	})
	if walkErr != nil {
		return "", walkErr
	}
	sort.Strings(entries)
	h := sha256.New()
	for _, e := range entries {
		h.Write([]byte(e))
		h.Write([]byte("\n"))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
