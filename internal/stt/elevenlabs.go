package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/provider"
	"mime/multipart"
	"net/http"
	"time"
)

// elevenLabsSTTModel is ElevenLabs' current Scribe model ID (confirmed
// against current API docs/examples, 2026-08-26: `model_id="scribe_v1"`).
// No per-provider model field exists on ProviderConfig yet (same reasoning
// as internal/tts's hardcoded TTS model defaults) — a future step can
// expose model choice there if needed.
const elevenLabsSTTModel = "scribe_v1"

type elevenLabsProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func newElevenLabsProvider(cfg ProviderConfig) (*elevenLabsProvider, error) {
	return &elevenLabsProvider{
		baseURL: "https://api.elevenlabs.io/v1",
		apiKey:  cfg.APIKey,
		client:  &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (p *elevenLabsProvider) Name() ProviderType  { return ProviderElevenLabs }
func (p *elevenLabsProvider) DisplayName() string { return "ElevenLabs" }

// Transcribe POSTs multipart/form-data to /v1/speech-to-text — mirrors
// internal/whisper.Server.Transcribe's "whole file in one request, one
// {"text": ...} reply" shape, just against ElevenLabs' endpoint instead of
// a local whisper-server subprocess.
func (p *elevenLabsProvider) Transcribe(ctx context.Context, audioData []byte) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("model_id", elevenLabsSTTModel); err != nil {
		return "", fmt.Errorf("stt elevenlabs: write model_id field: %w", err)
	}
	fw, err := w.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", fmt.Errorf("stt elevenlabs: create form file: %w", err)
	}
	if _, err := fw.Write(audioData); err != nil {
		return "", fmt.Errorf("stt elevenlabs: write audio: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("stt elevenlabs: close multipart writer: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/speech-to-text", &buf)
	if err != nil {
		return "", fmt.Errorf("stt elevenlabs: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", w.FormDataContentType())
	httpReq.Header.Set("xi-api-key", p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("stt elevenlabs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("stt elevenlabs: status %d: %s", resp.StatusCode, provider.ExtractErrorMessage(errBody))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("stt elevenlabs: decode response: %w", err)
	}
	return result.Text, nil
}
