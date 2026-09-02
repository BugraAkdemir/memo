package provider

// customAnthropicProvider is ProviderCustomAnthropic — see that constant's
// doc comment (provider.go) for why this exists alongside the OpenAI-shaped
// "custom" type. Thin wrapper embedding *claudeProvider, same pattern as
// kilo.go/openrouter.go/groq.go embedding *openAIProvider: claudeProvider
// already accepts an arbitrary BaseURL (newClaudeProvider only falls back to
// Anthropic's own api.anthropic.com when BaseURL is empty, which
// ProviderConfig.Validate rejects for this type before construction ever
// happens), so no request/response logic needs duplicating — just the
// identity methods.
type customAnthropicProvider struct {
	*claudeProvider
}

func newCustomAnthropicProvider(cfg ProviderConfig) (*customAnthropicProvider, error) {
	p, err := newClaudeProvider(cfg)
	if err != nil {
		return nil, err
	}
	return &customAnthropicProvider{claudeProvider: p}, nil
}

// DisplayName mirrors every other provider's Go-layer name (plain English,
// e.g. claudeProvider's "Anthropic Claude") — the frontend's own
// ProviderDefaults.displayNames map (provider_config.dart) is what actually
// localizes this for the UI, not this string.
func (p *customAnthropicProvider) Name() ProviderType  { return ProviderCustomAnthropic }
func (p *customAnthropicProvider) DisplayName() string { return "Custom (Anthropic-compatible)" }
