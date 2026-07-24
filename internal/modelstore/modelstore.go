package modelstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/gguf"
	"memo/internal/logx"
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
	ID           string   `json:"id"`
	Author       string   `json:"author"`
	Downloads    int      `json:"downloads"`
	Likes        int      `json:"likes"`
	Tags         []string `json:"tags"`
	LastModified string   `json:"lastModified"`
}

// modelMeta is stored as a sidecar JSON file (<model>.meta.json) next to each
// downloaded GGUF so we can surface HF capabilities without network calls.
type modelMeta struct {
	RepoID string   `json:"repo_id"`
	Tags   []string `json:"tags"`
	// SupportsTools is true if EITHER HF's own tags said so at download
	// time OR the GGUF file's own embedded chat template references
	// tool_calls (internal/gguf.Metadata.SupportsTools, read once and
	// folded in here by ListLocalModels — see GGUFMetaChecked) — either
	// source confirming it is enough, since HF tags on GGUF quantization
	// repos are frequently just missing even when the underlying model
	// genuinely supports tool calling.
	SupportsTools  bool `json:"supports_tools"`
	SupportsVision bool `json:"supports_vision"`
	SupportsCode   bool `json:"supports_code"`
	IsEmbedding    bool `json:"is_embedding_hf"` // from HF tags (distinct from filename heuristic)

	// MaxContext is the model's own trained context length, read once from
	// the GGUF file's metadata (internal/gguf.Metadata.ContextLength) and
	// cached here so ListLocalModels doesn't re-parse a multi-gigabyte file
	// on every poll. 0 means "checked, genuinely unknown" (see
	// GGUFMetaChecked), not "not yet checked."
	MaxContext int `json:"max_context,omitempty"`
	// GGUFMetaChecked distinguishes "we tried to read the GGUF header and
	// found nothing new" (MaxContext stays 0, SupportsTools stays whatever
	// HF tags said, correctly) from "this sidecar predates the GGUF-header
	// reading feature and has never been checked" (must still be attempted
	// once). Without this, a model whose architecture/template isn't
	// recognized would have its multi-gigabyte file's header re-read on
	// every single ListLocalModels call, forever.
	GGUFMetaChecked bool `json:"gguf_meta_checked,omitempty"`
}

// GGUFFile represents a single .gguf file within a HF repo.
type GGUFFile struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// tagCapability maps known HuggingFace tags to a capability name.
var tagCapabilities = map[string]string{
	// Tool calling
	"function-calling": "tools",
	"tool-use":         "tools",
	"tool-calling":     "tools",
	"tools":            "tools",
	// Vision / multimodal
	"image-to-text":              "vision",
	"visual-question-answering":  "vision",
	"image-text-to-text":         "vision",
	"vision":                     "vision",
	"multimodal":                 "vision",
	// Code generation
	"code":             "code",
	"code-generation":  "code",
	"coding":           "code",
	// Embeddings
	"feature-extraction":   "embedding",
	"sentence-similarity":  "embedding",
	"sentence-transformers": "embedding",
	"text-embeddings-inference": "embedding",
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
	Error      string  `json:"error,omitempty"`
}

// LocalModel represents a downloaded .gguf file on disk.
type LocalModel struct {
	RepoID         string   `json:"repo_id"`
	Filename       string   `json:"filename"`
	Size           int64    `json:"size"`
	Path           string   `json:"path"`
	IsEmbedding    bool     `json:"is_embedding"`
	MmprojPath     string   `json:"mmproj_path,omitempty"`
	SupportsTools  bool     `json:"supports_tools"`
	SupportsVision bool     `json:"supports_vision"`
	SupportsCode   bool     `json:"supports_code"`
	Tags           []string `json:"tags,omitempty"`
	// MaxContext is this model's own trained context length (read from its
	// GGUF metadata, see internal/gguf.Metadata.ContextLength), so a client
	// can bound a context-size control at what the model can actually
	// handle instead of accepting an arbitrary value that crashes
	// llama-server at startup. 0 means unknown — the file's architecture
	// wasn't one this reader recognizes the "<arch>.context_length"
	// convention for.
	MaxContext int `json:"max_context,omitempty"`
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

type hfModelInfo struct {
	Tags []string `json:"tags"`
}

// ─── Store ───────────────────────────────────────────────────────

// downloadEntry tracks one in-flight (or errored, not-yet-dismissed) download.
type downloadEntry struct {
	progress *DownloadProgress
	cancelFn context.CancelFunc
}

type Store struct {
	modelsDir string
	client    *http.Client

	mu        sync.RWMutex
	downloads map[string]*downloadEntry // key: downloadKey(repoID, filename)
	order     []string                  // insertion order of keys, for a stable list
}

func New(modelsDir string) *Store {
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		logx.Printf("modelstore: cannot create models dir: %v", err)
	}
	return &Store{
		modelsDir: modelsDir,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		downloads: make(map[string]*downloadEntry),
	}
}

// downloadKey uniquely identifies a download by repo + file, since the same
// repo can have multiple files downloading (e.g. a chat model + a separate
// mmproj file) and different repos can share a filename.
func downloadKey(repoID, filename string) string {
	return repoID + "/" + filename
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

	// HF's model-search API response (confirmed by hitting it directly) has
	// no top-level "author" field at all — only "id" ("google/gemma-2b"),
	// "likes", "downloads", "tags", etc. HFModelResult.Author therefore
	// always decoded to "" here, silently, for every single search result:
	// the frontend's AuthorAvatar had nothing to look up (empty org/user
	// name), so every avatar in Discover fell back to its "?" empty-string
	// placeholder regardless of the real repo owner.
	for i := range results {
		if results[i].Author == "" {
			results[i].Author = authorFromRepoID(results[i].ID)
		}
	}

	return results, nil
}

// authorFromRepoID derives the org/user namespace from a HF repo ID
// ("google/gemma-2b" -> "google"), same convention DiscoverItem.fromCurated
// already uses on the Flutter side for curated entries. Returns "" for an
// ID with no namespace segment at all (malformed, shouldn't happen for a
// real HF repo ID, but this must not panic on one).
func authorFromRepoID(id string) string {
	if slash := strings.Index(id, "/"); slash > 0 {
		return id[:slash]
	}
	return ""
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

// ─── HF Metadata ─────────────────────────────────────────────────

// fetchModelMeta calls the HF API to retrieve tags for a repo and returns
// a populated modelMeta. Non-fatal on error — caller logs and moves on.
func (s *Store) fetchModelMeta(repoID string) (*modelMeta, error) {
	u := fmt.Sprintf("https://huggingface.co/api/models/%s", repoID)
	resp, err := s.client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("fetchModelMeta: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetchModelMeta: status %d: %s", resp.StatusCode, string(body))
	}

	var info hfModelInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("fetchModelMeta: decode: %w", err)
	}

	caps := map[string]bool{}
	for _, tag := range info.Tags {
		if cap, ok := tagCapabilities[strings.ToLower(tag)]; ok {
			caps[cap] = true
		}
	}

	return &modelMeta{
		RepoID:         repoID,
		Tags:           info.Tags,
		SupportsTools:  caps["tools"],
		SupportsVision: caps["vision"],
		SupportsCode:   caps["code"],
		IsEmbedding:    caps["embedding"],
	}, nil
}

// saveModelMeta writes a modelMeta as a JSON sidecar next to destPath.
func saveModelMeta(destPath string, meta *modelMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(destPath+".meta.json", data, 0644)
}

// loadModelMeta reads the sidecar JSON for a GGUF file, if it exists.
func loadModelMeta(ggufPath string) *modelMeta {
	data, err := os.ReadFile(ggufPath + ".meta.json")
	if err != nil {
		return nil
	}
	var meta modelMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil
	}
	return &meta
}

// ─── Download ────────────────────────────────────────────────────

// DownloadModel starts downloading repoID/filename in the background.
// Multiple downloads can run concurrently as long as they're for different
// repo+file pairs — only a duplicate of an already-active download is
// rejected, so e.g. a chat model and a memory model can download at once.
func (s *Store) DownloadModel(repoID, filename string, expectedSize int64) error {
	key := downloadKey(repoID, filename)

	s.mu.Lock()
	if existing, ok := s.downloads[key]; ok && existing.progress.Active {
		s.mu.Unlock()
		return fmt.Errorf("modelstore.Download: %s is already downloading", filename)
	}

	ctx, cancel := context.WithCancel(context.Background())
	entry := &downloadEntry{
		cancelFn: cancel,
		progress: &DownloadProgress{
			Active:     true,
			RepoID:     repoID,
			Filename:   filename,
			TotalBytes: expectedSize,
		},
	}
	if _, exists := s.downloads[key]; !exists {
		s.order = append(s.order, key)
	}
	s.downloads[key] = entry
	s.mu.Unlock()

	go func() {
		defer logx.Recover("modelstore.Store download: " + repoID + "/" + filename)
		defer func() {
			s.mu.Lock()
			if entry.progress.Error == "" {
				entry.progress.Active = false
				// Drop the entry a little after it finishes so the UI gets a
				// beat to show "100%" before the download disappears from lists.
				time.AfterFunc(3*time.Second, func() {
					defer logx.Recover("modelstore.Store download cleanup")
					s.mu.Lock()
					if cur, ok := s.downloads[key]; ok && cur == entry {
						delete(s.downloads, key)
						s.removeFromOrderLocked(key)
					}
					s.mu.Unlock()
				})
			}
			entry.cancelFn = nil
			s.mu.Unlock()
		}()

		if err := s.doDownload(ctx, entry, repoID, filename, expectedSize); err != nil {
			errMsg := err.Error()
			s.mu.Lock()
			entry.progress.Error = errMsg
			entry.progress.Active = true // keep visible so Flutter shows the error
			s.mu.Unlock()
			if ctx.Err() != nil {
				logx.Printf("modelstore: download cancelled: %s/%s", repoID, filename)
			} else {
				logx.Printf("modelstore: download failed: %v", err)
			}
		} else {
			s.mu.Lock()
			entry.progress.Active = false
			entry.progress.Percent = 100
			s.mu.Unlock()
			logx.Printf("modelstore: download complete: %s/%s", repoID, filename)
		}
	}()

	return nil
}

// removeFromOrderLocked removes key from s.order. Caller must hold s.mu.
func (s *Store) removeFromOrderLocked(key string) {
	for i, k := range s.order {
		if k == key {
			s.order = append(s.order[:i], s.order[i+1:]...)
			return
		}
	}
}

func (s *Store) doDownload(ctx context.Context, entry *downloadEntry, repoID, filename string, expectedSize int64) error {
	escapedFile := url.PathEscape(filename)
	downloadURL := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repoID, escapedFile)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Memo/3.1 (https://github.com/anomalyco/memo)")

	dlClient := &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			ResponseHeaderTimeout: 60 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			MaxIdleConnsPerHost:   2,
		},
	}
	resp, err := dlClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download status %d: %s", resp.StatusCode, string(body))
	}

	var totalBytes int64
	if resp.ContentLength > 0 {
		totalBytes = resp.ContentLength
	} else if expectedSize > 0 {
		totalBytes = expectedSize
	}
	s.mu.Lock()
	entry.progress.TotalBytes = totalBytes
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
			entry.progress.Downloaded = downloaded
			entry.progress.Percent = percent
			entry.progress.Speed = speed
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

	// Verify completeness before promoting the temp file. A dropped/proxied
	// connection can deliver a short body that still ends in io.EOF; without this
	// check a truncated file would be renamed to the final .gguf and later crash
	// llama-server as a "valid" but corrupt model. The deferred cleanup removes
	// the partial temp file on this error path.
	// totalBytes is resolved from Content-Length or the HF API expectedSize
	// (set via GetModelFiles), so the check runs even when HF uses chunked
	// transfer encoding without Content-Length.
	if totalBytes > 0 && downloaded != totalBytes {
		return fmt.Errorf("download incomplete: got %d of %d bytes", downloaded, totalBytes)
	}

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
	entry.progress.Percent = 100
	entry.progress.Downloaded = downloaded
	s.mu.Unlock()

	// Fetch and save HF metadata (tool-calling capability etc.) as a sidecar.
	if meta, err := s.fetchModelMeta(repoID); err != nil {
		logx.Printf("modelstore: fetchModelMeta skipped for %s: %v", repoID, err)
	} else if err := saveModelMeta(destPath, meta); err != nil {
		logx.Printf("modelstore: saveModelMeta failed for %s: %v", destPath, err)
	}

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

// GetDownloadProgress returns every currently tracked download (active, or
// errored and not yet dismissed), oldest-started first.
func (s *Store) GetDownloadProgress() []*DownloadProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*DownloadProgress, 0, len(s.order))
	for _, key := range s.order {
		entry, ok := s.downloads[key]
		if !ok {
			continue
		}
		p := *entry.progress
		out = append(out, &p)
	}
	return out
}

// CancelDownload stops the download for repoID/filename if it's running, and
// dismisses it if it's sitting in an errored state.
func (s *Store) CancelDownload(repoID, filename string) {
	key := downloadKey(repoID, filename)

	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.downloads[key]
	if !ok {
		return
	}
	if entry.cancelFn != nil {
		entry.cancelFn()
	}
	// Clear error state so the user can dismiss a failed download banner.
	if entry.progress.Error != "" {
		delete(s.downloads, key)
		s.removeFromOrderLocked(key)
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

		lm := LocalModel{
			RepoID:      repoID,
			Filename:    info.Name(),
			Size:        info.Size(),
			Path:        path,
			IsEmbedding: isEmbeddingModel(info.Name(), repoID),
			MmprojPath:  findMmproj(path),
		}
		meta := loadModelMeta(path)
		if meta != nil {
			lm.SupportsTools = meta.SupportsTools
			lm.SupportsVision = meta.SupportsVision
			lm.SupportsCode = meta.SupportsCode
			lm.Tags = meta.Tags
			if lm.RepoID == "" && meta.RepoID != "" {
				lm.RepoID = meta.RepoID
			}
		}
		// Read (and cache) the model's real max context length + whether its
		// own chat template actually supports tool calling, straight from
		// its GGUF header, the first time we see it — GGUFMetaChecked makes
		// this a one-time cost per file, not a re-parse of a possibly
		// multi-gigabyte file on every ListLocalModels poll. SupportsTools
		// is OR'd with whatever HF's tags already said (see modelMeta's doc
		// comment) rather than replaced — either source confirming it is
		// enough, and this can only add true positives HF tags missed,
		// never remove a real one.
		if meta == nil || !meta.GGUFMetaChecked {
			ggufMeta, err := gguf.Read(path)
			if err != nil {
				logx.Printf("modelstore: read gguf metadata for %s: %v", path, err)
			}
			if meta == nil {
				meta = &modelMeta{RepoID: lm.RepoID}
			}
			meta.MaxContext = ggufMeta.ContextLength
			meta.SupportsTools = meta.SupportsTools || ggufMeta.SupportsTools
			meta.GGUFMetaChecked = true
			if err := saveModelMeta(path, meta); err != nil {
				logx.Printf("modelstore: cache gguf metadata for %s: %v", path, err)
			}
		}
		lm.MaxContext = meta.MaxContext
		lm.SupportsTools = meta.SupportsTools
		models = append(models, lm)

		return nil
	})

	if err != nil {
		logx.Printf("modelstore: walk error: %v", err)
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
