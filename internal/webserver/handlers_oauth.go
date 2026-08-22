package webserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/logx"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"memo/internal/provider"
)

type openRouterState struct {
	mu  sync.Mutex
	key string
}

var orState openRouterState

// openRouterModelsURL is a var (not a const) purely so tests can point it
// at an httptest server instead of the real OpenRouter API — see
// fetchOpenRouterModelEffortLevels below.
var openRouterModelsURL = "https://openrouter.ai/api/v1/models"

// claudeModelsBaseURL/geminiModelsBaseURL: same test-injection pattern as
// openRouterModelsURL, for fetchClaudeModelEffortLevels/
// fetchGeminiModelEffortLevels below.
var claudeModelsBaseURL = "https://api.anthropic.com/v1/models"
var geminiModelsBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

func (s *Server) handleOpenRouterConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}

	var req struct {
		APIKey string `json:"api_key"`
		Model  string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		http.Error(w, "API Key gerekli", http.StatusBadRequest)
		return
	}

	// Validate key
	valid, err := validateOpenRouterKey(apiKey)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"status": "error",
			"error":  "Doğrulama hatası: " + err.Error(),
		})
		return
	}
	if !valid {
		writeJSON(w, map[string]interface{}{
			"status": "error",
			"error":  "API Key geçersiz. openrouter.ai/keys adresinden kontrol et.",
		})
		return
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = "openai/gpt-4o"
	}

	cfg := provider.ProviderConfig{
		Type:    provider.ProviderOpenRouter,
		Name:    "OpenRouter",
		APIKey:  apiKey,
		BaseURL: provider.DefaultBaseURL(provider.ProviderOpenRouter),
		Model:   model,
		Enabled: true,
	}
	if s.fullBridge != nil {
		if err := s.fullBridge.UpdateProvider(cfg); err != nil {
			logx.Printf("OpenRouter save error: %v", err)
			writeJSON(w, map[string]interface{}{
				"status": "error",
				"error":  "Kayıt hatası: " + err.Error(),
			})
			return
		}
		// Set active provider to the first enabled OpenRouter config's name.
		// Default to the config we just saved (a valid Name), never the type string.
		name := cfg.Name
		if providers := s.fullBridge.GetProviders(); len(providers) > 0 {
			for _, p := range providers {
				if p.Type == provider.ProviderOpenRouter && p.Enabled {
					name = p.Name
					break
				}
			}
		}
		s.fullBridge.SetActiveProvider(name)
	}

	orState.mu.Lock()
	orState.key = apiKey
	orState.mu.Unlock()

	writeJSON(w, map[string]interface{}{
		"status":   "done",
		"provider": "openrouter",
	})
}

// effortDiscoveredTypes are the provider types with a real, live,
// per-model capability endpoint — see effort.go's package doc comment for
// why these four and no others. All four need a model selected before
// there's anything to discover.
var effortDiscoveredTypes = map[provider.ProviderType]bool{
	provider.ProviderOpenRouter: true,
	provider.ProviderClaude:     true,
	provider.ProviderGemini:     true,
	provider.ProviderOllama:     true,
}

// findProviderAPIKeyFor returns the first enabled config's API key for
// type t, or "" if none is configured — the same "first enabled match"
// lookup this file already did inline for OpenRouter, now shared by
// Claude/Gemini's live effort-level discovery too.
func (s *Server) findProviderAPIKeyFor(t provider.ProviderType) string {
	for _, p := range s.fullBridge.GetProviders() {
		if p.Type == t && p.APIKey != "" {
			return p.APIKey
		}
	}
	return ""
}

// findProviderBaseURLFor mirrors findProviderAPIKeyFor for BaseURL —
// Ollama's discovery needs the user's actual configured endpoint (a remote
// Ollama host, a non-default port, ...), not a hardcoded localhost guess.
func (s *Server) findProviderBaseURLFor(t provider.ProviderType) string {
	for _, p := range s.fullBridge.GetProviders() {
		if p.Type == t && p.BaseURL != "" {
			return p.BaseURL
		}
	}
	return ""
}

// handleProviderEffortLevels implements GET /api/providers/effort-levels
// ?type=<provider type>&model=<model id> — the "which reasoning-effort
// values does this provider+model actually accept" lookup behind both UI
// surfaces (provider config dialog, chat screen quick-select). Every type
// in effortDiscoveredTypes is queried live, per the exact model selected;
// every other type has no known capability signal at all (see effort.go's
// package doc comment) and gets an empty list — never a guessed one.
func (s *Server) handleProviderEffortLevels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	ptype := provider.ProviderType(r.URL.Query().Get("type"))
	if ptype == "" {
		http.Error(w, "type is required", http.StatusBadRequest)
		return
	}

	if !effortDiscoveredTypes[ptype] {
		levels := provider.EffortLevelsForType(ptype)
		if levels == nil {
			levels = []string{}
		}
		writeJSON(w, map[string]interface{}{"levels": levels})
		return
	}

	model := r.URL.Query().Get("model")
	if model == "" {
		// No model chosen yet — nothing to discover against. Not an
		// error: the UI just has nothing to show its dropdown until the
		// user picks a model, same as each vendor's own model picker
		// needing a selection before showing model-specific info.
		writeJSON(w, map[string]interface{}{"levels": []string{}})
		return
	}

	var levels []string
	var err error
	switch ptype {
	case provider.ProviderOpenRouter:
		apiKey := s.findProviderAPIKeyFor(provider.ProviderOpenRouter)
		if apiKey == "" {
			http.Error(w, "OpenRouter API key yapılandırılmamış", http.StatusBadRequest)
			return
		}
		levels, err = fetchOpenRouterModelEffortLevels(apiKey, model)
	case provider.ProviderClaude:
		apiKey := s.findProviderAPIKeyFor(provider.ProviderClaude)
		if apiKey == "" {
			http.Error(w, "Claude API key yapılandırılmamış", http.StatusBadRequest)
			return
		}
		levels, err = fetchClaudeModelEffortLevels(apiKey, model)
	case provider.ProviderGemini:
		apiKey := s.findProviderAPIKeyFor(provider.ProviderGemini)
		if apiKey == "" {
			http.Error(w, "Gemini API key yapılandırılmamış", http.StatusBadRequest)
			return
		}
		levels, err = fetchGeminiModelEffortLevels(apiKey, model)
	case provider.ProviderOllama:
		baseURL := s.findProviderBaseURLFor(provider.ProviderOllama)
		if baseURL == "" {
			baseURL = provider.DefaultBaseURL(provider.ProviderOllama)
		}
		levels, err = fetchOllamaModelEffortLevels(baseURL, model)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if levels == nil {
		levels = []string{}
	}
	writeJSON(w, map[string]interface{}{"levels": levels})
}

func (s *Server) handleOpenRouterModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}

	var req struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		// Try stored key from provider config
		if s.fullBridge != nil {
			providers := s.fullBridge.GetProviders()
			for _, p := range providers {
				if p.Type == provider.ProviderOpenRouter && p.Enabled {
					apiKey = p.APIKey
					break
				}
			}
		}
	}
	if apiKey == "" {
		writeJSON(w, map[string]interface{}{
			"status": "error",
			"error":  "API Key gerekli. Önce OpenRouter'ı API Provider'dan yapılandır.",
		})
		return
	}

	models, err := fetchOpenRouterModels(apiKey)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"status": "ok",
		"models": models,
	})
}

// ProviderModelInfo represents a model from an OpenRouter-shaped model
// catalog — shared by fetchOpenRouterModels (OpenRouter itself) and
// fetchKiloModels (Kilo Code's AI Gateway, which mirrors OpenRouter's
// /models response shape closely enough to reuse this same struct and the
// same rich, pricing/free-aware frontend model browser — see
// _ModelBrowserDialog in provider_config_dialog.dart).
type ProviderModelInfo struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	ContextLen  int     `json:"context_length"`
	PromptPrice float64 `json:"prompt_price"`
	CompPrice   float64 `json:"completion_price"`
	IsFree      bool    `json:"is_free"`
}

func fetchOpenRouterModels(apiKey string) ([]ProviderModelInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("request oluşturulamadı: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenRouter API hatası: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenRouter döndü %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			ContextLen  int    `json:"context_length"`
			Pricing     struct {
				Prompt     interface{} `json:"prompt"`
				Completion interface{} `json:"completion"`
			} `json:"pricing"`
			Architecture struct {
				Modality string `json:"modality"`
			} `json:"architecture"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse hatası: %w", err)
	}

	models := make([]ProviderModelInfo, 0, len(result.Data))
	for _, m := range result.Data {
		promptPrice := toFloat64(m.Pricing.Prompt)
		compPrice := toFloat64(m.Pricing.Completion)
		isFree := promptPrice == 0 && compPrice == 0
		if m.ID == "" {
			continue
		}
		models = append(models, ProviderModelInfo{
			ID:          m.ID,
			Name:        m.Name,
			Description: m.Description,
			ContextLen:  m.ContextLen,
			PromptPrice: promptPrice,
			CompPrice:   compPrice,
			IsFree:      isFree,
		})
	}

	return models, nil
}

// kiloModelsURL is a var (not a const) for the same test-injection reason
// as openRouterModelsURL above.
var kiloModelsURL = "https://api.kilo.ai/api/gateway/models"

// handleKiloModels implements POST /api/kilo/models — the rich,
// pricing/free-aware model browser behind Kilo Code's "Sağlayıcılardan
// modelleri gözat" button, same shape as handleOpenRouterModels but no API
// key requirement: unlike OpenRouter, Kilo's /models endpoint is public
// (kilo.ai/docs/gateway/models-and-providers — "No authentication is
// required"), so a user can browse and pick a model before ever entering a
// key. A key posted in the body is accepted but genuinely unused, kept only
// so the request shape matches every other provider's /models call for the
// frontend's sake.
func (s *Server) handleKiloModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	models, err := fetchKiloModels()
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"status": "ok",
		"models": models,
	})
}

// fetchKiloModels fetches Kilo's model catalog. Its /models response
// already carries a direct isFree boolean per model (verified live,
// 2026-08-22: 368 models, 18 free) — used as-is rather than re-derived from
// pricing like fetchOpenRouterModels does, since several of Kilo's own
// "auto-routing" models (e.g. kilo-auto/frontier) report pricing "-1"
// (routes through to whatever underlying model actually gets picked, not a
// fixed price) which a prompt==0-and-completion==0 heuristic would
// misclassify as free.
func fetchKiloModels() ([]ProviderModelInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", kiloModelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("request oluşturulamadı: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Kilo API hatası: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Kilo döndü %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			ContextLen  int    `json:"context_length"`
			Pricing     struct {
				Prompt     interface{} `json:"prompt"`
				Completion interface{} `json:"completion"`
			} `json:"pricing"`
			IsFree bool `json:"isFree"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse hatası: %w", err)
	}

	models := make([]ProviderModelInfo, 0, len(result.Data))
	for _, m := range result.Data {
		if m.ID == "" {
			continue
		}
		models = append(models, ProviderModelInfo{
			ID:          m.ID,
			Name:        m.Name,
			Description: m.Description,
			ContextLen:  m.ContextLen,
			PromptPrice: toFloat64(m.Pricing.Prompt),
			CompPrice:   toFloat64(m.Pricing.Completion),
			IsFree:      m.IsFree,
		})
	}

	return models, nil
}

// fetchOpenRouterModelEffortLevels is OpenRouter's one real point of
// runtime capability discovery (see provider/effort.go's package doc
// comment) — its /api/v1/models response includes a per-model "reasoning"
// object listing that exact model's supported_efforts, instead of Memo
// having to guess or hand-maintain a table like every other vendor
// requires. Returns (nil, nil) — not an error — for a model with no
// "reasoning" field at all (it genuinely doesn't support effort control),
// same as fetchOpenRouterModels above treats a missing field as absence,
// not failure.
func fetchOpenRouterModelEffortLevels(apiKey, modelID string) ([]string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", openRouterModelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("request oluşturulamadı: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenRouter API hatası: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenRouter döndü %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			ID        string `json:"id"`
			Reasoning *struct {
				SupportedEfforts []string `json:"supported_efforts"`
			} `json:"reasoning"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse hatası: %w", err)
	}

	for _, m := range result.Data {
		if m.ID == modelID {
			if m.Reasoning == nil {
				return nil, nil
			}
			return m.Reasoning.SupportedEfforts, nil
		}
	}
	return nil, fmt.Errorf("model %q OpenRouter kataloğunda bulunamadı", modelID)
}

// fetchClaudeModelEffortLevels queries Anthropic's GET /v1/models/{id} —
// verified live against current docs (2026-08-18) to return a real,
// per-model capabilities object, e.g. capabilities.effort.{low,medium,
// high,max,xhigh}.supported booleans and capabilities.thinking.supported.
// This is what makes effort control on Claude actually safe: sending
// adaptive-mode effort to a model whose capabilities.effort.supported is
// false 400s (see claude.go's claudeThinking doc comment) — gating on this
// live per-model answer instead of a hand-maintained table means Memo
// never offers a value a specific model will reject. Returns (nil, nil) —
// not an error — when the model has no effort capability at all.
func fetchClaudeModelEffortLevels(apiKey, modelID string) ([]string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", claudeModelsBaseURL+"/"+url.PathEscape(modelID), nil)
	if err != nil {
		return nil, fmt.Errorf("request oluşturulamadı: %w", err)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Claude API hatası: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Claude döndü %d: %s", resp.StatusCode, string(respBody))
	}

	type capSupport struct {
		Supported bool `json:"supported"`
	}
	var result struct {
		Capabilities *struct {
			Effort *struct {
				Supported bool       `json:"supported"`
				Low       capSupport `json:"low"`
				Medium    capSupport `json:"medium"`
				High      capSupport `json:"high"`
				XHigh     capSupport `json:"xhigh"`
				Max       capSupport `json:"max"`
			} `json:"effort"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse hatası: %w", err)
	}

	if result.Capabilities == nil || result.Capabilities.Effort == nil || !result.Capabilities.Effort.Supported {
		return nil, nil
	}
	eff := result.Capabilities.Effort
	var levels []string
	if eff.Low.Supported {
		levels = append(levels, "low")
	}
	if eff.Medium.Supported {
		levels = append(levels, "medium")
	}
	if eff.High.Supported {
		levels = append(levels, "high")
	}
	if eff.XHigh.Supported {
		levels = append(levels, "xhigh")
	}
	if eff.Max.Supported {
		levels = append(levels, "max")
	}
	return levels, nil
}

// fetchGeminiModelEffortLevels queries Google's GET
// /v1beta/models/{id} — verified live (2026-08-18) to return a per-model
// "thinking" boolean. Gemini's classic generateContent endpoint has no
// named effort levels of its own (see effort.go's GeminiThinkingBudgetForLevel),
// so this only gates WHETHER to offer provider.EffortLevelsForGemini()'s
// names at all for this specific model, not which subset — a model with
// thinking:false has no thinking control whatsoever, and offering a
// picker for it would be exactly the same class of mistake the OpenCode
// Zen/Go fix (effort.go) closed.
func fetchGeminiModelEffortLevels(apiKey, modelID string) ([]string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", geminiModelsBaseURL+"/"+url.PathEscape(modelID), nil)
	if err != nil {
		return nil, fmt.Errorf("request oluşturulamadı: %w", err)
	}
	req.Header.Set("x-goog-api-key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Gemini API hatası: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gemini döndü %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Thinking bool `json:"thinking"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse hatası: %w", err)
	}
	if !result.Thinking {
		return nil, nil
	}
	return provider.EffortLevelsForGemini(), nil
}

// fetchOllamaModelEffortLevels queries Ollama's NATIVE POST /api/show
// (not the OpenAI-compatibility endpoint ollama.go's ChatCompletion uses —
// that layer has no capability introspection of its own) — verified live
// against Ollama's own source (ollama/types/model/capability.go,
// 2026-08-18) to return a per-model capabilities array that includes
// "thinking" when the loaded model supports it. baseURL is the
// configured provider's OpenAI-compat URL (default ".../v1"); the trailing
// "/v1" is stripped to reach Ollama's native API root. Returns (nil, nil)
// — not an error — when "thinking" isn't in the model's capabilities.
func fetchOllamaModelEffortLevels(baseURL, modelID string) ([]string, error) {
	native := strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1")
	reqBody, err := json.Marshal(map[string]string{"model": modelID})
	if err != nil {
		return nil, fmt.Errorf("request oluşturulamadı: %w", err)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("POST", native+"/api/show", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("request oluşturulamadı: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Ollama API hatası: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama döndü %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse hatası: %w", err)
	}
	for _, c := range result.Capabilities {
		if c == "thinking" {
			// Ollama's real value set (its OpenAI-compat "think" param
			// accepts a bool or one of these four strings) — not "none",
			// which an earlier version of effort.go's static table wrongly
			// included as a level name rather than what false means.
			return []string{"low", "medium", "high", "max"}, nil
		}
	}
	return nil, nil
}

func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	case json.Number:
		f, _ := val.Float64()
		return f
	}
	return 0
}

func validateOpenRouterKey(apiKey string) (bool, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/auth/key", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}
