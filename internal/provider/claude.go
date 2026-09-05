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

func (p *claudeProvider) Name() ProviderType  { return ProviderClaude }
func (p *claudeProvider) DisplayName() string { return "Anthropic Claude" }

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
	Model     string      `json:"model"`
	MaxTokens int         `json:"max_tokens"`
	Messages  []claudeMsg `json:"messages"`
	// System is either a plain string or, when prompt caching is in play, a
	// []claudeSystemBlock so the last block can carry cache_control. Anthropic
	// accepts both shapes.
	System       any                 `json:"system,omitempty"`
	Temperature  float64             `json:"temperature,omitempty"`
	TopP         float64             `json:"top_p,omitempty"`
	Stream       bool                `json:"stream"`
	Thinking     *claudeThinking     `json:"thinking,omitempty"`
	OutputConfig *claudeOutputConfig `json:"output_config,omitempty"`
	// Tools is Anthropic's own {name, description, input_schema} shape —
	// not the OpenAI {type,function:{...}} envelope ChatRequest.Tools
	// carries in from the agent pipeline. buildClaudeRequest translates one
	// into the other. Omitted (not an empty array) when the caller passed
	// no tools, matching every other provider here and avoiding an
	// unnecessary tool_choice negotiation on a plain chat turn.
	Tools []claudeTool `json:"tools,omitempty"`
}

// claudeTool is one entry in the Messages API's "tools" array (Anthropic
// docs, 2026-08-18). Parameters is already JSON Schema (the same
// ToolFunction.Parameters every other provider forwards as-is), so no
// translation beyond the field name is needed.
type claudeTool struct {
	Name         string              `json:"name"`
	Description  string              `json:"description,omitempty"`
	InputSchema  json.RawMessage     `json:"input_schema"`
	CacheControl *claudeCacheControl `json:"cache_control,omitempty"`
}

// claudeCacheControl marks a prefix boundary for Anthropic prompt caching.
// Placed on the last tool and the system block, it lets the (stable, large)
// system-prompt + tool-schema prefix be served from cache on iterations
// 2..N of an agent turn at ~10% of the input price. Only sent to
// api.anthropic.com — a custom Anthropic-compatible endpoint that doesn't
// know the field would 400 on it.
type claudeCacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

// claudeSystemBlock is the array form of the "system" field, used only when
// cache control is being applied.
type claudeSystemBlock struct {
	Type         string              `json:"type"` // "text"
	Text         string              `json:"text"`
	CacheControl *claudeCacheControl `json:"cache_control,omitempty"`
}

// claudeThinking/claudeOutputConfig implement adaptive thinking — the
// vendor's current mechanism (thinking:{type:"adaptive"} paired with
// output_config.effort), verified against current Anthropic docs
// (2026-08-18). Deliberately NOT the older thinking:{type:"enabled",
// budget_tokens:N} manual-budget mode: that mode is deprecated on Claude
// 4.6-generation models (still works, with a warning) and rejected outright
// (400) on 4.7 and later — adaptive mode is what Anthropic tells
// integrators to move to. Adaptive mode itself 400s on Claude Sonnet 4.5,
// Opus 4.5, Haiku 4.5 and earlier, which support only the deprecated manual
// mode — this used to be an accepted, unguarded limitation here, but is now
// closed at the source: the effort-level picker (GET
// /api/providers/effort-levels, handlers_oauth.go's
// fetchClaudeModelEffortLevels) queries GET /v1/models/{id}'s
// capabilities.effort.supported live, per the exact model configured, so
// EffortLevel is never set to a non-empty value on a model that would 400
// on this request shape in the first place.
type claudeThinking struct {
	Type string `json:"type"` // always "adaptive" here
}

type claudeOutputConfig struct {
	Effort string `json:"effort,omitempty"`
}

type claudeMsg struct {
	Role    string        `json:"role"`
	Content []claudeBlock `json:"content"`
}

// claudeBlock is every content-block shape the Messages API uses, in one
// struct (Go's default marshaling drops the zero-value fields via
// omitempty, so a text block, a tool_use block, and a tool_result block
// each serialize to exactly their own three-or-so keys with nothing extra):
//   - {"type":"text","text":"..."}
//   - {"type":"tool_use","id":"...","name":"...","input":{...}}      — outgoing echo
//     of a previous assistant turn's tool call, and incoming in a response
//   - {"type":"tool_result","tool_use_id":"...","content":"..."}     — outgoing only
type claudeBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`

	// thinking (incoming only — BUG-THINK1). Anthropic requires a turn's
	// thinking block to be echoed back verbatim (with its signature) on any
	// follow-up request that continues that same turn's tool_use loop; this
	// package's buildClaudeRequest never replays thinking or text blocks
	// today, only tool_use ones (see its own doc comment), and the plain
	// chat path that owns effort levels/thinking never sends req.Tools at
	// all — so there is no such follow-up request to get wrong here. If
	// Claude ever gets extended thinking + tool use on the same turn (agent
	// pipeline, currently non-stream and out of this bug's scope), a
	// Signature field and replay logic would need to be added then.
	Thinking string `json:"thinking,omitempty"`

	// tool_use (both directions: sent back when replaying an assistant
	// turn's tool calls into history, and parsed out of a fresh response).
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result (outgoing only — Memo never receives one).
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type claudeResponse struct {
	ID         string        `json:"id"`
	Type       string        `json:"type"`
	Role       string        `json:"role"`
	Content    []claudeBlock `json:"content"`
	Model      string        `json:"model"`
	Usage      *claudeUsage  `json:"usage,omitempty"`
	StopReason string        `json:"stop_reason,omitempty"`
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

	clReq := p.buildClaudeRequest(req, model, false)
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
	var thinking strings.Builder
	var toolCalls []ToolCall
	for _, block := range result.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "thinking":
			// BUG-THINK1: this block type was silently ignored before —
			// falls through the switch, ChatResponse.Thinking stayed "".
			thinking.WriteString(block.Thinking)
		case "tool_use":
			// Same shape resp.ToolCalls always carries regardless of
			// provider (pipeline.go replays it back as Message.ToolCalls
			// verbatim) — Function.Arguments is raw JSON either way, so
			// block.Input (already json.RawMessage) needs no re-encoding.
			toolCalls = append(toolCalls, ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: ToolCallFunction{
					Name:      block.Name,
					Arguments: block.Input,
				},
			})
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
		Content:   content,
		Thinking:  thinking.String(),
		ToolCalls: toolCalls,
		Model:     result.Model,
		Usage:     usage,
	}, nil
}

func (p *claudeProvider) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	clReq := p.buildClaudeRequest(req, model, true)
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
	defer logx.Recover("claudeProvider.processSSE")

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

		var event struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_delta":
			// BUG-THINK1: a thinking_delta event's payload used to be
			// silently dropped — this struct only ever had a Text field, so
			// json.Unmarshal just left it at its zero value with no error.
			var delta struct {
				Delta struct {
					Text     string `json:"text"`
					Thinking string `json:"thinking"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &delta); err != nil {
				continue
			}
			if delta.Delta.Thinking != "" {
				trySend(ctx, ch, StreamChunk{Thinking: delta.Delta.Thinking})
				if ctx.Err() != nil {
					return
				}
			}
			if delta.Delta.Text != "" {
				fullContent.WriteString(delta.Delta.Text)
				trySend(ctx, ch, StreamChunk{Content: delta.Delta.Text})
				if ctx.Err() != nil {
					return
				}
			}

		case "message_stop":
			trySend(ctx, ch, StreamChunk{Done: true, FinishReason: "stop"})
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
			trySend(ctx, ch, StreamChunk{Error: errEvent.Error.Message, Done: true})
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

// buildClaudeRequest takes the already-resolved model (req.Model if set,
// else the provider's own configured default — see ChatCompletion/
// ChatCompletionStream) as an explicit parameter rather than reading
// req.Model directly. This used to read req.Model itself, silently
// discarding the resolved-model fallback its own callers had just computed
// into an unused local variable: any caller that left ChatRequest.Model
// empty (e.g. internal/app/llm.go's main chat streaming path, which never
// sets it) sent Anthropic's API an empty "model" field on every single
// request, regardless of what model the provider was actually configured
// with in Settings.
func (p *claudeProvider) buildClaudeRequest(req ChatRequest, model string, stream bool) claudeRequest {
	var systemText string
	var msgs []claudeMsg

	// A "tool" role message never stands alone in Anthropic's shape — every
	// tool_result belonging to one assistant turn's tool_use blocks must be
	// content blocks inside a SINGLE following user message, not separate
	// consecutive user turns (which the API doesn't accept: roles must
	// strictly alternate). pipeline.go appends one Message{Role:"tool",...}
	// per tool call (execute_tool_call.go), so a turn with N parallel calls
	// arrives here as N consecutive "tool" messages that must be merged.
	// pendingResults buffers that run; flush() closes it out into one
	// claudeMsg the moment a non-tool message (or the end of the slice)
	// breaks the run.
	var pendingResults []claudeBlock
	flush := func() {
		if len(pendingResults) == 0 {
			return
		}
		msgs = append(msgs, claudeMsg{Role: "user", Content: pendingResults})
		pendingResults = nil
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
			pendingResults = append(pendingResults, claudeBlock{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   text,
			})
			continue
		}
		flush()

		role := m.Role
		blocks := []claudeBlock{}
		switch v := m.Content.(type) {
		case string:
			if v != "" {
				blocks = append(blocks, claudeBlock{Type: "text", Text: v})
			}
		case []ContentPart:
			for _, part := range v {
				if part.Text != "" {
					blocks = append(blocks, claudeBlock{Type: "text", Text: part.Text})
				}
			}
		}
		// Echo this turn's own tool calls back as tool_use blocks — required
		// so the NEXT turn's tool_result blocks have a tool_use_id to point
		// at; Anthropic rejects a tool_result with no matching tool_use
		// earlier in the same conversation.
		for _, tc := range m.ToolCalls {
			blocks = append(blocks, claudeBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: tc.Function.Arguments,
			})
		}
		if len(blocks) == 0 {
			// Anthropic rejects a message with an empty content array outright.
			continue
		}
		msgs = append(msgs, claudeMsg{Role: role, Content: blocks})
	}
	flush()

	clReq := claudeRequest{
		Model:       model,
		MaxTokens:   req.MaxTokens,
		Messages:    msgs,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      stream,
		Tools:       toClaudeTools(req.Tools),
	}

	// Prompt caching: on the real Anthropic API, mark the system prompt and
	// the tail of the tool list as a cacheable prefix so an agent turn's
	// later iterations don't re-pay full input price for the (unchanging)
	// system + tool schema. Gated to api.anthropic.com — a custom
	// Anthropic-compatible endpoint may not accept the cache_control field.
	sys := strings.TrimSpace(systemText)
	cacheable := strings.Contains(p.baseURL, "anthropic.com")
	if sys != "" {
		if cacheable {
			clReq.System = []claudeSystemBlock{{
				Type:         "text",
				Text:         sys,
				CacheControl: &claudeCacheControl{Type: "ephemeral"},
			}}
		} else {
			clReq.System = sys
		}
	}
	if cacheable && len(clReq.Tools) > 0 {
		clReq.Tools[len(clReq.Tools)-1].CacheControl = &claudeCacheControl{Type: "ephemeral"}
	}

	if clReq.MaxTokens <= 0 {
		clReq.MaxTokens = 4096
	}
	if req.EffortLevel != "" {
		clReq.Thinking = &claudeThinking{Type: "adaptive"}
		clReq.OutputConfig = &claudeOutputConfig{Effort: req.EffortLevel}
	}

	return clReq
}

// toClaudeTools translates the OpenAI-shaped ToolDefinition list the agent
// pipeline builds (registry.ToOpenAITools) into Anthropic's flatter
// {name, description, input_schema} shape. Returns nil (not an empty slice)
// for no tools, so the "tools" field is omitted rather than sent empty.
func toClaudeTools(defs []ToolDefinition) []claudeTool {
	if len(defs) == 0 {
		return nil
	}
	out := make([]claudeTool, 0, len(defs))
	for _, d := range defs {
		out = append(out, claudeTool{
			Name:        d.Function.Name,
			Description: d.Function.Description,
			InputSchema: d.Function.Parameters,
		})
	}
	return out
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
	return &ProviderError{Provider: p.Name(), Err: fmt.Errorf("status %d: %s", resp.StatusCode, ExtractErrorMessage(body))}
}
