package provider

import "strings"

type openCodeZenProvider struct {
	*openAIProvider
}

func newOpenCodeZenProvider(cfg ProviderConfig) (*openCodeZenProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL(ProviderOpenCodeZen)
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
	return &openCodeZenProvider{openAIProvider: p}, nil
}

func (p *openCodeZenProvider) Name() ProviderType { return ProviderOpenCodeZen }
func (p *openCodeZenProvider) DisplayName() string { return "OpenCode Zen" }
