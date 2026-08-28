package tts

import (
	"context"
	"fmt"
)

// ProviderType enumerates supported external TTS providers. Unlike
// internal/provider.ProviderType there is no "local"/"llama.cpp" entry
// here — the local Piper engine (Synthesizer, tts.go) stays a separate,
// untouched fallback outside the router, mirroring how callLLMStream keeps
// the local llama.cpp client outside internal/provider.Router entirely
// (see docs/plans/PLAN_voice_live_mode_faz2.md's "Önemli mimari ayrım").
type ProviderType string

const (
	ProviderOpenAI     ProviderType = "openai"
	ProviderElevenLabs ProviderType = "elevenlabs"
	// ProviderCustom is a user-defined OpenAI-compatible TTS REST endpoint
	// (POST {base_url}/audio/speech, same request/response shape as
	// openAIProvider) — see docs/plans/PLAN_live_mode_v2.md's "Custom"
	// engine decision. BaseURL is required for this type; APIKey is
	// optional (a self-hosted endpoint may not need one).
	ProviderCustom ProviderType = "custom"
)

// TTSProvider is the common interface all external TTS providers implement.
// Mirrors internal/provider.Provider's shape, minus the chat-specific
// methods (ListModels has no TTS equivalent — voice names are a plain
// user-entered config field, not a discovered list).
type TTSProvider interface {
	Name() ProviderType
	DisplayName() string
	Synthesize(ctx context.Context, text, voice string) ([]byte, error)
}

// ProviderConfig configures one external TTS provider entry. Mirrors
// internal/provider.ProviderConfig's shape (Type/Name/APIKey/Enabled/
// Priority/Connected/Error), replacing Model/BaseURL/Temperature/TopP/
// MaxTokens/ContextTokens (all chat-specific, no TTS meaning) with Voice.
type ProviderConfig struct {
	Type   ProviderType `json:"type"`
	Name   string       `json:"name"`
	APIKey string       `json:"api_key,omitempty"`
	Voice  string       `json:"voice"`
	// BaseURL is only meaningful for ProviderCustom — a user-supplied
	// OpenAI-compatible TTS endpoint's base URL.
	BaseURL   string `json:"base_url,omitempty"`
	Enabled   bool   `json:"enabled"`
	Priority  int    `json:"priority"`
	Connected bool   `json:"connected,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Validate reports whether cfg has enough information to construct a
// provider. Mirrors internal/provider.ProviderConfig.Validate. ProviderCustom
// requires BaseURL instead of APIKey (a self-hosted endpoint may not need a
// key at all) — every other type requires APIKey, same as before.
func (c ProviderConfig) Validate() error {
	if c.Type == "" {
		return fmt.Errorf("tts provider type is required")
	}
	if c.Voice == "" {
		return fmt.Errorf("voice is required for %s", c.Type)
	}
	if c.Type == ProviderCustom {
		if c.BaseURL == "" {
			return fmt.Errorf("base_url is required for custom")
		}
		return nil
	}
	if c.APIKey == "" {
		return fmt.Errorf("API key is required for %s", c.Type)
	}
	return nil
}

// NewProvider constructs a TTSProvider from cfg.
func NewProvider(cfg ProviderConfig) (TTSProvider, error) {
	switch cfg.Type {
	case ProviderOpenAI:
		return newOpenAIProvider(cfg)
	case ProviderElevenLabs:
		return newElevenLabsProvider(cfg)
	case ProviderCustom:
		return newCustomProvider(cfg)
	default:
		return nil, fmt.Errorf("tts: unknown provider type %q", cfg.Type)
	}
}
