package llama

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestGPUConstants(t *testing.T) {
	if GPUTypeNVIDIA != "nvidia" {
		t.Errorf("GPUTypeNVIDIA = %q, want nvidia", GPUTypeNVIDIA)
	}
	if GPUTypeAMD != "amd" {
		t.Errorf("GPUTypeAMD = %q, want amd", GPUTypeAMD)
	}
	if GPUTypeCPU != "cpu" {
		t.Errorf("GPUTypeCPU = %q, want cpu", GPUTypeCPU)
	}
}

func TestRecommendLayers(t *testing.T) {
	tests := []struct {
		vram int
		want int
	}{
		{0, 0},
		{1 * 1024, 8},    // 1 GB
		{4 * 1024, 12},   // 4 GB
		{6 * 1024, 20},   // 6 GB
		{8 * 1024, 33},   // 8 GB
		{12 * 1024, 48},  // 12 GB
		{16 * 1024, 80},  // 16 GB
		{24 * 1024, 999}, // 24 GB — offload everything
		{48 * 1024, 999}, // 48 GB
		{3 * 1024, 8},    // 3 GB
		{5 * 1024, 12},   // 5 GB
		{7 * 1024, 20},   // 7 GB
		{10 * 1024, 33},  // 10 GB
		{14 * 1024, 48},  // 14 GB
		{20 * 1024, 80},  // 20 GB
	}
	for _, tt := range tests {
		got := recommendLayers(tt.vram)
		if got != tt.want {
			t.Errorf("recommendLayers(%d) = %d, want %d", tt.vram, got, tt.want)
		}
	}
}

func TestReadSysfsFile(t *testing.T) {
	dir := t.TempDir()
	// Create a fake sysfs-like structure
	os.MkdirAll(filepath.Join(dir, "device"), 0755)
	os.WriteFile(filepath.Join(dir, "device", "vendor"), []byte("0x1002\n"), 0644)

	got, err := readSysfsFile(filepath.Join(dir, "device", "vendor"))
	if err != nil {
		t.Fatalf("readSysfsFile() error = %v", err)
	}
	if got != "0x1002" {
		t.Errorf("readSysfsFile() = %q, want 0x1002", got)
	}
}

func TestReadSysfsFileNoMatch(t *testing.T) {
	dir := t.TempDir()
	_, err := readSysfsFile(filepath.Join(dir, "nonexistent", "*"))
	if err == nil {
		t.Error("readSysfsFile() should error on no match")
	}
}

func TestDetectGPUForceCPU(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "data"), 0755)
	os.WriteFile(filepath.Join(dir, "data", ".force_cpu"), []byte{}, 0644)

	// Change to temp dir temporarily so DetectGPU picks up .force_cpu
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	info := DetectGPU()
	if info.Type != GPUTypeCPU {
		t.Errorf("GPU type = %q, want cpu (force)", info.Type)
	}
}

func TestGPUInfoStruct(t *testing.T) {
	info := GPUInfo{
		Type:        GPUTypeNVIDIA,
		Name:        "NVIDIA RTX 4090",
		VRAM:        24576,
		GPULayers:   999,
		Description: "NVIDIA RTX 4090 — 24576 MB VRAM",
	}
	if string(info.Type) != "nvidia" {
		t.Errorf("Type = %q", info.Type)
	}
	if info.VRAM != 24576 {
		t.Errorf("VRAM = %d", info.VRAM)
	}
	if info.GPULayers != 999 {
		t.Errorf("GPULayers = %d", info.GPULayers)
	}
}

func TestWithPrependedEnvPathReplacesExistingValue(t *testing.T) {
	env := []string{
		"HOME=/tmp/memo",
		"LD_LIBRARY_PATH=/old/lib:/another/lib",
	}

	got := withPrependedEnvPath(env, "LD_LIBRARY_PATH", "/memo/binaries/linux/cpu", false)

	if countEnvKey(got, "LD_LIBRARY_PATH", false) != 1 {
		t.Fatalf("LD_LIBRARY_PATH entries = %d, want 1: %#v", countEnvKey(got, "LD_LIBRARY_PATH", false), got)
	}
	value := envValue(t, got, "LD_LIBRARY_PATH", false)
	if !strings.HasPrefix(value, "/memo/binaries/linux/cpu"+string(os.PathListSeparator)) {
		t.Fatalf("LD_LIBRARY_PATH = %q, want binary dir first", value)
	}
}

func TestWithPrependedEnvPathRemovesDuplicateKeys(t *testing.T) {
	env := []string{
		"LD_LIBRARY_PATH=/first",
		"LD_LIBRARY_PATH=/second",
	}

	got := withPrependedEnvPath(env, "LD_LIBRARY_PATH", "/memo/bin", false)

	if countEnvKey(got, "LD_LIBRARY_PATH", false) != 1 {
		t.Fatalf("LD_LIBRARY_PATH entries = %d, want 1: %#v", countEnvKey(got, "LD_LIBRARY_PATH", false), got)
	}
	if value := envValue(t, got, "LD_LIBRARY_PATH", false); value != "/memo/bin"+string(os.PathListSeparator)+"/first" {
		t.Fatalf("LD_LIBRARY_PATH = %q", value)
	}
}

func TestWithPrependedEnvPathAppendsMissingPath(t *testing.T) {
	env := []string{"HOME=/tmp/memo"}

	got := withPrependedEnvPath(env, "LD_LIBRARY_PATH", "/memo/bin", false)

	if value := envValue(t, got, "LD_LIBRARY_PATH", false); value != "/memo/bin" {
		t.Fatalf("LD_LIBRARY_PATH = %q", value)
	}
}

// TestBinarySearchBasesFrom_IncludesParentOfExeDir is a regression test: the
// installed CLI binary lives at ~/.memo/bin/memo, one level deeper than the
// bundled binaries/ tree it ships next to (~/.memo/binaries/...). Before this
// fix, only "." and the exe's own directory were searched, so resolveBinary
// never found llama-server when running as the CLI — only the GUI/AppImage
// binary, which sits flush with binaries/, worked.
func TestBinarySearchBasesFrom_IncludesParentOfExeDir(t *testing.T) {
	exePath := filepath.Join("/home/user/.memo/bin", "memo")

	bases := binarySearchBasesFrom(exePath)

	wantExeDir := filepath.Join("/home/user/.memo/bin")
	wantParent := filepath.Join("/home/user/.memo")
	if !slices.Contains(bases, wantExeDir) {
		t.Errorf("bases = %v, want to contain exe dir %q", bases, wantExeDir)
	}
	if !slices.Contains(bases, wantParent) {
		t.Errorf("bases = %v, want to contain parent dir %q", bases, wantParent)
	}
}

func countEnvKey(env []string, key string, caseInsensitive bool) int {
	count := 0
	for _, entry := range env {
		entryKey, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if entryKey == key || (caseInsensitive && strings.EqualFold(entryKey, key)) {
			count++
		}
	}
	return count
}

func TestExtractModelName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "", want: ""},
		{path: "gemma-3-4b-it.gguf", want: "gemma-3-4b-it"},
		{path: "/data/models/Qwen2.5-7B-Instruct-Q4_K_M.gguf", want: "Qwen2.5-7B-Instruct-Q4_K_M"},
		// No .gguf suffix — nothing to trim, basename is returned as-is.
		{path: "/data/models/no-extension", want: "no-extension"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractModelName(tt.path)
			if got != tt.want {
				t.Errorf("extractModelName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestFindMmproj(t *testing.T) {
	t.Run("finds a sibling mmproj gguf file", func(t *testing.T) {
		dir := t.TempDir()
		modelPath := filepath.Join(dir, "gemma-3-4b.gguf")
		mmprojPath := filepath.Join(dir, "mmproj-gemma-3-4b.gguf")
		for _, p := range []string{modelPath, mmprojPath} {
			if err := os.WriteFile(p, []byte("fake"), 0o644); err != nil {
				t.Fatalf("WriteFile(%s): %v", p, err)
			}
		}

		got := findMmproj(modelPath)
		if got != mmprojPath {
			t.Errorf("findMmproj = %q, want %q", got, mmprojPath)
		}
	})

	t.Run("ignores a same-named file that isn't a gguf", func(t *testing.T) {
		dir := t.TempDir()
		modelPath := filepath.Join(dir, "model.gguf")
		if err := os.WriteFile(modelPath, []byte("fake"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "mmproj-notes.txt"), []byte("fake"), 0o644); err != nil {
			t.Fatal(err)
		}

		if got := findMmproj(modelPath); got != "" {
			t.Errorf("findMmproj = %q, want empty (no matching .gguf)", got)
		}
	})

	t.Run("returns empty when the directory has no projector file", func(t *testing.T) {
		dir := t.TempDir()
		modelPath := filepath.Join(dir, "model.gguf")
		if err := os.WriteFile(modelPath, []byte("fake"), 0o644); err != nil {
			t.Fatal(err)
		}

		if got := findMmproj(modelPath); got != "" {
			t.Errorf("findMmproj = %q, want empty", got)
		}
	})

	t.Run("returns empty for a directory that doesn't exist", func(t *testing.T) {
		if got := findMmproj(filepath.Join(t.TempDir(), "missing", "model.gguf")); got != "" {
			t.Errorf("findMmproj = %q, want empty", got)
		}
	})
}

func envValue(t *testing.T, env []string, key string, caseInsensitive bool) string {
	t.Helper()
	for _, entry := range env {
		entryKey, entryValue, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if entryKey == key || (caseInsensitive && strings.EqualFold(entryKey, key)) {
			return entryValue
		}
	}
	t.Fatalf("%s not found in %#v", key, env)
	return ""
}
