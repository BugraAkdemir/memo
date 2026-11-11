package modelstore

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// validRepoFile accepts HF's slash-separated relative paths on every OS.
func validRepoFile(name string) bool {
	if name == "" || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return strings.HasSuffix(strings.ToLower(name), ".gguf")
}

func isProjector(name string) bool {
	lower := strings.ToLower(path.Base(name))
	return strings.HasSuffix(lower, ".gguf") && (strings.Contains(lower, "mmproj") || strings.Contains(lower, "multimodal"))
}

// modelTree includes subfolders and all pages: projectors often aren't on the
// first/root page. Only follow pagination URLs for this same HF endpoint.
func (s *Store) modelTree(ctx context.Context, repoID string) ([]hfTreeItem, error) {
	endpoint := fmt.Sprintf("https://huggingface.co/api/models/%s/tree/main", repoID)
	origin, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	next := endpoint + "?recursive=true"
	seen := map[string]bool{}
	var tree []hfTreeItem
	for next != "" {
		if seen[next] || len(seen) >= 100 {
			return nil, fmt.Errorf("modelstore: invalid or excessive tree pagination")
		}
		seen[next] = true
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, err
		}
		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("modelstore: repository files: %w", err)
		}
		var page []hfTreeItem
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("modelstore: repository files: HTTP %d", resp.StatusCode)
		}
		err = json.NewDecoder(resp.Body).Decode(&page)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("modelstore: repository files: %w", err)
		}
		tree = append(tree, page...)
		next = ""
		for _, link := range strings.Split(resp.Header.Get("Link"), ",") {
			parts := strings.Split(strings.TrimSpace(link), ";")
			for _, rel := range parts[1:] {
				if strings.TrimSpace(rel) != `rel="next"` {
					continue
				}
				u, err := url.Parse(strings.Trim(strings.TrimSpace(parts[0]), "<>"))
				if err != nil {
					return nil, fmt.Errorf("modelstore: invalid tree pagination URL")
				}
				u = origin.ResolveReference(u)
				if u.Scheme != origin.Scheme || u.Host != origin.Host || u.Path != origin.Path || u.User != nil {
					return nil, fmt.Errorf("modelstore: unsafe tree pagination URL")
				}
				next = u.String()
			}
		}
	}
	return tree, nil
}

var quantSuffix = regexp.MustCompile(`(?i)(?:[._-](?:u?i?q[0-9][a-z0-9_-]*|f16|bf16|f32|fp16|fp32))$`)

func projectorStem(name string) string {
	stem := strings.ToLower(strings.TrimSuffix(path.Base(name), path.Ext(name)))
	stem = strings.TrimPrefix(stem, "mmproj")
	stem = strings.TrimPrefix(stem, "multimodal")
	stem = quantSuffix.ReplaceAllString(stem, "")
	return strings.Trim(stem, "._-")
}

// Select by the repository's filenames, never by a hardcoded model family.
// Precision variants of one projector prefer F16, then BF16/F32. Distinct
// unidentified projectors are ambiguous and must not silently load the wrong one.
func selectProjector(filename string, files []hfTreeItem) (*hfTreeItem, error) {
	var candidates []hfTreeItem
	for _, f := range files {
		if f.Type != "directory" && isProjector(f.Path) && validRepoFile(f.Path) {
			candidates = append(candidates, f)
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	if len(candidates) > 1 {
		var matched []hfTreeItem
		stem := projectorStem(filename)
		for _, f := range candidates {
			if projectorStem(f.Path) == stem {
				matched = append(matched, f)
			}
		}
		if len(matched) > 0 {
			candidates = matched
		}
	}
	if len(candidates) > 1 {
		var local []hfTreeItem
		for _, f := range candidates {
			if path.Dir(f.Path) == path.Dir(filename) {
				local = append(local, f)
			}
		}
		if len(local) > 0 {
			candidates = local
		}
	}
	if len(candidates) == 1 {
		return &candidates[0], nil
	}
	stem := projectorStem(candidates[0].Path)
	for _, f := range candidates[1:] {
		if projectorStem(f.Path) != stem {
			return nil, fmt.Errorf("multiple multimodal projectors match %s; cannot select one safely", filename)
		}
	}
	for _, precision := range []string{"f16", "fp16", "bf16", "f32", "fp32"} {
		var matches []hfTreeItem
		for _, f := range candidates {
			base := strings.ToLower(strings.TrimSuffix(path.Base(f.Path), path.Ext(f.Path)))
			if strings.HasSuffix(base, "-"+precision) || strings.HasSuffix(base, "."+precision) || strings.HasSuffix(base, "_"+precision) {
				matches = append(matches, f)
			}
		}
		if len(matches) == 1 {
			return &matches[0], nil
		}
	}
	return nil, fmt.Errorf("multiple multimodal projectors match %s; cannot select one safely", filename)
}

func (s *Store) downloadWithProjector(ctx context.Context, entry *downloadEntry, repoID, filename string, expectedSize int64) error {
	if isProjector(filename) {
		return s.doDownload(ctx, entry, repoID, filename, expectedSize)
	}
	tree, err := s.modelTree(ctx, repoID)
	if err != nil {
		return err
	} // Don't report a vision download complete with unknown dependencies.
	if expectedSize <= 0 {
		for _, f := range tree {
			if f.Path == filename {
				expectedSize = f.Size
				break
			}
		}
	}
	projector, err := selectProjector(filename, tree)
	if err != nil {
		return err
	}
	modelPath := filepath.Join(s.modelsDir, filepath.FromSlash(filename))
	if projector != nil {
		// Same-named weights/projectors from different repos must never overwrite each other.
		repoHash := sha256.Sum256([]byte(repoID))
		modelPath = filepath.Join(s.modelsDir, fmt.Sprintf("repo-%x", repoHash[:16]), filepath.FromSlash(filename))
		entry.progressPending = true
		entry.progressRemaining = expectedSize
		// Preserve an older flat download only when its metadata identifies this repo.
		legacy := filepath.Join(s.modelsDir, filepath.FromSlash(filename))
		if meta := loadModelMeta(legacy); meta != nil && meta.RepoID == repoID {
			modelPath = legacy
		}
		var modelFile hfTreeItem
		for _, f := range tree {
			if f.Path == filename {
				modelFile = f
				break
			}
		}
		reuseModel := existingModelMatches(ctx, modelPath, modelFile)
		if reuseModel {
			entry.progressRemaining = 0
		}
		dest := modelPath + ".mmproj"
		if err = s.doDownloadTo(ctx, entry, repoID, projector.Path, projector.Size, filepath.Join(dest, "mmproj.gguf")); err != nil {
			return fmt.Errorf("download multimodal projector %s: %w", projector.Path, err)
		}
		if reuseModel {
			return nil
		}
		s.mu.RLock()
		entry.progressOffset = entry.progress.Downloaded
		s.mu.RUnlock()
		entry.progressRemaining = 0
		entry.progressPending = false
	}
	return s.doDownloadTo(ctx, entry, repoID, filename, expectedSize, modelPath)
}

// Re-download requests can repair older installations without fetching the
// multi-GB weights again. Size and the repo's LFS hash (when supplied) must match.
func existingModelMatches(ctx context.Context, filename string, item hfTreeItem) bool {
	if item.Size <= 0 {
		return false
	}
	f, err := os.Open(filename)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != item.Size {
		return false
	}
	if item.LFS == nil || item.LFS.OID == "" {
		return ctx.Err() == nil
	}
	hash := sha256.New()
	buf := make([]byte, 256*1024)
	for {
		if ctx.Err() != nil {
			return false
		}
		n, err := f.Read(buf)
		hash.Write(buf[:n])
		if err == io.EOF {
			return strings.EqualFold(fmt.Sprintf("%x", hash.Sum(nil)), item.LFS.OID)
		}
		if err != nil {
			return false
		}
	}
}
