// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
// determined. No extra permission check beyond normal request
// authentication: this exposes directory/file *names* only (never
// content) to any authenticated account, admin or not, which is no wider
// than what agent tools (ReadFile/WriteFile/RunCommand) already grant a
// "user"-role account's own agent chats — see yapacam.md's Faz 5.1 role
// boundary (users get full agent access; only server-*security*-config is
// admin-only).
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
