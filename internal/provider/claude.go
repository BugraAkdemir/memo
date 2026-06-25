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

type claudeProvider struct {
	cfg      ProviderConfig
	baseURL  string
	model    string
	apiKey   string
	client   *http.Client
	streamCl *http.Client
}

func newClaudeProvider(cfg ProviderConfig) (*claudeProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL(ProviderClaude)
	}
	return &claudeProvider{
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

func (p *claudeProvider) Name() ProviderType  { return ProviderClaude }
func (p *claudeProvider) DisplayName() string  { return "Anthropic Claude" }

func (p *claudeProvider) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	p.setAuth(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		if m.Type == "model" {
			models = append(models, m.ID)
		}
	}
	return models, nil
}

type claudeRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	Messages    []claudeMsg     `json:"messages"`
	System      string          `json:"system,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
	Stream      bool            `json:"stream"`
}

type claudeMsg struct {
	Role    string        `json:"role"`
	Content []claudeBlock `json:"content"`
}

type claudeBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type claudeResponse struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Content []claudeBlock   `json:"content"`
	Model   string          `json:"model"`
	Usage   *claudeUsage    `json:"usage,omitempty"`
	StopReason string       `json:"stop_reason,omitempty"`
}

type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func (p *claudeProvider) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	clReq := p.buildClaudeRequest(req, false)
	jsonBody, err := json.Marshal(clReq)
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("marshal: %w", err)}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	p.setAuth(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, p.wrapError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.parseError(resp)
	}

	var result claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("decode: %w", err)}
	}

	content := ""
	for _, block := range result.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	var usage *Usage
	if result.Usage != nil {
		usage = &Usage{
			PromptTokens:     result.Usage.InputTokens,
			CompletionTokens: result.Usage.OutputTokens,
			TotalTokens:      result.Usage.InputTokens + result.Usage.OutputTokens,
		}
	}

	return &ChatResponse{
		Content: content,
		Model:   result.Model,
		Usage:   usage,
	}, nil
}

func (p *claudeProvider) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	clReq := p.buildClaudeRequest(req, true)
	jsonBody, err := json.Marshal(clReq)
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("marshal: %w", err)}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	p.setAuth(httpReq)

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

func (p *claudeProvider) processSSE(ctx context.Context, body io.ReadCloser, ch chan<- StreamChunk) {
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
			select {
			case ch <- StreamChunk{Done: true}:
			case <-ctx.Done():
			}
			return
		}

		var event struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_delta":
			var delta struct {
				Delta struct {
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &delta); err != nil {
				continue
			}
			if delta.Delta.Text != "" {
				fullContent.WriteString(delta.Delta.Text)
				select {
				case ch <- StreamChunk{Content: delta.Delta.Text}:
				case <-ctx.Done():
					return
				}
			}

		case "message_stop":
			select {
			case ch <- StreamChunk{Done: true, FinishReason: "stop"}:
			case <-ctx.Done():
			}
			return

		case "error":
			var errEvent struct {
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(data), &errEvent); err != nil {
				continue
			}
			select {
			case ch <- StreamChunk{Error: errEvent.Error.Message, Done: true}:
			case <-ctx.Done():
			}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case ch <- StreamChunk{Error: err.Error(), Done: true}:
		case <-ctx.Done():
		}
		return
	}

	if fullContent.Len() > 0 {
		select {
		case ch <- StreamChunk{Done: true}:
		case <-ctx.Done():
		}
	}
}

func (p *claudeProvider) buildClaudeRequest(req ChatRequest, stream bool) claudeRequest {
	var systemText string
	var msgs []claudeMsg

	for _, m := range req.Messages {
		if m.Role == "system" || m.Role == "developer" {
			if text, ok := m.Content.(string); ok {
				systemText += text + "\n"
			}
			continue
		}
		role := m.Role
		blocks := []claudeBlock{}
		switch v := m.Content.(type) {
		case string:
			blocks = append(blocks, claudeBlock{Type: "text", Text: v})
		case []ContentPart:
			for _, part := range v {
				if part.Text != "" {
					blocks = append(blocks, claudeBlock{Type: "text", Text: part.Text})
				}
			}
		}
		msgs = append(msgs, claudeMsg{Role: role, Content: blocks})
	}

	clReq := claudeRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Messages:    msgs,
		System:      strings.TrimSpace(systemText),
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      stream,
	}
	if clReq.MaxTokens <= 0 {
		clReq.MaxTokens = 4096
	}

	return clReq
}

func (p *claudeProvider) setAuth(req *http.Request) {
	if p.apiKey != "" {
		req.Header.Set("x-api-key", p.apiKey)
	}
}

func (p *claudeProvider) wrapError(err error) error {
	if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "timeout") {
		return &ProviderError{Provider: p.Name(), Err: ErrTimeout}
	}
	return &ProviderError{Provider: p.Name(), Err: err}
}

func (p *claudeProvider) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return &ProviderError{Provider: p.Name(), Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
}
