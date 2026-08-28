package tts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/provider"
	"net/http"
	"time"
)

// ElevenLabsModel is the subset of ElevenLabs' GET /v1/models response this
// package cares about. Per the "never hardcode a model list" requirement
// (docs/plans/PLAN_live_mode_v2.md), the caller filters on
// CanDoTextToSpeech itself rather than this package returning a pre-filtered
// slice, so a future STT-only caller (internal/stt) can reuse the same
// fetch/parse logic and filter on a different field instead.
type ElevenLabsModel struct {
	ModelID              string `json:"model_id"`
	Name                 string `json:"name"`
	CanDoTextToSpeech    bool   `json:"can_do_text_to_speech"`
	CanDoVoiceConversion bool   `json:"can_do_voice_conversion"`
}

// ElevenLabsVoice is the subset of ElevenLabs' GET /v1/voices response this
// package cares about.
type ElevenLabsVoice struct {
	VoiceID string `json:"voice_id"`
	Name    string `json:"name"`
}

var elevenLabsDiscoveryClient = &http.Client{Timeout: 20 * time.Second}

// elevenLabsDiscoveryBaseURL is a var (not const) so tests can point it at
// an httptest server instead of the real API.
var elevenLabsDiscoveryBaseURL = "https://api.elevenlabs.io/v1"

// ListElevenLabsModels fetches every ElevenLabs model (not filtered) —
// callers that only want TTS-capable ones filter on CanDoTextToSpeech.
// Takes a bare apiKey rather than a ProviderConfig since discovery happens
// before a Voice/model choice exists to put one together.
func ListElevenLabsModels(ctx context.Context, apiKey string) ([]ElevenLabsModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, elevenLabsDiscoveryBaseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("tts elevenlabs: models request: %w", err)
	}
	req.Header.Set("xi-api-key", apiKey)

	resp, err := elevenLabsDiscoveryClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts elevenlabs: models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tts elevenlabs: models status %d: %s", resp.StatusCode, provider.ExtractErrorMessage(errBody))
	}

	var models []ElevenLabsModel
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("tts elevenlabs: decode models: %w", err)
	}
	return models, nil
}

// ListElevenLabsVoices fetches the account's available voices — the values
// that go into ProviderConfig.Voice (ElevenLabs' voice_id).
func ListElevenLabsVoices(ctx context.Context, apiKey string) ([]ElevenLabsVoice, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, elevenLabsDiscoveryBaseURL+"/voices", nil)
	if err != nil {
		return nil, fmt.Errorf("tts elevenlabs: voices request: %w", err)
	}
	req.Header.Set("xi-api-key", apiKey)

	resp, err := elevenLabsDiscoveryClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts elevenlabs: voices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tts elevenlabs: voices status %d: %s", resp.StatusCode, provider.ExtractErrorMessage(errBody))
	}

	var result struct {
		Voices []ElevenLabsVoice `json:"voices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("tts elevenlabs: decode voices: %w", err)
	}
	return result.Voices, nil
}
