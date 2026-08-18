package webserver

import (
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/logx"
	"net/http"
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

// handleProviderEffortLevels implements GET /api/providers/effort-levels
// ?type=<provider type>&model=<model id, openrouter only> — the "which
// reasoning-effort values does this provider actually accept" lookup
// behind both UI surfaces (provider config dialog, chat screen quick-
// select). Static, vendor-documented tables for most types
// (provider.EffortLevelsForType/EffortLevelsForGemini — see effort.go's
// package doc comment for why these can't be fetched at runtime for most
// vendors); OpenRouter alone queries its own /api/v1/models live, since it
// actually publishes this per-model.
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

	if ptype == provider.ProviderGemini {
		writeJSON(w, map[string]interface{}{"levels": provider.EffortLevelsForGemini()})
		return
	}

	if ptype == provider.ProviderOpenRouter {
		model := r.URL.Query().Get("model")
		if model == "" {
			// No model chosen yet — nothing to discover against. Not an
			// error: the UI just has nothing to show its dropdown until
			// the user picks a model, same as OpenRouter's own model
			// picker needing a selection before showing model-specific
			// info.
			writeJSON(w, map[string]interface{}{"levels": []string{}})
			return
		}
		apiKey := ""
		for _, p := range s.fullBridge.GetProviders() {
			if p.Type == provider.ProviderOpenRouter && p.APIKey != "" {
				apiKey = p.APIKey
				break
			}
		}
		if apiKey == "" {
			http.Error(w, "OpenRouter API key yapılandırılmamış", http.StatusBadRequest)
			return
		}
		levels, err := fetchOpenRouterModelEffortLevels(apiKey, model)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if levels == nil {
			levels = []string{}
		}
		writeJSON(w, map[string]interface{}{"levels": levels})
		return
	}

	levels := provider.EffortLevelsForType(ptype)
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

// OpenRouterModel represents a model from the OpenRouter API.
type OpenRouterModel struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	ContextLen  int     `json:"context_length"`
	PromptPrice float64 `json:"prompt_price"`
	CompPrice   float64 `json:"completion_price"`
	IsFree      bool    `json:"is_free"`
}

func fetchOpenRouterModels(apiKey string) ([]OpenRouterModel, error) {
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

	models := make([]OpenRouterModel, 0, len(result.Data))
	for _, m := range result.Data {
		promptPrice := toFloat64(m.Pricing.Prompt)
		compPrice := toFloat64(m.Pricing.Completion)
		isFree := promptPrice == 0 && compPrice == 0
		if m.ID == "" {
			continue
		}
		models = append(models, OpenRouterModel{
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
