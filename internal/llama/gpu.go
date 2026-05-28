package llama

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// GPUType represents the detected GPU vendor.
type GPUType string

const (
	GPUTypeNVIDIA GPUType = "nvidia"
	GPUTypeAMD    GPUType = "amd"
	GPUTypeCPU    GPUType = "cpu"
)

// GPUInfo holds detected GPU information.
type GPUInfo struct {
	Type          GPUType `json:"type"`
	Name          string  `json:"name"`
	VRAM          int     `json:"vram_mb"`         // Total VRAM in MB
	GPULayers     int     `json:"recommended_layers"` // Recommended n_gpu_layers
	Description   string  `json:"description"`
}

// DetectGPU probes the system for available GPU acceleration.
// Priority: NVIDIA (CUDA) → AMD (ROCm) → CPU fallback.
func DetectGPU() GPUInfo {
	if info, ok := detectNVIDIA(); ok {
		return info
	}
	if info, ok := detectAMD(); ok {
		return info
	}
	return GPUInfo{
		Type:        GPUTypeCPU,
		Name:        "CPU",
		VRAM:        0,
		GPULayers:   0,
		Description: "No GPU detected — CPU inference mode",
	}
}

// ─── NVIDIA Detection ────────────────────────────────────────────

func detectNVIDIA() (GPUInfo, bool) {
	// Check if nvidia-smi exists
	path, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return GPUInfo{}, false
	}
	_ = path

	// Get GPU name
	nameOut, err := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return GPUInfo{}, false
	}
	name := strings.TrimSpace(strings.Split(string(nameOut), "\n")[0])

	// Get total VRAM in MiB
	vramOut, err := exec.Command("nvidia-smi", "--query-gpu=memory.total", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return GPUInfo{}, false
	}
	vramStr := strings.TrimSpace(strings.Split(string(vramOut), "\n")[0])
	vram, err := strconv.Atoi(vramStr)
	if err != nil {
		vram = 0
	}

	layers := recommendLayers(vram)

	log.Printf("GPU detected: NVIDIA %s (%d MB VRAM, recommending %d layers)", name, vram, layers)

	return GPUInfo{
		Type:        GPUTypeNVIDIA,
		Name:        fmt.Sprintf("NVIDIA %s", name),
		VRAM:        vram,
		GPULayers:   layers,
		Description: fmt.Sprintf("NVIDIA %s — %d MB VRAM — CUDA acceleration", name, vram),
	}, true
}

// ─── AMD Detection ───────────────────────────────────────────────

func detectAMD() (GPUInfo, bool) {
	// Check for rocm-smi (ROCm)
	path, err := exec.LookPath("rocm-smi")
	if err != nil {
		// Fallback: check /sys/class/drm for AMD GPU
		return detectAMDSysfs()
	}
	_ = path

	// Try to get GPU info from rocm-smi
	out, err := exec.Command("rocm-smi", "--showproductname").Output()
	if err != nil {
		return GPUInfo{}, false
	}
	name := "AMD GPU"
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Card") || strings.Contains(line, "GPU") {
			if idx := strings.Index(line, ":"); idx != -1 {
				name = strings.TrimSpace(line[idx+1:])
				break
			}
		}
	}

	// Try to get VRAM
	vram := 0
	vramOut, err := exec.Command("rocm-smi", "--showmeminfo", "vram").Output()
	if err == nil {
		for _, line := range strings.Split(string(vramOut), "\n") {
			if strings.Contains(line, "Total") {
				fields := strings.Fields(line)
				for i, f := range fields {
					if f == "Total" && i+2 < len(fields) {
						if v, err := strconv.Atoi(fields[i+2]); err == nil {
							vram = v / (1024 * 1024) // bytes to MB
						}
					}
				}
			}
		}
	}

	layers := recommendLayers(vram)

	log.Printf("GPU detected: AMD %s (%d MB VRAM, recommending %d layers)", name, vram, layers)

	return GPUInfo{
		Type:        GPUTypeAMD,
		Name:        name,
		VRAM:        vram,
		GPULayers:   layers,
		Description: fmt.Sprintf("%s — %d MB VRAM — ROCm acceleration", name, vram),
	}, true
}

func detectAMDSysfs() (GPUInfo, bool) {
	if runtime.GOOS != "linux" {
		return GPUInfo{}, false
	}
	// Check /sys/class/drm/card*/device/vendor for AMD vendor ID 0x1002
	vendorOut, err := exec.Command("bash", "-c", "cat /sys/class/drm/card*/device/vendor 2>/dev/null | head -1").Output()
	if err != nil {
		return GPUInfo{}, false
	}

	vendor := strings.TrimSpace(string(vendorOut))
	if vendor != "0x1002" {
		return GPUInfo{}, false
	}

	// Get device ID
	deviceIDOut, err := exec.Command("bash", "-c", "cat /sys/class/drm/card*/device/device 2>/dev/null | head -1").Output()
	if err != nil {
		return GPUInfo{}, false
	}
	deviceID := strings.TrimSpace(string(deviceIDOut))

	// Try to get VRAM from sysfs
	vram := 0
	vramOut, err := exec.Command("bash", "-c", "cat /sys/class/drm/card*/device/mem_info_vram_total 2>/dev/null | head -1").Output()
	if err == nil {
		vramBytes, _ := strconv.ParseUint(strings.TrimSpace(string(vramOut)), 10, 64)
		if vramBytes > 0 {
			vram = int(vramBytes / (1024 * 1024)) // bytes to MB
		}
	}

	layers := recommendLayers(vram)

	log.Printf("GPU detected: AMD device %s via sysfs (VRAM: %d MB, recommending %d layers)", deviceID, vram, layers)

	return GPUInfo{
		Type:        GPUTypeAMD,
		Name:        fmt.Sprintf("AMD GPU (device %s)", deviceID),
		VRAM:        vram,
		GPULayers:   layers,
		Description: fmt.Sprintf("AMD GPU (device %s) — %d MB VRAM — ROCm recommended", deviceID, vram),
	}, true
}

// ─── Layer Recommendation ────────────────────────────────────────

// recommendLayers estimates the optimal n_gpu_layers based on VRAM.
// This is a conservative heuristic for common model sizes.
func recommendLayers(vramMB int) int {
	if vramMB <= 0 {
		return 0
	}

	vramGB := vramMB / 1024

	switch {
	case vramGB >= 24: // RTX 3090/4090, A5000
		return 999 // Offload everything
	case vramGB >= 16: // RTX 4080, A4000
		return 80
	case vramGB >= 12: // RTX 3060 12GB, RTX 4070
		return 48
	case vramGB >= 8: // RTX 3060 8GB, RTX 4060
		return 33
	case vramGB >= 6: // GTX 1660, RTX 2060
		return 20
	case vramGB >= 4:
		return 12
	default:
		return 8
	}
}
