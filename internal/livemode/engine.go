// Package livemode owns Live Mode's per-engine configuration and (in later
// phases) the realtime session clients for Google Live/OpenAI Realtime. See
// docs/plans/PLAN_live_mode_v2.md for the full design. This is deliberately
// NOT a priority-fallback router like internal/tts.Router/internal/stt.Router
// — Live Mode has exactly one active engine at a time (selected via
// config.LiveModeConfig.ActiveEngine, internal/config), not several
// simultaneously-enabled entries tried in order. What this package DOES
// mirror from internal/tts is the encrypted-config-persistence pattern
// (ConfigManager, same shared machine key) — see config.go.
package livemode

import "fmt"

// EngineType enumerates Live Mode's five voice engines.
type EngineType string

const (
	// EngineLocal is today's existing whisper.cpp (STT) + Piper (TTS)
	// pipeline (internal/whisper, internal/tts.Synthesizer) — it has no
	// EngineConfig entry of its own (no API key/model/voice to store here);
	// its settings live entirely in config.WhisperConfig/config.TTSConfig,
	// unchanged by this package.
	EngineLocal EngineType = "local"
	// EngineGoogleLive and EngineOpenAIRealtime are native audio-to-audio
	// reasoning models — the engine's own model plays both voice I/O and
	// the "live model" brain role (see PLAN_live_mode_v2.md's locked-in
	// design). Session clients land in later phases (internal/livemode/
	// google, internal/livemode/openai_realtime).
	EngineGoogleLive     EngineType = "google_live"
	EngineOpenAIRealtime EngineType = "openai_realtime"
	// EngineElevenLabs and EngineCustom are pure STT/TTS voice I/O, no
	// reasoning of their own — transcribed text goes straight to Memo's
	// existing main model routing (internal/app.callLLMStream), same as
	// EngineLocal. Their actual STT/TTS calls are handled by
	// internal/stt.ProviderElevenLabs/ProviderCustom and
	// internal/tts.ProviderElevenLabs/ProviderCustom respectively — the
	// EngineConfig saved here is the source of truth an App-level sync (a
	// later phase, once TranscribeAudio/SynthesizeSpeech actually dispatch
	// through the active engine) copies into those packages' own
	// ProviderConfig shapes.
	EngineElevenLabs EngineType = "elevenlabs"
	EngineCustom     EngineType = "custom"
)

var validEngineTypes = map[EngineType]bool{
	EngineLocal:          true,
	EngineGoogleLive:     true,
	EngineOpenAIRealtime: true,
	EngineElevenLabs:     true,
	EngineCustom:         true,
}

// IsValid reports whether t is one of the five recognized engine types.
func (t EngineType) IsValid() bool {
	return validEngineTypes[t]
}

// EngineConfig configures one non-local Live Mode engine. EngineLocal has
// no entry of its own (see EngineLocal's doc comment above) — callers
// should never construct one with Type: EngineLocal.
type EngineConfig struct {
	Type   EngineType `json:"type"`
	APIKey string     `json:"api_key,omitempty"`
	// Model is the engine's live-fetched model ID (Google Live/OpenAI
	// Realtime's realtime model, or ElevenLabs' TTS model) — populated via
	// a later phase's discovery endpoint, never hardcoded here.
	Model string `json:"model,omitempty"`
	// Voice is only meaningful for EngineElevenLabs (ElevenLabs' voice_id).
	Voice string `json:"voice,omitempty"`
	// BaseURL is only meaningful for EngineCustom — the user-supplied
	// OpenAI-compatible endpoint's base URL.
	BaseURL string `json:"base_url,omitempty"`
	Enabled bool   `json:"enabled"`
}

// Validate reports whether cfg has enough information to be usable.
func (c EngineConfig) Validate() error {
	if c.Type == EngineLocal {
		return fmt.Errorf("livemode: EngineLocal has no EngineConfig entry")
	}
	if !c.Type.IsValid() {
		return fmt.Errorf("livemode: unknown engine type %q", c.Type)
	}
	if c.Type == EngineCustom {
		if c.BaseURL == "" {
			return fmt.Errorf("livemode: base_url is required for custom")
		}
		return nil
	}
	if c.APIKey == "" {
		return fmt.Errorf("livemode: API key is required for %s", c.Type)
	}
	return nil
}
