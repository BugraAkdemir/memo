package provider

import "strings"

type ollamaProvider struct {
	*openAIProvider
}

func newOllamaProvider(cfg ProviderConfig) (*ollamaProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL(ProviderOllama)
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
	return &ollamaProvider{openAIProvider: p}, nil
}

func (p *ollamaProvider) Name() ProviderType  { return ProviderOllama }
func (p *ollamaProvider) DisplayName() string  { return "Ollama" }
