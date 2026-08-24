package tools

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type mockFileSender struct {
	gotCtx         context.Context
	gotFullPath    string
	gotDisplayName string
	gotIsTempFile  bool
	gotZipNames    map[string]bool // entries read from fullPath here, before ShareFile can delete it post-return
	ret            string
	retConsumed    bool
	err            error
}

func (m *mockFileSender) DeliverFile(ctx context.Context, fullPath, displayName string, isTempFile bool) (string, bool, error) {
	m.gotCtx = ctx
	m.gotFullPath = fullPath
	m.gotDisplayName = displayName
	m.gotIsTempFile = isTempFile
	if zr, err := zip.OpenReader(fullPath); err == nil {
		m.gotZipNames = map[string]bool{}
		for _, f := range zr.File {
			m.gotZipNames[f.Name] = true
		}
		zr.Close()
	}
	return m.ret, m.retConsumed, m.err
}

func TestShareFile_EmptyPath(t *testing.T) {
	t.Cleanup(func() { FileSender = nil })
	FileSender = &mockFileSender{}
	args, _ := json.Marshal(map[string]string{"path": "   "})
	if _, err := ShareFile(context.Background(), args, t.TempDir(), nil); err == nil {
		t.Error("ShareFile() with a blank path should error")
	}
}

func TestShareFile_NoFileSenderConfigured(t *testing.T) {
	FileSender = nil
	args, _ := json.Marshal(map[string]string{"path": "whatever.txt"})
	out, err := ShareFile(context.Background(), args, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Error("expected a non-empty 'not ready' message, not an error, when FileSender is nil")
	}
}

func TestShareFile_MissingPath(t *testing.T) {
	t.Cleanup(func() { FileSender = nil })
	FileSender = &mockFileSender{}
	args, _ := json.Marshal(map[string]string{"path": "does-not-exist.txt"})
	if _, err := ShareFile(context.Background(), args, t.TempDir(), nil); err == nil {
		t.Error("ShareFile() for a nonexistent path should error")
	}
}

// TestShareFile_SingleFile_SentAsIs confirms a plain file is handed to
// FileSender exactly as-is — no zip step, display name is just the file's
// own base name — and that the tool's reply is whatever FileSender itself
// returned as the confirmation message.
func TestShareFile_SingleFile_SentAsIs(t *testing.T) {
	t.Cleanup(func() { FileSender = nil })
	dir := t.TempDir()
	filePath := filepath.Join(dir, "report.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	mock := &mockFileSender{ret: "sent!", retConsumed: true}
	FileSender = mock

	args, _ := json.Marshal(map[string]string{"path": "report.txt"})
	out, err := ShareFile(context.Background(), args, dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "sent!" {
		t.Errorf("ShareFile() = %q, want the FileSender's own confirmation message", out)
	}
	if mock.gotDisplayName != "report.txt" {
		t.Errorf("DeliverFile() displayName = %q, want %q", mock.gotDisplayName, "report.txt")
	}
	if mock.gotFullPath != filePath {
		t.Errorf("DeliverFile() fullPath = %q, want %q", mock.gotFullPath, filePath)
	}
	if mock.gotIsTempFile {
		t.Error("isTempFile should be false for the user's own real file, not a temp zip")
	}
	// The real user file must still exist — ShareFile must never delete it.
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("original file should still exist after ShareFile(): %v", err)
	}
}

// TestShareFile_Directory_ZipsFirst confirms a directory is archived into a
// zip (flattened, with the directory's own files at its root) before being
// handed to FileSender, under a "<dirname>.zip" display name.
func TestShareFile_Directory_ZipsFirst(t *testing.T) {
	t.Cleanup(func() { FileSender = nil })
	dir := t.TempDir()
	folder := filepath.Join(dir, "notes")
	if err := os.MkdirAll(folder, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(folder, "a.txt"), []byte("A"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(folder, "b.txt"), []byte("B"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	mock := &mockFileSender{ret: "sent!", retConsumed: true}
	FileSender = mock

	args, _ := json.Marshal(map[string]string{"path": "notes"})
	if _, err := ShareFile(context.Background(), args, dir, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.gotDisplayName != "notes.zip" {
		t.Errorf("displayName = %q, want %q", mock.gotDisplayName, "notes.zip")
	}
	if filepath.Ext(mock.gotFullPath) != ".zip" {
		t.Fatalf("DeliverFile() should receive a .zip path, got %q", mock.gotFullPath)
	}
	if !mock.gotZipNames["a.txt"] || !mock.gotZipNames["b.txt"] {
		t.Errorf("zip entries = %v, want a.txt and b.txt at the root", mock.gotZipNames)
	}
	if !mock.gotIsTempFile {
		t.Error("isTempFile should be true for a zip share_file created itself")
	}

	// consumed=true → the temp zip must be cleaned up after ShareFile returns.
	if _, err := os.Stat(mock.gotFullPath); !os.IsNotExist(err) {
		t.Errorf("temp zip should be deleted once FileSender reports consumed=true, stat err = %v", err)
	}
}

// TestZipDirectory_SkipsSymlinks is the regression test for a real sandbox
// escape: filepath.Walk itself never descends into a symlinked directory,
// but a symlinked *file* entry was still being silently followed by a
// plain os.Open, so a directory shared via share_file could exfiltrate
// anything the backend process can read on disk — e.g. a symlink planted
// inside an agent-writable folder pointing at a file well outside the
// agent sandbox. validatePath's own check only ever covers the one
// top-level path share_file was given, never entries found while
// recursing through it, so this had to be closed in zipDirectory itself.
func TestZipDirectory_SkipsSymlinks(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "notes")
	if err := os.MkdirAll(folder, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(folder, "real.txt"), []byte("real"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	secretPath := filepath.Join(dir, "outside-sandbox-secret.txt")
	if err := os.WriteFile(secretPath, []byte("do not leak me"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Symlink(secretPath, filepath.Join(folder, "sneaky-link.txt")); err != nil {
		t.Skipf("symlinks not supported in this environment: %v", err)
	}

	zipPath, err := zipDirectory(folder)
	if err != nil {
		t.Fatalf("zipDirectory() error = %v", err)
	}
	defer os.Remove(zipPath)

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("resulting archive is not a valid zip: %v", err)
	}
	defer zr.Close()
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names["real.txt"] {
		t.Errorf("zip entries = %v, want the real file included", names)
	}
	if names["sneaky-link.txt"] {
		t.Error("zip should never include a symlink entry — this is a sandbox escape if it does")
	}
}

// TestShareFile_Directory_KeepsTempZipWhenNotConsumed confirms the outbox
// delivery path (consumed=false) is respected — ShareFile must not delete a
// temp zip FileSender is still holding onto for a later download.
func TestShareFile_Directory_KeepsTempZipWhenNotConsumed(t *testing.T) {
	t.Cleanup(func() { FileSender = nil })
	dir := t.TempDir()
	folder := filepath.Join(dir, "notes")
	if err := os.MkdirAll(folder, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(folder, "a.txt"), []byte("A"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	mock := &mockFileSender{ret: "[notes.zip indir](/api/files/outbox/tok)", retConsumed: false}
	FileSender = mock

	args, _ := json.Marshal(map[string]string{"path": "notes"})
	if _, err := ShareFile(context.Background(), args, dir, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(mock.gotFullPath); err != nil {
		t.Errorf("temp zip should still exist when FileSender reports consumed=false, stat err = %v", err)
	}
	os.Remove(mock.gotFullPath) // test cleanup
}

// TestShareFile_TooLarge confirms the shared size cap is enforced before
// handing anything to FileSender, using a sparse file (Truncate, not real
// writes) to stay fast at the exact boundary size.
func TestShareFile_TooLarge(t *testing.T) {
	t.Cleanup(func() { FileSender = nil })
	dir := t.TempDir()
	bigPath := filepath.Join(dir, "huge.bin")
	f, err := os.Create(bigPath)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := f.Truncate(maxShareFileBytes + 1); err != nil {
		f.Close()
		t.Fatalf("setup: %v", err)
	}
	f.Close()

	mock := &mockFileSender{}
	FileSender = mock

	args, _ := json.Marshal(map[string]string{"path": "huge.bin"})
	if _, err := ShareFile(context.Background(), args, dir, nil); err == nil {
		t.Error("ShareFile() for a file over maxShareFileBytes should error")
	}
	if mock.gotFullPath != "" {
		t.Error("FileSender.DeliverFile should never be called for an over-limit file")
	}
}
