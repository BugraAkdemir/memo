// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBrowseServerPath_ListsImmediateChildrenDirsFirst(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "zzz-subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	a := &App{}
	res, err := a.BrowseServerPath(dir)
	if err != nil {
		t.Fatalf("BrowseServerPath: %v", err)
	}
	result, ok := res.(ServerBrowseResult)
	if !ok {
		t.Fatalf("expected ServerBrowseResult, got %T", res)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result.Entries))
	}
	// Directories sort before files regardless of name — "zzz-subdir"
	// (alphabetically last) must still come first since it's the only dir.
	if !result.Entries[0].IsDir || result.Entries[0].Name != "zzz-subdir" {
		t.Errorf("entries[0] = %+v, want the directory first", result.Entries[0])
	}
	if result.Entries[1].Name != "a.txt" || result.Entries[2].Name != "b.txt" {
		t.Errorf("expected files alphabetically after the dir, got %+v then %+v", result.Entries[1], result.Entries[2])
	}
}

func TestBrowseServerPath_EmptyPathDefaultsToAnAbsoluteDirectory(t *testing.T) {
	a := &App{}
	res, err := a.BrowseServerPath("")
	if err != nil {
		t.Fatalf("BrowseServerPath: %v", err)
	}
	result := res.(ServerBrowseResult)
	if !filepath.IsAbs(result.Path) {
		t.Errorf("expected an absolute path, got %q", result.Path)
	}
}

func TestBrowseServerPath_NonexistentPathErrors(t *testing.T) {
	a := &App{}
	if _, err := a.BrowseServerPath("/this/path/definitely/does/not/exist/anywhere"); err == nil {
		t.Fatal("expected an error for a nonexistent path")
	}
}

func TestBrowseServerPath_FileArgumentBrowsesItsParentDirectory(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "model.gguf")
	if err := os.WriteFile(filePath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	a := &App{}
	res, err := a.BrowseServerPath(filePath)
	if err != nil {
		t.Fatalf("BrowseServerPath: %v", err)
	}
	result := res.(ServerBrowseResult)
	resolvedDir, _ := filepath.EvalSymlinks(dir)
	resolvedResult, _ := filepath.EvalSymlinks(result.Path)
	if resolvedResult != resolvedDir {
		t.Errorf("Path = %q, want the file's containing directory %q", result.Path, dir)
	}
	found := false
	for _, e := range result.Entries {
		if e.Name == "model.gguf" && !e.IsDir {
			found = true
		}
	}
	if !found {
		t.Error("expected the file itself to appear as an entry of its parent directory")
	}
}

func TestBrowseServerPath_ParentOfFilesystemRootIsEmpty(t *testing.T) {
	root := "/"
	if os.PathSeparator == '\\' {
		t.Skip("filesystem-root parent test is POSIX-specific")
	}
	a := &App{}
	res, err := a.BrowseServerPath(root)
	if err != nil {
		t.Fatalf("BrowseServerPath: %v", err)
	}
	result := res.(ServerBrowseResult)
	if result.Parent != "" {
		t.Errorf("expected Parent to be empty at the filesystem root, got %q", result.Parent)
	}
}

func TestBrowseServerPath_SubdirectoryReportsCorrectParent(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "child")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}

	a := &App{}
	res, err := a.BrowseServerPath(sub)
	if err != nil {
		t.Fatalf("BrowseServerPath: %v", err)
	}
	result := res.(ServerBrowseResult)
	resolvedDir, _ := filepath.EvalSymlinks(dir)
	resolvedParent, _ := filepath.EvalSymlinks(result.Parent)
	if resolvedParent != resolvedDir {
		t.Errorf("Parent = %q, want %q", result.Parent, dir)
	}
}

func TestMakeServerDirectory_CreatesOneChildAndReturnsIt(t *testing.T) {
	parent := t.TempDir()
	a := &App{}
	res, err := a.MakeServerDirectory(parent, "  my-project  ")
	if err != nil {
		t.Fatalf("MakeServerDirectory: %v", err)
	}
	result, ok := res.(ServerBrowseResult)
	if !ok {
		t.Fatalf("expected ServerBrowseResult, got %T", res)
	}
	want := filepath.Join(parent, "my-project")
	if result.Path != want || result.Parent != parent {
		t.Errorf("got path %q parent %q, want %q / %q", result.Path, result.Parent, want, parent)
	}
	if len(result.Entries) != 0 {
		t.Errorf("a new folder is empty, got %d entries", len(result.Entries))
	}
	if fi, err := os.Stat(want); err != nil || !fi.IsDir() {
		t.Fatalf("folder was not created on disk: %v", err)
	}
}

func TestMakeServerDirectory_RefusesNamesThatCouldEscapeTheParent(t *testing.T) {
	parent := t.TempDir()
	a := &App{}
	for _, name := range []string{"", "   ", ".", "..", "a/b", "../x", `a\b`, "x\x00y", "/etc"} {
		if _, err := a.MakeServerDirectory(parent, name); err == nil {
			t.Errorf("name %q was accepted", name)
		}
	}
	// Nothing was created next to or inside the parent.
	if entries, _ := os.ReadDir(parent); len(entries) != 0 {
		t.Errorf("rejected names still created %d entries", len(entries))
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(parent), "x")); err == nil {
		t.Error("a name escaped the parent directory")
	}
}

func TestMakeServerDirectory_ExistingNameParentAndFileParentAreErrors(t *testing.T) {
	parent := t.TempDir()
	a := &App{}
	if _, err := a.MakeServerDirectory(parent, "dup"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.MakeServerDirectory(parent, "dup"); err == nil {
		t.Error("creating an existing folder must fail, not silently succeed")
	}
	if _, err := a.MakeServerDirectory(filepath.Join(parent, "missing"), "x"); err == nil {
		t.Error("a missing parent must fail (no implicit MkdirAll)")
	}
	file := filepath.Join(parent, "f.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.MakeServerDirectory(file, "x"); err == nil {
		t.Error("a file as parent must fail")
	}
	if _, err := a.MakeServerDirectory("", "x"); err == nil {
		t.Error("an empty parent must fail rather than guess a directory")
	}
}
