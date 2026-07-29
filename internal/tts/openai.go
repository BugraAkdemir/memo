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

// openAIProvider calls OpenAI's text-to-speech endpoint. Unlike the chat
// providers in internal/provider, there is no streaming and no model
// listing — a single POST returns the full audio body.
type openAIProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// openAITTSDefaultModel is OpenAI's cheaper, lower-latency TTS model.
// There is no per-provider model field on tts.ProviderConfig (Faz 2 keeps
// TTS config to Type/Voice/APIKey, mirroring how Piper's voice selection
// in Faz 1 is a single path, not a model+voice combination) — if a future
// step needs to expose model choice, add it there rather than hardcoding
// a second constant here.
const openAITTSDefaultModel = "tts-1"

func newOpenAIProvider(cfg ProviderConfig) (*openAIProvider, error) {
	return &openAIProvider{
		baseURL: "https://api.openai.com/v1",
		apiKey:  cfg.APIKey,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

func (p *openAIProvider) Name() ProviderType  { return ProviderOpenAI }
func (p *openAIProvider) DisplayName() string { return "OpenAI" }

type openAISpeechRequest struct {
	Model          string `json:"model"`
	Input          string `json:"input"`
	Voice          string `json:"voice"`
	ResponseFormat string `json:"response_format"`
}

// Synthesize requests response_format=wav explicitly — the webserver's
// handleTTSSynthesize (internal/webserver/handlers_flutter.go) hardcodes
// Content-Type: audio/wav on every response regardless of which provider
// produced it, so every provider in this package must actually return WAV
// bytes, not just whatever its own default happens to be (OpenAI's
// unrequested default is mp3).
func (p *openAIProvider) Synthesize(ctx context.Context, text, voice string) ([]byte, error) {
	body := openAISpeechRequest{
		Model:          openAITTSDefaultModel,
		Input:          text,
		Voice:          voice,
		ResponseFormat: "wav",
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("tts openai: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/audio/speech", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("tts openai: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") {
			return nil, fmt.Errorf("tts openai: timeout: %w", err)
		}
		return nil, fmt.Errorf("tts openai: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tts openai: status %d: %s", resp.StatusCode, provider.ExtractErrorMessage(errBody))
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tts openai: read response: %w", err)
	}
	return audio, nil
}
