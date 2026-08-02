// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestListProjectFiles_FiltersByQueryAndExcludesJunkDirs(t *testing.T) {
	root := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "lib"), 0755))
	must(os.MkdirAll(filepath.Join(root, "node_modules", "pkg"), 0755))
	must(os.WriteFile(filepath.Join(root, "lib", "main.dart"), []byte("x"), 0644))
	must(os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0644))
	must(os.WriteFile(filepath.Join(root, "node_modules", "pkg", "index.js"), []byte("x"), 0644))

	a := &App{}
	got := a.ListProjectFiles(root, "dart")
	if len(got) != 1 || got[0] != "lib/main.dart" {
		t.Fatalf("got %v, want [lib/main.dart]", got)
	}

	all := a.ListProjectFiles(root, "")
	sort.Strings(all)
	for _, f := range all {
		if f == "node_modules/pkg/index.js" {
			t.Errorf("node_modules should be excluded, got %v", all)
		}
	}
}

// TestListProjectFiles_DotEntriesDontCrowdOutRealFiles is the regression
// test for a real reported bug: a project's usual handful of dotfiles/dirs
// (.git, .claude, .github, .env.example, ...) sort before every ordinary
// name, so with enough of them at the root the result cap was reached
// before a single real project file was ever listed — the "@" dropdown
// showed nothing but dotfiles.
func TestListProjectFiles_DotEntriesDontCrowdOutRealFiles(t *testing.T) {
	root := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	// More dot-entries than maxFileMentionMatches, so the old behavior
	// would exhaust the cap on dot-entries alone.
	for i := 0; i < maxFileMentionMatches+5; i++ {
		must(os.WriteFile(filepath.Join(root, ".dotfile"+string(rune('a'+i))), []byte("x"), 0644))
	}
	must(os.MkdirAll(filepath.Join(root, "lib"), 0755))
	must(os.WriteFile(filepath.Join(root, "lib", "main.dart"), []byte("x"), 0644))

	a := &App{}
	got := a.ListProjectFiles(root, "")
	foundRealFile := false
	for _, f := range got {
		if f == "lib/main.dart" {
			foundRealFile = true
		}
		if filepath.Base(f)[0] == '.' {
			t.Errorf("dotfile %q should be excluded by default, got %v", f, got)
		}
	}
	if !foundRealFile {
		t.Errorf("lib/main.dart not found in %v — dot-entries still crowding out real files", got)
	}
}

// TestListProjectFiles_DotQueryStillFindsDotfiles verifies dotfiles are
// still reachable when explicitly searched for (query starts with ".").
func TestListProjectFiles_DotQueryStillFindsDotfiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env.example"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	got := a.ListProjectFiles(root, ".env")
	if len(got) != 1 || got[0] != ".env.example" {
		t.Fatalf("got %v, want [.env.example]", got)
	}
}

func TestListProjectFiles_EmptyRootReturnsNil(t *testing.T) {
	a := &App{}
	if got := a.ListProjectFiles("", "anything"); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}
