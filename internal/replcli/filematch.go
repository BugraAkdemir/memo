package replcli

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// maxFileMatches caps how many entries the "@" dropdown shows — mirrors
// SearchFiles' own maxResults (internal/agent/tools/search.go) so a huge
// repo doesn't stall the composer or spam the dropdown.
const maxFileMatches = 20

// fileMatches walks root (the CLI's project directory) and returns files
// whose project-relative path contains query (case-insensitive; empty query
// matches everything up to the cap) as dropdown entries. Uses the same
// directory-exclusion list SearchFiles already uses (internal/agent/tools/
// search.go) so build artifacts and dependency trees never clutter the
// list. root == "" (no project path for this run) yields no matches rather
// than walking the whole filesystem.
func fileMatches(root, query string) []commandSpec {
	if root == "" {
		return nil
	}
	query = strings.ToLower(query)
	var out []commandSpec
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if len(out) >= maxFileMatches {
			return filepath.SkipAll
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "build", ".dart_tool":
				return filepath.SkipDir
			}
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
		out = append(out, commandSpec{label: rel})
		return nil
	})
	return out
}
