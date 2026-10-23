package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSubAgentMergeValidationChecksMixedModules(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses local POSIX fake SDK executables")
	}
	root := t.TempDir()
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "checks")
	t.Setenv("MEMO_VALIDATION_LOG", log)
	for _, tool := range []string{"go", "flutter"} {
		script := "#!/bin/sh\nprintf '%s\\n' '" + tool + "' >> \"$MEMO_VALIDATION_LOG\"\n"
		if err := os.WriteFile(filepath.Join(bin, tool), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, b)
		}
	}
	git("init", "-b", "main")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.invalid")
	if err := os.Mkdir(filepath.Join(root, "frontend"), 0700); err != nil {
		t.Fatal(err)
	}
	for p, s := range map[string]string{"go.mod": "module example.invalid/task\n", "frontend/pubspec.yaml": "name: task\nflutter:\n", "frontend/main.dart": "old\n"} {
		if err := os.WriteFile(filepath.Join(root, p), []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("commit", "-m", "initial")
	if err := os.WriteFile(filepath.Join(root, "frontend", "main.dart"), []byte("new\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	if err := validateSubAgentMerge(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "go\n") || !strings.Contains(string(b), "flutter\n") {
		t.Fatalf("mixed-project validation omitted a suite: %s", b)
	}
}
