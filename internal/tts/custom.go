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

// customProvider calls a user-defined OpenAI-compatible TTS REST endpoint
// (POST {base_url}/audio/speech, same request/response shape as
// openAIProvider) — see docs/plans/PLAN_live_mode_v2.md's "Custom" engine
// decision, confirmed with the user: Custom means OpenAI-compatible
// STT/TTS REST, not a realtime/WebSocket engine.
type customProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func newCustomProvider(cfg ProviderConfig) (*customProvider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("tts custom: base_url is required")
	}
	return &customProvider{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

func (p *customProvider) Name() ProviderType  { return ProviderCustom }
func (p *customProvider) DisplayName() string { return "Custom" }

// Synthesize mirrors openAIProvider.Synthesize's request shape exactly
// (model/input/voice/response_format=wav) — the whole point of scoping
// Custom to "OpenAI-compatible" is that any endpoint speaking this same
// contract (a self-hosted TTS server, a proxy in front of another vendor,
// etc.) works without Memo needing to know anything vendor-specific about
// it. Model is the same hardcoded default as OpenAI's own provider, since
// ProviderConfig has no per-provider model field yet — a self-hosted server
// that ignores the model field entirely (common for single-model servers)
// is unaffected either way.
func (p *customProvider) Synthesize(ctx context.Context, text, voice string) ([]byte, error) {
	body := openAISpeechRequest{
		Model:          openAITTSDefaultModel,
		Input:          text,
		Voice:          voice,
		ResponseFormat: "wav",
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("tts custom: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/audio/speech", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("tts custom: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") {
			return nil, fmt.Errorf("tts custom: timeout: %w", err)
		}
		return nil, fmt.Errorf("tts custom: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tts custom: status %d: %s", resp.StatusCode, provider.ExtractErrorMessage(errBody))
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tts custom: read response: %w", err)
	}
	return audio, nil
}
