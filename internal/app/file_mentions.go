// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// maxFileMentionMatches caps how many entries the "@" dropdown in
// chat_input.dart shows — mirrors internal/replcli/filematch.go's own cap
// (fileMatches), same feature, HTTP-callable form since the Flutter
// frontend has no local filesystem access of its own.
const maxFileMentionMatches = 20

// ListProjectFiles walks root and returns paths (relative to root, forward
// slashes) whose path contains query (case-insensitive; empty query matches
// everything up to the cap). root == "" yields no matches rather than
// walking the whole filesystem. Same exclusion list as
// internal/replcli/filematch.go / internal/agent/tools/search.go, so build
// artifacts and dependency trees never clutter the list.
func (a *App) ListProjectFiles(root, query string) []string {
	if root == "" {
		return nil
	}
	query = strings.ToLower(query)
	// Dotfiles/dot-directories (.git, .claude, .github, .env.example, ...)
	// sort before every ordinary name in filepath.WalkDir's lexical order —
	// with a real project's usual handful of dot-entries at the root, the
	// result cap below was reached before a single non-dot file was ever
	// reached, so the dropdown showed nothing BUT dotfiles. Skip them by
	// default, matching how "@ mention" pickers in other editors behave;
	// still reachable by explicitly typing a query that starts with ".".
	skipDotEntries := !strings.HasPrefix(query, ".")
	var out []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if len(out) >= maxFileMentionMatches {
			return filepath.SkipAll
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "build", ".dart_tool":
				return filepath.SkipDir
			}
			if skipDotEntries && path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if skipDotEntries && strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if query != "" && !strings.Contains(strings.ToLower(rel), query) {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	return out
}
