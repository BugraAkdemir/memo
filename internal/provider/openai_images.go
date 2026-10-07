package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"memo/internal/logx"
)

// Image generation for OpenAI-compatible ("custom") endpoints — in practice the
// Subscriptions sidecar, whose Codex accounts serve gpt-image-* and whose
// Antigravity accounts serve Gemini image models.
//
// Nothing here names an image model. Whether a model makes pictures is asked of
// the endpoint itself, because a model id says little (and the list changes):
//
//  1. POST {base}/images/generations with only a model. The sidecar answers in
//     about a millisecond, locally, before any upstream call: a model it serves
//     there gets "prompt is required", every other one "is not supported …".
//     So the answer costs nothing and cannot trip a vendor rate limit — unlike
//     probing /chat/completions, which reaches the vendor (1–2s each, and a
//     Google 500 on an image model put that account's model into cooldown).
//  2. Failing that, the endpoint's Gemini-style catalogue (GET {root}/v1beta/models)
//     lists each model's supportedOutputModalities; one that outputs "image" is a
//     chat model that draws when asked with modalities=[image,text].
//
// A model neither test recognises is an ordinary chat model, and a probe that
// fails (network) is not remembered, so a blip cannot reroute chats.

type imageKind int

const (
	imageKindNone     imageKind = iota // an ordinary chat model
	imageKindEndpoint                  // served by /images/generations and /images/edits
	imageKindChat                      // chat model that returns images (modalities=[image,text])
)

const (
	imageKindTTLFound = 30 * time.Minute
	imageKindTTLNone  = 10 * time.Minute
	imageProbeTimeout = 3 * time.Second // the sidecar answers in ~1ms; a hung endpoint must not stall a turn for long
	// imageKindTTLFailed: after a probe that could not get an answer, treat the
	// model as a chat model for this long instead of paying the probe timeout on
	// every turn against a slow or unreachable endpoint.
	imageKindTTLFailed = 30 * time.Second
)

type imageKindEntry struct {
	kind   imageKind
	at     time.Time
	failed bool // the probe itself failed; remembered only briefly
}

type imageCatalogEntry struct {
	imageOut map[string]bool // model id → outputs images
	at       time.Time
}

var (
	imageKindMu    sync.Mutex
	imageKindCache = map[string]imageKindEntry{}    // baseURL\x00model
	imageCatalogs  = map[string]imageCatalogEntry{} // baseURL
)

// resetOpenAIImageCaches drops what was learned about image models. Test-only.
func resetOpenAIImageCaches() {
	imageKindMu.Lock()
	defer imageKindMu.Unlock()
	imageKindCache = map[string]imageKindEntry{}
	imageCatalogs = map[string]imageCatalogEntry{}
}

// IsImageOnlyModel implements ImageGenerator. Only a "custom" endpoint is asked
// at all: a real OpenAI key keeps the chat path it always had. Best effort, as
// the interface requires — any doubt answers false.
func (p *openAIProvider) IsImageOnlyModel(ctx context.Context, model string) bool {
	if p.provType != ProviderCustom || model == "" {
		return false
	}
	kind, err := p.imageKindOf(ctx, model)
	if err != nil {
		logx.Printf("PROVIDER: [custom] could not tell whether %q makes images (%v) — treating it as a chat model", model, err)
		return false
	}
	return kind != imageKindNone
}

func (p *openAIProvider) imageKindOf(ctx context.Context, model string) (imageKind, error) {
	key := p.baseURL + "\x00" + model
	imageKindMu.Lock()
	e, ok := imageKindCache[key]
	imageKindMu.Unlock()
	if ok {
		ttl := imageKindTTLNone
		switch {
		case e.failed:
			ttl = imageKindTTLFailed
		case e.kind != imageKindNone:
			ttl = imageKindTTLFound
		}
		if time.Since(e.at) < ttl {
			return e.kind, nil
		}
	}

	pctx, cancel := context.WithTimeout(ctx, imageProbeTimeout)
	defer cancel()

	kind := imageKindNone
	supported, err := p.imagesEndpointServes(pctx, model)
	if err != nil {
		imageKindMu.Lock()
		imageKindCache[key] = imageKindEntry{kind: imageKindNone, at: time.Now(), failed: true}
		imageKindMu.Unlock()
		return imageKindNone, err
	}
	if supported {
		kind = imageKindEndpoint
	} else if out, cerr := p.catalogOutputsImages(pctx, model); cerr == nil && out {
		kind = imageKindChat
	}

	imageKindMu.Lock()
	imageKindCache[key] = imageKindEntry{kind: kind, at: time.Now()}
	imageKindMu.Unlock()
	return kind, nil
}

// imagesEndpointServes asks the endpoint whether /images/generations accepts the
// model, without a prompt so that nothing is generated. "prompt is required"
// means the model passed the endpoint's own model check.
func (p *openAIProvider) imagesEndpointServes(ctx context.Context, model string) (bool, error) {
	body, _ := json.Marshal(map[string]string{"model": model})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	p.setAuth(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		return false, nil // an endpoint without the images API at all
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode >= 500 {
		return false, fmt.Errorf("images endpoint answered %d", resp.StatusCode)
	}
	return resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(string(raw)), "prompt is required"), nil
}

// catalogOutputsImages reads the endpoint's Gemini-style model catalogue (cached
// for the whole base URL) and reports whether model lists "image" among its
// output modalities.
func (p *openAIProvider) catalogOutputsImages(ctx context.Context, model string) (bool, error) {
	imageKindMu.Lock()
	c, ok := imageCatalogs[p.baseURL]
	imageKindMu.Unlock()
	if ok && time.Since(c.at) < imageKindTTLNone {
		return c.imageOut[model], nil
	}

	root := strings.TrimSuffix(p.baseURL, "/v1")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, root+"/v1beta/models", nil)
	if err != nil {
		return false, err
	}
	// Gemini-style header ONLY: sent together with a Bearer token the endpoint
	// answers in its OpenAI shape ({"data":[…]}, no modalities), which parses to
	// an empty catalogue and silently loses every image-capable chat model.
	if p.apiKey != "" {
		req.Header.Set("x-goog-api-key", p.apiKey)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("catalogue answered %d", resp.StatusCode)
	}
	var cat struct {
		Models []struct {
			Name                      string   `json:"name"`
			SupportedOutputModalities []string `json:"supportedOutputModalities"`
		} `json:"models"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&cat); err != nil {
		return false, err
	}
	out := make(map[string]bool, len(cat.Models))
	for _, m := range cat.Models {
		id := strings.TrimPrefix(m.Name, "models/")
		for _, mod := range m.SupportedOutputModalities {
			if strings.EqualFold(mod, "image") {
				out[id] = true
			}
		}
	}
	imageKindMu.Lock()
	imageCatalogs[p.baseURL] = imageCatalogEntry{imageOut: out, at: time.Now()}
	imageKindMu.Unlock()
	return out[model], nil
}

// GenerateImage implements ImageGenerator: text-to-image, or — when req.Images
// carries source pictures — image-to-image.
func (p *openAIProvider) GenerateImage(ctx context.Context, req ImageRequest) (*ImageResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("image prompt is empty")}
	}
	kind, err := p.imageKindOf(ctx, model)
	if err != nil {
		// Could not ask; the images endpoint is the one that is right for the
		// models that reach here, and its own error will say if it is not.
		kind = imageKindEndpoint
	}
	if kind == imageKindChat {
		return p.generateImageViaChat(ctx, model, req)
	}
	return p.generateImageViaEndpoint(ctx, model, req)
}

func (p *openAIProvider) generateImageViaEndpoint(ctx context.Context, model string, req ImageRequest) (*ImageResponse, error) {
	body := map[string]any{
		"model":           model,
		"prompt":          req.Prompt,
		"response_format": "b64_json",
	}
	if req.N > 0 {
		body["n"] = req.N
	}
	if req.Size != "" {
		body["size"] = req.Size
	}
	if req.OutputFormat != "" {
		body["output_format"] = req.OutputFormat
	}
	path := "/images/generations"
	if len(req.Images) > 0 {
		path = "/images/edits"
		imgs := make([]map[string]string, 0, len(req.Images))
		for _, in := range req.Images {
			imgs = append(imgs, map[string]string{"image_url": in.DataURL()})
		}
		body["images"] = imgs
	}

	raw, err := p.postJSON(ctx, path, body)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
		OutputFormat string `json:"output_format"`
		Usage        struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("decode: %w", err)}
	}
	mediaType := "image/png"
	switch strings.ToLower(result.OutputFormat) {
	case "jpeg", "jpg":
		mediaType = "image/jpeg"
	case "webp":
		mediaType = "image/webp"
	}
	out := &ImageResponse{
		Model: model,
		Usage: &Usage{PromptTokens: result.Usage.InputTokens, CompletionTokens: result.Usage.OutputTokens, TotalTokens: result.Usage.TotalTokens},
	}
	for _, d := range result.Data {
		if d.B64JSON != "" {
			out.Images = append(out.Images, GeneratedImage{B64JSON: d.B64JSON, MediaType: mediaType})
		}
	}
	if len(out.Images) == 0 {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("image response carried no image data")}
	}
	return out, nil
}

// generateImageViaChat draws with a chat model that returns pictures: the
// request carries modalities=[image,text] (and any source pictures as ordinary
// image_url parts), and the pictures come back in message.images as data URLs.
func (p *openAIProvider) generateImageViaChat(ctx context.Context, model string, req ImageRequest) (*ImageResponse, error) {
	parts := []map[string]any{{"type": "text", "text": req.Prompt}}
	for _, in := range req.Images {
		parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]string{"url": in.DataURL()}})
	}
	body := map[string]any{
		"model":      model,
		"modalities": []string{"image", "text"},
		"messages":   []map[string]any{{"role": "user", "content": parts}},
	}
	if req.AspectRatio != "" {
		body["image_config"] = map[string]string{"aspect_ratio": req.AspectRatio}
	}
	raw, err := p.postJSON(ctx, "/chat/completions", body)
	if err != nil {
		return nil, err
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"`
				Images  []struct {
					ImageURL struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"images"`
			} `json:"message"`
		} `json:"choices"`
		Usage openAIUsage `json:"usage"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("decode: %w", err)}
	}
	out := &ImageResponse{
		Model: model,
		Usage: &Usage{PromptTokens: result.Usage.PromptTokens, CompletionTokens: result.Usage.CompletionTokens, TotalTokens: result.Usage.TotalTokens},
	}
	var text string
	if len(result.Choices) > 0 {
		m := result.Choices[0].Message
		if s, ok := m.Content.(string); ok {
			text = strings.TrimSpace(s)
		}
		for _, im := range m.Images {
			if in, ok := ParseImageInput(im.ImageURL.URL); ok {
				out.Images = append(out.Images, GeneratedImage{B64JSON: in.B64JSON, MediaType: in.MediaType})
			}
		}
	}
	if len(out.Images) == 0 {
		msg := "the model returned no image"
		if text != "" {
			msg += ": " + text
		}
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("%s", msg)}
	}
	return out, nil
}

// postJSON sends body to {base}{path} and returns the 200 response body.
func (p *openAIProvider) postJSON(ctx context.Context, path string, body any) ([]byte, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("marshal: %w", err)}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	p.setAuth(req)
	resp, err := p.client.Do(req)
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
	return raw, nil
}
