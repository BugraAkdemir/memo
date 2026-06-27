package cloudsync

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestArchiveIncludesSQLiteWALSidecars verifies that SQLite databases using
// WAL mode are archived. After a WAL checkpoint (TRUNCATE), committed data is
// flushed to the main DB and sidecar files may be removed — so only the main
// DB is required; -wal/-shm are optional.
func TestArchiveIncludesSQLiteWALSidecars(t *testing.T) {
	tmp := t.TempDir()
	persistDir := filepath.Join(tmp, "memory")
	dataDir := filepath.Join(tmp, "data")
	if err := os.MkdirAll(persistDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Memory store files (WAL mode)
	writeFile(t, filepath.Join(persistDir, "memory.db"), []byte("main db"))
	writeFile(t, filepath.Join(persistDir, "memory.db-wal"), []byte("wal data"))
	writeFile(t, filepath.Join(persistDir, "memory.db-shm"), []byte("shm data"))

	// Mood DB files (WAL mode)
	if err := os.MkdirAll(filepath.Join(dataDir, "mood"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dataDir, "mood", "mood.db"), []byte("mood db"))
	writeFile(t, filepath.Join(dataDir, "mood", "mood.db-wal"), []byte("mood wal"))
	// mood.db-shm intentionally missing to verify missing sidecars are ignored.

	// A session file so archive() does not fail with "no data files".
	writeFile(t, filepath.Join(dataDir, "sessions", "s1.json"), []byte("{}"))

	m := &Manager{
		ctx:        context.Background(),
		persistDir: persistDir,
		dataDir:    dataDir,
		passphrase: "test",
	}

	zipData, err := m.archive()
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}

	// Main DB must always be present; WAL/SHM are optional after TRUNCATE checkpoint.
	required := map[string]bool{
		"memory/memory.db": false,
	}

	for _, f := range zr.File {
		if _, ok := required[f.Name]; ok {
			required[f.Name] = true
		}
	}

	for name, found := range required {
		if !found {
			t.Errorf("expected %s in archive", name)
		}
	}
}

// TestRestoreExtractsWALSidecars verifies that restoreZip writes the SQLite
// sidecar files back to the correct locations alongside the main DB.
func TestRestoreExtractsWALSidecars(t *testing.T) {
	tmp := t.TempDir()
	persistDir := filepath.Join(tmp, "memory")
	dataDir := filepath.Join(tmp, "data")
	if err := os.MkdirAll(persistDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "mood"), 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string][]byte{
		"memory/memory.db":     []byte("main db"),
		"memory/memory.db-wal": []byte("wal data"),
		"memory/memory.db-shm": []byte("shm data"),
		"mood/mood.db":         []byte("mood db"),
		"mood/mood.db-wal":     []byte("mood wal"),
	}
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	m := &Manager{
		ctx:        context.Background(),
		persistDir: persistDir,
		dataDir:    dataDir,
		passphrase: "test",
	}

	if err := m.restoreZip(buf.Bytes()); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	expectedPaths := []string{
		filepath.Join(persistDir, "memory.db"),
		filepath.Join(persistDir, "memory.db-wal"),
		filepath.Join(persistDir, "memory.db-shm"),
		filepath.Join(dataDir, "mood", "mood.db"),
		filepath.Join(dataDir, "mood", "mood.db-wal"),
	}
	for _, p := range expectedPaths {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected restored file %s: %v", p, err)
		}
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
