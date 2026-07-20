package replcli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupFileMatchTree builds a small tree with a few real files plus
// excluded-directory noise (.git, node_modules) that fileMatches must never
// surface.
func setupFileMatchTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := []string{
		"main.go",
		"README.md",
		filepath.Join("internal", "repl.go"),
		filepath.Join("internal", "readme_notes.txt"),
		filepath.Join(".git", "HEAD"),
		filepath.Join("node_modules", "pkg", "index.js"),
	}
	for _, f := range files {
		full := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestFileMatches_EmptyRootReturnsNil(t *testing.T) {
	if got := fileMatches("", "anything"); got != nil {
		t.Errorf("fileMatches with empty root = %v, want nil", got)
	}
}

func TestFileMatches_FiltersByQueryCaseInsensitive(t *testing.T) {
	root := setupFileMatchTree(t)
	got := fileMatches(root, "REAdme")
	if len(got) != 2 { // README.md, internal/readme_notes.txt
		t.Fatalf("fileMatches(REAdme) = %v, want 2 entries", got)
	}
	for _, c := range got {
		if !strings.Contains(strings.ToLower(c.label), "readme") {
			t.Errorf("unexpected match %q", c.label)
		}
	}
}

func TestFileMatches_EmptyQueryExcludesGitAndNodeModules(t *testing.T) {
	root := setupFileMatchTree(t)
	got := fileMatches(root, "")
	if len(got) != 4 { // main.go, README.md, internal/repl.go, internal/readme_notes.txt
		t.Fatalf("fileMatches(\"\") = %d entries, want 4: %v", len(got), got)
	}
	for _, c := range got {
		if strings.Contains(c.label, ".git") || strings.Contains(c.label, "node_modules") {
			t.Errorf("excluded dir leaked into matches: %q", c.label)
		}
	}
}

func TestFileMatches_CapsAtMaxFileMatches(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < maxFileMatches+10; i++ {
		name := fmt.Sprintf("file%02d.txt", i)
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := fileMatches(root, "")
	if len(got) != maxFileMatches {
		t.Errorf("fileMatches count = %d, want cap %d", len(got), maxFileMatches)
	}
}
