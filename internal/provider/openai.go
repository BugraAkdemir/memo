package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/logx"
	"net/http"
	"strings"
	"time"
)

type openAIProvider struct {
	cfg      ProviderConfig
	provType ProviderType // configured type — may be "openai" or "custom"
	baseURL  string
	model    string
	apiKey   string
	client   *http.Client
	streamCl *http.Client
}

func newOpenAIProvider(cfg ProviderConfig) (*openAIProvider, error) {
	provType := cfg.Type
	if provType == "" {
		provType = ProviderOpenAI
	}
	baseURL := cfg.BaseURL
	// Only the real OpenAI type has a sane default endpoint. A "custom" provider
	// must supply its own Base URL — silently defaulting it to api.openai.com
	// would send the user's requests to the wrong place.
	if baseURL == "" && provType == ProviderOpenAI {
		baseURL = DefaultBaseURL(ProviderOpenAI)
	}
	return &openAIProvider{
		cfg:      cfg,
		provType: provType,
		baseURL:  strings.TrimRight(baseURL, "/"),
		model:    cfg.Model,
		apiKey:   cfg.APIKey,
		client: &http.Client{
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		// No overall Timeout — streaming responses are long-lived and the request
		// context bounds total duration. ResponseHeaderTimeout still guards the
		// "connection accepted, then silence" failure mode, but at 240s rather
		// than 90s: a reasoning model doing long hidden thinking, or a request
		// queued behind other traffic, legitimately needs more than 90s to send
		// its first byte on a big planning prompt (the Self-Driving planner hit
		// this on a queued free endpoint). A genuinely dead endpoint still fails
		// in 4 minutes. Mid-stream, the caller's own idle guard takes over (see
		// the Self-Driving loop's drainStreamIdle).
		streamCl: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:          10,
				MaxIdleConnsPerHost:   10,
				IdleConnTimeout:       90 * time.Second,
				ResponseHeaderTimeout: 240 * time.Second,
			},
		},
	}, nil
}

// Name reports the configured provider type so a "custom" OpenAI-compatible
// endpoint is routed and selected as itself, not as "openai".
func (p *openAIProvider) Name() ProviderType {
	if p.provType != "" {
		return p.provType
	}
	return ProviderOpenAI
}
func (p *openAIProvider) DisplayName() string {
	if p.provType == ProviderCustom {
		return "Custom"
	}
	return "OpenAI"
}

// applyEffortLevel sets body's reasoning-effort field in whichever shape
// p's actual configured type expects — see openAIChatRequest's
// ReasoningEffort/Reasoning doc comments for why OpenRouter alone needs the
// nested form. No-ops on an empty level (the "let the model use its own
// default" case).
func (p *openAIProvider) applyEffortLevel(body *openAIChatRequest, level string) {
	if level == "" {
		return
	}
	if p.provType == ProviderOpenRouter {
		body.Reasoning = &openAIReasoning{Effort: level}
		return
	}
	body.ReasoningEffort = level
}

func (p *openAIProvider) ListModels(ctx context.Context) ([]string, error) {
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
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

type openAIChatRequest struct {
	Model       string           `json:"model"`
	Messages    []openAIMessage  `json:"messages"`
	Temperature float64          `json:"temperature,omitempty"`
	TopP        float64          `json:"top_p,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Stream      bool             `json:"stream"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	// ReasoningEffort is Chat Completions' flat "reasoning_effort" field
	// (verified against current OpenAI API docs, 2026-08-18) — accepted,
	// silently ignored by non-reasoning models. Same field name/shape also
	// covers Grok, Groq, Ollama's OpenAI-compat endpoint, and llama-server
	// (see grok.go/groq.go/ollama.go/llamacpp.go), and OpenCode Zen/Go for
	// free via this struct. OpenRouter is the one wrapper that does NOT
	// use this field — see Reasoning below.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	// Reasoning is OpenRouter's own nested shape (verified against current
	// OpenRouter docs, 2026-08-18) — OpenRouter's API does not accept the
	// flat reasoning_effort field at all, only {"reasoning":{"effort":...}}.
	// Populated instead of ReasoningEffort when provType==ProviderOpenRouter
	// (see buildOpenAIChatRequest below); every other OpenAI-compatible
	// wrapper leaves this nil.
	Reasoning *openAIReasoning `json:"reasoning,omitempty"`
}

type openAIReasoning struct {
	Effort string `json:"effort,omitempty"`
}

type openAIMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
}

type openAIChoice struct {
	Index        int               `json:"index"`
	Message      openAIResponseMsg `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

type openAIResponseMsg struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

func (p *openAIProvider) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	body := openAIChatRequest{
		Model:       model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
		Tools:       req.Tools,
	}
	p.applyEffortLevel(&body, req.EffortLevel)
	body.Messages = p.toOpenAIMessages(req.Messages)

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("marshal: %w", err)}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	p.setAuth(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, p.wrapError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.parseError(resp)
	}

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("decode: %w", err)}
	}

	if len(result.Choices) == 0 {
		return &ChatResponse{Model: model}, nil
	}

	choice := result.Choices[0]
	return &ChatResponse{
		Content:   choice.Message.Content,
		ToolCalls: choice.Message.ToolCalls,
		Model:     result.Model,
		Usage: &Usage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}, nil
}

func (p *openAIProvider) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	body := openAIChatRequest{
		Model:       model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
		Tools:       req.Tools,
	}
	p.applyEffortLevel(&body, req.EffortLevel)
	body.Messages = p.toOpenAIMessages(req.Messages)

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("marshal: %w", err)}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")
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

func (p *openAIProvider) processSSE(ctx context.Context, body io.ReadCloser, ch chan<- StreamChunk) {
	defer body.Close()
	defer close(ch)
	defer logx.Recover("openAIProvider.processSSE")

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

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			fullContent.WriteString(delta.Content)
			trySend(ctx, ch, StreamChunk{Content: delta.Content})
			if ctx.Err() != nil {
				return
			}
		}

		if chunk.Choices[0].FinishReason != nil {
			trySend(ctx, ch, StreamChunk{Done: true, FinishReason: *chunk.Choices[0].FinishReason})
			return
		}
	}

	if err := scanner.Err(); err != nil {
		trySend(ctx, ch, StreamChunk{Error: err.Error(), Done: true})
		return
	}

	// Always send Done when the stream ends without [DONE] or FinishReason —
	// tool-use-only responses produce empty fullContent but still need to
	// unblock the consumer.
	trySend(ctx, ch, StreamChunk{Done: true})
}

type openAIStreamChunk struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []openAIStreamChoice `json:"choices"`
}

type openAIStreamChoice struct {
	Index        int               `json:"index"`
	Delta        openAIStreamDelta `json:"delta"`
	FinishReason *string           `json:"finish_reason"`
}

type openAIStreamDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

func (p *openAIProvider) setAuth(req *http.Request) {
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
}

func (p *openAIProvider) toOpenAIMessages(msgs []Message) []openAIMessage {
	out := make([]openAIMessage, 0, len(msgs))
	for _, m := range msgs {
		om := openAIMessage{Role: m.Role, Content: m.Content}
		if m.ToolCallID != "" {
			om.ToolCallID = m.ToolCallID
		}
		if len(m.ToolCalls) > 0 {
			om.ToolCalls = m.ToolCalls
		}
		out = append(out, om)
	}
	return out
}

func (p *openAIProvider) wrapError(err error) error {
	if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "timeout") {
		return &ProviderError{Provider: p.Name(), Err: ErrTimeout}
	}
	return &ProviderError{Provider: p.Name(), Err: err}
}

func (p *openAIProvider) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	errMsg := ExtractErrorMessage(body)

	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return &ProviderError{Provider: p.Name(), Err: fmt.Errorf("%w: %s", ErrRateLimited, errMsg)}
	case http.StatusUnauthorized, http.StatusForbidden:
		return &ProviderError{Provider: p.Name(), Err: fmt.Errorf("authentication error (status %d): %s", resp.StatusCode, errMsg)}
	default:
		return &ProviderError{Provider: p.Name(), Err: fmt.Errorf("status %d: %s", resp.StatusCode, errMsg)}
	}
}
