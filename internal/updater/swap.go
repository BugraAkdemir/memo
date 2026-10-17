// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SwapItems lists what an update replaces inside a script-installed Memo
// (Linux/macOS: ~/.memo). It mirrors what scripts/get-memo.sh has always
// replaced on an update, and — just as important — what it has always left
// alone: config/, .env, the rest of data/ (models, memory, sessions, providers)
// are never listed here, so no update can touch the user's things.
//
// Paths are relative to the install directory, slash-separated.
var SwapItems = []string{
	"binaries",
	"memo-backend",
	"memo_flutter",
	"lib",
	"run_memo.sh",
	"run_memo.command",
	"Memo.app",
	"memo-updater",
	"icon.png",
	"data/icudtl.dat",
	"data/flutter_assets",
}

// cliPath is the CLI binary (a copy of memo-backend), which get-memo.sh keeps
// in bin/.
const cliPath = "bin/memo"

// renameFn is os.Rename; tests replace it to inject a failure mid-swap.
var renameFn = os.Rename

// Swap is a completed in-place replacement that can still be undone.
type Swap struct {
	install, backup string
	ops             []swapOp
}

type swapOp struct {
	item   string // relative path
	hadOld bool   // the old one was moved into backup
	placed bool   // the new one was moved into install
	copied bool   // item was created by copying (bin/memo)
}

// PlaceNew replaces the install's items with those under staged, moving the
// old ones into backup. Everything is a rename on one filesystem (staged and
// backup must live inside the install directory's filesystem), so it is fast
// even for a 700 MB binaries/ tree and — if anything fails halfway — fully
// reversible: on error the already-moved items are put back before returning.
func PlaceNew(install, staged, backup string) (*Swap, error) {
	if install == "" || staged == "" || backup == "" {
		return nil, errors.New("updater: swap needs install, staged and backup directories")
	}
	if _, err := os.Stat(filepath.Join(staged, "memo-backend")); err != nil {
		return nil, fmt.Errorf("updater: the new version has no memo-backend: %w", err)
	}
	if err := os.MkdirAll(backup, 0o700); err != nil {
		return nil, err
	}
	sw := &Swap{install: install, backup: backup}

	for _, item := range SwapItems {
		src := filepath.Join(staged, filepath.FromSlash(item))
		if _, err := os.Lstat(src); err != nil {
			continue // this release doesn't ship it (e.g. run_memo.command on Linux)
		}
		dst := filepath.Join(install, filepath.FromSlash(item))
		op := swapOp{item: item}
		if _, err := os.Lstat(dst); err == nil {
			bak := filepath.Join(backup, filepath.FromSlash(item))
			if err := os.MkdirAll(filepath.Dir(bak), 0o700); err != nil {
				sw.rollbackQuiet()
				return nil, err
			}
			if err := renameFn(dst, bak); err != nil {
				sw.rollbackQuiet()
				return nil, fmt.Errorf("updater: moving %s aside: %w", item, err)
			}
			op.hadOld = true
		}
		sw.ops = append(sw.ops, op)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			sw.rollbackQuiet()
			return nil, err
		}
		if err := renameFn(src, dst); err != nil {
			sw.rollbackQuiet()
			return nil, fmt.Errorf("updater: installing %s: %w", item, err)
		}
		sw.ops[len(sw.ops)-1].placed = true
	}

	if err := sw.placeCLI(); err != nil {
		sw.rollbackQuiet()
		return nil, err
	}
	if err := fixPermissions(install); err != nil {
		sw.rollbackQuiet()
		return nil, err
	}
	return sw, nil
}

// placeCLI refreshes bin/memo, the terminal command, from the new memo-backend.
func (s *Swap) placeCLI() error {
	srcBackend := filepath.Join(s.install, "memo-backend")
	dst := filepath.Join(s.install, filepath.FromSlash(cliPath))
	op := swapOp{item: cliPath}
	if _, err := os.Lstat(dst); err == nil {
		bak := filepath.Join(s.backup, filepath.FromSlash(cliPath))
		if err := os.MkdirAll(filepath.Dir(bak), 0o700); err != nil {
			return err
		}
		if err := renameFn(dst, bak); err != nil {
			return fmt.Errorf("updater: moving %s aside: %w", cliPath, err)
		}
		op.hadOld = true
	}
	s.ops = append(s.ops, op)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".new"
	if err := copyFile(srcBackend, tmp, 0o755); err != nil {
		return err
	}
	if err := renameFn(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("updater: installing %s: %w", cliPath, err)
	}
	s.ops[len(s.ops)-1].placed = true
	s.ops[len(s.ops)-1].copied = true
	return nil
}

// Rollback puts the previous version back exactly as it was.
func (s *Swap) Rollback() error {
	var first error
	for i := len(s.ops) - 1; i >= 0; i-- {
		op := s.ops[i]
		dst := filepath.Join(s.install, filepath.FromSlash(op.item))
		if op.placed {
			if err := os.RemoveAll(dst); err != nil && first == nil {
				first = err
			}
		}
		if op.hadOld {
			bak := filepath.Join(s.backup, filepath.FromSlash(op.item))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil && first == nil {
				first = err
			}
			// Use the real os.Rename here, not the test hook: a rollback that
			// shares the injected failure could never be shown to work.
			if err := os.Rename(bak, dst); err != nil && first == nil {
				first = err
			}
		}
	}
	s.ops = nil
	return first
}

func (s *Swap) rollbackQuiet() { _ = s.Rollback() }

// Commit discards the backup once the new version has proven itself.
func (s *Swap) Commit() error { return os.RemoveAll(s.backup) }

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// fixPermissions restores the executable bits an archive may not have kept —
// the same chmods scripts/get-memo.sh applies (a 0644 llama-server once made
// every model start fail with EACCES).
func fixPermissions(install string) error {
	for _, rel := range []string{"memo-backend", "memo_flutter", "memo-updater", "run_memo.sh", "run_memo.command", cliPath} {
		p := filepath.Join(install, filepath.FromSlash(rel))
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			if err := os.Chmod(p, st.Mode().Perm()|0o755); err != nil {
				return err
			}
		}
	}
	bin := filepath.Join(install, "binaries")
	return filepath.WalkDir(bin, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, "llama-server") || strings.Contains(name, ".so") {
			st, err := os.Stat(path)
			if err != nil {
				return err
			}
			return os.Chmod(path, st.Mode().Perm()|0o755)
		}
		return nil
	})
}
