package provider

import "strings"

type kiloProvider struct {
	*openAIProvider
}

func newKiloProvider(cfg ProviderConfig) (*kiloProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL(ProviderKilo)
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
	return &kiloProvider{openAIProvider: p}, nil
}

func (p *kiloProvider) Name() ProviderType  { return ProviderKilo }
func (p *kiloProvider) DisplayName() string { return "Kilo Code" }
