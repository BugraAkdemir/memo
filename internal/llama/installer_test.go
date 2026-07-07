package llama

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.bin")
	dst := filepath.Join(dir, "nested", "dest.bin")
	content := []byte("fake llama-server binary contents")

	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatalf("WriteFile(src): %v", err)
	}
	// copyFile doesn't create the destination directory itself — matches how
	// its only caller (extractFile) always mkdirs the parent first.
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := copyFile(src, dst, 0o755); err != nil {
		t.Fatalf("copyFile: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile(dst): %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("copied content = %q, want %q", got, content)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(dst)
		if err != nil {
			t.Fatalf("Stat(dst): %v", err)
		}
		if info.Mode().Perm() != 0o755 {
			t.Errorf("dest permissions = %v, want 0755", info.Mode().Perm())
		}
	}
}

func TestCopyFile_MissingSource(t *testing.T) {
	dir := t.TempDir()
	err := copyFile(filepath.Join(dir, "does-not-exist"), filepath.Join(dir, "dest"), 0o644)
	if err == nil {
		t.Fatal("expected an error copying a nonexistent source file")
	}
}

func TestHasGPUSupport(t *testing.T) {
	t.Run("CPU always reports supported regardless of installed files", func(t *testing.T) {
		if !HasGPUSupport(filepath.Join(t.TempDir(), "llama-server"), GPUTypeCPU) {
			t.Error("expected GPUTypeCPU to always return true")
		}
	})

	t.Run("finds a matching nvidia backend library", func(t *testing.T) {
		dir := t.TempDir()
		binPath := filepath.Join(dir, "llama-server")
		if err := os.WriteFile(filepath.Join(dir, "libggml-cuda.so"), []byte{}, 0o644); err != nil {
			t.Fatal(err)
		}
		if !HasGPUSupport(binPath, GPUTypeNVIDIA) {
			t.Error("expected libggml-cuda.so to satisfy NVIDIA GPU support")
		}
	})

	t.Run("does not match an unrelated dynamic library", func(t *testing.T) {
		dir := t.TempDir()
		binPath := filepath.Join(dir, "llama-server")
		if err := os.WriteFile(filepath.Join(dir, "libc.so.6"), []byte{}, 0o644); err != nil {
			t.Fatal(err)
		}
		if HasGPUSupport(binPath, GPUTypeNVIDIA) {
			t.Error("expected an unrelated .so to NOT satisfy NVIDIA GPU support")
		}
	})

	t.Run("returns false when the binary's directory doesn't exist", func(t *testing.T) {
		binPath := filepath.Join(t.TempDir(), "missing-dir", "llama-server")
		if HasGPUSupport(binPath, GPUTypeNVIDIA) {
			t.Error("expected false when the directory can't be read")
		}
	})
}

func TestIsInstalled_ConfiguredPathExists(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "llama-server")
	if err := os.WriteFile(binPath, []byte("fake binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	i := NewInstaller(dir)
	if !i.IsInstalled(binPath) {
		t.Error("expected IsInstalled to be true for a configured path that exists on disk")
	}
}

func TestPickBestAsset(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("pickBestAsset's preference table is asserted here for linux/amd64 only")
	}

	t.Run("prefers the CUDA build for an NVIDIA GPU", func(t *testing.T) {
		assets := []githubAsset{
			{Name: "llama-b1-bin-ubuntu-x64.zip"},             // wrong extension for linux (zip, not tar.gz)
			{Name: "llama-b1-bin-macos-arm64.tar.gz"},         // wrong platform
			{Name: "llama-b1-bin-ubuntu-cuda-12.4-x64.tar.gz"}, // best match for NVIDIA
			{Name: "llama-b1-bin-ubuntu-x64.tar.gz"},          // plain CPU build
		}
		got, err := pickBestAsset(assets, GPUInfo{Type: GPUTypeNVIDIA})
		if err != nil {
			t.Fatalf("pickBestAsset: %v", err)
		}
		if got.Name != "llama-b1-bin-ubuntu-cuda-12.4-x64.tar.gz" {
			t.Errorf("got %q, want the cuda build", got.Name)
		}
	})

	t.Run("falls back to the plain build for CPU when no GPU variant is present", func(t *testing.T) {
		assets := []githubAsset{
			{Name: "llama-b1-bin-macos-arm64.tar.gz"}, // wrong platform
			{Name: "llama-b1-bin-ubuntu-x64.tar.gz"},  // only linux asset available
		}
		got, err := pickBestAsset(assets, GPUInfo{Type: GPUTypeCPU})
		if err != nil {
			t.Fatalf("pickBestAsset: %v", err)
		}
		if got.Name != "llama-b1-bin-ubuntu-x64.tar.gz" {
			t.Errorf("got %q, want the plain ubuntu build", got.Name)
		}
	})

	t.Run("errors when nothing matches this platform", func(t *testing.T) {
		onlyWindows := []githubAsset{{Name: "llama-b1-bin-win-cuda-x64.zip"}}
		if _, err := pickBestAsset(onlyWindows, GPUInfo{Type: GPUTypeNVIDIA}); err == nil {
			t.Error("expected an error when no asset matches the current platform")
		}
	})
}
