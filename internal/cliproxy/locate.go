// SPDX-License-Identifier: AGPL-3.0-or-later

package cliproxy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"memo/internal/config"
)

// binInfo is a verified, ready-to-run executable.
type binInfo struct {
	Path    string
	Version string
	size    int64
	mtime   time.Time
}

// binaryFile is the executable's file name for this platform inside
// binaries/<os>/cliproxy/. Release archives name it cli-proxy-api[.exe]; the
// macOS tree carries both architectures side by side (the build is a single
// universal package), so they are told apart by suffix.
func binaryFile(goos, goarch string) string {
	switch goos {
	case "windows":
		return "cli-proxy-api.exe"
	case "darwin":
		if goarch == "arm64" {
			return "cli-proxy-api-arm64"
		}
		return "cli-proxy-api-x64"
	}
	return "cli-proxy-api"
}

// searchRoots lists the directories that may hold a binaries/ tree, best first.
//
// The executable's own directory comes first (packaged builds), then ITS PARENT:
// the installed CLI lives at ~/.memo/bin/memo, one level below the tree the
// installer copied (AGENTS.md "Paths & data"). "." covers `go run` from a
// checkout, and DataDir() covers the first-run copy the launch scripts make.
func (m *Manager) searchRoots() []string {
	if m.roots != nil {
		return m.roots
	}
	var roots []string
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		d := filepath.Dir(exe)
		roots = append(roots, d, filepath.Dir(d))
	}
	roots = append(roots, ".", config.DataDir())
	return roots
}

// Binary returns the bundled sidecar executable, verified against the SHA-256
// file shipped beside it. A binary with no checksum, or the wrong one, is
// refused: Memo runs this with the user's credentials in reach, so "found it on
// disk" is not enough.
func (m *Manager) Binary() (path, version string, err error) {
	m.binMu.Lock()
	defer m.binMu.Unlock()

	goos, goarch := m.goos, m.goarch
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	name := binaryFile(goos, goarch)

	var refused error
	for _, root := range m.searchRoots() {
		dir := filepath.Join(root, "binaries", goos, "cliproxy")
		p := filepath.Join(dir, name)
		st, statErr := os.Stat(p)
		if statErr != nil || st.IsDir() {
			continue
		}
		if c := m.binCache; c != nil && c.Path == p && c.size == st.Size() && c.mtime.Equal(st.ModTime()) {
			return c.Path, c.Version, nil
		}
		if verr := verifyAgainstSidecarFile(p, dir, name); verr != nil {
			refused = verr
			continue
		}
		ver := ""
		if b, rerr := os.ReadFile(filepath.Join(dir, "VERSION")); rerr == nil {
			ver = strings.TrimSpace(string(b))
		}
		m.binCache = &binInfo{Path: p, Version: ver, size: st.Size(), mtime: st.ModTime()}
		return p, ver, nil
	}
	if refused != nil {
		return "", "", refused
	}
	return "", "", ErrNotBundled
}

// verifyAgainstSidecarFile checks p against the `SHA256` file in dir
// (sha256sum format, one line per binary — the macOS tree has two).
func verifyAgainstSidecarFile(p, dir, name string) error {
	raw, err := os.ReadFile(filepath.Join(dir, "SHA256"))
	if err != nil {
		return fmt.Errorf("%w: no SHA256 file beside %s", ErrUnverified, p)
	}
	want := ""
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name {
			want = strings.ToLower(f[0])
		}
	}
	if want == "" {
		return fmt.Errorf("%w: SHA256 file has no entry for %s", ErrUnverified, name)
	}
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("%w: %s is %s, expected %s", ErrUnverified, name, got, want)
	}
	return nil
}
