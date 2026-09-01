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
			// Non-stream path also bounds agent-pipeline turns (planner/coder
			// / escalator in the Self-Driving loop). 120s/30s were too tight
			// for a big planning call to a reasoning model.
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
				ResponseHeaderTimeout: 30 * time.Second,
			},
		},
	}, nil
}

func (p *geminiProvider) Name() ProviderType  { return ProviderGemini }
func (p *geminiProvider) DisplayName() string { return "Google Gemini" }

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
			Name        string `json:"name"`
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
	Contents          []geminiContent  `json:"contents"`
	GenerationConfig  *geminiGenConfig `json:"generationConfig,omitempty"`
	SystemInstruction *geminiContent   `json:"system_instruction,omitempty"`
	// Tools is Gemini's own {functionDeclarations:[...]} wrapper — a single
	// entry holding every declared function, not one entry per function
	// (verified against current docs, 2026-09-01). nil (field omitted), not
	// an empty slice, when the caller passed no tools.
	Tools []geminiToolDecl `json:"tools,omitempty"`
}

// geminiToolDecl is generateContent's tools[] entry. Gemini expects exactly
// one such entry carrying every function the model may call, not one entry
// per function — see buildGeminiRequest.
type geminiToolDecl struct {
	FunctionDeclarations []geminiFunctionDecl `json:"functionDeclarations"`
}

type geminiFunctionDecl struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Parameters is already JSON Schema (the same ToolFunction.Parameters
	// every other provider forwards as-is) — Gemini accepts the OpenAPI 3.0
	// schema subset directly, no translation needed for Memo's own tool
	// definitions (object/string/array/enum — nothing from JSON Schema's
	// wider vocabulary that OpenAPI 3.0 lacks).
	Parameters json.RawMessage `json:"parameters,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

// geminiPart covers every part shape generateContent uses. Exactly one of
// Text / FunctionCall / FunctionResponse is set per part; omitempty keeps
// each JSON-encoded part down to just its own fields, same reasoning as
// claudeBlock's doc comment in claude.go.
type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

// geminiFunctionCall is both directions: echoed back into history when
// replaying a prior "model" turn's tool calls, and parsed out of a fresh
// response. ID is only populated on newer models/API versions (per Google's
// docs, "Gemini 3 now always returns a unique id with every functionCall");
// when the upstream response omits it, ToolCall.ID comes back empty and
// pipeline.go's own generateID() fallback (execute_tool_call.go) covers it,
// same as every other provider that can omit an id.
type geminiFunctionCall struct {
	ID   string          `json:"id,omitempty"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

// geminiFunctionResponse is outgoing only — Memo never receives one.
// Response must be a JSON *object*, not a bare string (unlike Anthropic's
// tool_result.content, which accepts plain text) — Memo's tool results are
// always plain strings, so buildGeminiRequest wraps one as {"result": "..."}.
// The wrapper key is cosmetic; the model only needs valid JSON back.
type geminiFunctionResponse struct {
	ID       string          `json:"id,omitempty"`
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

type geminiGenConfig struct {
	Temperature     float64               `json:"temperature,omitempty"`
	TopP            float64               `json:"topP,omitempty"`
	MaxOutputTokens int                   `json:"maxOutputTokens,omitempty"`
	ThinkingConfig  *geminiThinkingConfig `json:"thinkingConfig,omitempty"`
}

// geminiThinkingConfig is generateContent's own thinking control (verified
// against current Gemini API docs, 2026-08-18) — a token budget, not a
// named effort level like every other vendor in this package. See
// GeminiThinkingBudgetForLevel (effort.go) for the label→budget mapping
// Memo's own UI uses to keep the same low/medium/high/max vocabulary
// consistent across providers despite Gemini's API shape being different
// underneath.
type geminiThinkingConfig struct {
	ThinkingBudget int `json:"thinkingBudget"`
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
	var toolCalls []ToolCall
	for _, part := range result.Candidates[0].Content.Parts {
		if part.Text != "" {
			content += part.Text
		}
		if part.FunctionCall != nil {
			// Function.Arguments is json.RawMessage on ToolCall too (the same
			// shape every provider returns; pipeline.go replays it back
			// verbatim as Message.ToolCalls) — part.FunctionCall.Args needs
			// no re-encoding, it's already the raw JSON object Gemini sent.
			toolCalls = append(toolCalls, ToolCall{
				ID:   part.FunctionCall.ID,
				Type: "function",
				Function: ToolCallFunction{
					Name:      part.FunctionCall.Name,
					Arguments: part.FunctionCall.Args,
				},
			})
		}
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
		Content:   content,
		ToolCalls: toolCalls,
		Model:     model,
		Usage:     usage,
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
	defer logx.Recover("geminiProvider.processSSE")

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
	if budget, ok := GeminiThinkingBudgetForLevel(req.EffortLevel); ok {
		gReq.GenerationConfig.ThinkingConfig = &geminiThinkingConfig{ThinkingBudget: budget}
	}

	var systemText string

	// Same reasoning as claude.go's buildClaudeRequest: pipeline.go appends
	// one Message{Role:"tool",...} per tool call (execute_tool_call.go), so
	// a turn with N parallel calls arrives here as N consecutive "tool"
	// messages. Gemini's functionResponse parts for one "model" turn's
	// functionCalls are conventionally batched into a single following
	// "user" content the same way Claude's tool_result blocks are — pending
	// buffers that run, flushed the moment a non-tool message (or the end
	// of the slice) breaks it.
	var pending []geminiPart
	flush := func() {
		if len(pending) == 0 {
			return
		}
		gReq.Contents = append(gReq.Contents, geminiContent{Role: "user", Parts: pending})
		pending = nil
	}

	for _, m := range req.Messages {
		if m.Role == "system" || m.Role == "developer" {
			if text, ok := m.Content.(string); ok {
				systemText += text + "\n"
			}
			continue
		}

		if m.Role == "tool" {
			text, _ := m.Content.(string)
			respJSON, err := json.Marshal(map[string]string{"result": text})
			if err != nil {
				respJSON = []byte(`{"result":""}`)
			}
			pending = append(pending, geminiPart{
				FunctionResponse: &geminiFunctionResponse{
					ID:       m.ToolCallID,
					Name:     toolNameForCallID(req.Messages, m.ToolCallID),
					Response: respJSON,
				},
			})
			continue
		}
		flush()

		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		gContent := geminiContent{Role: role}
		switch v := m.Content.(type) {
		case string:
			if v != "" {
				gContent.Parts = append(gContent.Parts, geminiPart{Text: v})
			}
		case []ContentPart:
			for _, part := range v {
				if part.Text != "" {
					gContent.Parts = append(gContent.Parts, geminiPart{Text: part.Text})
				}
			}
		}
		// Echo this turn's own tool calls back as functionCall parts — the
		// following turn's functionResponse parts need one of these earlier
		// in the conversation to attach to.
		for _, tc := range m.ToolCalls {
			gContent.Parts = append(gContent.Parts, geminiPart{
				FunctionCall: &geminiFunctionCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
					Args: tc.Function.Arguments,
				},
			})
		}
		if len(gContent.Parts) == 0 {
			continue
		}
		gReq.Contents = append(gReq.Contents, gContent)
	}
	flush()

	if systemText != "" {
		gReq.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: strings.TrimSpace(systemText)}},
		}
	}

	gReq.Tools = toGeminiTools(req.Tools)

	return gReq
}

// toolNameForCallID recovers the tool name for a "tool" role message —
// unlike OpenAI/Anthropic, Gemini's functionResponse wants the function
// name repeated (not just its call id), so this walks back through the
// messages already seen for the assistant ToolCall with matching ID. Always
// finds one in practice: pipeline.go never appends a "tool" message without
// having just appended the assistant turn that requested it.
func toolNameForCallID(msgs []Message, callID string) string {
	for _, m := range msgs {
		for _, tc := range m.ToolCalls {
			if tc.ID == callID {
				return tc.Function.Name
			}
		}
	}
	return ""
}

// toGeminiTools translates the OpenAI-shaped ToolDefinition list into
// Gemini's single-entry {functionDeclarations:[...]} wrapper. Returns nil
// (field omitted) for no tools.
func toGeminiTools(defs []ToolDefinition) []geminiToolDecl {
	if len(defs) == 0 {
		return nil
	}
	decls := make([]geminiFunctionDecl, 0, len(defs))
	for _, d := range defs {
		decls = append(decls, geminiFunctionDecl{
			Name:        d.Function.Name,
			Description: d.Function.Description,
			Parameters:  d.Function.Parameters,
		})
	}
	return []geminiToolDecl{{FunctionDeclarations: decls}}
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
	return &ProviderError{Provider: p.Name(), Err: fmt.Errorf("status %d: %s", resp.StatusCode, ExtractErrorMessage(body))}
}
