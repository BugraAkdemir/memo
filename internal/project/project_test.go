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

// fakeHome builds <tmp>/home with a Memo install in home/.memo and returns the
// guard a Manager would use on that machine.
func fakeHome(t *testing.T) (Guard, string) {
	t.Helper()
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(tmp, "home")
	for _, d := range []string{"bin", "binaries", "data", "config"} {
		if err := os.MkdirAll(filepath.Join(home, ".memo", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	g := Guard{Home: home, Protected: []string{filepath.Join(home, ".memo")}}
	return g, home
}

func mkdir(t *testing.T, parts ...string) string {
	t.Helper()
	p := filepath.Join(parts...)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGuard_RefusesHomeItsParentsAndTheInstall(t *testing.T) {
	g, home := fakeHome(t)
	for name, dir := range map[string]string{
		"home":            home,
		"parent of home":  filepath.Dir(home),
		"install dir":     filepath.Join(home, ".memo"),
		"inside install":  filepath.Join(home, ".memo", "data"),
		"filesystem root": string(filepath.Separator),
		"nonexistent":     filepath.Join(home, "nope"),
		"empty":           "",
	} {
		_, err := g.Check(dir)
		if !errors.Is(err, ErrForbiddenRoot) {
			t.Errorf("%s: Check(%q) = %v, want ErrForbiddenRoot", name, dir, err)
		}
	}
}

func TestGuard_AcceptsAFolderInsideHome(t *testing.T) {
	g, home := fakeHome(t)
	proj := mkdir(t, home, "Documents", "app")
	got, err := g.Check(proj)
	if err != nil || got != proj {
		t.Fatalf("Check(%q) = %q, %v", proj, got, err)
	}
}

func TestGuard_RefusesAnInstallLookalikeEvenOutsideHome(t *testing.T) {
	g, home := fakeHome(t)
	// A second Memo install in another place (e.g. a runner workspace).
	other := mkdir(t, filepath.Dir(home), "runner")
	for _, d := range []string{"bin", "data"} {
		mkdir(t, other, ".memo", d)
	}
	if _, err := g.Check(other); !errors.Is(err, ErrForbiddenRoot) {
		t.Fatalf("Check(install lookalike) = %v, want ErrForbiddenRoot", err)
	}
}

func TestGuard_AllowsAFolderThatContainsMemosDataDir(t *testing.T) {
	// Memo's own source checkout keeps ./data inside the project.
	g, home := fakeHome(t)
	repo := mkdir(t, home, "memo")
	data := mkdir(t, repo, "data")
	g.Protected = append(g.Protected, data)
	if _, err := g.Check(repo); err != nil {
		t.Fatalf("a project that merely contains the data dir must be allowed: %v", err)
	}
	if _, err := g.Check(filepath.Join(data)); !errors.Is(err, ErrForbiddenRoot) {
		t.Fatalf("the data dir itself must be refused, got %v", err)
	}
}

func TestGuard_ResolvesSymlinksToHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	g, home := fakeHome(t)
	link := filepath.Join(filepath.Dir(home), "shortcut")
	if err := os.Symlink(home, link); err != nil {
		t.Skip("symlink unavailable:", err)
	}
	if _, err := g.Check(link); !errors.Is(err, ErrForbiddenRoot) {
		t.Fatalf("a symlink to home must be refused, got %v", err)
	}
}

func TestGuard_RefusesSystemFolders(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix system dirs")
	}
	g, _ := fakeHome(t)
	for _, d := range []string{"/usr", "/etc"} {
		if _, err := os.Stat(d); err != nil {
			continue
		}
		if _, err := g.Check(d); !errors.Is(err, ErrForbiddenRoot) {
			t.Errorf("Check(%s) = %v, want ErrForbiddenRoot", d, err)
		}
	}
}

func TestReadInfo_DistinguishesNoneForeignAndProject(t *testing.T) {
	g, home := fakeHome(t)
	_ = g
	plain := mkdir(t, home, "plain")
	if _, err := ReadInfo(plain); !errors.Is(err, ErrNoProject) {
		t.Fatalf("no .memo: %v", err)
	}
	if _, err := ReadInfo(home); !errors.Is(err, ErrForeignDir) {
		t.Fatalf("the install's .memo has no project.json and must read as foreign: %v", err)
	}
	bad := mkdir(t, home, "bad", ".memo")
	if err := os.WriteFile(filepath.Join(bad, InfoFile), []byte(`{"kind":"project","id":"nope"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadInfo(filepath.Dir(bad)); !errors.Is(err, ErrForeignDir) {
		t.Fatalf("invalid id must read as foreign: %v", err)
	}
}

func TestFindRoot_WalksUpButStopsAtHome(t *testing.T) {
	g, home := fakeHome(t)
	m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
	proj := mkdir(t, home, "work", "app")
	if _, err := m.Init(proj); err != nil {
		t.Fatal(err)
	}
	deep := mkdir(t, proj, "src", "pkg")
	if root, ok := g.FindRoot(deep); !ok || root != proj {
		t.Fatalf("FindRoot(%s) = %q, %v; want %q", deep, root, ok, proj)
	}

	// A stray project.json in the home folder must never be picked up from a
	// sibling folder.
	if err := os.WriteFile(filepath.Join(home, ".memo", InfoFile),
		[]byte(`{"kind":"project","schema":1,"id":"p_`+strings.Repeat("a", 32)+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sibling := mkdir(t, home, "other", "x")
	if root, ok := g.FindRoot(sibling); ok {
		t.Fatalf("FindRoot climbed into home: %q", root)
	}
}

func TestOpen_OnHomeIsRefusedAndNothingIsWritten(t *testing.T) {
	g, home := fakeHome(t)
	m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
	before := dirListing(t, filepath.Join(home, ".memo"))
	if _, err := m.Open(home); !errors.Is(err, ErrForbiddenRoot) {
		t.Fatalf("Open(home) = %v", err)
	}
	if _, err := m.Init(home); !errors.Is(err, ErrForbiddenRoot) {
		t.Fatalf("Init(home) = %v", err)
	}
	if after := dirListing(t, filepath.Join(home, ".memo")); after != before {
		t.Fatalf("the install dir changed:\nbefore %s\nafter  %s", before, after)
	}
}

func dirListing(t *testing.T, dir string) string {
	t.Helper()
	var names []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil {
			names = append(names, p)
		}
		return nil
	})
	return strings.Join(names, "\n")
}

func TestPendingIDBecomesTheProjectID(t *testing.T) {
	g, home := fakeHome(t)
	regPath := filepath.Join(t.TempDir(), "projects.json")
	m := NewManager(g, regPath)
	proj := mkdir(t, home, "app")

	pending, err := m.Open(proj)
	if err != nil || pending.SetUp || !ValidID(pending.ID) {
		t.Fatalf("Open before setup = %+v, %v", pending, err)
	}
	if _, err := os.Stat(Dir(proj)); !os.IsNotExist(err) {
		t.Fatal("Open must not create .memo")
	}

	// A new process (new Manager, same registry) sees the same pending id.
	again, _ := NewManager(g, regPath).Open(proj)
	if again.ID != pending.ID {
		t.Fatalf("pending id not persisted: %s vs %s", again.ID, pending.ID)
	}

	set, err := m.Init(proj)
	if err != nil || !set.SetUp || set.ID != pending.ID {
		t.Fatalf("Init = %+v, %v; want id %s", set, err, pending.ID)
	}
	info, err := ReadInfo(proj)
	if err != nil || info.ID != pending.ID || info.Name != "app" || info.Kind != Kind {
		t.Fatalf("project.json = %+v, %v", info, err)
	}
	if again2, _ := m.Init(proj); again2.ID != pending.ID {
		t.Fatal("Init must be idempotent")
	}
}

func TestInit_NeverWritesIntoAForeignMemoDir(t *testing.T) {
	g, home := fakeHome(t)
	m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
	proj := mkdir(t, home, "app", ".memo")
	if err := os.WriteFile(filepath.Join(proj, "something.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Init(filepath.Dir(proj)); !errors.Is(err, ErrForeignDir) {
		t.Fatalf("Init over a foreign .memo = %v, want ErrForeignDir", err)
	}
	if _, err := os.Stat(filepath.Join(proj, InfoFile)); !os.IsNotExist(err) {
		t.Fatal("project.json was written into a foreign .memo")
	}
}

func TestMovedFolderKeepsItsID(t *testing.T) {
	g, home := fakeHome(t)
	m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
	a := mkdir(t, home, "a")
	first, err := m.Init(a)
	if err != nil {
		t.Fatal(err)
	}
	b := filepath.Join(home, "renamed")
	if err := os.Rename(a, b); err != nil {
		t.Fatal(err)
	}
	moved, err := m.Open(b)
	if err != nil || moved.ID != first.ID {
		t.Fatalf("moved folder id = %v (%v), want %s", moved, err, first.ID)
	}
	if _, stale := m.reg.Projects[a]; stale {
		t.Fatal("the old path was not forgotten after the move")
	}
}

func TestCopiedFolderGetsANewID(t *testing.T) {
	g, home := fakeHome(t)
	m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
	a := mkdir(t, home, "a")
	orig, err := m.Init(a)
	if err != nil {
		t.Fatal(err)
	}
	b := mkdir(t, home, "b")
	data, _ := os.ReadFile(InfoPath(a))
	mkdir(t, Dir(b))
	if err := os.WriteFile(InfoPath(b), data, 0o644); err != nil {
		t.Fatal(err)
	}

	cp, err := m.Open(b)
	if err != nil {
		t.Fatal(err)
	}
	if cp.ID == orig.ID {
		t.Fatal("a copy kept the original's id: two projects would share one memory")
	}
	if onDisk, _ := ReadInfo(b); onDisk.ID != cp.ID {
		t.Fatal("the copy's project.json was not rewritten with its new id")
	}
	if onDiskA, _ := ReadInfo(a); onDiskA.ID != orig.ID {
		t.Fatal("the original's project.json must not change")
	}
	if again, _ := m.Open(a); again.ID != orig.ID {
		t.Fatal("the original lost its id")
	}
	if again, _ := m.Open(b); again.ID != cp.ID {
		t.Fatal("the copy's new id is not stable")
	}
}

func TestRegistry_CorruptFileStartsEmptyAndKeepsACopy(t *testing.T) {
	g, home := fakeHome(t)
	regPath := filepath.Join(t.TempDir(), "projects.json")
	if err := os.WriteFile(regPath, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := NewManager(g, regPath)
	if _, err := m.Open(mkdir(t, home, "app")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(regPath + ".bad"); err != nil {
		t.Fatalf("corrupt registry copy missing: %v", err)
	}
}

func TestDeclinedIsRemembered(t *testing.T) {
	g, home := fakeHome(t)
	regPath := filepath.Join(t.TempDir(), "projects.json")
	m := NewManager(g, regPath)
	proj := mkdir(t, home, "app")
	if err := m.SetDeclined(proj, true); err != nil {
		t.Fatal(err)
	}
	if r, _ := NewManager(g, regPath).Open(proj); !r.Declined {
		t.Fatal("declined flag lost")
	}
	if _, err := m.Init(proj); err != nil {
		t.Fatal(err)
	}
	if r, _ := m.Open(proj); r.Declined {
		t.Fatal("setting the folder up must clear the declined flag")
	}
}
