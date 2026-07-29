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
	ProviderOpenAI ProviderType = "openai"
	// ProviderElevenLabs is declared for Validate/NewProvider completeness
	// but has no implementation yet — Faz 2.2 only builds OpenAI. Selecting
	// it fails at NewProvider with a clear "not yet supported" error rather
	// than silently doing nothing.
	ProviderElevenLabs ProviderType = "elevenlabs"
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
	Type      ProviderType `json:"type"`
	Name      string       `json:"name"`
	APIKey    string       `json:"api_key,omitempty"`
	Voice     string       `json:"voice"`
	Enabled   bool         `json:"enabled"`
	Priority  int          `json:"priority"`
	Connected bool         `json:"connected,omitempty"`
	Error     string       `json:"error,omitempty"`
}

// Validate reports whether cfg has enough information to construct a
// provider. Mirrors internal/provider.ProviderConfig.Validate.
func (c ProviderConfig) Validate() error {
	if c.Type == "" {
		return fmt.Errorf("tts provider type is required")
	}
	if c.Voice == "" {
		return fmt.Errorf("voice is required for %s", c.Type)
	}
	if c.APIKey == "" {
		return fmt.Errorf("API key is required for %s", c.Type)
	}
	return nil
}

// NewProvider constructs a TTSProvider from cfg. Only ProviderOpenAI has a
// real implementation (Faz 2.2); other declared types fail with a clear
// "not yet supported" error rather than being silently accepted and then
// failing opaquely at Synthesize time.
func NewProvider(cfg ProviderConfig) (TTSProvider, error) {
	switch cfg.Type {
	case ProviderOpenAI:
		return newOpenAIProvider(cfg)
	case ProviderElevenLabs:
		return nil, fmt.Errorf("tts: ElevenLabs provider not yet supported")
	default:
		return nil, fmt.Errorf("tts: unknown provider type %q", cfg.Type)
	}
}
