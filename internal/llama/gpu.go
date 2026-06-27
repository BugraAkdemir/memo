package llama

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"memo/internal/config"
)

// GPUType represents the detected GPU vendor.
type GPUType string

const (
	GPUTypeNVIDIA GPUType = "nvidia"
	GPUTypeAMD    GPUType = "amd"
	GPUTypeMetal  GPUType = "metal"
	GPUTypeCPU    GPUType = "cpu"
)

// GPUInfo holds detected GPU information.
type GPUInfo struct {
	Type        GPUType `json:"type"`
	Name        string  `json:"name"`
	VRAM        int     `json:"vram_mb"`            // Total VRAM in MB
	GPULayers   int     `json:"recommended_layers"` // Recommended n_gpu_layers
	Description string  `json:"description"`
	RAMTotalMb  int     `json:"ram_total_mb"` // Total system RAM in MB (0 if unknown)
}

// DetectGPU probes the system for available GPU acceleration and system RAM.
func DetectGPU() GPUInfo {
	info := detectGPUInner()
	info.RAMTotalMb = systemRAMMb()
	return info
}

// detectGPUInner probes for GPU acceleration only.
// Priority: NVIDIA (CUDA) → AMD (ROCm) → Apple Silicon (Metal) → CPU fallback.
func detectGPUInner() GPUInfo {
	// Check for manual CPU override
	if _, err := os.Stat(config.DataPath(".force_cpu")); err == nil {
		return GPUInfo{
			Type:        GPUTypeCPU,
			Name:        "CPU (Zorunlu)",
			VRAM:        0,
			GPULayers:   0,
			Description: "Manual override via .force_cpu file",
		}
	}

	if info, ok := detectNVIDIA(); ok {
		return info
	}
	if info, ok := detectAMD(); ok {
		return info
	}
	if info, ok := detectAppleSilicon(); ok {
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
	// Check if nvidia-smi exists (also check common Windows paths)
	nvidiaSmi := "nvidia-smi"
	if runtime.GOOS == "windows" {
		winPaths := []string{
			`C:\Program Files\NVIDIA Corporation\NVSMI\nvidia-smi.exe`,
			`C:\Program Files\NVIDIA Corporation\NVSMI\nvidia-smi`,
		}
		for _, p := range winPaths {
			if _, err := os.Stat(p); err == nil {
				nvidiaSmi = p
				break
			}
		}
	}
	_, err := exec.LookPath(nvidiaSmi)
	if err != nil {
		log.Printf("GPU: nvidia-smi not found: %v", err)
		return GPUInfo{}, false
	}

	// Get GPU name
	nameOut, err := exec.Command(nvidiaSmi, "--query-gpu=name", "--format=csv,noheader,nounits").Output()
	if err != nil {
		log.Printf("GPU: nvidia-smi name query failed: %v", err)
		return GPUInfo{}, false
	}
	name := strings.TrimSpace(strings.Split(string(nameOut), "\n")[0])

	// Get total VRAM in MiB
	vramOut, err := exec.Command(nvidiaSmi, "--query-gpu=memory.total", "--format=csv,noheader,nounits").Output()
	if err != nil {
		log.Printf("GPU: nvidia-smi VRAM query failed: %v", err)
		return GPUInfo{}, false
	}
	vramStr := strings.TrimSpace(strings.Split(string(vramOut), "\n")[0])
	vram, err := strconv.Atoi(vramStr)
	if err != nil {
		log.Printf("GPU: failed to parse VRAM value %q: %v", vramStr, err)
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
	// Windows: use WMI via PowerShell to detect AMD GPUs
	if runtime.GOOS == "windows" {
		if info, ok := detectAMDWindows(); ok {
			return info, true
		}
		return GPUInfo{}, false
	}

	// Check for rocm-smi (ROCm)
	_, err := exec.LookPath("rocm-smi")
	if err != nil {
		// Fallback: check /sys/class/drm for AMD GPU
		return detectAMDSysfs()
	}

	// Parse GPU name — look specifically for "Card Series:" which is the
	// human-readable model name in all known rocm-smi versions.
	name := "AMD GPU"
	nameOut, err := exec.Command("rocm-smi", "--showproductname").Output()
	if err != nil {
		log.Printf("GPU: rocm-smi --showproductname failed: %v", err)
		return GPUInfo{}, false
	}
	for _, line := range strings.Split(string(nameOut), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Card Series:") {
			// Format: "GPU[0]          : Card Series: Navi 21 GL XL [Radeon PRO W6800]"
			// Use LastIndex to skip the "GPU[0] :" prefix colon.
			if idx := strings.LastIndex(line, ":"); idx != -1 {
				if candidate := strings.TrimSpace(line[idx+1:]); candidate != "" {
					name = candidate
					break
				}
			}
		}
	}

	// Parse VRAM — rocm-smi reports bytes, not MB.
	// Expected line: "GPU[0]          : VRAM Total Memory (B): 8589934592"
	vram := 0
	vramOut, err := exec.Command("rocm-smi", "--showmeminfo", "vram").Output()
	if err == nil {
		for _, line := range strings.Split(string(vramOut), "\n") {
			if strings.Contains(line, "VRAM Total Memory (B):") {
				// Split on ":" keeping at most 3 parts to handle the GPU[0] prefix.
				parts := strings.SplitN(line, ":", 3)
				if len(parts) == 3 {
					if v, err := strconv.ParseUint(strings.TrimSpace(parts[2]), 10, 64); err == nil {
						vram = int(v / (1024 * 1024)) // bytes → MB
						break
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

// ─── Windows AMD Detection ──────────────────────────────────────────

func detectAMDWindows() (GPUInfo, bool) {
	// Use PowerShell to query WMI for video controllers
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"Get-CimInstance Win32_VideoController | Select-Object Name, AdapterRAM | ConvertTo-Csv -NoHeader")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("GPU: PowerShell WMI query failed: %v", err)
		return GPUInfo{}, false
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// CSV format: "Name","AdapterRAM"
		parts := strings.Split(line, ",")
		if len(parts) < 1 {
			continue
		}
		name := strings.Trim(parts[0], "\" ")
		if !strings.Contains(strings.ToLower(name), "amd") &&
			!strings.Contains(strings.ToLower(name), "radeon") &&
			!strings.Contains(strings.ToLower(name), "advanced micro devices") {
			continue
		}

		vram := 0
		if len(parts) >= 2 {
			vramStr := strings.Trim(parts[1], "\" ")
			if v, err := strconv.ParseInt(vramStr, 10, 64); err == nil {
				vram = int(v / (1024 * 1024)) // bytes → MB
			}
		}

		layers := recommendLayers(vram)
		log.Printf("GPU detected: AMD %s (%d MB VRAM, recommending %d layers)", name, vram, layers)
		return GPUInfo{
			Type:        GPUTypeAMD,
			Name:        name,
			VRAM:        vram,
			GPULayers:   layers,
			Description: fmt.Sprintf("%s — %d MB VRAM — DirectML/Vulkan acceleration", name, vram),
		}, true
	}

	return GPUInfo{}, false
}

// readSysfsFile reads the first matching sysfs file via a glob pattern.
func readSysfsFile(pattern string) (string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", fmt.Errorf("no match for %s", pattern)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// detectAMDLspci tries to detect AMD GPUs via lspci (works in most containers).
func detectAMDLspci() (GPUInfo, bool) {
	out, err := exec.Command("lspci").Output()
	if err != nil {
		return GPUInfo{}, false
	}
	for _, line := range strings.Split(string(out), "\n") {
		// Look for AMD/ATI VGA or 3D controller
		if !strings.Contains(line, "AMD") && !strings.Contains(line, "ATI") && !strings.Contains(line, "Advanced Micro Devices") {
			continue
		}
		if !strings.Contains(line, "VGA") && !strings.Contains(line, "3D") {
			continue
		}
		name := strings.TrimSpace(line)
		// Try to get VRAM from sysfs
		vram := 0
		matches, _ := filepath.Glob("/sys/class/drm/card*/device/mem_info_vram_total")
		if len(matches) > 0 {
			if data, err := os.ReadFile(matches[0]); err == nil {
				log.Printf("GPU: lspci found AMD: %s", name)
				if v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err == nil {
					vram = int(v / (1024 * 1024)) // bytes → MB
				}
			}
		}
		if vram <= 0 {
			log.Printf("GPU: lspci found AMD: %s (VRAM unknown)", name)
		}
		layers := recommendLayers(vram)
		return GPUInfo{
			Type:        GPUTypeAMD,
			Name:        name,
			VRAM:        vram,
			GPULayers:   layers,
			Description: fmt.Sprintf("%s — %d MB VRAM — ROCm acceleration", name, vram),
		}, true
	}
	return GPUInfo{}, false
}

func detectAMDSysfs() (GPUInfo, bool) {
	if runtime.GOOS != "linux" {
		return GPUInfo{}, false
	}

	// Try lspci first (more portable across containers/environments)
	if info, ok := detectAMDLspci(); ok {
		return info, true
	}

	// Fallback: /sys/class/drm
	vendor, err := readSysfsFile("/sys/class/drm/*/device/vendor")
	if err != nil || vendor != "0x1002" {
		return GPUInfo{}, false
	}

	deviceID, err := readSysfsFile("/sys/class/drm/*/device/device")
	if err != nil {
		return GPUInfo{}, false
	}

	// Read VRAM via sysfs — pure Go, no bash dependency.
	vram := 0
	sysfsVRAM, _ := filepath.Glob("/sys/class/drm/card*/device/mem_info_vram_total")
	for _, match := range sysfsVRAM {
		if data, err := os.ReadFile(match); err == nil {
			if v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err == nil && v > 0 {
				vram = int(v / (1024 * 1024)) // bytes → MB
				break
			}
		}
	}

	// Read VRAM via sysfs — pure Go, no bash dependency.
	sysfsMatches, _ := filepath.Glob("/sys/class/drm/card*/device/mem_info_vram_total")
	for _, match := range sysfsMatches {
		if data, err := os.ReadFile(match); err == nil {
			if v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err == nil && v > 0 {
				vram = int(v / (1024 * 1024)) // bytes → MB
				break
			}
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

// ─── Apple Silicon (Metal) Detection ──────────────────────────────

func detectAppleSilicon() (GPUInfo, bool) {
	if runtime.GOOS != "darwin" {
		return GPUInfo{}, false
	}

	// Check for Apple Silicon via sysctl
	out, err := exec.Command("sysctl", "-n", "hw.optional.arm64").Output()
	if err != nil || strings.TrimSpace(string(out)) != "1" {
		log.Printf("GPU: not Apple Silicon (sysctl hw.optional.arm64: %v)", err)
		return GPUInfo{}, false
	}

	// Get chip name (e.g. "Apple M3 Pro")
	chipOut, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output()
	chipName := "Apple Silicon"
	if err == nil {
		if c := strings.TrimSpace(string(chipOut)); c != "" {
			chipName = c
		}
	}

	// Get total unified memory in MB
	ramOut, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	ramMB := 0
	if err == nil {
		if v, parseErr := strconv.ParseInt(strings.TrimSpace(string(ramOut)), 10, 64); parseErr == nil {
			ramMB = int(v / (1024 * 1024))
		}
	}

	// Apple Silicon uses unified memory — offload all layers
	// Models up to ~30B params fit comfortably in 16GB+ unified memory.
	log.Printf("GPU detected: %s (%d MB unified memory — Metal acceleration)", chipName, ramMB)

	return GPUInfo{
		Type:        GPUTypeMetal,
		Name:        chipName,
		VRAM:        ramMB,
		GPULayers:   999,
		Description: fmt.Sprintf("%s — %d MB Unified Memory — Metal acceleration", chipName, ramMB),
	}, true
}

// ─── Layer Recommendation ────────────────────────────────────────

// recommendLayers estimates the optimal n_gpu_layers based on VRAM.
// Uses float division so that e.g. 6143 MB (≈6 GB) hits the ≥6 GB bucket
// instead of being truncated to 5 GB by integer division.
func recommendLayers(vramMB int) int {
	if vramMB <= 0 {
		return 0
	}

	vramGB := float64(vramMB) / 1024.0

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
