package llama

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// copyFile copies a file from src to dst with the given permissions.
func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// Installer manages the automated compilation and installation of llama-server.
type Installer struct {
	BaseDir string
}

func NewInstaller(baseDir string) *Installer {
	return &Installer{
		BaseDir: baseDir,
	}
}

// CheckPrerequisites verifies that minimum build tools are available.
func (i *Installer) CheckPrerequisites() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed")
	}
	if _, err := exec.LookPath("make"); err != nil {
		return fmt.Errorf("make is not installed")
	}
	if _, err := exec.LookPath("gcc"); err != nil && func() bool { _, e := exec.LookPath("clang"); return e != nil }() {
		return fmt.Errorf("C/C++ compiler (gcc or clang) is not installed")
	}
	return nil
}

// IsInstalled quickly checks if the server is already compiled in our data/bin folder,
// or if the configured system binary exists.
func (i *Installer) IsInstalled(configuredPath string) bool {
	// First check system/configured paths via resolving logic
	if path, err := resolveBinary(configuredPath); err == nil && path != "" {
		return true
	}

	// Then check our managed data/bin location
	targetBin := filepath.Join(i.BaseDir, "bin", "llama-server")
	if _, err := os.Stat(targetBin); err == nil {
		return true
	}

	return false
}

// Install runs the compilation process and streams output via Wails events.
func (i *Installer) Install(ctx context.Context, logger func(string)) (string, error) {
	if err := i.CheckPrerequisites(); err != nil {
		return "", fmt.Errorf("prerequisites missing: %v. Please run: sudo apt install build-essential git", err)
	}

	binDir := filepath.Join(i.BaseDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create bin dir: %w", err)
	}

	workDir := filepath.Join(i.BaseDir, "tmp_llama")
	_ = os.RemoveAll(workDir) // Ensure clean slate
	defer os.RemoveAll(workDir)

	logger("--- Starting automated llama.cpp installation ---")

	// 1. Clone
	logger("1/4 Cloning repository...")
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "https://github.com/ggerganov/llama.cpp.git", workDir)
	if err := i.runCmdStream(cloneCmd, logger); err != nil {
		return "", fmt.Errorf("failed to clone: %w", err)
	}

	// 2. Detect GPU & Configure CMake Flags
	logger("2/4 Detecting GPU capabilities...")
	cmakeArgs := []string{"-B", "build"}
	
	gpuInfo := DetectGPU()
	if gpuInfo.Type == GPUTypeNVIDIA {
		if _, err := exec.LookPath("nvcc"); err == nil {
			logger("NVIDIA GPU detected and nvcc found. Enabling CUDA...")
			cmakeArgs = append(cmakeArgs, "-DGGML_CUDA=ON")
		} else {
			logger("NVIDIA GPU detected, but 'nvcc' not found. Compiling CPU-only version.")
		}
	} else if gpuInfo.Type == GPUTypeAMD {
		if _, err := exec.LookPath("hipcc"); err == nil {
			logger("AMD GPU detected and hipcc found. Enabling ROCm/HIP...")
			cmakeArgs = append(cmakeArgs, "-DGGML_HIP=ON", "-DAMDGPU_TARGETS=gfx1030;gfx1100;gfx1101;gfx1102") 
		} else {
			logger("AMD GPU detected, but 'hipcc' not found. Compiling CPU-only version.")
		}
	} else {
		logger("No dedicated GPU toolkit detected. Compiling standard CPU version.")
	}

	// 3. Compile via CMake
	logger(fmt.Sprintf("3/4 Generating build files (cmake %s)...", strings.Join(cmakeArgs, " ")))
	
	// Check if cmake is installed
	if _, err := exec.LookPath("cmake"); err != nil {
		return "", fmt.Errorf("cmake is not installed. Please install it (e.g. sudo apt install cmake)")
	}

	cmakeGenCmd := exec.CommandContext(ctx, "cmake", cmakeArgs...)
	cmakeGenCmd.Dir = workDir
	if err := i.runCmdStream(cmakeGenCmd, logger); err != nil {
		return "", fmt.Errorf("cmake generate failed: %w", err)
	}

	logger("Compiling llama.cpp (this may take 2-10 minutes)...")
	cmakeBuildCmd := exec.CommandContext(ctx, "cmake", "--build", "build", "--config", "Release", "-j", "4", "--target", "llama-server")
	cmakeBuildCmd.Dir = workDir
	if err := i.runCmdStream(cmakeBuildCmd, logger); err != nil {
		return "", fmt.Errorf("cmake build failed: %w", err)
	}

	// 4. Move Binary and Shared Libraries
	logger("4/4 Finalizing installation...")
	compiledBin := filepath.Join(workDir, "build", "bin", "llama-server")
	compiledLibDir := filepath.Join(workDir, "build", "bin")

	// Sometimes cmake puts it directly in build/ on Linux
	if _, err := os.Stat(compiledBin); err != nil {
		compiledBin = filepath.Join(workDir, "build", "llama-server")
		compiledLibDir = filepath.Join(workDir, "build")
	}

	targetBin := filepath.Join(binDir, "llama-server")

	if _, err := os.Stat(compiledBin); err != nil {
		return "", fmt.Errorf("compiled binary not found at %s", compiledBin)
	}

	// Copy the main binary
	if err := copyFile(compiledBin, targetBin, 0755); err != nil {
		return "", fmt.Errorf("failed to copy binary: %w", err)
	}

	// Copy all shared libraries (.so files) that llama-server depends on
	soFiles, _ := filepath.Glob(filepath.Join(compiledLibDir, "*.so*"))
	// Also check lib/ subdirectory
	soFilesLib, _ := filepath.Glob(filepath.Join(compiledLibDir, "..", "lib", "*.so*"))
	soFiles = append(soFiles, soFilesLib...)
	// Also check src/ subdirectories where ggml libs may be built
	soFilesSrc, _ := filepath.Glob(filepath.Join(workDir, "build", "src", "*.so*"))
	soFiles = append(soFiles, soFilesSrc...)
	soFilesGgml, _ := filepath.Glob(filepath.Join(workDir, "build", "ggml", "src", "*.so*"))
	soFiles = append(soFiles, soFilesGgml...)
	// Check common lib output directories
	soFilesLib2, _ := filepath.Glob(filepath.Join(workDir, "build", "ggml", "src", "**", "*.so*"))
	soFiles = append(soFiles, soFilesLib2...)

	for _, soFile := range soFiles {
		info, err := os.Lstat(soFile)
		if err != nil || info.IsDir() {
			continue
		}
		// Follow symlinks — copy the actual file
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(soFile)
			if err != nil {
				continue
			}
			soFile = resolved
		}
		destSo := filepath.Join(binDir, filepath.Base(soFile))
		if err := copyFile(soFile, destSo, 0755); err != nil {
			logger(fmt.Sprintf("Warning: failed to copy %s: %v", filepath.Base(soFile), err))
		} else {
			logger(fmt.Sprintf("Copied library: %s", filepath.Base(soFile)))
		}
	}

	logger("=== Installation Successful! ===")
	logger(fmt.Sprintf("Binary installed to: %s", targetBin))

	return targetBin, nil
}

// runCmdStream executes a command and streams its stdout/stderr line by line to the logger.
func (i *Installer) runCmdStream(cmd *exec.Cmd, logger func(string)) error {
	stdoutR, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderrR, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Stream stdout
	go func() {
		scanner := bufio.NewScanner(stdoutR)
		for scanner.Scan() {
			logger(scanner.Text())
		}
	}()

	// Stream stderr
	go func() {
		scanner := bufio.NewScanner(stderrR)
		for scanner.Scan() {
			// Some commands write progress to stderr
			logger(scanner.Text())
		}
	}()

	return cmd.Wait()
}

// StreamToFrontend is a helper to wrap the Wails runtime emit.
func StreamToFrontend(ctx context.Context, line string) {
	runtime.EventsEmit(ctx, "llama:install-log", line)
}
