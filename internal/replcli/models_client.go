package replcli

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// LocalModel mirrors the fields of modelstore.LocalModel the REPL needs to
// list and pick models by name.
type LocalModel struct {
	Filename      string `json:"filename"`
	Path          string `json:"path"`
	Size          int64  `json:"size"`
	IsEmbedding   bool   `json:"is_embedding"`
	SupportsTools bool   `json:"supports_tools"`
}

// ModelStatus mirrors llama.ServerStatus.
type ModelStatus struct {
	Running   bool   `json:"running"`
	ModelPath string `json:"model_path"`
	ModelName string `json:"model_name"`
	Port      int    `json:"port"`
}

// ProviderConfig mirrors the fields of provider.ProviderConfig the REPL's
// /connect command and `memo provider` (cli_provider.go) need to configure
// an external, OpenAI-compatible endpoint (custom base URL + API key +
// model). Priority/Connected added for `memo provider list`'s display —
// deliberately not a full mirror (Temperature/TopP/MaxTokens/ContextTokens
// aren't needed by either CLI caller).
type ProviderConfig struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	APIKey    string `json:"api_key"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	Enabled   bool   `json:"enabled"`
	Priority  int    `json:"priority"`
	Connected bool   `json:"connected,omitempty"`
}

// ListLocalModels returns every model the backend knows about (chat and
// embedding models alike — check IsEmbedding to tell them apart).
func (c *Client) ListLocalModels(ctx context.Context) ([]LocalModel, error) {
	var models []LocalModel
	if err := c.doJSON(ctx, http.MethodGet, "/api/models/local", nil, &models); err != nil {
		return nil, err
	}
	return models, nil
}

// ModelStatus reports whether a local chat model is currently running.
func (c *Client) ModelStatus(ctx context.Context) (ModelStatus, error) {
	var status ModelStatus
	err := c.doJSON(ctx, http.MethodGet, "/api/models/status", nil, &status)
	return status, err
}

// EmbeddingStatus reports whether a local embedding model is currently running.
func (c *Client) EmbeddingStatus(ctx context.Context) (ModelStatus, error) {
	var status ModelStatus
	err := c.doJSON(ctx, http.MethodGet, "/api/models/embedding/status", nil, &status)
	return status, err
}

// modelLoadTimeout and embeddingLoadTimeout give the client a few seconds of
// slack over the backend's own WaitReady budgets (internal/app/llama.go,
// internal/app/embedding.go) so a load that's genuinely still in progress at
// the backend's own timeout gets that error back instead of the client
// giving up first and reporting a false failure for a model that goes on to
// load successfully in the background.
const (
	modelLoadTimeout     = 185 * time.Second
	embeddingLoadTimeout = 125 * time.Second
)

// StartModel loads a local chat model. ctxSize=0, port=0 and gpuLayers=-1
// all mean "use the backend's defaults" (8192 ctx, auto port, auto GPU
// layers) — see internal/llama.Server.Start. Model loading can legitimately
// take up to 180s server-side, so this uses the no-fixed-timeout client with
// an explicit deadline matching that budget, not the plain 10s JSON timeout.
func (c *Client) StartModel(ctx context.Context, path string, ctxSize, port, gpuLayers int) error {
	ctx, cancel := context.WithTimeout(ctx, modelLoadTimeout)
	defer cancel()
	return c.doJSONWith(ctx, c.longOpHTTP, http.MethodPost, "/api/models/start", map[string]any{
		"path":       path,
		"ctx_size":   ctxSize,
		"port":       port,
		"gpu_layers": gpuLayers,
	}, nil)
}

// StartEmbedding loads a local embedding model. gpuLayers=-1 means "auto".
// Same long-timeout reasoning as StartModel — the backend budget here is
// 120s.
func (c *Client) StartEmbedding(ctx context.Context, path string, gpuLayers int) error {
	ctx, cancel := context.WithTimeout(ctx, embeddingLoadTimeout)
	defer cancel()
	return c.doJSONWith(ctx, c.longOpHTTP, http.MethodPost, "/api/models/embedding/start", map[string]any{
		"path":       path,
		"gpu_layers": gpuLayers,
	}, nil)
}

// UpdateProvider creates or updates an external provider configuration.
func (c *Client) UpdateProvider(ctx context.Context, cfg ProviderConfig) error {
	return c.doJSON(ctx, http.MethodPut, "/api/providers", cfg, nil)
}

// SetActiveProvider switches the backend's active provider by name. An empty
// name routes back to the local llama.cpp model.
func (c *Client) SetActiveProvider(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodPut, "/api/providers/active", map[string]string{"provider": name}, nil)
}

// ListProviders returns every configured external provider (OpenAI, Claude,
// custom, etc.) — not just the local llama.cpp model /models/local covers.
func (c *Client) ListProviders(ctx context.Context) ([]ProviderConfig, error) {
	var providers []ProviderConfig
	if err := c.doJSON(ctx, http.MethodGet, "/api/providers", nil, &providers); err != nil {
		return nil, err
	}
	return providers, nil
}

// ActiveProviderName returns the name of the currently active external
// provider, or "" if none is active (routing to the local model instead).
func (c *Client) ActiveProviderName(ctx context.Context) (string, error) {
	var resp struct {
		Provider string `json:"provider"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/providers/active", nil, &resp); err != nil {
		return "", err
	}
	return resp.Provider, nil
}

// GetAgentEnabled reports whether agent (tool-use) mode is currently on.
// Counterpart to SetAgentEnabled above.
func (c *Client) GetAgentEnabled(ctx context.Context) (bool, error) {
	var resp struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/agent/enabled", nil, &resp); err != nil {
		return false, err
	}
	return resp.Enabled, nil
}

// HFModel mirrors modelstore.HFModelResult — one Hugging Face repo hit from
// a model search.
type HFModel struct {
	ID           string   `json:"id"`
	Author       string   `json:"author"`
	Downloads    int      `json:"downloads"`
	Likes        int      `json:"likes"`
	Tags         []string `json:"tags"`
	LastModified string   `json:"lastModified"`
}

// SearchModels searches Hugging Face for GGUF model repos matching query.
func (c *Client) SearchModels(ctx context.Context, query string) ([]HFModel, error) {
	var results []HFModel
	if err := c.doJSON(ctx, http.MethodPost, "/api/models/search", map[string]string{"query": query}, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// GGUFFile mirrors modelstore.GGUFFile — one downloadable .gguf file within
// a Hugging Face repo.
type GGUFFile struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// ListModelFiles lists the .gguf files available in a Hugging Face repo
// (repoID, e.g. "nomic-ai/nomic-embed-text-v1.5-GGUF").
func (c *Client) ListModelFiles(ctx context.Context, repoID string) ([]GGUFFile, error) {
	var files []GGUFFile
	if err := c.doJSON(ctx, http.MethodGet, "/api/models/files?repo="+url.QueryEscape(repoID), nil, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// DownloadModel starts (asynchronously, backend-side) downloading filename
// from repoID. Poll DownloadProgress to track it — this call itself returns
// as soon as the download is queued, not when it finishes.
func (c *Client) DownloadModel(ctx context.Context, repoID, filename string, expectedSize int64) error {
	return c.doJSON(ctx, http.MethodPost, "/api/models/download", map[string]any{
		"repo_id":       repoID,
		"filename":      filename,
		"expected_size": expectedSize,
	}, nil)
}

// ModelDownloadProgress mirrors modelstore.DownloadProgress.
type ModelDownloadProgress struct {
	Active     bool    `json:"active"`
	RepoID     string  `json:"repo_id"`
	Filename   string  `json:"filename"`
	TotalBytes int64   `json:"total_bytes"`
	Downloaded int64   `json:"downloaded"`
	Percent    float64 `json:"percent"`
	Speed      string  `json:"speed"`
	Error      string  `json:"error,omitempty"`
}

// DownloadProgress returns every currently-active download's progress.
func (c *Client) DownloadProgress(ctx context.Context) ([]ModelDownloadProgress, error) {
	var progress []ModelDownloadProgress
	if err := c.doJSON(ctx, http.MethodGet, "/api/models/download/progress", nil, &progress); err != nil {
		return nil, err
	}
	return progress, nil
}
