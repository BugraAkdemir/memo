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
	"strings"
	"time"
)

// customSTTModel mirrors OpenAI's own Whisper API model field — most
// OpenAI-compatible self-hosted STT servers accept (and often ignore) this
// exact field name/shape, same convention as internal/tts.customProvider.
const customSTTModel = "whisper-1"

// customProvider calls a user-defined OpenAI-compatible STT REST endpoint
// (POST {base_url}/audio/transcriptions, multipart/form-data — the Whisper
// API shape). See docs/plans/PLAN_live_mode_v2.md's "Custom" engine
// decision.
type customProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func newCustomProvider(cfg ProviderConfig) (*customProvider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("stt custom: base_url is required")
	}
	return &customProvider{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		client:  &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (p *customProvider) Name() ProviderType  { return ProviderCustom }
func (p *customProvider) DisplayName() string { return "Custom" }

func (p *customProvider) Transcribe(ctx context.Context, audioData []byte) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("model", customSTTModel); err != nil {
		return "", fmt.Errorf("stt custom: write model field: %w", err)
	}
	fw, err := w.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", fmt.Errorf("stt custom: create form file: %w", err)
	}
	if _, err := fw.Write(audioData); err != nil {
		return "", fmt.Errorf("stt custom: write audio: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("stt custom: close multipart writer: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/audio/transcriptions", &buf)
	if err != nil {
		return "", fmt.Errorf("stt custom: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", w.FormDataContentType())
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("stt custom: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("stt custom: status %d: %s", resp.StatusCode, provider.ExtractErrorMessage(errBody))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("stt custom: decode response: %w", err)
	}
	return result.Text, nil
}
