package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SearchContentArgs represents arguments for the search_content tool.
type SearchContentArgs struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path"`
	Glob       string `json:"glob"`
	IgnoreCase bool   `json:"ignore_case"`
	Context    int    `json:"context"`
	MaxResults int    `json:"max_results"`
}

const (
	searchContentDefaultMax  = 100
	searchContentHardMax     = 500
	searchContentMaxFileSize = 1 << 20 // 1 MiB: bigger files are generated/data, not source
	searchContentMaxLine     = 300     // characters kept per matching line
	searchContentMaxOutput   = 30 << 10
)

// searchContentSkipDirs are never descended into: VCS metadata, dependency
// and build output. Matches search_files' own list, plus the usual caches.
var searchContentSkipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "build": true, "dist": true,
	".dart_tool": true, "target": true, "__pycache__": true, ".venv": true, ".idea": true,
}

// SearchContent searches INSIDE files for a regular expression and returns
// "path:line: text" hits — the grep a coding agent needs to find where a
// symbol is defined and every place it is used before changing it. search_files
// only matches file NAMES, so without this the model had to guess usages (or
// remember to shell out to grep), and edits made without seeing all call sites
// were the commonest way a change broke something else.
func SearchContent(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args SearchContentArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if strings.TrimSpace(args.Pattern) == "" {
		return "", fmt.Errorf("pattern is required")
	}
	expr := args.Pattern
	if args.IgnoreCase {
		expr = "(?i)" + expr
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return "", fmt.Errorf("invalid regular expression %q: %v (escape special characters such as ( ) [ ] { } . * + ? with a backslash to search for them literally)", args.Pattern, err)
	}

	root := args.Path
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	searchRoot, err := validatePath(root, basePath)
	if err != nil {
		return "", err
	}

	maxResults := args.MaxResults
	if maxResults <= 0 {
		maxResults = searchContentDefaultMax
	}
	if maxResults > searchContentHardMax {
		maxResults = searchContentHardMax
	}
	around := args.Context
	if around < 0 {
		around = 0
	}
	if around > 3 {
		around = 3
	}

	searchCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		searchCtx, cancel = context.WithTimeout(ctx, DefaultToolTimeout)
		defer cancel()
	}

	var out strings.Builder
	hits := 0
	filesWithHits := 0
	truncated := ""

	searchFile := func(path string) error {
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || info.Size() > searchContentMaxFileSize {
			return nil
		}
		head := make([]byte, 8000)
		n, _ := f.Read(head)
		if bytes.IndexByte(head[:n], 0) >= 0 {
			return nil // binary
		}
		if _, err := f.Seek(0, 0); err != nil {
			return nil
		}

		rel, relErr := filepath.Rel(basePath, path)
		if relErr != nil {
			rel = path
		}
		var lines []string
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1<<20)
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}

		trim := func(s string) string {
			if len(s) > searchContentMaxLine {
				return s[:searchContentMaxLine] + "…"
			}
			return s
		}
		lastPrinted := -1
		fileHit := false
		for i, line := range lines {
			if !re.MatchString(line) {
				continue
			}
			if hits >= maxResults {
				truncated = fmt.Sprintf("... (stopped at %d matches; narrow the pattern, path or glob)\n", maxResults)
				return fs.SkipAll
			}
			if out.Len() > searchContentMaxOutput {
				truncated = "... (output limit reached; narrow the pattern, path or glob)\n"
				return fs.SkipAll
			}
			start, end := i-around, i+around
			if start < 0 {
				start = 0
			}
			if end >= len(lines) {
				end = len(lines) - 1
			}
			if around > 0 && lastPrinted >= 0 && start > lastPrinted+1 {
				out.WriteString("--\n")
			}
			for j := start; j <= end; j++ {
				if j <= lastPrinted {
					continue
				}
				sep := "-"
				if j == i {
					sep = ":"
				}
				fmt.Fprintf(&out, "%s:%d%s %s\n", rel, j+1, sep, trim(lines[j]))
				lastPrinted = j
			}
			hits++
			fileHit = true
		}
		if fileHit {
			filesWithHits++
		}
		return nil
	}

	info, err := os.Stat(searchRoot)
	if err != nil {
		return "", fmt.Errorf("stat failed: %w", err)
	}
	if !info.IsDir() {
		if err := searchFile(searchRoot); err != nil && err != fs.SkipAll {
			return "", err
		}
	} else {
		walkErr := filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			select {
			case <-searchCtx.Done():
				truncated = "... (search timed out; narrow the path or glob)\n"
				return fs.SkipAll
			default:
			}
			if d.IsDir() {
				if path != searchRoot && searchContentSkipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if args.Glob != "" {
				ok, mErr := filepath.Match(args.Glob, d.Name())
				if mErr != nil {
					return fmt.Errorf("invalid glob %q: %w", args.Glob, mErr)
				}
				if !ok {
					return nil
				}
			}
			return searchFile(path)
		})
		if walkErr != nil && walkErr != fs.SkipAll {
			return "", walkErr
		}
	}

	if hits == 0 {
		return fmt.Sprintf("No matches for %q. (Searches file contents as a regular expression; use search_files to find files by name.)", args.Pattern), nil
	}
	summary := fmt.Sprintf("%d match(es) in %d file(s)\n", hits, filesWithHits)
	return summary + out.String() + truncated, nil
}
