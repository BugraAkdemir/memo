package provider

import "strings"

// llamaCPPProvider talks to a local llama-server instance over its
// OpenAI-compatible API (/v1/chat/completions). It reuses the OpenAI
// implementation since llama-server mirrors the OpenAI schema, including
// tool calling when the server is launched with --jinja.
type llamaCPPProvider struct {
	*openAIProvider
}

func newLlamaCPPProvider(cfg ProviderConfig) (*llamaCPPProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL(ProviderLlamaCPP)
	}
	p, err := newOpenAIProvider(ProviderConfig{
		Type:    cfg.Type,
		Name:    cfg.Name,
		APIKey:  cfg.APIKey, // llama-server ignores auth; empty is fine
		BaseURL: strings.TrimRight(baseURL, "/"),
		Model:   cfg.Model,
	})
	if err != nil {
		return nil, err
	}
	return &llamaCPPProvider{openAIProvider: p}, nil
}

func (p *llamaCPPProvider) Name() ProviderType { return ProviderLlamaCPP }
func (p *llamaCPPProvider) DisplayName() string { return "Local (llama.cpp)" }
