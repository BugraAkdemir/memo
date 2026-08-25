package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeSandbox struct {
	basePath string
	calls    int
}

func (f *fakeSandbox) SetBasePath(p string) {
	f.basePath = p
	f.calls++
}

type fakeProjectPathStore struct {
	byID map[string]string
	err  error
}

func (f *fakeProjectPathStore) SetProjectPath(sessionID, path string) error {
	if f.err != nil {
		return f.err
	}
	if f.byID == nil {
		f.byID = map[string]string{}
	}
	f.byID[sessionID] = path
	return nil
}

func changeDirArgs(t *testing.T, path string) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(ChangeDirectoryArgs{Path: path})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return b
}

func TestChangeDirectory_SwitchesLiveSandboxImmediately(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "sub")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}

	sandbox := &fakeSandbox{basePath: base}
	ctx := WithSandboxSetter(context.Background(), sandbox)

	result, err := ChangeDirectory(ctx, changeDirArgs(t, "sub"), base, nil)
	if err != nil {
		t.Fatalf("ChangeDirectory() error = %v", err)
	}
	if sandbox.calls != 1 {
		t.Errorf("SetBasePath called %d times, want 1", sandbox.calls)
	}
	resolvedTarget, _ := filepath.EvalSymlinks(target)
	if sandbox.basePath != resolvedTarget {
		t.Errorf("sandbox.basePath = %q, want %q", sandbox.basePath, resolvedTarget)
	}
	if !strings.Contains(result, resolvedTarget) {
		t.Errorf("result %q should mention the new directory %q", result, resolvedTarget)
	}
}

func TestChangeDirectory_PersistsViaProjectPathSetter(t *testing.T) {
	base := t.TempDir()
	sandbox := &fakeSandbox{basePath: base}
	store := &fakeProjectPathStore{}

	ctx := WithSandboxSetter(context.Background(), sandbox)
	ctx = WithProjectPathSetter(ctx, store, "session-1")

	if _, err := ChangeDirectory(ctx, changeDirArgs(t, base), base, nil); err != nil {
		t.Fatalf("ChangeDirectory() error = %v", err)
	}
	resolvedBase, _ := filepath.EvalSymlinks(base)
	if got := store.byID["session-1"]; got != resolvedBase {
		t.Errorf("persisted project path = %q, want %q", got, resolvedBase)
	}
}

func TestChangeDirectory_WorksWithoutProjectPathSetter(t *testing.T) {
	// No session attached (e.g. a background/anonymous run) — must still
	// succeed and switch the live sandbox; persistence is simply skipped.
	base := t.TempDir()
	sandbox := &fakeSandbox{basePath: base}
	ctx := WithSandboxSetter(context.Background(), sandbox)

	if _, err := ChangeDirectory(ctx, changeDirArgs(t, base), base, nil); err != nil {
		t.Fatalf("ChangeDirectory() error = %v", err)
	}
	if sandbox.calls != 1 {
		t.Errorf("SetBasePath called %d times, want 1", sandbox.calls)
	}
}

func TestChangeDirectory_NoSandboxInContext(t *testing.T) {
	base := t.TempDir()
	if _, err := ChangeDirectory(context.Background(), changeDirArgs(t, base), base, nil); err == nil {
		t.Error("ChangeDirectory() with no sandbox in context should error")
	}
}

func TestChangeDirectory_RejectsNonexistentPath(t *testing.T) {
	base := t.TempDir()
	ctx := WithSandboxSetter(context.Background(), &fakeSandbox{basePath: base})
	if _, err := ChangeDirectory(ctx, changeDirArgs(t, filepath.Join(base, "does-not-exist")), base, nil); err == nil {
		t.Error("ChangeDirectory() with a nonexistent path should error")
	}
}

func TestChangeDirectory_RejectsFileNotDirectory(t *testing.T) {
	base := t.TempDir()
	filePath := filepath.Join(base, "afile.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := WithSandboxSetter(context.Background(), &fakeSandbox{basePath: base})
	if _, err := ChangeDirectory(ctx, changeDirArgs(t, filePath), base, nil); err == nil {
		t.Error("ChangeDirectory() targeting a file, not a directory, should error")
	}
}

func TestChangeDirectory_RejectsEmptyPath(t *testing.T) {
	base := t.TempDir()
	ctx := WithSandboxSetter(context.Background(), &fakeSandbox{basePath: base})
	if _, err := ChangeDirectory(ctx, changeDirArgs(t, "   "), base, nil); err == nil {
		t.Error("ChangeDirectory() with a blank path should error")
	}
}

func TestChangeDirectory_RejectsHardDenylistedRoots(t *testing.T) {
	base := t.TempDir()
	sandbox := &fakeSandbox{basePath: base}
	ctx := WithSandboxSetter(context.Background(), sandbox)

	for _, root := range []string{"/", "/etc", "/root"} {
		if _, err := os.Stat(root); err != nil {
			continue // not present in this sandboxed test environment
		}
		if _, err := ChangeDirectory(ctx, changeDirArgs(t, root), base, nil); err == nil {
			t.Errorf("ChangeDirectory(%q) should have been rejected as a protected root", root)
		}
	}
	if sandbox.calls != 0 {
		t.Errorf("SetBasePath should never have been called, was called %d times", sandbox.calls)
	}
}

func TestChangeDirectory_AllowsTmp(t *testing.T) {
	// /tmp itself is on file.go's defaultProtectedPaths (an escape guard for
	// validatePath) but is deliberately NOT on hardDenylistedRoots here —
	// switching the sandbox root to /tmp is exactly what this tool is for.
	if _, err := os.Stat("/tmp"); err != nil {
		t.Skip("/tmp not present in this environment")
	}
	sandbox := &fakeSandbox{basePath: "/tmp"}
	ctx := WithSandboxSetter(context.Background(), sandbox)

	dir, err := os.MkdirTemp("/tmp", "changedir-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	if _, err := ChangeDirectory(ctx, changeDirArgs(t, dir), "/tmp", nil); err != nil {
		t.Errorf("ChangeDirectory() to a /tmp subdirectory should succeed, got error: %v", err)
	}
}

func TestChangeDirectory_ResolvesTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory in this environment")
	}
	sandbox := &fakeSandbox{basePath: home}
	ctx := WithSandboxSetter(context.Background(), sandbox)

	if _, err := ChangeDirectory(ctx, changeDirArgs(t, "~"), home, nil); err != nil {
		t.Errorf("ChangeDirectory(~) should resolve to the home directory, got error: %v", err)
	}
	resolvedHome, _ := filepath.EvalSymlinks(home)
	if sandbox.basePath != resolvedHome {
		t.Errorf("sandbox.basePath = %q, want %q", sandbox.basePath, resolvedHome)
	}
}

func TestChangeDirectoryPreview(t *testing.T) {
	base := t.TempDir()
	preview, err := ChangeDirectoryPreview(changeDirArgs(t, base), base)
	if err != nil {
		t.Fatalf("ChangeDirectoryPreview() error = %v", err)
	}
	resolvedBase, _ := filepath.EvalSymlinks(base)
	if !strings.Contains(preview, resolvedBase) {
		t.Errorf("preview %q should mention the target directory %q", preview, resolvedBase)
	}
}
