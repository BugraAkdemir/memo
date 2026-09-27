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
	"sync/atomic"
	"time"
)

type claudeProvider struct {
	cfg      ProviderConfig
	baseURL  string
	model    string
	apiKey   string
	client   *http.Client
	streamCl *http.Client
	// noCacheControl latches when this endpoint has rejected a request
	// carrying cache_control, so the retry costs one wasted request per
	// process rather than one per message. Only reachable for a
	// custom-anthropic endpoint — api.anthropic.com defines the field.
	// Atomic because one provider instance serves concurrent streams.
	noCacheControl atomic.Bool
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

	// A 401/403 (bad/expired key) returns a well-formed JSON error body,
	// which used to decode "successfully" into a zero-value result (no
	// "data" key present) — CheckConnection (router.go) reads a nil error
	// from ListModels as Connected=true, so a provider with a bad key was
	// showing as "Connected" with an empty model list instead of surfacing
	// the real auth error. Found in a 2026-09-23 audit.
	if resp.StatusCode != http.StatusOK {
		return nil, p.parseError(resp)
	}

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
	// CacheCreationInputTokens/CacheReadInputTokens are only ever non-zero
	// when buildClaudeRequest actually marked something cacheable (see its
	// doc comment — anthropic.com + a tool-carrying turn only). Both were
	// missing from this struct entirely until now: Anthropic has reported
	// them on every response since prompt caching launched, but nothing
	// here ever read them, so there was no way to tell a cache hit from a
	// cache miss from a cache write that never happens — the request-side
	// logic was tested and correct, but its effect was unobservable.
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// logClaudeCachePerformance writes one log line classifying this response's
// usage as a cache hit, a cache write (first turn of a new 5-minute TTL
// window), or no cache breakpoint at all (plain chat, or a non-anthropic.com
// endpoint) — the only way to verify from the running app which turns are
// actually benefiting from prompt caching. See buildClaudeRequest for what
// makes a request cacheable in the first place.
func logClaudeCachePerformance(u claudeUsage) {
	switch {
	case u.CacheReadInputTokens > 0:
		logx.Printf("CLAUDE CACHE: HIT — cache_read=%d cache_creation=%d fresh_input=%d output=%d",
			u.CacheReadInputTokens, u.CacheCreationInputTokens, u.InputTokens, u.OutputTokens)
	case u.CacheCreationInputTokens > 0:
		logx.Printf("CLAUDE CACHE: MISS (wrote a new cache entry) — cache_creation=%d fresh_input=%d output=%d",
			u.CacheCreationInputTokens, u.InputTokens, u.OutputTokens)
	default:
		logx.Printf("CLAUDE CACHE: none — no cache breakpoint on this request — input=%d output=%d",
			u.InputTokens, u.OutputTokens)
	}
}

// toUsage normalizes Anthropic's accounting into provider.Usage's contract.
//
// The arithmetic is the whole point: Anthropic reports input_tokens as the
// tokens it actually *prefilled*, with cache_read_input_tokens and
// cache_creation_input_tokens listed SEPARATELY rather than included — so the
// better the cache performs, the smaller input_tokens gets. Copying it
// straight into PromptTokens (which is what happened before) meant a turn
// whose 8k-token system+tool prefix came entirely from cache was recorded as
// a few hundred prompt tokens, i.e. the usage stats under-reported exactly
// the turns where caching worked, and a cache-enabled agent turn looked
// *cheaper in input size* than the same turn with caching off. Adding both
// cache figures back in restores "PromptTokens = the whole input", which is
// what every OpenAI-compatible provider here already reports and what the
// stats screen's per-model totals assume.
func (u claudeUsage) toUsage() Usage {
	prompt := u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
	return Usage{
		PromptTokens:       prompt,
		CompletionTokens:   u.OutputTokens,
		TotalTokens:        prompt + u.OutputTokens,
		CachedPromptTokens: u.CacheReadInputTokens,
		CacheWriteTokens:   u.CacheCreationInputTokens,
	}
}

// carriesCacheControl reports whether clReq asks for prompt caching anywhere —
// the system block or the last tool. Used to decide whether a rejection could
// plausibly be about that field at all.
func carriesCacheControl(clReq claudeRequest) bool {
	if blocks, ok := clReq.System.([]claudeSystemBlock); ok {
		for _, b := range blocks {
			if b.CacheControl != nil {
				return true
			}
		}
	}
	for _, t := range clReq.Tools {
		if t.CacheControl != nil {
			return true
		}
	}
	return false
}

// stripCacheControl returns clReq with every cache_control marker removed and
// the system field collapsed back to a plain string — the shape an endpoint
// that predates prompt caching expects.
func stripCacheControl(clReq claudeRequest) claudeRequest {
	if blocks, ok := clReq.System.([]claudeSystemBlock); ok {
		var sb strings.Builder
		for _, b := range blocks {
			sb.WriteString(b.Text)
		}
		if sb.Len() > 0 {
			clReq.System = sb.String()
		} else {
			clReq.System = nil
		}
	}
	// Tools is a fresh slice per request (toClaudeTools allocates), but copy
	// anyway: mutating a caller's slice in place to un-ask for caching would be
	// a nasty surprise if that ever stops being true.
	if len(clReq.Tools) > 0 {
		tools := make([]claudeTool, len(clReq.Tools))
		copy(tools, clReq.Tools)
		for i := range tools {
			tools[i].CacheControl = nil
		}
		clReq.Tools = tools
	}
	return clReq
}

// postMessages sends one /messages request, retrying once without
// cache_control when that is what the endpoint rejected.
//
// Same compatibility valve, and the same narrow trigger, as openai.go's
// doStreamRequest: only 400/422 can mean "I don't understand this request", so
// a 401 (key), 404 (route), 429 (quota) or 5xx (server) is returned as-is
// rather than being misread as a field problem — which would both double every
// failed request and permanently disable caching because of an expired key. If
// the retry also fails, the ORIGINAL error is reported and nothing latches.
func (p *claudeProvider) postMessages(ctx context.Context, cl *http.Client, clReq claudeRequest, stream bool) (*http.Response, error) {
	resp, err := p.sendMessages(ctx, cl, clReq, stream)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusOK || !carriesCacheControl(clReq) {
		return resp, nil
	}
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusUnprocessableEntity {
		return resp, nil
	}

	firstErr := p.parseError(resp)
	resp.Body.Close()

	retry, retryErr := p.sendMessages(ctx, cl, stripCacheControl(clReq), stream)
	if retryErr != nil {
		return nil, retryErr
	}
	if retry.StatusCode != http.StatusOK {
		// cache_control wasn't the problem; hand back the error describing the
		// request the caller actually made, and don't latch.
		retry.Body.Close()
		return nil, firstErr
	}

	p.noCacheControl.Store(true)
	logx.Printf("PROVIDER: %s rejected cache_control (%v) — prompt caching disabled for this session on %s",
		p.Name(), firstErr, p.baseURL)
	return retry, nil
}

func (p *claudeProvider) sendMessages(ctx context.Context, cl *http.Client, clReq claudeRequest, stream bool) (*http.Response, error) {
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
	if stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}
	p.setAuth(httpReq)

	resp, err := cl.Do(httpReq)
	if err != nil {
		return nil, p.wrapError(err)
	}
	return resp, nil
}

func (p *claudeProvider) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	resp, err := p.postMessages(ctx, p.client, p.buildClaudeRequest(req, model, false), false)
	if err != nil {
		return nil, err
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
		u := result.Usage.toUsage()
		usage = &u
		logClaudeCachePerformance(*result.Usage)
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

	resp, err := p.postMessages(ctx, p.streamCl, p.buildClaudeRequest(req, model, true), true)
	if err != nil {
		return nil, err
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
	// streamUsage accumulates this stream's accounting so the terminal chunk
	// can carry it. Anthropic splits it across two events: message_start
	// carries input_tokens plus the (already final) cache figures, and each
	// message_delta carries the running output_tokens. Every terminal path
	// below attaches it via usageChunk() — without this, a streamed chat turn
	// (the plain non-agent path, which never goes through
	// agent/pipeline.go's own accounting) reported no usage at all and the
	// stats store fell back to a len/3 estimate that knows nothing about
	// caching.
	var streamUsage claudeUsage
	sawUsage := false
	usageChunk := func(c StreamChunk) StreamChunk {
		if !sawUsage {
			return c
		}
		u := streamUsage.toUsage()
		c.Usage = &u
		return c
	}

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
			trySend(ctx, ch, usageChunk(StreamChunk{Done: true}))
			return
		}

		var event struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "message_start":
			// The only place cache_creation/cache_read_input_tokens appear
			// on a streamed response — message_delta's usage only ever
			// carries the running output_tokens count, and message_stop
			// carries no usage at all. Logged as soon as it arrives (the
			// cache figures are already final here; only output_tokens
			// still grows) so streamed chat turns are visible in the same
			// cache hit/miss log as non-streaming ChatCompletion, which
			// previously had no visibility into caching whatsoever.
			var ms struct {
				Message struct {
					Usage claudeUsage `json:"usage"`
				} `json:"message"`
			}
			if err := json.Unmarshal([]byte(data), &ms); err == nil {
				streamUsage = ms.Message.Usage
				sawUsage = true
				logClaudeCachePerformance(ms.Message.Usage)
			}

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

		case "message_delta":
			// Carries the running output_tokens (and, on the final delta, the
			// total for the message). Assigned rather than accumulated: it is
			// a running total, so summing would multiply it.
			var md struct {
				Usage struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &md); err == nil && md.Usage.OutputTokens > 0 {
				streamUsage.OutputTokens = md.Usage.OutputTokens
				sawUsage = true
			}

		case "message_stop":
			trySend(ctx, ch, usageChunk(StreamChunk{Done: true, FinishReason: "stop"}))
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
			// Usage rides along on the error chunk too: whatever
			// message_start already reported was billed regardless of how the
			// stream ended, and drainAgentStream reads chunk.Usage before it
			// looks at chunk.Error, so dropping it here would silently
			// undercount every failed turn.
			trySend(ctx, ch, usageChunk(StreamChunk{Error: errEvent.Error.Message, Done: true}))
			return
		}
	}

	if err := scanner.Err(); err != nil {
		trySend(ctx, ch, usageChunk(StreamChunk{Error: err.Error(), Done: true}))
		return
	}

	if fullContent.Len() > 0 {
		trySend(ctx, ch, usageChunk(StreamChunk{Done: true}))
	}
}

// normalizeClaudeMessages enforces the two shape rules the Anthropic
// Messages API applies to the messages array, so callers upstream don't
// have to: (1) roles strictly alternate — consecutive same-role messages
// are merged by concatenating their content blocks; (2) the array cannot
// begin with an assistant message — a minimal user bridge is prepended if
// it would. Without (1) an intra-turn context-trim note landing next to the
// real user turn produced two consecutive user messages and a 400
// (BUG-SCAN2); (2) is defence-in-depth for any future caller.
func normalizeClaudeMessages(msgs []claudeMsg) []claudeMsg {
	if len(msgs) == 0 {
		return msgs
	}
	merged := make([]claudeMsg, 0, len(msgs))
	for _, m := range msgs {
		if n := len(merged); n > 0 && merged[n-1].Role == m.Role {
			merged[n-1].Content = append(merged[n-1].Content, m.Content...)
			continue
		}
		merged = append(merged, m)
	}
	if merged[0].Role == "assistant" {
		merged = append([]claudeMsg{{
			Role:    "user",
			Content: []claudeBlock{{Type: "text", Text: "(continued)"}},
		}}, merged...)
	}
	return merged
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

	msgs = normalizeClaudeMessages(msgs)

	clReq := claudeRequest{
		Model:       model,
		MaxTokens:   req.MaxTokens,
		Messages:    msgs,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      stream,
		Tools:       toClaudeTools(req.Tools),
	}

	// Prompt caching: mark the system prompt and the tail of the tool list as
	// a cacheable prefix so an agent turn's later iterations don't re-pay full
	// input price for the (unchanging) system + tool schema.
	//
	// This used to be gated to api.anthropic.com on the theory that a custom
	// Anthropic-compatible endpoint might not accept cache_control — which
	// meant a user pointing Memo at a Claude proxy got no caching at all, for
	// the whole class of endpoint where the saving matters just as much. It is
	// now attempted everywhere and withdrawn on refusal (see
	// cacheControlRejected / p.noCacheControl), the same compatibility valve
	// openai.go uses for stream_options.
	//
	// Still gated to tool-carrying (agent) turns: only there is the same
	// system prefix reused inside the 5-minute TTL (iterations 2..N of one
	// turn, system built once). On the plain chat path every turn rebuilds
	// the system prompt with a freshly retrieved memory block appended last,
	// so a breakpoint there would score ~0% hits while still paying the
	// 1.25x cache-write premium on a multi-KB prompt (BUG-SCAN15).
	sys := strings.TrimSpace(systemText)
	cacheable := len(clReq.Tools) > 0 && !p.noCacheControl.Load()
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
