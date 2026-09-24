package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/logx"
	"net/http"
	"sync"
	"time"
)

// OpenRouter serves image-generation models from a dedicated endpoint
// (POST {base}/images) and rejects them on /chat/completions with an HTTP
// 404 whose message literally says "... is an image generation model and
// cannot be used with the chat/completions endpoint. Use the /api/v1/images
// endpoint instead." Before this file, Memo had no image path at all, so
// selecting such a model (e.g. inclusionai/ming-image-0.1-design) made
// *every* turn fail with that 404 — the whole provider chain fell through
// and the chat showed "all providers failed".
//
// Detection is catalog-driven rather than a guess at the model id: the
// image-only models are not even in OpenRouter's default /models response
// (458 entries, none with output_modalities == ["image"]) — they are only
// returned when the output_modalities=image filter is passed (57 entries,
// verified live 2026-09-24). That filtered list is therefore exactly the
// set of ids that must not go to /chat/completions, so it is fetched once
// and cached instead of pattern-matching on substrings like "image" or
// "flux", which would both miss models and misfire on chat models whose
// name merely mentions images.

// imageModelCacheTTL bounds how stale the image-only model set may get.
// OpenRouter adds models continuously; an hour keeps a long-running Memo
// process current without re-fetching a 57-entry catalog per message.
const imageModelCacheTTL = time.Hour

type imageModelCacheEntry struct {
	ids       map[string]bool
	fetchedAt time.Time
}

var (
	imageModelCacheMu sync.Mutex
	imageModelCache   = map[string]imageModelCacheEntry{} // keyed by base URL
)

// resetImageModelCache drops every cached catalog. Test-only.
func resetImageModelCache() {
	imageModelCacheMu.Lock()
	defer imageModelCacheMu.Unlock()
	imageModelCache = map[string]imageModelCacheEntry{}
}

// IsImageOnlyModel reports whether model appears in OpenRouter's
// image-output-only catalog. Best-effort by contract (see ImageGenerator):
// a catalog fetch that fails reports false so an unrelated network blip
// can't silently divert a normal chat model to the images endpoint.
func (p *openRouterProvider) IsImageOnlyModel(ctx context.Context, model string) bool {
	if model == "" {
		return false
	}
	ids, err := p.imageOnlyModelIDs(ctx)
	if err != nil {
		logx.Printf("PROVIDER: [openrouter] image-model catalog unavailable (%v) — treating %q as a chat model", err, model)
		return false
	}
	return ids[model]
}

func (p *openRouterProvider) imageOnlyModelIDs(ctx context.Context) (map[string]bool, error) {
	imageModelCacheMu.Lock()
	entry, ok := imageModelCache[p.baseURL]
	imageModelCacheMu.Unlock()
	if ok && time.Since(entry.fetchedAt) < imageModelCacheTTL {
		return entry.ids, nil
	}

	url := p.baseURL + "/models?output_modalities=image"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// The catalog is public — no key needed — but send one when we have it
	// so the request is attributed to the user's account like every other.
	p.setAuth(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, p.parseError(resp)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	ids := make(map[string]bool, len(result.Data))
	for _, m := range result.Data {
		if m.ID != "" {
			ids[m.ID] = true
		}
	}

	imageModelCacheMu.Lock()
	imageModelCache[p.baseURL] = imageModelCacheEntry{ids: ids, fetchedAt: time.Now()}
	imageModelCacheMu.Unlock()
	return ids, nil
}

type openRouterImageRequest struct {
	Model        string `json:"model"`
	Prompt       string `json:"prompt"`
	N            int    `json:"n,omitempty"`
	Size         string `json:"size,omitempty"`
	AspectRatio  string `json:"aspect_ratio,omitempty"`
	OutputFormat string `json:"output_format,omitempty"`
}

type openRouterImageResponse struct {
	Created int64 `json:"created"`
	Data    []struct {
		B64JSON   string `json:"b64_json"`
		MediaType string `json:"media_type"`
	} `json:"data"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// GenerateImage implements ImageGenerator against POST {base}/images.
func (p *openRouterProvider) GenerateImage(ctx context.Context, req ImageRequest) (*ImageResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}
	if req.Prompt == "" {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("image prompt is empty")}
	}

	body := openRouterImageRequest{
		Model:        model,
		Prompt:       req.Prompt,
		N:            req.N,
		Size:         req.Size,
		AspectRatio:  req.AspectRatio,
		OutputFormat: req.OutputFormat,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("marshal: %w", err)}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/images", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	p.setAuth(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, p.wrapError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, p.parseError(resp)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("read body: %w", err)}
	}

	var result openRouterImageResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("decode: %w", err)}
	}
	if len(result.Data) == 0 {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("image response contained no images")}
	}

	out := &ImageResponse{
		Model:  model,
		Images: make([]GeneratedImage, 0, len(result.Data)),
		Usage: &Usage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}
	for _, d := range result.Data {
		if d.B64JSON == "" {
			continue
		}
		out.Images = append(out.Images, GeneratedImage{B64JSON: d.B64JSON, MediaType: d.MediaType})
	}
	if len(out.Images) == 0 {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("image response carried no decodable image data")}
	}
	return out, nil
}
