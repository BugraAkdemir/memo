package provider

import "strings"

type openCodeGoProvider struct {
	*openAIProvider
}

func newOpenCodeGoProvider(cfg ProviderConfig) (*openCodeGoProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL(ProviderOpenCodeGo)
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
	return &openCodeGoProvider{openAIProvider: p}, nil
}

func (p *openCodeGoProvider) Name() ProviderType { return ProviderOpenCodeGo }
func (p *openCodeGoProvider) DisplayName() string { return "OpenCode Go" }
