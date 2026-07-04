package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRemoveCLI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no CLI wrapper concept on windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	localBin := filepath.Join(home, ".local", "bin")
	memoBin := filepath.Join(home, ".memo", "bin")
	if err := os.MkdirAll(localBin, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(memoBin, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localBin, "memo"), []byte("wrapper"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memoBin, "memo"), []byte("binary"), 0755); err != nil {
		t.Fatal(err)
	}

	a := &App{}
	if err := a.RemoveCLI(); err != nil {
		t.Fatalf("RemoveCLI() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(localBin, "memo")); !os.IsNotExist(err) {
		t.Errorf("~/.local/bin/memo should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(memoBin, "memo")); !os.IsNotExist(err) {
		t.Errorf("~/.memo/bin/memo should be removed, stat err = %v", err)
	}
}

func TestRemoveCLI_MissingFilesIsNotError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no CLI wrapper concept on windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	a := &App{}
	if err := a.RemoveCLI(); err != nil {
		t.Fatalf("RemoveCLI() on a clean home should not error, got %v", err)
	}
}

func TestReinstallCLI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no CLI wrapper concept on windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	a := &App{}
	if err := a.ReinstallCLI(); err != nil {
		t.Fatalf("ReinstallCLI() error = %v", err)
	}

	targetBinary := filepath.Join(home, ".memo", "bin", "memo")
	info, err := os.Stat(targetBinary)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", targetBinary, err)
	}
	if info.Size() == 0 {
		t.Error("copied binary is empty")
	}

	wrapperPath := filepath.Join(home, ".local", "bin", "memo")
	wrapperInfo, err := os.Lstat(wrapperPath)
	if err != nil {
		t.Fatalf("expected wrapper at %s: %v", wrapperPath, err)
	}
	if wrapperInfo.Mode()&os.ModeSymlink != 0 {
		t.Error("wrapper should be a regular file, not a symlink")
	}

	content, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatal(err)
	}
	wantDataDir := filepath.Join(home, ".memo", "data")
	if !strings.Contains(string(content), wantDataDir) {
		t.Errorf("wrapper missing MEMO_DATA_DIR=%s, got:\n%s", wantDataDir, content)
	}
	if !strings.Contains(string(content), targetBinary) {
		t.Errorf("wrapper missing exec target %s, got:\n%s", targetBinary, content)
	}
}

// TestReinstallCLI_ReplacesStaleSymlink reproduces the exact corruption bug:
// a pre-existing plain symlink at ~/.local/bin/memo (from before the wrapper
// approach) must be replaced with an independent regular file, not written
// through into whatever it points at.
func TestReinstallCLI_ReplacesStaleSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no CLI wrapper concept on windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	memoBin := filepath.Join(home, ".memo", "bin")
	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(memoBin, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(localBin, 0755); err != nil {
		t.Fatal(err)
	}
	oldReal := filepath.Join(memoBin, "memo")
	if err := os.WriteFile(oldReal, []byte("old real binary content"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(oldReal, filepath.Join(localBin, "memo")); err != nil {
		t.Fatal(err)
	}

	a := &App{}
	if err := a.ReinstallCLI(); err != nil {
		t.Fatalf("ReinstallCLI() error = %v", err)
	}

	info, err := os.Lstat(filepath.Join(localBin, "memo"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("stale symlink should have been replaced with a regular file")
	}

	realContent, err := os.ReadFile(oldReal)
	if err != nil {
		t.Fatal(err)
	}
	if string(realContent) == "old real binary content" {
		t.Error("real binary at ~/.memo/bin/memo should have been overwritten with the freshly copied binary, not left untouched by a corrupting write-through")
	}
}

func TestPreserveMemoryDir(t *testing.T) {
	dataDir := t.TempDir()
	memoryDir := filepath.Join(dataDir, "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memoryDir, "memory.db"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := preserveMemoryDir(dataDir); err != nil {
		t.Fatalf("preserveMemoryDir() error = %v", err)
	}

	if _, err := os.Stat(memoryDir); !os.IsNotExist(err) {
		t.Errorf("original memory dir should be gone, stat err = %v", err)
	}
	backupFile := filepath.Join(home, "memo-memory-backup", "memory.db")
	if content, err := os.ReadFile(backupFile); err != nil {
		t.Fatalf("expected backup at %s: %v", backupFile, err)
	} else if string(content) != "data" {
		t.Errorf("backup content = %q, want %q", content, "data")
	}
}

func TestPreserveMemoryDir_NoMemoryDirIsNotError(t *testing.T) {
	dataDir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := preserveMemoryDir(dataDir); err != nil {
		t.Fatalf("preserveMemoryDir() with no memory dir should not error, got %v", err)
	}
}
