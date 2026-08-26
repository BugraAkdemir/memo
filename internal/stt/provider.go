// Package stt provides external speech-to-text providers for Live Mode's
// non-native engines (ElevenLabs, Custom) — see docs/plans/PLAN_live_mode_v2.md
// §2. Mirrors internal/tts's shape one-for-one (this is the first time STT
// has needed a provider-abstraction layer at all: until now only
// whisper.cpp existed, wired directly, with no interface). Local whisper.cpp
// stays entirely outside this package, exactly like Piper stays outside
// internal/tts's Router — see internal/whisper and internal/app/stt.go,
// untouched by this addition.
package stt

import (
	"context"
	"fmt"
)

// ProviderType enumerates supported external STT providers.
type ProviderType string

const (
	ProviderElevenLabs ProviderType = "elevenlabs"
	// ProviderCustom is a user-defined OpenAI-compatible STT REST endpoint
	// (POST {base_url}/audio/transcriptions, multipart form — the same
	// contract internal/tts.ProviderCustom uses for its TTS half).
	ProviderCustom ProviderType = "custom"
)

// STTProvider is the common interface all external STT providers implement.
type STTProvider interface {
	Name() ProviderType
	DisplayName() string
	Transcribe(ctx context.Context, audioData []byte) (string, error)
}

// ProviderConfig configures one external STT provider entry. Mirrors
// internal/tts.ProviderConfig's shape, minus Voice (no STT equivalent).
type ProviderConfig struct {
	Type      ProviderType `json:"type"`
	Name      string       `json:"name"`
	APIKey    string       `json:"api_key,omitempty"`
	BaseURL   string       `json:"base_url,omitempty"` // ProviderCustom only
	Enabled   bool         `json:"enabled"`
	Priority  int          `json:"priority"`
	Connected bool         `json:"connected,omitempty"`
	Error     string       `json:"error,omitempty"`
}

// Validate reports whether cfg has enough information to construct a
// provider. Mirrors internal/tts.ProviderConfig.Validate.
func (c ProviderConfig) Validate() error {
	if c.Type == "" {
		return fmt.Errorf("stt provider type is required")
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

// NewProvider constructs an STTProvider from cfg.
func NewProvider(cfg ProviderConfig) (STTProvider, error) {
	switch cfg.Type {
	case ProviderElevenLabs:
		return newElevenLabsProvider(cfg)
	case ProviderCustom:
		return newCustomProvider(cfg)
	default:
		return nil, fmt.Errorf("stt: unknown provider type %q", cfg.Type)
	}
}
