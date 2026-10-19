// SPDX-License-Identifier: AGPL-3.0-or-later

package project

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestScaffold_CreatesTheStarterFiles(t *testing.T) {
	g, home := fakeHome(t)
	m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
	proj := mkdir(t, home, "app")

	res, err := m.Scaffold(proj)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{".memo/project.json", ".memo/AGENTS.md", ".memo/notes.md", ".memo/.gitignore", "AGENTS.md", "handoff.md"} {
		if _, err := os.Stat(filepath.Join(proj, filepath.FromSlash(f))); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	if len(res.Created) != 5 { // project.json is written by Init, not listed
		t.Errorf("Created = %v", res.Created)
	}
	if got := read(t, filepath.Join(proj, ".memo", ".gitignore")); !strings.Contains(got, "permissions.json") {
		t.Errorf(".gitignore must keep the per-user permissions file out of the repo: %q", got)
	}
	info, err := ReadInfo(proj)
	if err != nil || info.RulesVersion != RulesVersion || info.RulesHash == "" {
		t.Fatalf("project.json = %+v, %v", info, err)
	}

	// The root AGENTS.md is read by other tools too: it must not talk about Memo's folder.
	if root := read(t, filepath.Join(proj, "AGENTS.md")); strings.Contains(root, ".memo") || strings.Contains(strings.ToLower(root), "memo ") {
		t.Errorf("root AGENTS.md must stay tool-neutral:\n%s", root)
	}
	// Generic rules are what gets read first every session: keep them lean.
	if n := len(read(t, filepath.Join(proj, ".memo", "AGENTS.md"))); n > 6500 {
		t.Errorf(".memo/AGENTS.md is %d bytes; it is read at the start of every session, keep it small", n)
	}

	again, err := m.Scaffold(proj)
	if err != nil || len(again.Created) != 0 || len(again.Refreshed) != 0 {
		t.Fatalf("second Scaffold must change nothing: %+v, %v", again, err)
	}
}

func TestScaffold_NeverOverwritesTheUsersFiles(t *testing.T) {
	g, home := fakeHome(t)
	m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
	proj := mkdir(t, home, "app")
	own := map[string]string{"AGENTS.md": "# my rules\n", "handoff.md": "# my log\n"}
	for f, c := range own {
		if err := os.WriteFile(filepath.Join(proj, f), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := m.Scaffold(proj)
	if err != nil {
		t.Fatal(err)
	}
	for f, c := range own {
		if got := read(t, filepath.Join(proj, f)); got != c {
			t.Errorf("%s was overwritten: %q", f, got)
		}
	}
	if len(res.Skipped) < 2 {
		t.Errorf("Skipped = %v, want both existing files reported", res.Skipped)
	}
}

func TestScaffold_ACLAUDEmdMeansNoSecondRulesFile(t *testing.T) {
	g, home := fakeHome(t)
	m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
	proj := mkdir(t, home, "app")
	if err := os.WriteFile(filepath.Join(proj, "CLAUDE.md"), []byte("# rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Scaffold(proj); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(proj, "AGENTS.md")) {
		t.Fatal("a project that already has CLAUDE.md must not get a second, competing AGENTS.md skeleton")
	}
}

func TestScaffold_RefreshesUntouchedRulesButNeverAnEditedCopy(t *testing.T) {
	g, home := fakeHome(t)
	m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
	proj := mkdir(t, home, "app")
	if _, err := m.Scaffold(proj); err != nil {
		t.Fatal(err)
	}
	rulesPath := filepath.Join(proj, ".memo", "AGENTS.md")

	// Pretend an older Memo wrote an older text.
	old := []byte("# Memo Rules v0\n")
	if err := os.WriteFile(rulesPath, old, 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := ReadInfo(proj)
	info.RulesHash = sha(old)
	if err := writeInfo(proj, info); err != nil {
		t.Fatal(err)
	}
	res, err := m.Scaffold(proj)
	if err != nil || len(res.Refreshed) != 1 {
		t.Fatalf("untouched old rules must be refreshed: %+v, %v", res, err)
	}
	if !strings.Contains(read(t, rulesPath), "Memo Rules (managed by Memo") {
		t.Fatal("rules were not refreshed to the current text")
	}

	// Now the user edits it: a later refresh must leave it alone.
	edited := "# my own rules\n"
	if err := os.WriteFile(rulesPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = m.Scaffold(proj)
	if err != nil || len(res.Refreshed) != 0 {
		t.Fatalf("an edited rules file was refreshed: %+v, %v", res, err)
	}
	if read(t, rulesPath) != edited {
		t.Fatal("the user's edit was overwritten")
	}
}

func TestScaffold_RefusesForeignAndSymlinkedMemoDirs(t *testing.T) {
	g, home := fakeHome(t)
	m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))

	foreign := mkdir(t, home, "foreign", ".memo")
	if err := os.WriteFile(filepath.Join(foreign, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Scaffold(filepath.Dir(foreign)); !errors.Is(err, ErrForeignDir) {
		t.Fatalf("foreign .memo: %v", err)
	}
	if exists(filepath.Join(filepath.Dir(foreign), "AGENTS.md")) || exists(filepath.Join(foreign, InfoFile)) {
		t.Fatal("files were written next to a foreign .memo")
	}

	if runtime.GOOS == "windows" {
		return
	}
	// The dangerous link points at a folder that already holds a VALID
	// project.json (here: another real project's .memo), so only the symlink
	// check itself can stop Memo writing through it.
	other := mkdir(t, home, "other-project")
	if _, err := m.Init(other); err != nil {
		t.Fatal(err)
	}
	linked := mkdir(t, home, "linked")
	if err := os.Symlink(Dir(other), filepath.Join(linked, ".memo")); err != nil {
		t.Skip("symlink unavailable:", err)
	}
	before := dirListing(t, Dir(other))
	if _, err := m.Scaffold(linked); !errors.Is(err, ErrForeignDir) {
		t.Fatalf("symlinked .memo: %v", err)
	}
	if dirListing(t, Dir(other)) != before {
		t.Fatal("Memo wrote through a symlinked .memo into another project's folder")
	}
}

func TestScaffold_RefusesTheHomeFolder(t *testing.T) {
	g, home := fakeHome(t)
	m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
	before := dirListing(t, filepath.Join(home, ".memo"))
	if _, err := m.Scaffold(home); !errors.Is(err, ErrForbiddenRoot) {
		t.Fatalf("Scaffold(home) = %v", err)
	}
	if exists(filepath.Join(home, "AGENTS.md")) || exists(filepath.Join(home, "handoff.md")) {
		t.Fatal("project files were written into the home folder")
	}
	if dirListing(t, filepath.Join(home, ".memo")) != before {
		t.Fatal("the install's .memo changed")
	}
}

func TestStatus(t *testing.T) {
	g, home := fakeHome(t)
	m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
	proj := mkdir(t, home, "app")

	if st := m.Status(home); st.Allowed || st.Reason == "" {
		t.Errorf("home: %+v", st)
	}
	st := m.Status(proj)
	if !st.Allowed || st.SetUp || st.ID == "" {
		t.Errorf("fresh folder: %+v", st)
	}
	if _, err := m.Scaffold(proj); err != nil {
		t.Fatal(err)
	}
	if st2 := m.Status(proj); !st2.SetUp || st2.ID != st.ID || st2.Name != "app" {
		t.Errorf("after setup: %+v (pending id was %s)", st2, st.ID)
	}
}
