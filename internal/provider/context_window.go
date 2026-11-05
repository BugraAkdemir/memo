// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"memo/internal/routecatalog"
)

// A model's context window is a fact the vendor publishes, so Memo asks for it
// instead of deriving it from the model's name. Where it asks, in order:
//
//  1. the user's own ContextTokens for that provider (an explicit choice);
//  2. the Subscriptions sidecar's published capability registry;
//  3. the provider's live model metadata (ContextWindower below) — Anthropic
//     max_input_tokens, Gemini inputTokenLimit, OpenRouter context_length,
//     Ollama /api/ps + /api/show, llama-server /props, the context_length /
//     context_window / max_model_len fields other gateways put on /models;
//  4. the models.dev catalogue (routecatalog), exact ids only;
//  5. nothing: 0 means "unknown", never a guess.
//
// Every network read happens off the caller's goroutine: CachedContextWindow
// answers from memory and starts a refresh when it has nothing fresh, so the
// context gauge and the prompt budget never wait on a vendor.

// ContextWindower is implemented by providers that can report a model's
// context window, in tokens, from their own API. 0 with a nil error means the
// endpoint answered but does not publish one.
type ContextWindower interface {
	ContextWindow(ctx context.Context, model string) (int, error)
}

const (
	windowTTL          = 6 * time.Hour
	windowRetry        = time.Minute
	windowFetchTimeout = 8 * time.Second
)

type windowEntry struct {
	tokens   int
	source   string
	at       time.Time // last successful answer
	tried    time.Time // last attempt, successful or not
	inflight bool
}

var windowCache = struct {
	mu sync.Mutex
	m  map[string]*windowEntry
}{m: map[string]*windowEntry{}}

// Seams for tests: the live provider lookup and the models.dev lookup.
var (
	windowFromProvider = providerContextWindow
	windowFromCatalog  = catalogContextWindow
)

// catalogVendor is the models.dev vendor key that owns a provider type's own
// endpoint — an identity mapping, not a capacity: the vendor's entry for the
// model says the number. Types with no entry (custom gateways, Ollama, …) are
// matched by endpoint or by an unambiguous model id instead.
var catalogVendor = map[ProviderType]string{
	ProviderClaude:     "anthropic",
	ProviderGemini:     "google",
	ProviderOpenAI:     "openai",
	ProviderGroq:       "groq",
	ProviderGrok:       "xai",
	ProviderOpenRouter: "openrouter",
}

func windowKey(cfg ProviderConfig, model string) string {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = DefaultBaseURL(cfg.Type)
	}
	// No credential in the key: two keys for one endpoint see one model list.
	return string(cfg.Type) + "|" + base + "|" + model
}

// CachedContextWindow returns the context window of model on cfg and where the
// figure came from ("config", "subscriptions", "api", "models.dev"). tokens is
// 0 when nothing published one yet; a refresh is then running in the
// background and a later call answers. Never blocks on the network.
func CachedContextWindow(cfg ProviderConfig, model string) (tokens int, source string) {
	if model == "" {
		model = cfg.Model
	}
	if cfg.ContextTokens > 0 {
		return cfg.ContextTokens, "config"
	}
	if model == "" {
		return 0, ""
	}
	if cfg.Name == "Subscriptions" && cfg.Type == ProviderCustom {
		if w := SubscriptionContextWindow(cfg, model); w > 0 {
			return w, "subscriptions"
		}
		return 0, ""
	}
	key := windowKey(cfg, model)
	now := time.Now()
	windowCache.mu.Lock()
	e := windowCache.m[key]
	if e == nil {
		e = &windowEntry{}
		windowCache.m[key] = e
	}
	fresh := e.tokens > 0 && now.Sub(e.at) < windowTTL
	retryOK := now.Sub(e.tried) >= windowRetry
	start := !fresh && retryOK && !e.inflight
	if start {
		e.inflight = true
		e.tried = now
	}
	tokens, source = e.tokens, e.source
	windowCache.mu.Unlock()
	if start {
		go func() { _, _ = refreshContextWindow(context.Background(), cfg, model, key) }()
	}
	return tokens, source
}

// WarmContextWindow is CachedContextWindow's blocking sibling: it asks now and
// returns what it learned. For callers already off the hot path (a model was
// just picked, a provider was just saved, a test).
func WarmContextWindow(ctx context.Context, cfg ProviderConfig, model string) (int, string) {
	if model == "" {
		model = cfg.Model
	}
	if cfg.ContextTokens > 0 {
		return cfg.ContextTokens, "config"
	}
	if cfg.Name == "Subscriptions" && cfg.Type == ProviderCustom {
		if w := SubscriptionContextWindow(cfg, model); w > 0 {
			return w, "subscriptions"
		}
		return 0, ""
	}
	if model == "" {
		return 0, ""
	}
	key := windowKey(cfg, model)
	windowCache.mu.Lock()
	if e := windowCache.m[key]; e != nil && e.tokens > 0 && time.Since(e.at) < windowTTL {
		n, src := e.tokens, e.source
		windowCache.mu.Unlock()
		return n, src
	}
	windowCache.mu.Unlock()
	return refreshContextWindow(ctx, cfg, model, key)
}

func refreshContextWindow(ctx context.Context, cfg ProviderConfig, model, key string) (int, string) {
	ctx, cancel := context.WithTimeout(ctx, 2*windowFetchTimeout)
	defer cancel()
	n, src := 0, ""
	if v, err := windowFromProvider(ctx, cfg, model); err == nil && v > 0 {
		n, src = v, "api"
	}
	if n == 0 {
		endpoint := strings.TrimRight(cfg.BaseURL, "/")
		if endpoint == "" {
			endpoint = DefaultBaseURL(cfg.Type)
		}
		if v := windowFromCatalog(ctx, cfg.Type, endpoint, model); v > 0 {
			n, src = v, "models.dev"
		}
	}
	windowCache.mu.Lock()
	e := windowCache.m[key]
	if e == nil {
		e = &windowEntry{}
		windowCache.m[key] = e
	}
	e.inflight = false
	e.tried = time.Now()
	if n > 0 {
		e.tokens, e.source, e.at = n, src, e.tried
	}
	tokens, source := e.tokens, e.source // a failed refresh keeps the last good figure
	windowCache.mu.Unlock()
	return tokens, source
}

// ResetContextWindowCache forgets every cached figure (tests; a provider's
// endpoint or key was edited).
func ResetContextWindowCache() {
	windowCache.mu.Lock()
	windowCache.m = map[string]*windowEntry{}
	windowCache.mu.Unlock()
}

func providerContextWindow(ctx context.Context, cfg ProviderConfig, model string) (int, error) {
	c := cfg
	c.Model = model
	p, err := NewProvider(c)
	if err != nil {
		return 0, err
	}
	w, ok := p.(ContextWindower)
	if !ok {
		return 0, nil
	}
	ctx, cancel := context.WithTimeout(ctx, windowFetchTimeout)
	defer cancel()
	return w.ContextWindow(ctx, model)
}

// catalogContextWindow reads the models.dev snapshot the app keeps loaded
// (routecatalog.Shared().Ensure, run at startup). It does not download
// anything itself, so a unit test or a provider probe never writes a catalogue
// file as a side effect.
func catalogContextWindow(_ context.Context, typ ProviderType, endpoint, model string) int {
	c := routecatalog.Shared()
	if vendor, ok := catalogVendor[typ]; ok {
		if m, ok := c.LookupVendor(vendor, strings.TrimPrefix(model, "models/")); ok && m.Limit.Context > 0 {
			return m.Limit.Context
		}
	}
	if m, ok := c.Lookup(endpoint, model); ok && m.Limit.Context > 0 {
		return m.Limit.Context
	}
	return 0
}

// --- wire helpers -----------------------------------------------------------

func fetchJSON(ctx context.Context, client *http.Client, method, u string, body any, auth func(*http.Request), out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth != nil {
		auth(req)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return fmt.Errorf("%s %s: HTTP %d", method, u, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(out)
}

// windowKeys are the field names gateways use for "how many tokens fit" on a
// model entry, most specific first. Output-only limits (max_tokens,
// max_completion_tokens) are deliberately absent.
var windowKeys = []string{
	"context_length", "context_window", "max_context_length", "max_model_len",
	"max_input_tokens", "input_token_limit", "inputTokenLimit", "context_size",
}

// windowOfEntry reads a model entry from a /models list.
func windowOfEntry(m map[string]any) int {
	for _, k := range windowKeys {
		if n := asPositiveInt(m[k]); n > 0 {
			return n
		}
	}
	if tp, ok := m["top_provider"].(map[string]any); ok {
		if n := asPositiveInt(tp["context_length"]); n > 0 {
			return n
		}
	}
	return 0
}

func asPositiveInt(v any) int {
	if n, ok := v.(float64); ok && n > 0 && n == float64(int(n)) {
		return int(n)
	}
	return 0
}

func sameModelID(a, b string) bool {
	return strings.TrimPrefix(a, "models/") == strings.TrimPrefix(b, "models/")
}

// --- OpenAI-compatible (openai, custom, openrouter, groq, grok, kilo, …) -----

// ContextWindow reads the window from the model's entry on /models. OpenAI's
// own list carries none (0, nil — the catalogue answers); OpenRouter, Groq,
// Mistral, Together, vLLM and others do.
func (p *openAIProvider) ContextWindow(ctx context.Context, model string) (int, error) {
	if model == "" {
		model = p.model
	}
	var list struct {
		Data []map[string]any `json:"data"`
	}
	if err := fetchJSON(ctx, p.client, http.MethodGet, p.baseURL+"/models", nil, p.setAuth, &list); err != nil {
		return 0, err
	}
	for _, m := range list.Data {
		if id, _ := m["id"].(string); sameModelID(id, model) {
			return windowOfEntry(m), nil
		}
	}
	return 0, nil
}

// serverOrigin is baseURL without its OpenAI-style "/v1" suffix: Ollama's and
// llama-server's own endpoints (/api/show, /props) live beside it.
func serverOrigin(baseURL string) string {
	return strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1")
}

// ContextWindow asks Ollama what the model actually runs with, which is not
// what it was trained for: a loaded model reports its live context_length on
// /api/ps; otherwise a Modelfile's `num_ctx` parameter; only then the trained
// maximum in model_info (<arch>.context_length).
func (p *ollamaProvider) ContextWindow(ctx context.Context, model string) (int, error) {
	if model == "" {
		model = p.model
	}
	origin := serverOrigin(p.baseURL)
	var ps struct {
		Models []struct {
			Name          string `json:"name"`
			Model         string `json:"model"`
			ContextLength int    `json:"context_length"`
		} `json:"models"`
	}
	if err := fetchJSON(ctx, p.client, http.MethodGet, origin+"/api/ps", nil, p.setAuth, &ps); err == nil {
		for _, m := range ps.Models {
			if (sameModelID(m.Name, model) || sameModelID(m.Model, model)) && m.ContextLength > 0 {
				return m.ContextLength, nil
			}
		}
	}
	var show struct {
		Parameters string         `json:"parameters"`
		ModelInfo  map[string]any `json:"model_info"`
	}
	if err := fetchJSON(ctx, p.client, http.MethodPost, origin+"/api/show", map[string]any{"model": model}, p.setAuth, &show); err != nil {
		return 0, err
	}
	for _, line := range strings.Split(show.Parameters, "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[0] == "num_ctx" {
			var n int
			if _, err := fmt.Sscanf(f[1], "%d", &n); err == nil && n > 0 {
				return n, nil
			}
		}
	}
	for k, v := range show.ModelInfo {
		if strings.HasSuffix(k, ".context_length") {
			if n := asPositiveInt(v); n > 0 {
				return n, nil
			}
		}
	}
	return 0, nil
}

// ContextWindow reads llama-server's running window from /props
// (default_generation_settings.n_ctx; older builds put n_ctx at the top).
func (p *llamaCPPProvider) ContextWindow(ctx context.Context, model string) (int, error) {
	var props struct {
		NCtx     int `json:"n_ctx"`
		Settings struct {
			NCtx int `json:"n_ctx"`
		} `json:"default_generation_settings"`
	}
	if err := fetchJSON(ctx, p.client, http.MethodGet, serverOrigin(p.baseURL)+"/props", nil, p.setAuth, &props); err != nil {
		return 0, err
	}
	if props.Settings.NCtx > 0 {
		return props.Settings.NCtx, nil
	}
	return props.NCtx, nil
}

// --- Anthropic ---------------------------------------------------------------

// ContextWindow reads max_input_tokens from GET /v1/models/{id}.
func (p *claudeProvider) ContextWindow(ctx context.Context, model string) (int, error) {
	if model == "" {
		model = p.model
	}
	var m struct {
		MaxInputTokens int `json:"max_input_tokens"`
	}
	auth := func(r *http.Request) { p.setAuth(ctx, r) }
	if err := fetchJSON(ctx, p.client, http.MethodGet, p.baseURL+"/models/"+url.PathEscape(model), nil, auth, &m); err != nil {
		return 0, err
	}
	return m.MaxInputTokens, nil
}

// --- Gemini --------------------------------------------------------------------

// ContextWindow reads inputTokenLimit from models.get.
func (p *geminiProvider) ContextWindow(ctx context.Context, model string) (int, error) {
	if model == "" {
		model = p.model
	}
	name := strings.TrimPrefix(model, "models/")
	var m struct {
		InputTokenLimit int `json:"inputTokenLimit"`
	}
	auth := func(r *http.Request) { r.Header.Set("x-goog-api-key", p.apiKey) }
	if err := fetchJSON(ctx, p.client, http.MethodGet, p.baseURL+"/models/"+url.PathEscape(name), nil, auth, &m); err != nil {
		return 0, err
	}
	return m.InputTokenLimit, nil
}
