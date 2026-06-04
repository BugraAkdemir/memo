package provider

import "strings"

type grokProvider struct {
	*openAIProvider
}

func newGrokProvider(cfg ProviderConfig) (*grokProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL(ProviderGrok)
	}
	// xAI Grok uses OpenAI-compatible API
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
	return &grokProvider{openAIProvider: p}, nil
}

func (p *grokProvider) Name() ProviderType    { return ProviderGrok }
func (p *grokProvider) DisplayName() string    { return "xAI Grok" }
