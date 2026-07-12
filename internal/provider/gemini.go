package provider

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
)

type geminiProvider struct {
	cfg      ProviderConfig
	baseURL  string
	model    string
	apiKey   string
	client   *http.Client
	streamCl *http.Client
}

func newGeminiProvider(cfg ProviderConfig) (*geminiProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL(ProviderGemini)
	}
	return &geminiProvider{
		cfg:     cfg,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   cfg.Model,
		apiKey:  cfg.APIKey,
		client: &http.Client{
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:          90 * time.Second,
				ResponseHeaderTimeout:    30 * time.Second,
			},
		},
		streamCl: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:          90 * time.Second,
				ResponseHeaderTimeout:    30 * time.Second,
			},
		},
	}, nil
}

func (p *geminiProvider) Name() ProviderType  { return ProviderGemini }
func (p *geminiProvider) DisplayName() string  { return "Google Gemini" }

func (p *geminiProvider) ListModels(ctx context.Context) ([]string, error) {
	u := fmt.Sprintf("%s/models", p.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-goog-api-key", p.apiKey)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name       string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(result.Models))
	for _, m := range result.Models {
		models = append(models, m.Name)
	}
	return models, nil
}

type geminiRequest struct {
	Contents         []geminiContent `json:"contents"`
	GenerationConfig *geminiGenConfig `json:"generationConfig,omitempty"`
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type geminiGenConfig struct {
	Temperature float64  `json:"temperature,omitempty"`
	TopP        float64  `json:"topP,omitempty"`
	MaxOutputTokens int `json:"maxOutputTokens,omitempty"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
	Usage      *geminiUsage      `json:"usageMetadata,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

func (p *geminiProvider) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}
	if !strings.Contains(model, "/") {
		model = "models/" + model
	}

	gReq := p.buildGeminiRequest(req)

	jsonBody, err := json.Marshal(gReq)
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("marshal: %w", err)}
	}

	u := fmt.Sprintf("%s/%s:generateContent", p.baseURL, model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, p.wrapError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.parseError(resp)
	}

	var result geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("decode: %w", err)}
	}

	if len(result.Candidates) == 0 {
		return &ChatResponse{Model: model}, nil
	}

	content := ""
	for _, p := range result.Candidates[0].Content.Parts {
		content += p.Text
	}

	var usage *Usage
	if result.Usage != nil {
		usage = &Usage{
			PromptTokens:     result.Usage.PromptTokenCount,
			CompletionTokens: result.Usage.CandidatesTokenCount,
			TotalTokens:      result.Usage.TotalTokenCount,
		}
	}

	return &ChatResponse{
		Content: content,
		Model:   model,
		Usage:   usage,
	}, nil
}

func (p *geminiProvider) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}
	if !strings.Contains(model, "/") {
		model = "models/" + model
	}

	gReq := p.buildGeminiRequest(req)
	jsonBody, err := json.Marshal(gReq)
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("marshal: %w", err)}
	}

	u := fmt.Sprintf("%s/%s:streamGenerateContent?alt=sse", p.baseURL, model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.apiKey)

	resp, err := p.streamCl.Do(httpReq)
	if err != nil {
		return nil, p.wrapError(err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, p.parseError(resp)
	}

	ch := make(chan StreamChunk, 128)
	go p.processSSE(ctx, resp.Body, ch)
	return ch, nil
}

func (p *geminiProvider) processSSE(ctx context.Context, body io.ReadCloser, ch chan<- StreamChunk) {
	defer body.Close()
	defer close(ch)

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 65536), 10*1024*1024)

	var fullContent strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			trySend(ctx, ch, StreamChunk{Done: true})
			return
		}

		var result geminiResponse
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			continue
		}

		if len(result.Candidates) == 0 {
			continue
		}

		for _, part := range result.Candidates[0].Content.Parts {
			if part.Text != "" {
				fullContent.WriteString(part.Text)
				trySend(ctx, ch, StreamChunk{Content: part.Text})
				if ctx.Err() != nil {
					return
				}
			}
		}

		fr := result.Candidates[0].FinishReason
		if fr != "" && fr != "STOP" {
			trySend(ctx, ch, StreamChunk{Done: true, FinishReason: fr})
			return
		}
	}

	if err := scanner.Err(); err != nil {
		trySend(ctx, ch, StreamChunk{Error: err.Error(), Done: true})
		return
	}

	if fullContent.Len() > 0 {
		trySend(ctx, ch, StreamChunk{Done: true})
	}
}

func (p *geminiProvider) buildGeminiRequest(req ChatRequest) geminiRequest {
	gReq := geminiRequest{
		GenerationConfig: &geminiGenConfig{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxOutputTokens: req.MaxTokens,
		},
	}

	var systemText string
	for _, m := range req.Messages {
		if m.Role == "system" || m.Role == "developer" {
			if text, ok := m.Content.(string); ok {
				systemText += text + "\n"
			}
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		gContent := geminiContent{Role: role}
		switch v := m.Content.(type) {
		case string:
			gContent.Parts = append(gContent.Parts, geminiPart{Text: v})
		case []ContentPart:
			for _, part := range v {
				if part.Text != "" {
					gContent.Parts = append(gContent.Parts, geminiPart{Text: part.Text})
				}
			}
		}
		gReq.Contents = append(gReq.Contents, gContent)
	}

	if systemText != "" {
		gReq.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: strings.TrimSpace(systemText)}},
		}
	}

	return gReq
}

func (p *geminiProvider) setAuth(req *http.Request) {
	// Gemini uses API key as query parameter, not header
}

func (p *geminiProvider) wrapError(err error) error {
	if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "timeout") {
		return &ProviderError{Provider: p.Name(), Err: ErrTimeout}
	}
	return &ProviderError{Provider: p.Name(), Err: err}
}

func (p *geminiProvider) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return &ProviderError{Provider: p.Name(), Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
}
