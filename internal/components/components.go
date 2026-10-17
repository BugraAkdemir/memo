// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components answers "which version of each bundled engine is
// installed here?" — the question the component updater (PLAN_auto_update.md)
// has to ask before it can decide anything.
//
// A component is a set of binaries that ships beside the app but moves on its
// own clock: llama.cpp (llama-server and its libraries), whisper.cpp
// (whisper-server) and the CLIProxyAPI sidecar. Each engine folder
// (binaries/<os>/<flavor>/) carries a one-line marker file naming the version
// it holds, because nothing else is reliable: the llama.cpp release tarballs
// carry no version file at all, and a library's file name stops encoding the
// build number after llama.cpp b11xxx.
//
// Files fetched by the updater never overwrite the install (on Windows it
// lives in Program Files, which needs an administrator); they go to an
// override tree under the data directory that has the same layout. A folder
// there counts only if it carries a valid marker — a half-written or hand-made
// folder is ignored, never trusted.
package components

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Name identifies a component.
type Name string

const (
	Llama    Name = "llama"
	Whisper  Name = "whisper"
	CLIProxy Name = "cliproxy"
)

// Markers are the version files, one line each, in the component's folder.
// CLIProxy's is the pre-existing VERSION that scripts/vendor_cliproxy.sh writes.
const (
	llamaMarker   = "LLAMA_VERSION"
	whisperMarker = "WHISPER_VERSION"
	cliproxyMark  = "VERSION"
)

// MarkerFile is the name of the file that records n's version.
func MarkerFile(n Name) string {
	switch n {
	case Llama:
		return llamaMarker
	case Whisper:
		return whisperMarker
	case CLIProxy:
		return cliproxyMark
	}
	return ""
}

// OverrideDirName is the folder under the data directory that holds
// component files the updater downloaded. It mirrors the install's own layout:
// <data>/components/binaries/<os>/<folder>/…
const OverrideDirName = "components"

// OverrideRoot is the base directory of the override tree for dataDir. Add it
// at the FRONT of a binary search so it wins over the bundled copy.
func OverrideRoot(dataDir string) string {
	return filepath.Join(dataDir, OverrideDirName)
}

// BundledRoots lists the directories that may hold the install's own
// binaries/ tree, best first: the working directory, the running executable's
// directory and its parent (the installed CLI lives one level deeper than the
// tree the installer copied). It mirrors the search the engines themselves do.
func BundledRoots() []string {
	roots := []string{"."}
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		d := filepath.Dir(exe)
		roots = append(roots, d, filepath.Dir(d))
	}
	return roots
}

// Installed describes one component as found on disk.
type Installed struct {
	Name Name
	// Version is the marker's text ("b9441", "1.8.6", "v8.0.25"), or "" when the
	// folder exists but has no valid marker — an install from before markers.
	// An unknown version is never treated as "outdated".
	Version string
	// Dir is the folder the answer came from; "" when the component is absent.
	Dir string
	// Override is true when Dir is in the override tree.
	Override bool
}

// Found reports whether the component is present at all.
func (i Installed) Found() bool { return i.Dir != "" }

// Known reports whether the version is known.
func (i Installed) Known() bool { return i.Version != "" }

var markerRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,31}$`)

// ReadMarker returns the version recorded in dir for n. A missing file gives
// ("", nil). Anything that is not one plain token — empty, several lines, odd
// characters, longer than 32 bytes — is an error and the caller treats the
// version as unknown: the value ends up in URLs and file names later, so it is
// validated at the door rather than escaped at every use.
func ReadMarker(dir string, n Name) (string, error) {
	name := MarkerFile(n)
	if name == "" {
		return "", fmt.Errorf("components: unknown component %q", n)
	}
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	s := strings.TrimSpace(string(b))
	if !markerRE.MatchString(s) {
		return "", fmt.Errorf("components: %s in %s is not a version token", name, dir)
	}
	return s, nil
}

// Detect finds component n for the given OS and engine folder ("cpu", "amd",
// "nvidia", "cpu-arm64", "metal", "cliproxy", …).
//
// overrideRoot (may be "") is searched first and counts only when the folder
// there has a valid marker. Then bundledRoots are searched in order; the first
// root whose folder exists decides, even without a marker (Version "").
func Detect(n Name, goos, folder, overrideRoot string, bundledRoots []string) Installed {
	out := Installed{Name: n}
	if MarkerFile(n) == "" || folder == "" {
		return out
	}
	sub := filepath.Join("binaries", goos, folder)
	if overrideRoot != "" {
		dir := filepath.Join(overrideRoot, sub)
		if v, err := ReadMarker(dir, n); err == nil && v != "" {
			return Installed{Name: n, Version: v, Dir: dir, Override: true}
		}
	}
	for _, root := range bundledRoots {
		dir := filepath.Join(root, sub)
		st, err := os.Stat(dir)
		if err != nil || !st.IsDir() {
			continue
		}
		v, err := ReadMarker(dir, n)
		if err != nil {
			v = ""
		}
		return Installed{Name: n, Version: v, Dir: dir}
	}
	return out
}

// Compare orders two versions of component n: -1 if a < b, 0 if equal, +1 if
// a > b. llama.cpp tags are build numbers ("b9441" < "b11456" — compared as
// numbers, never as text); the others are dotted numbers with an optional
// leading v ("1.8.6" < "1.10.0"). Mixing kinds, or anything that does not
// parse, is an error: an updater that cannot order two versions must not guess.
func Compare(n Name, a, b string) (int, error) {
	switch n {
	case Llama:
		x, err := parseBuild(a)
		if err != nil {
			return 0, err
		}
		y, err := parseBuild(b)
		if err != nil {
			return 0, err
		}
		return cmpInt(x, y), nil
	case Whisper, CLIProxy:
		x, err := parseDotted(a)
		if err != nil {
			return 0, err
		}
		y, err := parseDotted(b)
		if err != nil {
			return 0, err
		}
		for i := 0; i < len(x) || i < len(y); i++ {
			var p, q int
			if i < len(x) {
				p = x[i]
			}
			if i < len(y) {
				q = y[i]
			}
			if c := cmpInt(p, q); c != 0 {
				return c, nil
			}
		}
		return 0, nil
	}
	return 0, fmt.Errorf("components: unknown component %q", n)
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func parseBuild(s string) (int, error) {
	t := strings.TrimSpace(s)
	if len(t) < 2 || (t[0] != 'b' && t[0] != 'B') {
		return 0, fmt.Errorf("components: %q is not a llama.cpp build tag (bNNNN)", s)
	}
	n, err := strconv.Atoi(t[1:])
	if err != nil || n < 0 {
		return 0, fmt.Errorf("components: %q is not a llama.cpp build tag (bNNNN)", s)
	}
	return n, nil
}

func parseDotted(s string) ([]int, error) {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(strings.TrimPrefix(t, "v"), "V")
	if t == "" {
		return nil, fmt.Errorf("components: empty version")
	}
	parts := strings.Split(t, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("components: %q is not a dotted version", s)
		}
		out = append(out, n)
	}
	return out, nil
}
