package ngrok

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("")
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.apiPort != 4040 {
		t.Errorf("apiPort = %d, want 4040", m.apiPort)
	}
}

func TestNewManager_ExplicitBin(t *testing.T) {
	m := NewManager("/usr/local/bin/ngrok")
	if m.binPath != "/usr/local/bin/ngrok" {
		t.Errorf("binPath = %q, want %q", m.binPath, "/usr/local/bin/ngrok")
	}
}

func TestIsRunning_New(t *testing.T) {
	m := NewManager("")
	if m.IsRunning() {
		t.Error("new manager should not be running")
	}
}

func TestPublicURL_New(t *testing.T) {
	m := NewManager("")
	if url := m.PublicURL(); url != "" {
		t.Errorf("expected empty URL, got %q", url)
	}
}

func TestLastError_New(t *testing.T) {
	m := NewManager("")
	if err := m.LastError(); err != "" {
		t.Errorf("expected empty error, got %q", err)
	}
}

func TestBinaryName(t *testing.T) {
	name := binaryName()
	if runtime.GOOS == "windows" {
		if name != "ngrok.exe" {
			t.Errorf("got %q, want %q", name, "ngrok.exe")
		}
	} else {
		if name != "ngrok" {
			t.Errorf("got %q, want %q", name, "ngrok")
		}
	}
}

func TestFindBinary_ToolPath(t *testing.T) {
	// findBinary returns "ngrok" when nothing is found
	got := findBinary()
	if got == "" {
		t.Error("findBinary returned empty")
	}
}

func TestExtractTGZ(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Write a test binary entry
	hdr := &tar.Header{
		Name: "ngrok",
		Mode: 0755,
		Size: int64(len("binary content")),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tw.Write([]byte("binary content")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	tw.Close()
	gw.Close()

	dir := t.TempDir()
	if err := extractTGZ(&buf, dir); err != nil {
		t.Fatalf("extractTGZ failed: %v", err)
	}

	outPath := filepath.Join(dir, "ngrok")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "binary content" {
		t.Errorf("content = %q, want %q", string(data), "binary content")
	}
}

func TestExtractTGZ_NoBinary(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: "other-file",
		Size: 4,
	}
	tw.WriteHeader(hdr)
	tw.Write([]byte("data"))
	tw.Close()
	gw.Close()

	dir := t.TempDir()
	err := extractTGZ(&buf, dir)
	if err == nil {
		t.Error("expected error when binary not found in archive")
	}
}

func TestExtractTGZ_InvalidArchive(t *testing.T) {
	dir := t.TempDir()
	err := extractTGZ(bytes.NewReader([]byte("not a tar.gz")), dir)
	if err == nil {
		t.Error("expected error for invalid archive")
	}
}

func TestExtractZIP(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	w, err := zw.Create("ngrok")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := w.Write([]byte("zip binary")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	zw.Close()

	dir := t.TempDir()
	if err := extractZIP(&buf, dir); err != nil {
		t.Fatalf("extractZIP failed: %v", err)
	}

	outPath := filepath.Join(dir, "ngrok")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "zip binary" {
		t.Errorf("content = %q, want %q", string(data), "zip binary")
	}
}

func TestExtractZIP_NoBinary(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	zw.Create("other-file")
	zw.Close()

	dir := t.TempDir()
	err := extractZIP(&buf, dir)
	if err == nil {
		t.Error("expected error when binary not found in archive")
	}
}

func TestExtractZIP_InvalidArchive(t *testing.T) {
	dir := t.TempDir()
	err := extractZIP(bytes.NewReader([]byte("not a zip")), dir)
	if err == nil {
		t.Error("expected error for invalid archive")
	}
}

func TestInstall_Existing(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "binaries", runtime.GOOS)
	os.MkdirAll(binDir, 0755)
	binPath := filepath.Join(binDir, binaryName())
	os.WriteFile(binPath, []byte("existing"), 0755)

	got, err := Install(dir)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	if got != binPath {
		t.Errorf("got %q, want %q", got, binPath)
	}
}

func TestInstall_Bundled(t *testing.T) {
	dir := t.TempDir()

	// Create bundled binary in cwd-like dir structure
	cwd, _ := os.Getwd()
	bundledDir := filepath.Join(cwd, "binaries", runtime.GOOS)
	os.MkdirAll(bundledDir, 0755)
	bundledPath := filepath.Join(bundledDir, binaryName())
	os.WriteFile(bundledPath, []byte("bundled"), 0755)
	defer os.RemoveAll(filepath.Join(cwd, "binaries"))

	got, err := Install(dir)
	if err != nil {
		t.Fatalf("Install with bundled failed: %v", err)
	}
	if got == "" {
		t.Error("expected a bundled path")
	}
}

func TestFetchURL_NoServer(t *testing.T) {
	m := NewManager("")
	url, err := m.fetchURL()
	if err == nil {
		t.Error("expected error when no ngrok API server is running")
	}
	if url != "" {
		t.Errorf("expected empty URL, got %q", url)
	}
}

func TestLogWriter(t *testing.T) {
	lw := logWriter{}
	msg := []byte("test log")
	n, err := lw.Write(msg)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(msg) {
		t.Errorf("written = %d, want %d", n, len(msg))
	}
}

func TestDownloadURLs(t *testing.T) {
	key := runtime.GOOS + "/" + runtime.GOARCH
	url, ok := downloadURLs[key]
	if !ok {
		t.Skipf("unsupported platform: %s", key)
	}
	if url == "" {
		t.Error("download URL is empty")
	}
	if !strings.HasPrefix(url, cdnPrefix) {
		t.Errorf("URL %q doesn't start with %q", url, cdnPrefix)
	}
}
