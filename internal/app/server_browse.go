// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// ServerBrowseEntry is one immediate child of a browsed directory.
type ServerBrowseEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// ServerBrowseResult is BrowseServerPath's response DTO.
type ServerBrowseResult struct {
	Path    string              `json:"path"`
	Parent  string              `json:"parent,omitempty"`
	Entries []ServerBrowseEntry `json:"entries"`
}

// BrowseServerPath lists path's immediate children — the backend half of
// the in-app server-side file browser. Every existing "pick a folder/file"
// control in the Flutter frontend (agent project folder, CLI workdir,
// model import) used the OS-native file_picker plugin, which only ever
// sees the CONNECTING CLIENT's own filesystem — indistinguishable from the
// right answer when client and backend are the same machine (Memo's
// original desktop-only design), but silently wrong against a self-hosted
// remote backend: the picked path is sent to the backend as-is, which
// then looks for it on ITS OWN disk and fails (concretely reported: model
// import failing outright, agent/CLI folder selection pointing at a path
// that doesn't exist server-side).
//
// This lets the Flutter client browse the *backend's* disk directly
// instead, so whatever it ends up sending (to NewAgentChat/
// SetChatCLIWorkdir/ImportLocalModel — none of which need to change,
// since they always expected a server-local path) is always correct
// regardless of where the client happens to be running.
//
// path == "" starts at the server's home directory (matching where a
// native file dialog usually opens) — falls back to "/" if that can't be
// determined. This function itself does no permission check (walks
// whatever path it's given with no boundary) — that's enforced one layer
// up, at the route: internal/webserver/server.go gates GET /api/files/browse
// behind requirePermissionStrict(..., hasModelsPerm), not just normal
// authentication, precisely because this exposes directory/file *names*
// (never content) system-wide to whoever can call it. Models was chosen
// there as the narrowest existing permission that already implies "may
// point the backend at an arbitrary local path" (importing a local model
// file), rather than treating every authenticated account as equivalent —
// see that route's own comment for the full reasoning.
func (a *App) BrowseServerPath(path string) (interface{}, error) {
	if path == "" {
		if home, err := os.UserHomeDir(); err == nil {
			path = home
		} else {
			path = "/"
		}
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}
	// A file path (e.g. re-opening the browser at a previously-picked
	// model file) browses its containing directory instead of erroring —
	// friendlier than making the caller strip the filename first.
	if !info.IsDir() {
		absPath = filepath.Dir(absPath)
	}

	dirEntries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	entries := make([]ServerBrowseEntry, 0, len(dirEntries))
	for _, d := range dirEntries {
		var size int64
		if fi, err := d.Info(); err == nil {
			size = fi.Size()
		}
		entries = append(entries, ServerBrowseEntry{Name: d.Name(), IsDir: d.IsDir(), Size: size})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir // directories before files
		}
		return entries[i].Name < entries[j].Name
	})

	parent := filepath.Dir(absPath)
	if parent == absPath {
		parent = "" // already at the filesystem root
	}

	return ServerBrowseResult{Path: absPath, Parent: parent, Entries: entries}, nil
}

// MakeServerDirectory creates ONE new directory called name inside parent and
// returns it as a (still empty) ServerBrowseResult, so the in-app folder
// picker can step straight into it.
//
// The picker used to be browse-only: a user choosing a project folder for an
// agent chat on a remote server, with no folder yet for the new project, had
// to leave Memo and make one over SSH. Creating it where the user is already
// looking is the whole feature.
//
// name must be a single path element. A separator, "." / "..", a NUL byte or
// an over-long name is refused so a name can never address anything but a
// direct child of parent (MkdirAll is deliberately not used for the same
// reason: no implicit parents, no silent success on an existing name).
// parent must already be an existing directory. Like BrowseServerPath this
// does no boundary check of its own; the route gates it the same way
// (internal/webserver/server.go, requirePermissionStrict + hasModelsPerm).
func (a *App) MakeServerDirectory(parent, name string) (interface{}, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return nil, fmt.Errorf("folder name is required")
	case name == "." || name == "..":
		return nil, fmt.Errorf("invalid folder name %q", name)
	case strings.ContainsAny(name, "/\\\x00"):
		return nil, fmt.Errorf("folder name cannot contain a path separator")
	case !utf8.ValidString(name) || len(name) > 255:
		return nil, fmt.Errorf("invalid folder name")
	}
	if parent == "" {
		return nil, fmt.Errorf("parent folder is required")
	}
	absParent, err := filepath.Abs(parent)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Stat(absParent)
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a folder", absParent)
	}
	target := filepath.Join(absParent, name)
	if err := os.Mkdir(target, 0o755); err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("%q already exists", name)
		}
		return nil, fmt.Errorf("create folder: %w", err)
	}
	return ServerBrowseResult{Path: target, Parent: absParent, Entries: []ServerBrowseEntry{}}, nil
}
