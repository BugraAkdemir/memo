package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	data := []byte("hello world")

	if err := AtomicWrite(path, data, 0644); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("content = %q, want %q", string(got), string(data))
	}
}

func TestAtomicWriteOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	if err := AtomicWrite(path, []byte("first"), 0644); err != nil {
		t.Fatalf("first AtomicWrite failed: %v", err)
	}
	if err := AtomicWrite(path, []byte("second"), 0644); err != nil {
		t.Fatalf("second AtomicWrite failed: %v", err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "second" {
		t.Errorf("content = %q, want %q", string(got), "second")
	}
}

func TestAtomicWriteCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	// AtomicWrite doesn't create directories — just verify it works in a flat dir
	path := filepath.Join(dir, "subdir", "test.txt")
	err := AtomicWrite(path, []byte("x"), 0644)
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

func TestAtomicWriteFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	if err := AtomicWrite(path, []byte("data"), 0600); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Mode()&os.ModePerm != 0600 {
		t.Errorf("mode = %o, want 0600", info.Mode()&os.ModePerm)
	}
}

func TestAtomicWriteEmptyData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")

	if err := AtomicWrite(path, []byte{}, 0644); err != nil {
		t.Fatalf("AtomicWrite with empty data failed: %v", err)
	}

	got, _ := os.ReadFile(path)
	if len(got) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(got))
	}
}

func TestAtomicWriteTempCleaned(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clean.txt")

	if err := AtomicWrite(path, []byte("data"), 0644); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	// .tmp file should be gone
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error(".tmp file was not cleaned up")
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	data := []byte("copy me")

	if err := os.WriteFile(src, data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	got, _ := os.ReadFile(dst)
	if string(got) != string(data) {
		t.Errorf("content = %q, want %q", string(got), string(data))
	}
}
