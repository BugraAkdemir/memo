package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/provider"
	"net/http"
	"strings"
	"time"
)

// elevenLabsProvider calls ElevenLabs' text-to-speech endpoint. Mirrors
// openAIProvider's shape (single POST, no streaming).
type elevenLabsProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// elevenLabsTTSDefaultModel is a safe, widely-available default — same
// reasoning as openai.go's openAITTSDefaultModel: tts.ProviderConfig has no
// per-provider model field yet (Voice is the only sound-selection knob), so
// this is a hardcoded constant until that changes.
const elevenLabsTTSDefaultModel = "eleven_multilingual_v2"

// elevenLabsOutputFormat requests WAV directly — confirmed via ElevenLabs'
// current docs that output_format accepts wav_8000/16000/22050/24000/32000/
// 44100/48000 (not just mp3/pcm/ulaw). 24kHz is available on every paid
// tier (44.1/48kHz require Pro+), and handleTTSSynthesize
// (internal/webserver/handlers_flutter.go) hardcodes Content-Type: audio/wav
// on every response regardless of which provider produced it — every
// provider in this package must actually return WAV bytes, exactly the
// same constraint openai.go's Synthesize doc comment already states.
const elevenLabsOutputFormat = "wav_24000"

func newElevenLabsProvider(cfg ProviderConfig) (*elevenLabsProvider, error) {
	return &elevenLabsProvider{
		baseURL: "https://api.elevenlabs.io/v1",
		apiKey:  cfg.APIKey,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

func (p *elevenLabsProvider) Name() ProviderType  { return ProviderElevenLabs }
func (p *elevenLabsProvider) DisplayName() string { return "ElevenLabs" }

type elevenLabsSpeechRequest struct {
	Text    string `json:"text"`
	ModelID string `json:"model_id"`
}

// Synthesize POSTs to /text-to-speech/{voice_id} — voice is ElevenLabs'
// voice_id (from ProviderConfig.Voice, populated via a later phase's voice
// discovery endpoint, GET /v1/voices).
func (p *elevenLabsProvider) Synthesize(ctx context.Context, text, voice string) ([]byte, error) {
	if voice == "" {
		return nil, fmt.Errorf("tts elevenlabs: voice (voice_id) is required")
	}
	body := elevenLabsSpeechRequest{
		Text:    text,
		ModelID: elevenLabsTTSDefaultModel,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("tts elevenlabs: marshal: %w", err)
	}

	url := fmt.Sprintf("%s/text-to-speech/%s?output_format=%s", p.baseURL, voice, elevenLabsOutputFormat)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("tts elevenlabs: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("xi-api-key", p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") {
			return nil, fmt.Errorf("tts elevenlabs: timeout: %w", err)
		}
		return nil, fmt.Errorf("tts elevenlabs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tts elevenlabs: status %d: %s", resp.StatusCode, provider.ExtractErrorMessage(errBody))
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tts elevenlabs: read response: %w", err)
	}
	return audio, nil
}
