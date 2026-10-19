// SPDX-License-Identifier: AGPL-3.0-or-later

package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrForbiddenRoot is wrapped by every RootError.
var ErrForbiddenRoot = errors.New("project: folder cannot be a Memo project")

// RootError says why a folder was refused as a project root.
type RootError struct {
	Path   string
	Reason string
}

func (e *RootError) Error() string {
	return fmt.Sprintf("%s cannot be a Memo project: %s", e.Path, e.Reason)
}

func (e *RootError) Unwrap() error { return ErrForbiddenRoot }

// Guard decides which folders may be project roots.
//
// The reason it exists: Memo's own install lives in `~/.memo` (bin/, binaries/,
// data/, config/ …) and the updater/uninstaller manage that directory. A
// project folder is also called `.memo`, so a project rooted at the user's home
// would put the project files into the install directory — and the next
// update or uninstall would delete them (or the project's removal would delete
// the install).
type Guard struct {
	// Home is the user's home directory. It and every parent of it are refused.
	Home string
	// Protected are Memo's own directories (install home, data dir, config
	// dir). A project may not live inside any of them. A project may *contain*
	// one (Memo's own source checkout keeps ./data) — only the home chain and
	// install look-alikes are refused on that side.
	Protected []string
}

// systemDirs are never projects. Matched as the directory itself or anything
// inside it (lower-cased on case-insensitive platforms).
func systemDirs() []string {
	switch runtime.GOOS {
	case "windows":
		var out []string
		for _, k := range []string{"SystemRoot", "ProgramFiles", "ProgramFiles(x86)", "ProgramData"} {
			if v := os.Getenv(k); v != "" {
				out = append(out, v)
			}
		}
		return out
	case "darwin":
		return []string{"/System", "/usr", "/bin", "/sbin", "/etc", "/Library", "/private/etc", "/dev"}
	default:
		return []string{"/etc", "/usr", "/bin", "/sbin", "/lib", "/lib32", "/lib64", "/proc", "/sys", "/dev", "/boot", "/run"}
	}
}

// installMarkers are entries whose presence in `<dir>/.memo` means that `.memo`
// is a Memo installation, not a project.
var installMarkers = []string{"bin", "binaries", "data", "config", "memo_flutter"}

func foldCase() bool { return runtime.GOOS == "windows" || runtime.GOOS == "darwin" }

// within reports whether child is parent itself or lies inside it.
func within(parent, child string) bool {
	if foldCase() {
		parent, child = strings.ToLower(parent), strings.ToLower(child)
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// canonical returns an absolute, symlink-resolved, cleaned path. A path that
// does not exist keeps its absolute form (used for Protected entries that may
// not have been created yet).
func canonical(p string) string {
	if p == "" {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = filepath.Clean(p)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real
	}
	return abs
}

// Check validates dir as a project root and returns its canonical path. dir
// must exist and be a directory.
func (g Guard) Check(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", &RootError{Path: dir, Reason: "no folder given"}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", &RootError{Path: dir, Reason: err.Error()}
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", &RootError{Path: dir, Reason: "folder does not exist"}
	}
	st, err := os.Stat(real)
	if err != nil || !st.IsDir() {
		return "", &RootError{Path: dir, Reason: "not a folder"}
	}

	if filepath.Dir(real) == real {
		return "", &RootError{Path: real, Reason: "it is a filesystem root"}
	}
	// The home directory and everything above it.
	if home := canonical(g.Home); home != "" && within(real, home) {
		return "", &RootError{Path: real, Reason: "it is your home folder (or a parent of it); pick a folder inside it"}
	}
	for _, p := range g.Protected {
		if p = canonical(p); p != "" && within(p, real) {
			return "", &RootError{Path: real, Reason: "it is inside Memo's own data or install folder"}
		}
	}
	for _, s := range systemDirs() {
		if s = canonical(s); s != "" && within(s, real) {
			return "", &RootError{Path: real, Reason: "it is a system folder"}
		}
	}
	if looksLikeInstall(real) {
		return "", &RootError{Path: real, Reason: "its .memo folder is a Memo installation, not a project"}
	}
	return real, nil
}

// looksLikeInstall reports whether `<dir>/.memo` has the shape of a Memo
// install (as opposed to a project folder, which only holds small files).
func looksLikeInstall(dir string) bool {
	memo := filepath.Join(dir, DirName)
	for _, m := range installMarkers {
		if _, err := os.Lstat(filepath.Join(memo, m)); err == nil {
			return true
		}
	}
	return false
}

// FindRoot walks up from start to the nearest folder that holds a valid
// `.memo/project.json`, like git finds `.git`. The walk stops — without
// finding anything — at the first folder the guard refuses, so it never climbs
// into the home folder or above.
func (g Guard) FindRoot(start string) (string, bool) {
	cur, err := g.Check(start)
	if err != nil {
		return "", false
	}
	for {
		if _, err := ReadInfo(cur); err == nil {
			return cur, true
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", false
		}
		if _, err := g.Check(parent); err != nil {
			return "", false
		}
		cur = parent
	}
}
