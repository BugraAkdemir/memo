package provider

import (
	"strings"
)

// Groq uses OpenAI-compatible API (https://console.groq.com/docs)
type groqProvider struct {
	*openAIProvider
}

func newGroqProvider(cfg ProviderConfig) (*groqProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.groq.com/openai/v1"
	}
	p, err := newOpenAIProvider(ProviderConfig{
		Type:    cfg.Type,
		Name:    cfg.Name,
		APIKey:  cfg.APIKey,
		BaseURL: strings.TrimRight(baseURL, "/"),
		Model:   cfg.Model,
	})
	if err != nil {
		return nil, err
	}
	return &groqProvider{openAIProvider: p}, nil
}

func (p *groqProvider) Name() ProviderType  { return ProviderGroq }
func (p *groqProvider) DisplayName() string { return "Groq" }

// ListModels is inherited from openAIProvider — groqProvider used to
// re-implement it with byte-for-byte identical logic (same request, same
// auth, same decode), which meant a fix to the shared implementation (e.g.
// the 2026-09-23 HTTP-status-check fix) had to be applied twice and could
// easily be missed here. Removed as dead/duplicate code; behavior is
// unchanged.
