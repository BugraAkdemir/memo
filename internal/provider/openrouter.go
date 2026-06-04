package provider

import "strings"

type openRouterProvider struct {
	*openAIProvider
}

func newOpenRouterProvider(cfg ProviderConfig) (*openRouterProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL(ProviderOpenRouter)
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
	return &openRouterProvider{openAIProvider: p}, nil
}

func (p *openRouterProvider) Name() ProviderType  { return ProviderOpenRouter }
func (p *openRouterProvider) DisplayName() string  { return "OpenRouter" }
