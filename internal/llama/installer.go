package llama

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
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

// Installer manages the automated installation of llama-server.
type Installer struct {
	BaseDir string
}

func NewInstaller(baseDir string) *Installer {
	return &Installer{BaseDir: baseDir}
}

// CheckPrerequisites verifies build tools on Linux/macOS.
// On Windows, pre-built binaries are downloaded — no build tools needed.
func (i *Installer) CheckPrerequisites() error {
	if goruntime.GOOS == "windows" {
		return nil
	}
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed")
	}
	if _, err := exec.LookPath("cmake"); err != nil {
		return fmt.Errorf("cmake is not installed")
	}
	hasGCC := func() bool { _, e := exec.LookPath("gcc"); return e == nil }
	hasClang := func() bool { _, e := exec.LookPath("clang"); return e == nil }
	if !hasGCC() && !hasClang() {
		return fmt.Errorf("C/C++ compiler (gcc or clang) is not installed")
	}
	return nil
}

// IsInstalled quickly checks if llama-server is available.
func (i *Installer) IsInstalled(configuredPath string) bool {
	if path, err := resolveBinary(configuredPath); err == nil && path != "" {
		return true
	}
	targetBin := filepath.Join(i.BaseDir, "bin", llamaServerBinary())
	_, err := os.Stat(targetBin)
	return err == nil
}

// Install downloads pre-built binaries when possible, falls back to source compilation.
// Windows: always downloads (no build tools available).
// Linux/macOS: tries GitHub download first, falls back to cmake compilation.
func (i *Installer) Install(ctx context.Context, logger func(string)) (string, error) {
	if goruntime.GOOS == "windows" {
		return i.installFromRelease(ctx, logger)
	}
	// Try pre-built first — no build tools required
	path, err := i.installFromRelease(ctx, logger)
	if err == nil {
		return path, nil
	}
	logger(fmt.Sprintf("GitHub'dan indirme başarısız (%v), kaynak kodu derlemeye geçiliyor...", err))
	return i.installFromSource(ctx, logger)
}

// ─── Windows: Download pre-built release ─────────────────────────────────────

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// GPU type → preferred asset keywords per platform (ordered best → fallback)
var assetPrefs = map[string]map[GPUType][]string{
	"windows": {
		GPUTypeNVIDIA: {"cuda12", "cuda-12", "cuda11", "cuda", "vulkan", "avx2", "cpu"},
		GPUTypeAMD:    {"vulkan", "avx2", "cpu"},
		GPUTypeCPU:    {"avx2", "cpu", "noavx"},
	},
	"linux": {
		GPUTypeNVIDIA: {"cuda-12", "cuda12", "cuda-11", "cuda", "ubuntu"},
		GPUTypeAMD:    {"rocm", "vulkan", "ubuntu"},
		GPUTypeCPU:    {"ubuntu"},
	},
}

func (i *Installer) installFromRelease(ctx context.Context, logger func(string)) (string, error) {
	logger("--- Memo AI Engine Kurulumu (Windows) ---")
	logger("1/3 GitHub'dan en son sürüm bilgisi alınıyor...")

	rel, err := fetchLatestRelease(ctx)
	if err != nil {
		return "", fmt.Errorf("sürüm bilgisi alınamadı: %w", err)
	}
	logger(fmt.Sprintf("Sürüm: %s (%d paket bulundu)", rel.TagName, len(rel.Assets)))

	gpu := DetectGPU()
	logger(fmt.Sprintf("GPU: %s", gpu.Description))

	asset, err := pickBestAsset(rel.Assets, gpu)
	if err != nil {
		return "", err
	}
	logger(fmt.Sprintf("Seçilen paket: %s", asset.Name))

	// Download zip to temp
	zipPath := filepath.Join(i.BaseDir, "tmp_llama_release.zip")
	defer os.Remove(zipPath)

	logger(fmt.Sprintf("2/3 İndiriliyor: %s", asset.Name))
	if err := downloadFileProgress(ctx, asset.BrowserDownloadURL, zipPath, func(pct int) {
		logger(fmt.Sprintf("  İndirme: %d%%", pct))
	}); err != nil {
		return "", fmt.Errorf("indirme başarısız: %w", err)
	}

	// Extract
	logger("3/3 Dosyalar çıkarılıyor...")
	binDir := filepath.Join(i.BaseDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", err
	}
	if err := extractZipToBin(zipPath, binDir, logger); err != nil {
		return "", fmt.Errorf("çıkarma başarısız: %w", err)
	}

	targetBin := filepath.Join(binDir, "llama-server.exe")
	if _, err := os.Stat(targetBin); err != nil {
		return "", fmt.Errorf("llama-server.exe kurulum sonrası bulunamadı")
	}

	logger("=== Kurulum Başarılı! ===")
	logger(fmt.Sprintf("Konum: %s", targetBin))
	return targetBin, nil
}

func fetchLatestRelease(ctx context.Context) (*githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://api.github.com/repos/ggerganov/llama.cpp/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "memo-app/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API yanıtı: %s", resp.Status)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func pickBestAsset(assets []githubAsset, gpu GPUInfo) (githubAsset, error) {
	os := goruntime.GOOS
	platformPrefs := assetPrefs[os]
	if platformPrefs == nil {
		return githubAsset{}, fmt.Errorf("desteklenmeyen platform: %s", os)
	}

	prefs := platformPrefs[gpu.Type]
	if prefs == nil {
		prefs = platformPrefs[GPUTypeCPU]
	}

	// Platform keyword used to filter assets (win / ubuntu / macos)
	platformKeyword := map[string]string{
		"windows": "win",
		"linux":   "ubuntu",
		"darwin":  "macos",
	}[os]

	for _, pref := range prefs {
		for _, a := range assets {
			name := strings.ToLower(a.Name)
			if strings.Contains(name, platformKeyword) &&
				strings.Contains(name, pref) &&
				strings.HasSuffix(name, ".zip") {
				return a, nil
			}
		}
	}
	// Last resort: any zip for this platform
	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if strings.Contains(name, platformKeyword) && strings.HasSuffix(name, ".zip") {
			return a, nil
		}
	}
	return githubAsset{}, fmt.Errorf("uygun %s paketi bulunamadı", os)
}

func downloadFileProgress(ctx context.Context, url, dest string, progress func(int)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	total := resp.ContentLength
	var downloaded int64
	lastPct := -1
	buf := make([]byte, 64*1024)

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			downloaded += int64(n)
			if total > 0 && progress != nil {
				pct := int(downloaded * 100 / total)
				if pct != lastPct {
					progress(pct)
					lastPct = pct
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}

func extractZipToBin(zipPath, destDir string, logger func(string)) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	extracted := 0
	for _, f := range r.File {
		// Flatten directory structure — only filename matters
		name := filepath.Base(f.Name)
		lname := strings.ToLower(name)

		// Only extract the server binary and required DLLs
		if name != "llama-server.exe" && !strings.HasSuffix(lname, ".dll") {
			continue
		}

		destPath := filepath.Join(destDir, name)
		if err := extractZipEntry(f, destPath); err != nil {
			logger(fmt.Sprintf("Uyarı: %s çıkarılamadı: %v", name, err))
			continue
		}
		logger(fmt.Sprintf("  Çıkarıldı: %s", name))
		extracted++
	}

	if extracted == 0 {
		return fmt.Errorf("zip içinde hiçbir binary bulunamadı")
	}
	return nil
}

func extractZipEntry(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

// ─── Linux/macOS: Compile from source ────────────────────────────────────────

func (i *Installer) installFromSource(ctx context.Context, logger func(string)) (string, error) {
	if err := i.CheckPrerequisites(); err != nil {
		return "", fmt.Errorf("gereksinimler eksik: %v. Lütfen çalıştırın: sudo apt install build-essential git cmake", err)
	}

	binDir := filepath.Join(i.BaseDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("bin dizini oluşturulamadı: %w", err)
	}

	workDir := filepath.Join(i.BaseDir, "tmp_llama")
	_ = os.RemoveAll(workDir)
	defer os.RemoveAll(workDir)

	logger("--- llama.cpp Kurulumu Başlatılıyor ---")

	// 1. Clone
	logger("1/4 Depo klonlanıyor...")
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1",
		"https://github.com/ggerganov/llama.cpp.git", workDir)
	if err := i.runCmdStream(cloneCmd, logger); err != nil {
		return "", fmt.Errorf("klonlama başarısız: %w", err)
	}

	// 2. Detect GPU & Configure CMake Flags
	logger("2/4 GPU algılanıyor...")
	cmakeArgs := []string{"-B", "build"}
	gpuInfo := DetectGPU()
	if gpuInfo.Type == GPUTypeNVIDIA {
		if _, err := exec.LookPath("nvcc"); err == nil {
			logger("NVIDIA GPU + CUDA bulundu, CUDA etkinleştiriliyor...")
			cmakeArgs = append(cmakeArgs, "-DGGML_CUDA=ON")
		} else {
			logger("NVIDIA GPU var ama nvcc yok — CPU sürümü derleniyor.")
		}
	} else if gpuInfo.Type == GPUTypeAMD {
		if _, err := exec.LookPath("hipcc"); err == nil {
			logger("AMD GPU + ROCm bulundu, HIP etkinleştiriliyor...")
			cmakeArgs = append(cmakeArgs, "-DGGML_HIP=ON", "-DAMDGPU_TARGETS=gfx1030;gfx1100;gfx1101;gfx1102")
		} else {
			logger("AMD GPU var ama hipcc yok — CPU sürümü derleniyor.")
		}
	} else {
		logger("Özel GPU bulunamadı — CPU sürümü derleniyor.")
	}

	// 3. CMake generate
	logger(fmt.Sprintf("3/4 Build dosyaları oluşturuluyor (cmake %s)...", strings.Join(cmakeArgs, " ")))
	cmakeGenCmd := exec.CommandContext(ctx, "cmake", cmakeArgs...)
	cmakeGenCmd.Dir = workDir
	if err := i.runCmdStream(cmakeGenCmd, logger); err != nil {
		return "", fmt.Errorf("cmake generate başarısız: %w", err)
	}

	logger("Derleniyor (2-10 dakika sürebilir)...")
	cmakeBuildCmd := exec.CommandContext(ctx, "cmake", "--build", "build",
		"--config", "Release", "-j", "4", "--target", "llama-server")
	cmakeBuildCmd.Dir = workDir
	if err := i.runCmdStream(cmakeBuildCmd, logger); err != nil {
		return "", fmt.Errorf("cmake build başarısız: %w", err)
	}

	// 4. Copy binary and shared libraries
	logger("4/4 Kurulum tamamlanıyor...")
	binName := llamaServerBinary()
	compiledBin := filepath.Join(workDir, "build", "bin", binName)
	compiledLibDir := filepath.Join(workDir, "build", "bin")
	if _, err := os.Stat(compiledBin); err != nil {
		compiledBin = filepath.Join(workDir, "build", binName)
		compiledLibDir = filepath.Join(workDir, "build")
	}
	targetBin := filepath.Join(binDir, binName)
	if _, err := os.Stat(compiledBin); err != nil {
		return "", fmt.Errorf("derlenen binary bulunamadı: %s", compiledBin)
	}
	if err := copyFile(compiledBin, targetBin, 0755); err != nil {
		return "", fmt.Errorf("binary kopyalanamadı: %w", err)
	}

	// Copy shared libraries
	var libFiles []string
	for _, dir := range []string{
		compiledLibDir,
		filepath.Join(compiledLibDir, "..", "lib"),
		filepath.Join(workDir, "build", "src"),
		filepath.Join(workDir, "build", "ggml", "src"),
	} {
		files, _ := filepath.Glob(filepath.Join(dir, "*.so*"))
		libFiles = append(libFiles, files...)
	}
	for _, soFile := range libFiles {
		info, err := os.Lstat(soFile)
		if err != nil || info.IsDir() {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(soFile)
			if err != nil {
				continue
			}
			soFile = resolved
		}
		dest := filepath.Join(binDir, filepath.Base(soFile))
		if err := copyFile(soFile, dest, 0755); err != nil {
			logger(fmt.Sprintf("Uyarı: %s kopyalanamadı: %v", filepath.Base(soFile), err))
		} else {
			logger(fmt.Sprintf("Kütüphane kopyalandı: %s", filepath.Base(soFile)))
		}
	}

	logger("=== Kurulum Başarılı! ===")
	logger(fmt.Sprintf("Konum: %s", targetBin))
	return targetBin, nil
}

// runCmdStream executes a command and streams its output line by line to the logger.
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
	go func() {
		scanner := bufio.NewScanner(stdoutR)
		for scanner.Scan() {
			logger(scanner.Text())
		}
	}()
	go func() {
		scanner := bufio.NewScanner(stderrR)
		for scanner.Scan() {
			logger(scanner.Text())
		}
	}()
	return cmd.Wait()
}

// StreamToFrontend is a helper to wrap the Wails runtime emit.
func StreamToFrontend(ctx context.Context, line string) {
	runtime.EventsEmit(ctx, "llama:install-log", line)
}
