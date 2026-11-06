package autonomy

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if b, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, b)
	}
}

func newRepo(t *testing.T, files ...string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "init", "-b", "main")
	gitIn(t, dir, "config", "user.email", "t@example.com")
	gitIn(t, dir, "config", "user.name", "T")
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("v1"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-m", "base")
	return dir
}

func TestDevWatchReportsOnlyProjectsWithUnfinishedWork(t *testing.T) {
	dirty := newRepo(t, "a.go", "b.go", "c.go", "d.go")
	for _, f := range []string{"a.go", "b.go"} {
		if err := os.WriteFile(filepath.Join(dirty, f), []byte("v2"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dirty, "new.txt"), []byte("n"), 0o600); err != nil {
		t.Fatal(err)
	}
	clean := newRepo(t, "x.go")
	lines := DevWatch(context.Background(), []RegisteredProject{
		{ID: "p1", Name: "Memo", Root: dirty},
		{ID: "p2", Name: "Temiz", Root: clean},
	})
	if len(lines) != 1 {
		t.Fatalf("lines=%q", lines)
	}
	l := lines[0]
	if !strings.HasPrefix(l, "Memo (3)") || !strings.Contains(l, "a.go") || !strings.Contains(l, "new.txt") {
		t.Fatalf("line=%q", l)
	}
	if strings.Contains(l, "Temiz") {
		t.Fatal("a clean project was reported")
	}
}

func TestDevWatchListsAtMostThreeFilesPerProject(t *testing.T) {
	dir := newRepo(t, "a", "b", "c", "d", "e")
	for _, f := range []string{"a", "b", "c", "d", "e"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("v2"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lines := DevWatch(context.Background(), []RegisteredProject{{ID: "p", Name: "P", Root: dir}})
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "P (5)") || strings.Count(lines[0], ",") != 2 || !strings.HasSuffix(lines[0], "…") {
		t.Fatalf("lines=%q", lines)
	}
}

func TestDevWatchSkipsWhatItCannotReadAndNeverFails(t *testing.T) {
	notRepo := t.TempDir()
	missing := filepath.Join(t.TempDir(), "gone")
	good := newRepo(t, "a.go")
	if err := os.WriteFile(filepath.Join(good, "a.go"), []byte("v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	lines := DevWatch(context.Background(), []RegisteredProject{
		{ID: "bad1", Name: "NotARepo", Root: notRepo},
		{ID: "bad2", Name: "Missing", Root: missing},
		{ID: "ok", Name: "Good", Root: good},
	})
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "Good (1)") {
		t.Fatalf("lines=%q", lines)
	}
}

func TestDevWatchHandlesRenamesAndLimitsProjects(t *testing.T) {
	dir := newRepo(t, "old.go")
	gitIn(t, dir, "mv", "old.go", "new.go")
	lines := DevWatch(context.Background(), []RegisteredProject{{ID: "p", Name: "R", Root: dir}})
	if len(lines) != 1 || !strings.Contains(lines[0], "new.go") || strings.Contains(lines[0], "->") {
		t.Fatalf("a rename must show the new name only: %q", lines)
	}
	many := []RegisteredProject{}
	for i := 0; i < devWatchMaxProjects+4; i++ {
		d := newRepo(t, "f")
		if err := os.WriteFile(filepath.Join(d, "f"), []byte("v2"), 0o600); err != nil {
			t.Fatal(err)
		}
		many = append(many, RegisteredProject{ID: string(rune('a' + i)), Name: "P", Root: d})
	}
	if got := DevWatch(context.Background(), many); len(got) != devWatchMaxProjects {
		t.Fatalf("reported %d projects, cap is %d", len(got), devWatchMaxProjects)
	}
}

// Found live: registering a project makes Memo write its own .memo/ folder, which
// then showed up as untracked "unfinished work" in every freshly set-up project.
func TestDevWatchIgnoresMemosOwnMetadataFolder(t *testing.T) {
	dir := newRepo(t, "a.go")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".memo"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"AGENTS.md", "project.json", "notes.md"} {
		if err := os.WriteFile(filepath.Join(dir, ".memo", f), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lines := DevWatch(context.Background(), []RegisteredProject{{ID: "p", Name: "P", Root: dir}})
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "P (1): a.go") || strings.Contains(lines[0], ".memo") {
		t.Fatalf("Memo's own folder was counted: %q", lines)
	}
	// A project whose ONLY change is Memo's folder has nothing unfinished.
	clean := newRepo(t, "x.go")
	if err := os.MkdirAll(filepath.Join(clean, ".memo"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clean, ".memo", "project.json"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := DevWatch(context.Background(), []RegisteredProject{{ID: "q", Name: "Q", Root: clean}}); len(got) != 0 {
		t.Fatalf("a project with only Memo metadata was reported: %q", got)
	}
}

func TestDevWatchWithNoProjectsIsEmpty(t *testing.T) {
	if got := DevWatch(context.Background(), nil); len(got) != 0 {
		t.Fatalf("%q", got)
	}
}
