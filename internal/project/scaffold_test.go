// SPDX-License-Identifier: AGPL-3.0-or-later

package project

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

func TestScaffold_PreservesCaseVariants(t *testing.T) {
	for _, name := range []string{"agents.md", "Agents.md", "claude.md", "ClAuDe.Md", "Handoff.md", "HANDOFF.MD"} {
		t.Run(name, func(t *testing.T) {
			g, home := fakeHome(t)
			m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
			root := mkdir(t, home, "app")
			path := filepath.Join(root, name)
			if err := os.WriteFile(path, []byte("user content"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := m.Scaffold(root); err != nil {
				t.Fatal(err)
			}
			if got := read(t, path); got != "user content" {
				t.Fatalf("overwritten: %q", got)
			}
			canonical := "AGENTS.md"
			if strings.EqualFold(name, "handoff.md") {
				canonical = "handoff.md"
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.Name() == canonical {
					t.Fatalf("created duplicate %s beside %s", canonical, name)
				}
			}
		})
	}
}

func TestScaffold_PreservesManagedCaseVariants(t *testing.T) {
	for _, name := range []string{"agents.md", "Notes.md", ".GITIGNORE"} {
		t.Run(name, func(t *testing.T) {
			g, home := fakeHome(t)
			m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
			root := mkdir(t, home, "app")
			if _, err := m.Init(root); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(Dir(root), name)
			if err := os.WriteFile(path, []byte("user content"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := m.Scaffold(root); err != nil {
				t.Fatal(err)
			}
			if got := read(t, path); got != "user content" {
				t.Fatal("overwritten")
			}
			entries, _ := os.ReadDir(Dir(root))
			count := 0
			for _, entry := range entries {
				if strings.EqualFold(entry.Name(), name) {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("case duplicates: %d", count)
			}
		})
	}
}

func TestCreateStarter_NeverReplacesAnExistingEntry(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(path, []byte("editor won the race"), 0600); err != nil {
		t.Fatal(err)
	}
	created, err := createStarter(path, []byte("template"))
	if err != nil || created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if got := read(t, path); got != "editor won the race" {
		t.Fatalf("overwritten: %s", got)
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(root, "handoff.md")
		if err := os.Symlink(filepath.Join(root, "missing"), link); err != nil {
			t.Fatal(err)
		}
		if created, err := createStarter(link, []byte("template")); err != nil || created {
			t.Fatalf("replaced link: %v %v", created, err)
		}
	}
}

func TestScaffold_PreservesManagedRulesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges")
	}
	g, home := fakeHome(t)
	m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
	root := mkdir(t, home, "app")
	res, err := m.Scaffold(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(Dir(root), "AGENTS.md")
	target := filepath.Join(t.TempDir(), "rules")
	old := []byte("previous generated template")
	if err := os.WriteFile(target, old, 0600); err != nil {
		t.Fatal(err)
	}
	info, err := ReadInfo(root)
	if err != nil {
		t.Fatal(err)
	}
	info.RulesHash = sha(old)
	if err := writeInfo(root, info); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Scaffold(res.Root); err != nil {
		t.Fatal(err)
	}
	st, err := os.Lstat(path)
	if err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("link replaced: %v %v", st, err)
	}
	if got := read(t, target); got != string(old) {
		t.Fatalf("target changed: %s", got)
	}
}

func TestInit_PreservesCaseVariantMemoAndMarkerLinks(t *testing.T) {
	for _, mode := range []string{"directory", "marker-link"} {
		t.Run(mode, func(t *testing.T) {
			if runtime.GOOS == "windows" && mode == "marker-link" {
				t.Skip("symlink privileges")
			}
			g, home := fakeHome(t)
			m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
			root := mkdir(t, home, "app")
			if mode == "directory" {
				mkdir(t, root, ".Memo")
			} else {
				other := mkdir(t, home, "other")
				if _, err := m.Init(other); err != nil {
					t.Fatal(err)
				}
				mkdir(t, root, DirName)
				if err := os.Symlink(InfoPath(other), InfoPath(root)); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := m.Scaffold(root); !errors.Is(err, ErrForeignDir) {
				t.Fatalf("expected refusal, got %v", err)
			}
			if exists(filepath.Join(root, "AGENTS.md")) {
				t.Fatal("wrote starter into refused project")
			}
		})
	}
}

func TestRulesPath_UsesExistingSpelling(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root, DirName)
	path := filepath.Join(Dir(root), "agents.md")
	if err := os.WriteFile(path, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := RulesPath(root)
	if err != nil || got != path {
		t.Fatalf("got %q, %v; want %q", got, err, path)
	}
}

func TestCreateInfo_NeverAdoptsDirectoryCreatedByAnotherWriter(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root, DirName)
	marker := InfoPath(root)
	if err := os.WriteFile(marker, []byte("foreign marker"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := createInfo(root, newInfo(root, NewID(), time.Now())); !errors.Is(err, ErrForeignDir) {
		t.Fatalf("got %v", err)
	}
	if got := read(t, marker); got != "foreign marker" {
		t.Fatalf("overwritten marker: %s", got)
	}
}

func TestCreateStarter_ConcurrentWritersPublishOneCompleteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	results := make(chan bool, 16)
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		go func() {
			created, err := createStarter(path, []byte(strings.Repeat("complete", 1024)))
			results <- created
			errs <- err
		}()
	}
	created := 0
	for i := 0; i < 16; i++ {
		if <-results {
			created++
		}
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if created != 1 {
		t.Fatalf("created %d times", created)
	}
	if got := read(t, path); got != strings.Repeat("complete", 1024) {
		t.Fatal("partial content")
	}
}

func TestScaffold_RefreshPreservesExistingTemporaryNames(t *testing.T) {
	g, home := fakeHome(t)
	m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
	root := mkdir(t, home, "app")
	if _, err := m.Scaffold(root); err != nil {
		t.Fatal(err)
	}
	rules := filepath.Join(Dir(root), "AGENTS.md")
	old := []byte("old generated rules")
	if err := os.WriteFile(rules, old, 0600); err != nil {
		t.Fatal(err)
	}
	info, err := ReadInfo(root)
	if err != nil {
		t.Fatal(err)
	}
	info.RulesHash = sha(old)
	if err := writeInfo(root, info); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{rules + ".tmp", InfoPath(root) + ".tmp"} {
		if err := os.WriteFile(path, []byte("user data"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.Scaffold(root); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{rules + ".tmp", InfoPath(root) + ".tmp"} {
		if got, err := os.ReadFile(path); err != nil || string(got) != "user data" {
			t.Errorf("lost %s: %q %v", path, got, err)
		}
	}
}

func TestScaffold_PreservesDirectoriesAndDanglingLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privileges")
	}
	g, home := fakeHome(t)
	m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
	root := mkdir(t, home, "app")
	if _, err := m.Init(root); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(root, "agents.md"), filepath.Join(Dir(root), "AGENTS.md")} {
		mkdir(t, path)
	}
	handoff := filepath.Join(root, "Handoff.md")
	if err := os.Symlink("missing-target", handoff); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Scaffold(root); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(root, "agents.md"), filepath.Join(Dir(root), "AGENTS.md")} {
		if st, err := os.Lstat(path); err != nil || !st.IsDir() {
			t.Fatalf("directory replaced: %s %v", path, err)
		}
	}
	if target, err := os.Readlink(handoff); err != nil || target != "missing-target" {
		t.Fatalf("link replaced: %q %v", target, err)
	}
}

func TestStarterCreationFailsClosedWhenParentCannotBeRead(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "missing")
	path := filepath.Join(parent, "AGENTS.md")
	if found, err := existingPath(path); err == nil || found != "" {
		t.Fatalf("missing parent treated as writable: %q %v", found, err)
	}
	if created, err := createStarter(path, []byte("template")); err == nil || created {
		t.Fatalf("created into missing parent: %v %v", created, err)
	}
	if path, err := RulesPath(root); err == nil || path != "" {
		t.Fatalf("missing .memo ignored: %q %v", path, err)
	}
	// A plain file where the parent should be must also fail without changing it.
	if err := os.WriteFile(parent, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := existingPath(path); err == nil {
		t.Fatal("non-directory parent accepted")
	}
	if _, err := createStarter(path, []byte("template")); err == nil {
		t.Fatal("non-directory parent accepted")
	}
	if got := read(t, parent); got != "keep" {
		t.Fatalf("parent replaced: %s", got)
	}
}
