package replcli

import (
	"context"
	"net/http"
	"net/url"
)

// HFModelResult mirrors modelstore.HFModelResult — a Hugging Face search hit.
type HFModelResult struct {
	ID        string   `json:"id"`
	Author    string   `json:"author"`
	Downloads int      `json:"downloads"`
	Likes     int      `json:"likes"`
	Tags      []string `json:"tags"`
}

// GGUFFile mirrors modelstore.GGUFFile — one downloadable file in a repo.
type GGUFFile struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// DownloadProgress mirrors modelstore.DownloadProgress.
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

// SearchModels searches Hugging Face for GGUF models. An empty query returns
// the most-downloaded GGUF models (the backend's search endpoint sorts by
// downloads regardless of query) — used as the "suggest something" default.
func (c *Client) SearchModels(ctx context.Context, query string) ([]HFModelResult, error) {
	var results []HFModelResult
	if err := c.doJSON(ctx, http.MethodPost, "/api/models/search", map[string]string{"query": query}, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// ModelFiles lists the GGUF files available in a Hugging Face repo.
func (c *Client) ModelFiles(ctx context.Context, repoID string) ([]GGUFFile, error) {
	var files []GGUFFile
	path := "/api/models/files?repo=" + url.QueryEscape(repoID)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// DownloadModel starts downloading a model file in the background. Progress
// is polled separately via DownloadProgress.
func (c *Client) DownloadModel(ctx context.Context, repoID, filename string, expectedSize int64) error {
	return c.doJSON(ctx, http.MethodPost, "/api/models/download", map[string]any{
		"repo_id":       repoID,
		"filename":      filename,
		"expected_size": expectedSize,
	}, nil)
}

// DownloadProgress reports the state of the current (or most recent)
// download. Active is false once it finishes, successfully or not.
func (c *Client) DownloadProgress(ctx context.Context) (DownloadProgress, error) {
	var p DownloadProgress
	err := c.doJSON(ctx, http.MethodGet, "/api/models/download/progress", nil, &p)
	return p, err
}
