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
