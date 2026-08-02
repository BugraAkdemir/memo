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

func TestListProjectFiles_EmptyRootReturnsNil(t *testing.T) {
	a := &App{}
	if got := a.ListProjectFiles("", "anything"); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}
