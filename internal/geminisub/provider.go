// SPDX-License-Identifier: AGPL-3.0-or-later

package geminisub

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"memo/internal/logx"
	"memo/internal/provider"

	"golang.org/x/oauth2"
)

func init() {
	provider.RegisterConstructor(provider.ProviderGeminiSub, NewProvider)
}

// defaultModels is what ListModels reports. Code Assist has no models list
// endpoint in the public GenerateContent shape, so this is a fixed set of
// the models the subscription tiers expose.
var defaultModels = []string{"gemini-2.5-pro", "gemini-2.5-flash"}

// geminiSubProvider is the provider.Provider for ProviderGeminiSub. It holds
// no credentials of its own — every call resolves the live token and Code
// Assist project through the process-wide Manager (Default()), so connecting
// or disconnecting the Google account takes effect without rebuilding the
// provider.
type geminiSubProvider struct {
	mgr      *Manager
	model    string
	client   *http.Client
	streamCl *http.Client
}

// NewProvider is the RegisterConstructor entry point. It never fails on a
// missing/expired token: a disconnected account must not break
// provider.NewRouter construction or the whole provider subsystem. Every
// method returns ErrNotConnected (wrapped) instead, until the user connects
// in Settings › Developer.
func NewProvider(cfg provider.ProviderConfig) (provider.Provider, error) {
	return &geminiSubProvider{
		mgr:   Default(),
		model: cfg.Model,
		client: &http.Client{
			Timeout: 300 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:          10,
				MaxIdleConnsPerHost:   10,
				IdleConnTimeout:       90 * time.Second,
				ResponseHeaderTimeout: 120 * time.Second,
			},
		},
		streamCl: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:          10,
				MaxIdleConnsPerHost:   10,
				IdleConnTimeout:       90 * time.Second,
				ResponseHeaderTimeout: 120 * time.Second,
			},
		},
	}, nil
}

func (p *geminiSubProvider) Name() provider.ProviderType { return provider.ProviderGeminiSub }
func (p *geminiSubProvider) DisplayName() string         { return "Google Gemini (subscription)" }

func (p *geminiSubProvider) ListModels(ctx context.Context) ([]string, error) {
	if !p.mgr.Connected() {
		return nil, ErrNotConnected
	}
	out := make([]string, len(defaultModels))
	copy(out, defaultModels)
	return out, nil
}

// resolveModel normalises the model id to the bare form Code Assist wants in
// the envelope's "model" field (no "models/" or "gemini-sub/" prefix).
func (p *geminiSubProvider) resolveModel(reqModel string) string {
	m := reqModel
	if m == "" {
		m = p.model
	}
	m = strings.TrimPrefix(m, "models/")
	if _, after, ok := strings.Cut(m, "/"); ok {
		m = after
	}
	if m == "" {
		m = defaultModels[0]
	}
	return m
}

// prepare runs the bootstrap handshake and builds the wrapped request body
// plus the OAuth token source for one call.
func (p *geminiSubProvider) prepare(ctx context.Context, req provider.ChatRequest) (body []byte, model string, ts oauth2.TokenSource, err error) {
	if !p.mgr.Connected() {
		return nil, "", nil, ErrNotConnected
	}
	boot, err := p.mgr.ensureBootstrap(ctx)
	if err != nil {
		return nil, "", nil, err
	}
	ts, err = p.mgr.TokenSource(ctx)
	if err != nil {
		return nil, "", nil, err
	}
	model = p.resolveModel(req.Model)
	wrapped := caGenerateRequest{
		Model:   model,
		Project: boot.ProjectID,
		Request: buildGenContentRequest(req),
	}
	body, err = json.Marshal(wrapped)
	if err != nil {
		return nil, "", nil, &provider.ProviderError{Provider: p.Name(), Err: fmt.Errorf("marshal: %w", err)}
	}
	return body, model, ts, nil
}

// bearerClient wraps base with the OAuth token source so every request
// carries "Authorization: Bearer <access token>", keeping base's timeout.
func bearerClient(base *http.Client, ts oauth2.TokenSource) *http.Client {
	return &http.Client{
		Timeout:   base.Timeout,
		Transport: &oauth2.Transport{Source: ts, Base: base.Transport},
	}
}

func (p *geminiSubProvider) ChatCompletion(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	body, model, ts, err := p.prepare(ctx, req)
	if err != nil {
		return nil, err
	}
	hc := bearerClient(p.client, ts)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, codeAssistBase()+":generateContent", bytes.NewReader(body))
	if err != nil {
		return nil, &provider.ProviderError{Provider: p.Name(), Err: fmt.Errorf("request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := hc.Do(httpReq)
	if err != nil {
		return nil, p.wrapError(err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, &provider.ProviderError{Provider: p.Name(), Err: fmt.Errorf("status %d: %s", resp.StatusCode, provider.ExtractErrorMessage(raw))}
	}
	out, err := parseGenerateResponse(raw, model)
	if err != nil {
		return nil, &provider.ProviderError{Provider: p.Name(), Err: fmt.Errorf("decode: %w", err)}
	}
	return out, nil
}

func (p *geminiSubProvider) ChatCompletionStream(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	body, _, ts, err := p.prepare(ctx, req)
	if err != nil {
		return nil, err
	}
	hc := bearerClient(p.streamCl, ts)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, codeAssistBase()+":streamGenerateContent?alt=sse", bytes.NewReader(body))
	if err != nil {
		return nil, &provider.ProviderError{Provider: p.Name(), Err: fmt.Errorf("request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := hc.Do(httpReq)
	if err != nil {
		return nil, p.wrapError(err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		return nil, &provider.ProviderError{Provider: p.Name(), Err: fmt.Errorf("status %d: %s", resp.StatusCode, provider.ExtractErrorMessage(raw))}
	}

	ch := make(chan provider.StreamChunk, 128)
	go p.processSSE(ctx, resp.Body, ch)
	return ch, nil
}

func (p *geminiSubProvider) processSSE(ctx context.Context, body io.ReadCloser, ch chan<- provider.StreamChunk) {
	defer body.Close()
	defer close(ch)
	defer logx.Recover("geminiSubProvider.processSSE")

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 65536), 10*1024*1024)

	var any bool
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			trySend(ctx, ch, provider.StreamChunk{Done: true})
			return
		}
		ev, ok := parseSSELine(data)
		if !ok {
			continue
		}
		if ev.Text != "" {
			any = true
			trySend(ctx, ch, provider.StreamChunk{Content: ev.Text})
			if ctx.Err() != nil {
				return
			}
		}
		if ev.FinishReason != "" && ev.FinishReason != "STOP" {
			trySend(ctx, ch, provider.StreamChunk{Done: true, FinishReason: ev.FinishReason})
			return
		}
	}

	if err := scanner.Err(); err != nil {
		trySend(ctx, ch, provider.StreamChunk{Error: err.Error(), Done: true})
		return
	}
	if any {
		trySend(ctx, ch, provider.StreamChunk{Done: true})
	}
}

func (p *geminiSubProvider) wrapError(err error) error {
	if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "timeout") {
		return &provider.ProviderError{Provider: p.Name(), Err: provider.ErrTimeout}
	}
	return &provider.ProviderError{Provider: p.Name(), Err: err}
}

// trySend mirrors provider.trySend (unexported there): prefer the buffered
// send over ctx cancellation so the terminal Done chunk is never dropped.
func trySend(ctx context.Context, ch chan<- provider.StreamChunk, chunk provider.StreamChunk) {
	select {
	case ch <- chunk:
		return
	default:
	}
	select {
	case ch <- chunk:
	case <-ctx.Done():
	}
}
