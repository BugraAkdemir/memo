package llama

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"
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

// HasGPUSupport checks if the installed binaries in binPath's directory contain GPU dynamic libraries
func HasGPUSupport(binPath string, gpuType GPUType) bool {
	if gpuType == GPUTypeCPU {
		return true
	}
	dir := filepath.Dir(binPath)
	files, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	gType := strings.ToLower(string(gpuType))

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := strings.ToLower(file.Name())
		// Look for ggml backend dynamic libraries
		// e.g., libggml-cuda.so, ggml-cuda.dll, libggml-vulkan.so, ggml-vulkan.dll, libggml-hip.so, etc.
		if strings.Contains(name, "ggml") && (strings.HasSuffix(name, ".so") || strings.HasSuffix(name, ".dll") || strings.Contains(name, ".so.")) {
			if gType == "nvidia" && (strings.Contains(name, "cuda") || strings.Contains(name, "vulkan")) {
				return true
			}
			if gType == "amd" && (strings.Contains(name, "hip") || strings.Contains(name, "rocm") || strings.Contains(name, "vulkan")) {
				return true
			}
		}
	}
	return false
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

// IsInstalled quickly checks if llama-server is available and compatible with the system's GPU.
func (i *Installer) IsInstalled(configuredPath string) bool {
	// 1. First, try to resolve via standard logic (which now includes bundled paths)
	if path, err := resolveBinary(configuredPath, ""); err == nil && path != "" {
		// If it's a bundled binary, we consider it "installed" regardless of dynamic GPU checks
		// because the user/bundler is responsible for providing the right one.
		normalized := filepath.ToSlash(path)
		if strings.Contains(normalized, "binaries/") || strings.HasPrefix(normalized, "bin/") {
			return true
		}
		
		_, err := os.Stat(path)
		if err == nil {
			return true
		}
	}

	return false
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
	"darwin": {
		GPUTypeNVIDIA: {"macos"},
		GPUTypeAMD:    {"macos"},
		GPUTypeCPU:    {"macos"},
	},
}

func (i *Installer) installFromRelease(ctx context.Context, logger func(string)) (string, error) {
	logger("--- Memo AI Engine Kurulumu ---")
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

	// Determine archive type and temp path
	isTarGz := strings.HasSuffix(strings.ToLower(asset.Name), ".tar.gz")
	tempExt := ".zip"
	if isTarGz {
		tempExt = ".tar.gz"
	}
	tempPath := filepath.Join(os.TempDir(), "memo-llama-release"+tempExt)
	defer os.Remove(tempPath)

	logger(fmt.Sprintf("2/3 İndiriliyor: %s", asset.Name))
	if err := downloadFileProgress(ctx, asset.BrowserDownloadURL, tempPath, func(pct int) {
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

	if isTarGz {
		if err := extractTarGzToBin(tempPath, binDir, logger); err != nil {
			return "", fmt.Errorf("çıkarma başarısız: %w", err)
		}
	} else {
		if err := extractZipToBin(tempPath, binDir, logger); err != nil {
			return "", fmt.Errorf("çıkarma başarısız: %w", err)
		}
	}

	targetBin := filepath.Join(binDir, llamaServerBinary())
	if _, err := os.Stat(targetBin); err != nil {
		return "", fmt.Errorf("%s kurulum sonrası bulunamadı", llamaServerBinary())
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

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
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
	currentOS := goruntime.GOOS
	platformPrefs := assetPrefs[currentOS]
	if platformPrefs == nil {
		return githubAsset{}, fmt.Errorf("desteklenmeyen platform: %s", currentOS)
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
	}[currentOS]

	// Architecture keyword
	archKeyword := goruntime.GOARCH
	if archKeyword == "amd64" {
		archKeyword = "x64"
	}

	// Windows uses .zip, Linux/macOS use .tar.gz
	wantZip := currentOS == "windows"

	matchesExt := func(name string) bool {
		if wantZip {
			return strings.HasSuffix(name, ".zip")
		}
		return strings.HasSuffix(name, ".tar.gz")
	}

	for _, pref := range prefs {
		for _, a := range assets {
			name := strings.ToLower(a.Name)
			if strings.Contains(name, platformKeyword) &&
				strings.Contains(name, archKeyword) &&
				strings.Contains(name, pref) &&
				matchesExt(name) {
				return a, nil
			}
		}
	}
	// Fallback without preference but with architecture
	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if strings.Contains(name, platformKeyword) &&
			strings.Contains(name, archKeyword) &&
			matchesExt(name) {
			return a, nil
		}
	}
	// Last resort: any matching archive for this platform
	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if strings.Contains(name, platformKeyword) && matchesExt(name) {
			return a, nil
		}
	}
	return githubAsset{}, fmt.Errorf("uygun %s paketi bulunamadı", currentOS)
}

func downloadFileProgress(ctx context.Context, url, dest string, progress func(int)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
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

// extractTarGzToBin extracts llama-server and shared libraries from a .tar.gz archive.
func extractTarGzToBin(archivePath, destDir string, logger func(string)) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip okuma hatası: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	extracted := 0

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar okuma hatası: %w", err)
		}

		if hdr.Typeflag == tar.TypeDir {
			continue
		}

		name := filepath.Base(hdr.Name)
		lname := strings.ToLower(name)

		// Extract llama-server binary and shared libraries (.so)
		isServerBin := name == "llama-server"
		isSharedLib := strings.HasSuffix(lname, ".so") || strings.Contains(lname, ".so.")
		if !isServerBin && !isSharedLib {
			continue
		}

		destPath := filepath.Join(destDir, name)

		if hdr.Typeflag == tar.TypeSymlink {
			if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
				logger(fmt.Sprintf("Uyarı: Eski sembolik bağ silinemedi %s: %v", name, err))
			}
			if err := os.Symlink(hdr.Linkname, destPath); err != nil {
				// Windows: copy instead of symlink (requires admin/Developer Mode)
				if goruntime.GOOS == "windows" {
					srcPath := filepath.Join(filepath.Dir(destPath), hdr.Linkname)
					if copyErr := copyFile(srcPath, destPath, 0755); copyErr == nil {
						logger(fmt.Sprintf("  Kopyalandı (symlink yerine): %s -> %s", name, hdr.Linkname))
						extracted++
						continue
					}
				}
				logger(fmt.Sprintf("Uyarı: Sembolik bağ oluşturulamadı %s -> %s: %v", name, hdr.Linkname, err))
			} else {
				logger(fmt.Sprintf("  Bağlandı: %s -> %s", name, hdr.Linkname))
				extracted++
			}
			continue
		}

		if err := extractFile(tr, destPath, name, logger); err != nil {
			logger(fmt.Sprintf("Uyarı: %s çıkarılamadı: %v", name, err))
			continue
		}
		logger(fmt.Sprintf("  Çıkarıldı: %s", name))
		extracted++
	}

	if extracted == 0 {
		return fmt.Errorf("tar.gz içinde hiçbir binary bulunamadı")
	}
	return nil
}

func extractFile(tr *tar.Reader, destPath, name string, logger func(string)) error {
	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("dosya oluşturulamadı: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, tr); err != nil {
		return fmt.Errorf("kopyalanamadı: %w", err)
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
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutR)
		for scanner.Scan() {
			logger(scanner.Text())
		}
	}()
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrR)
		for scanner.Scan() {
			logger(scanner.Text())
		}
	}()
	err = cmd.Wait()
	wg.Wait()
	return err
}

// StreamToFrontend is a helper to wrap the Wails runtime emit.
func StreamToFrontend(ctx context.Context, line string) {
	log.Println("INSTALL:", line)
}
