package replcli

import (
	"context"
	"net/http"
)

// LocalModel mirrors the fields of modelstore.LocalModel the REPL needs to
// list and pick models by name.
type LocalModel struct {
	Filename    string `json:"filename"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	IsEmbedding bool   `json:"is_embedding"`
}

// ModelStatus mirrors llama.ServerStatus.
type ModelStatus struct {
	Running   bool   `json:"running"`
	ModelPath string `json:"model_path"`
	ModelName string `json:"model_name"`
	Port      int    `json:"port"`
}

// ProviderConfig mirrors the fields of provider.ProviderConfig the REPL's
// /connect command needs to configure an external, OpenAI-compatible
// endpoint (custom base URL + API key + model).
type ProviderConfig struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	Enabled bool   `json:"enabled"`
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

// StartModel loads a local chat model. ctxSize=0, port=0 and gpuLayers=-1
// all mean "use the backend's defaults" (4096 ctx, auto port, auto GPU
// layers) — see internal/llama.Server.Start.
func (c *Client) StartModel(ctx context.Context, path string, ctxSize, port, gpuLayers int) error {
	return c.doJSON(ctx, http.MethodPost, "/api/models/start", map[string]any{
		"path":       path,
		"ctx_size":   ctxSize,
		"port":       port,
		"gpu_layers": gpuLayers,
	}, nil)
}

// StartEmbedding loads a local embedding model. gpuLayers=-1 means "auto".
func (c *Client) StartEmbedding(ctx context.Context, path string, gpuLayers int) error {
	return c.doJSON(ctx, http.MethodPost, "/api/models/embedding/start", map[string]any{
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
