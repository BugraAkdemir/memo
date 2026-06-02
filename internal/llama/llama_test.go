package llama

import (
	"os"
	"path/filepath"
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
		{1 * 1024, 8},     // 1 GB
		{4 * 1024, 12},    // 4 GB
		{6 * 1024, 20},    // 6 GB
		{8 * 1024, 33},    // 8 GB
		{12 * 1024, 48},   // 12 GB
		{16 * 1024, 80},   // 16 GB
		{24 * 1024, 999},  // 24 GB — offload everything
		{48 * 1024, 999},  // 48 GB
		{3 * 1024, 8},     // 3 GB
		{5 * 1024, 12},    // 5 GB
		{7 * 1024, 20},    // 7 GB
		{10 * 1024, 33},   // 10 GB
		{14 * 1024, 48},   // 14 GB
		{20 * 1024, 80},   // 20 GB
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
