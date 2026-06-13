package modelstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ─── Types ───────────────────────────────────────────────────────

// HFModelResult represents a model returned from Hugging Face search.
type HFModelResult struct {
	ID        string   `json:"id"`
	Author    string   `json:"author"`
	Downloads int      `json:"downloads"`
	Likes     int      `json:"likes"`
	Tags      []string `json:"tags"`
}

// GGUFFile represents a single .gguf file within a HF repo.
type GGUFFile struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// DownloadProgress tracks the state of an active download.
type DownloadProgress struct {
	Active     bool    `json:"active"`
	RepoID     string  `json:"repo_id"`
	Filename   string  `json:"filename"`
	TotalBytes int64   `json:"total_bytes"`
	Downloaded int64   `json:"downloaded"`
	Percent    float64 `json:"percent"`
	Speed      string  `json:"speed"`
}

// LocalModel represents a downloaded .gguf file on disk.
type LocalModel struct {
	RepoID      string `json:"repo_id"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	Path        string `json:"path"`
	IsEmbedding bool   `json:"is_embedding"`
	MmprojPath  string `json:"mmproj_path,omitempty"`
}

// findMmproj looks for a multimodal projector file next to the model.
func findMmproj(modelPath string) string {
	dir := filepath.Dir(modelPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if strings.Contains(lower, "mmproj") || strings.Contains(lower, "multimodal") {
			if strings.HasSuffix(lower, ".gguf") {
				return filepath.Join(dir, e.Name())
			}
		}
	}
	return ""
}

// isEmbeddingModel checks if a model filename/repo suggests it's an embedding model.
func isEmbeddingModel(filename, repoID string) bool {
	lower := strings.ToLower(filename + " " + repoID)
	keywords := []string{"embed", "nomic", "bge", "e5-", "gte-", "mxbai-embed", "snowflake-arctic-embed"}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// ─── HF API response types (internal) ────────────────────────────

type hfTreeItem struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// ─── Store ───────────────────────────────────────────────────────

type Store struct {
	modelsDir string
	client    *http.Client

	mu       sync.RWMutex
	progress *DownloadProgress
	cancelFn context.CancelFunc
}

func New(modelsDir string) *Store {
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		log.Printf("modelstore: cannot create models dir: %v", err)
	}
	return &Store{
		modelsDir: modelsDir,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		progress: &DownloadProgress{},
	}
}

// ─── Search ──────────────────────────────────────────────────────

func (s *Store) SearchModels(query string) ([]HFModelResult, error) {
	u := fmt.Sprintf(
		"https://huggingface.co/api/models?search=%s&filter=gguf&sort=downloads&direction=-1&limit=20",
		url.QueryEscape(query),
	)

	resp, err := s.client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("modelstore.Search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("modelstore.Search: HF API status %d: %s", resp.StatusCode, string(body))
	}

	var results []HFModelResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("modelstore.Search: decode: %w", err)
	}

	return results, nil
}

// ─── Get files for a repo ────────────────────────────────────────

func (s *Store) GetModelFiles(repoID string) ([]GGUFFile, error) {
	u := fmt.Sprintf("https://huggingface.co/api/models/%s/tree/main", repoID)

	resp, err := s.client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("modelstore.GetFiles: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("modelstore.GetFiles: HF API status %d: %s", resp.StatusCode, string(body))
	}

	var tree []hfTreeItem
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, fmt.Errorf("modelstore.GetFiles: decode: %w", err)
	}

	var files []GGUFFile
	for _, item := range tree {
		if strings.HasSuffix(strings.ToLower(item.Path), ".gguf") {
			files = append(files, GGUFFile{
				Filename: item.Path,
				Size:     item.Size,
			})
		}
	}

	return files, nil
}

// ─── Download ────────────────────────────────────────────────────

func (s *Store) DownloadModel(repoID, filename string) error {
	s.mu.Lock()
	if s.progress.Active {
		s.mu.Unlock()
		return fmt.Errorf("modelstore.Download: another download in progress")
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFn = cancel
	s.progress = &DownloadProgress{
		Active:   true,
		RepoID:   repoID,
		Filename: filename,
	}
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.progress.Active = false
			s.cancelFn = nil
			s.mu.Unlock()
		}()

		if err := s.doDownload(ctx, repoID, filename); err != nil {
			if ctx.Err() != nil {
				log.Printf("modelstore: download cancelled: %s/%s", repoID, filename)
			} else {
				log.Printf("modelstore: download failed: %v", err)
			}
		} else {
			log.Printf("modelstore: download complete: %s/%s", repoID, filename)
		}
	}()

	return nil
}

func (s *Store) doDownload(ctx context.Context, repoID, filename string) error {
	downloadURL := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repoID, filename)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Use a client without timeout for large downloads
	dlClient := &http.Client{}
	resp, err := dlClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download status %d: %s", resp.StatusCode, string(body))
	}

	totalBytes := resp.ContentLength
	s.mu.Lock()
	s.progress.TotalBytes = totalBytes
	s.mu.Unlock()

	// Create destination directory (flat: directly in modelsDir, no subdirectory)
	if err := os.MkdirAll(s.modelsDir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	destPath := filepath.Join(s.modelsDir, filename)
	tmpPath := destPath + ".downloading"

	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer func() {
		f.Close()
		os.Remove(tmpPath)
	}()

	buf := make([]byte, 256*1024) // 256KB chunks
	var downloaded int64
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write file: %w", writeErr)
			}
			downloaded += int64(n)

			elapsed := time.Since(startTime).Seconds()
			var speed string
			if elapsed > 0 {
				bytesPerSec := float64(downloaded) / elapsed
				speed = formatSpeed(bytesPerSec)
			}

			var percent float64
			if totalBytes > 0 {
				percent = float64(downloaded) / float64(totalBytes) * 100
			}

			s.mu.Lock()
			s.progress.Downloaded = downloaded
			s.progress.Percent = percent
			s.progress.Speed = speed
			s.mu.Unlock()
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read body: %w", readErr)
		}
	}

	f.Close()

	// Rename temp file to final name
	if err := os.Rename(tmpPath, destPath); err != nil {
		// Cross-volume rename fallback: copy + delete
		if copyErr := copyFile(tmpPath, destPath); copyErr != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("rename and copy fallback both failed: rename: %w, copy: %v", err, copyErr)
		}
		os.Remove(tmpPath)
	}

	s.mu.Lock()
	s.progress.Percent = 100
	s.progress.Downloaded = downloaded
	s.mu.Unlock()

	return nil
}

// copyFile copies a file from src to dst (cross-device safe).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// ─── Progress & Cancel ───────────────────────────────────────────

func (s *Store) GetDownloadProgress() *DownloadProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy
	p := *s.progress
	return &p
}

func (s *Store) CancelDownload() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelFn != nil {
		s.cancelFn()
	}
}

// ─── Local Model Management ─────────────────────────────────────

func (s *Store) ListLocalModels() []LocalModel {
	var models []LocalModel

	err := filepath.Walk(s.modelsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".gguf") {
			return nil
		}
		// Skip in-progress downloads
		if strings.HasSuffix(info.Name(), ".downloading") {
			return nil
		}
		// Skip multimodal projector files (they are not standalone models)
		lower := strings.ToLower(info.Name())
		if strings.Contains(lower, "mmproj") || strings.Contains(lower, "multimodal") {
			return nil
		}

		relPath, _ := filepath.Rel(s.modelsDir, path)
		parts := strings.SplitN(relPath, string(os.PathSeparator), 2)

		repoID := ""
		if len(parts) >= 2 {
			repoID = unsanitizePath(parts[0])
		}

		models = append(models, LocalModel{
			RepoID:      repoID,
			Filename:    info.Name(),
			Size:        info.Size(),
			Path:        path,
			IsEmbedding: isEmbeddingModel(info.Name(), repoID),
			MmprojPath:  findMmproj(path),
		})

		return nil
	})

	if err != nil {
		log.Printf("modelstore: walk error: %v", err)
	}

	return models
}

func (s *Store) DeleteLocalModel(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("modelstore.Delete: %w", err)
	}
	absModelsDir, err := filepath.Abs(s.modelsDir)
	if err != nil {
		return fmt.Errorf("modelstore.Delete: %w", err)
	}
	// Resolve symlinks to prevent TOCTOU symlink attacks.
	absPath, err = filepath.EvalSymlinks(absPath)
	if err != nil {
		return fmt.Errorf("modelstore.Delete: %w", err)
	}
	evalModelsDir, err := filepath.EvalSymlinks(absModelsDir)
	if err != nil {
		return fmt.Errorf("modelstore.Delete: %w", err)
	}
	if !strings.HasPrefix(absPath, evalModelsDir) {
		return fmt.Errorf("modelstore.Delete: path outside models directory")
	}

	// Re-resolve symlinks right before removal to minimize TOCTOU window.
	// An attacker could replace the resolved path with a symlink between
	// the initial EvalSymlinks call and os.Remove.
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return fmt.Errorf("modelstore.Delete: %w", err)
	}
	if !strings.HasPrefix(resolved, evalModelsDir) {
		return fmt.Errorf("modelstore.Delete: path outside models directory after re-resolution")
	}

	if err := os.Remove(resolved); err != nil {
		return fmt.Errorf("modelstore.Delete: %w", err)
	}

	// Clean up empty parent directories
	dir := filepath.Dir(resolved)
	for dir != absModelsDir {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}
		os.Remove(dir)
		dir = filepath.Dir(dir)
	}

	return nil
}

const maxImportSize int64 = 50 << 30 // 50 GiB — covers virtually all LLM models

func (s *Store) ImportLocalModel(sourcePath string) error {
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return err
	}
	info, err := os.Stat(absSource)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("source is a directory")
	}
	if info.Size() > maxImportSize {
		return fmt.Errorf("file too large (%d bytes, max %d bytes)", info.Size(), maxImportSize)
	}

	if err := os.MkdirAll(s.modelsDir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	destPath := filepath.Join(s.modelsDir, info.Name())

	input, err := os.Open(absSource)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer output.Close()

	if _, err = io.Copy(output, input); err != nil {
		return err
	}
	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────

// sanitizePath replaces "/" in repo IDs with "__" for safe directory names.
func sanitizePath(repoID string) string {
	return strings.ReplaceAll(repoID, "/", "__")
}

func unsanitizePath(dirName string) string {
	return strings.ReplaceAll(dirName, "__", "/")
}

func formatSpeed(bytesPerSec float64) string {
	switch {
	case bytesPerSec >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB/s", bytesPerSec/(1024*1024*1024))
	case bytesPerSec >= 1024*1024:
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/(1024*1024))
	case bytesPerSec >= 1024:
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
	default:
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	}
}
