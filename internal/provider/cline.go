// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import "strings"

// clineProvider is Cline's hosted API gateway (api.cline.bot) — see
// ProviderCline's doc comment. Same thin-wrapper shape as openRouterProvider
// and kiloProvider: fully OpenAI-compatible, so nothing beyond the base URL
// and display name differs from openAIProvider.
type clineProvider struct {
	*openAIProvider
}

func newClineProvider(cfg ProviderConfig) (*clineProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL(ProviderCline)
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
	return &clineProvider{openAIProvider: p}, nil
}

func (p *clineProvider) Name() ProviderType  { return ProviderCline }
func (p *clineProvider) DisplayName() string { return "Cline" }
