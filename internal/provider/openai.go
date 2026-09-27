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

type openAIProvider struct {
	cfg      ProviderConfig
	provType ProviderType // configured type — may be "openai" or "custom"
	baseURL  string
	model    string
	apiKey   string
	client   *http.Client
	streamCl *http.Client
	// noStreamUsage latches when this endpoint has rejected
	// stream_options.include_usage, so the retry happens once per provider
	// instance instead of on every message. Atomic because a provider is
	// shared across concurrent streams (chat + background calls).
	noStreamUsage atomic.Bool
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
			// The non-streaming path — used by the agent pipeline
			// (pipeline.go calls ChatCompletion, not ...Stream), so this
			// also bounds every planner / coder / escalator turn in the
			// Self-Driving loop. 120s was too tight: a big planning call to a
			// slow or queued endpoint returns one large JSON body with no
			// progress signal, and 120s killed it mid-generation (the
			// planner then failed the whole list). 300s still fails a
			// genuinely dead endpoint; the loop's own retry covers a
			// transient one.
			Timeout: 300 * time.Second,
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

	// A 401/403 (bad/expired key) from a real OpenAI-compatible backend
	// typically returns a well-formed JSON error body ({"error":{...}}),
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
	// StreamOptions asks the server to append a final chunk carrying this
	// stream's usage — including prompt_tokens_details.cached_tokens, which is
	// the only way a *streaming* turn can report its prompt-cache hit. Sent
	// only on streaming requests, and dropped for the rest of the process's
	// life if the endpoint rejects it (see streamUsageUnsupported): it is a
	// standard OpenAI field that OpenRouter, Groq, xAI, Ollama and
	// llama-server all accept, but this package also talks to arbitrary
	// user-configured "custom" endpoints, and a hard 400 on every chat
	// message would be a far worse outcome than missing a cache figure.
	StreamOptions *openAIStreamOptions `json:"stream_options,omitempty"`
}

type openAIReasoning struct {
	Effort string `json:"effort,omitempty"`
}

type openAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
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
	// PromptTokensDetails carries OpenAI's automatic prompt-cache accounting.
	// Nothing has to be sent to earn it — any prompt whose stable prefix is
	// long enough is cached server-side — but it is only *observable* through
	// this field, which nothing here read until now. Every OpenAI-compatible
	// provider in this package (grok, groq, openrouter, ollama, llamacpp,
	// opencode, kilo, cline) shares this struct; the ones whose backends
	// don't report it simply leave it nil, which reads as zero.
	PromptTokensDetails *openAIPromptTokensDetails `json:"prompt_tokens_details,omitempty"`
}

// openAIPromptTokensDetails is the breakdown OpenAI (and the vendors that
// copy its schema) attaches to usage.prompt_tokens. CachedTokens is a SUBSET
// of prompt_tokens, not an addition to it — see provider.Usage's doc comment
// for why that distinction matters when comparing against Anthropic.
type openAIPromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

// cachedTokens reports the cache-hit portion of this usage, clamped to the
// prompt total. A provider that omits the details object (most
// OpenAI-compatible backends) yields 0.
func (u openAIUsage) cachedTokens() int {
	if u.PromptTokensDetails == nil {
		return 0
	}
	c := u.PromptTokensDetails.CachedTokens
	if c < 0 {
		return 0
	}
	if c > u.PromptTokens {
		// A cache figure larger than the input it belongs to is nonsense;
		// trusting it would make FreshPromptTokens clamp to 0 and report a
		// free turn. Cap instead of propagating the contradiction.
		return u.PromptTokens
	}
	return c
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

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("read body: %w", err)}
	}

	var result openAIResponse
	if p.provType == ProviderCline {
		// Cline's hosted gateway wraps the non-streaming response in its own
		// envelope — {"success":true,"data":{...the real OpenAI-shaped chat
		// completion...}} — instead of returning it at the top level like
		// every other thin-wrapper provider (OpenRouter/Kilo/OpenCode Zen).
		// Undetected, this silently discarded every completion Cline ever
		// returned: a live capture (2026-09-17) showed a fully-formed
		// open_app tool call — finish_reason "tool_calls", the right
		// app_name argument, a "reasoning" field explaining the model's
		// choice — sitting unseen inside `data` while the code that only
		// looked at the top-level `choices` field saw nothing and reported
		// a fake-empty response. Streaming (ChatCompletionStream/processSSE
		// below) is unaffected — verified separately against the same
		// gateway, SSE deltas arrive unwrapped — so only this path needs it.
		var envelope struct {
			Data openAIResponse `json:"data"`
		}
		if err := json.Unmarshal(rawBody, &envelope); err != nil {
			return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("decode: %w", err)}
		}
		result = envelope.Data
	} else if err := json.Unmarshal(rawBody, &result); err != nil {
		return nil, &ProviderError{Provider: p.Name(), Err: fmt.Errorf("decode: %w", err)}
	}

	if len(result.Choices) == 0 {
		// An HTTP 200 with no choices at all is unusual enough across every
		// OpenAI-compatible provider this app talks to that it's worth a
		// permanent, low-volume log line — this exact shape (silently
		// treated as a legitimate empty turn) is what hid the Cline envelope
		// bug above from every earlier "why did the model say nothing" report.
		logx.Printf("PROVIDER: %s returned HTTP 200 with zero choices (model=%q) — treating as an empty turn", p.Name(), result.Model)
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
			// OpenAI reports cached tokens as a subset of prompt_tokens, which
			// is already provider.Usage's contract — pass it through as-is, no
			// arithmetic. There is no cache-write figure to report: automatic
			// caching carries no write premium.
			CachedPromptTokens: result.Usage.cachedTokens(),
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

	// Ask for the trailing usage chunk unless this endpoint has already
	// refused it once. Without this, a streaming (plain chat) turn reports no
	// usage at all and the stats store falls back to a len/3 estimate — which
	// can't know anything about prompt caching, so a cached turn and an
	// uncached one look identical.
	wantUsage := !p.noStreamUsage.Load()
	if wantUsage {
		body.StreamOptions = &openAIStreamOptions{IncludeUsage: true}
	}

	resp, err := p.doStreamRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	ch := make(chan StreamChunk, 128)
	go p.processSSE(ctx, resp.Body, ch, wantUsage)
	return ch, nil
}

// doStreamRequest posts one streaming chat request, retrying once without
// stream_options when that field is what the endpoint rejected.
//
// The retry exists because this code path serves arbitrary user-configured
// "custom" OpenAI-compatible endpoints alongside the vendors that certainly
// support the field. A gateway that validates unknown parameters strictly
// would otherwise 400 on every single chat message the moment this feature
// shipped — trading "no cache figure" for "the app is broken". The refusal is
// latched on the provider instance, so the cost is one wasted request per
// process, not per message.
func (p *openAIProvider) doStreamRequest(ctx context.Context, body openAIChatRequest) (*http.Response, error) {
	resp, err := p.postStream(ctx, body)
	if err == nil && resp.StatusCode == http.StatusOK {
		return resp, nil
	}

	// A transport/marshal error tells us nothing about field support — never
	// latch on it, or one flaky connection would permanently disable usage
	// reporting.
	if err != nil {
		return nil, err
	}

	if body.StreamOptions == nil {
		defer resp.Body.Close()
		return nil, p.parseError(resp)
	}

	// Only 400 and 422 mean "I don't understand this request". 401/403 is the
	// API key, 404 is the route, 429 is the quota, 5xx is the server — none of
	// them say anything about field support, and retrying on them would both
	// double every failed request and permanently disable usage reporting
	// because of, say, an expired key.
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusUnprocessableEntity {
		defer resp.Body.Close()
		return nil, p.parseError(resp)
	}

	firstErr := p.parseError(resp)
	resp.Body.Close()

	body.StreamOptions = nil
	retry, retryErr := p.postStream(ctx, body)
	if retryErr != nil {
		return nil, retryErr
	}
	if retry.StatusCode != http.StatusOK {
		// The field wasn't the problem after all — report the ORIGINAL error,
		// which is the one describing the request the caller actually made,
		// and don't latch: a bad API key must not also disable usage
		// reporting for the rest of the session.
		defer retry.Body.Close()
		return nil, firstErr
	}

	p.noStreamUsage.Store(true)
	logx.Printf("PROVIDER: %s rejected stream_options.include_usage (%v) — streaming turns will fall back to estimated token counts for this session",
		p.Name(), firstErr)
	return retry, nil
}

func (p *openAIProvider) postStream(ctx context.Context, body openAIChatRequest) (*http.Response, error) {
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
	return resp, nil
}

// streamUsageGrace bounds how long processSSE will keep reading after the
// model's final content chunk, waiting for the separate usage chunk that
// stream_options.include_usage produces. A conforming server sends it
// immediately (same TCP burst, typically sub-millisecond); the timer exists
// only so an endpoint that accepted the field but never emits the chunk — and
// also never sends [DONE] nor closes the connection — can't hold the reply's
// completion open. On expiry the body is closed, which unblocks the scanner
// and lets the tail path finish the turn normally.
// A var, not a const, only so the watchdog test doesn't have to sleep for two
// seconds. Never reassigned in production code.
var streamUsageGrace = 2 * time.Second

// processSSE translates an OpenAI-compatible SSE stream into StreamChunks.
// wantUsage tells it whether this request asked for the trailing usage chunk;
// when it did, the terminal Done is held back until that chunk (or [DONE], or
// end of stream) arrives, so the usage can ride on it — every consumer of this
// channel stops reading at the first Done, so a usage chunk sent afterwards
// would be discarded.
func (p *openAIProvider) processSSE(ctx context.Context, body io.ReadCloser, ch chan<- StreamChunk, wantUsage bool) {
	defer body.Close()
	defer close(ch)
	defer logx.Recover("openAIProvider.processSSE")

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 65536), 10*1024*1024)

	var fullContent strings.Builder
	var usage *Usage
	// pendingFinish holds the finish_reason of a stream whose content is done
	// but whose usage chunk hasn't arrived yet; empty means nothing pending.
	// A separate bool would be redundant — finish_reason is never empty when a
	// choice reports one.
	pendingFinish := ""
	var graceTimer *time.Timer
	stopGrace := func() {
		if graceTimer != nil {
			graceTimer.Stop()
			graceTimer = nil
		}
	}
	defer stopGrace()

	// terminal builds the final chunk, attaching usage if any was reported.
	terminal := func(finishReason string) StreamChunk {
		return StreamChunk{Done: true, FinishReason: finishReason, Usage: usage}
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
			stopGrace()
			trySend(ctx, ch, terminal(pendingFinish))
			return
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Read usage BEFORE the empty-choices guard: the include_usage chunk
		// is defined to carry an empty choices array, so checking choices
		// first would skip the only chunk that has the numbers on it.
		if chunk.Usage != nil && (chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0) {
			usage = &Usage{
				PromptTokens:       chunk.Usage.PromptTokens,
				CompletionTokens:   chunk.Usage.CompletionTokens,
				TotalTokens:        chunk.Usage.TotalTokens,
				CachedPromptTokens: chunk.Usage.cachedTokens(),
			}
			if pendingFinish != "" {
				// This is what the grace period was waiting for.
				stopGrace()
				trySend(ctx, ch, terminal(pendingFinish))
				return
			}
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
			finish := *chunk.Choices[0].FinishReason
			if !wantUsage || usage != nil {
				trySend(ctx, ch, terminal(finish))
				return
			}
			// Hold the Done back for the trailing usage chunk, under a
			// watchdog so a server that promised usage and never delivers it
			// can't stall the turn. Closing the body is what unblocks the
			// scanner below; net/http supports Close concurrent with a
			// blocked Read precisely for this.
			pendingFinish = finish
			graceTimer = time.AfterFunc(streamUsageGrace, func() { body.Close() })
			continue
		}
	}

	stopGrace()

	if err := scanner.Err(); err != nil {
		// A pending finish means the content already arrived in full and this
		// error is from the post-content wait (very likely our own watchdog
		// closing the body). Reporting it as a stream error would turn a
		// complete, correct answer into a visible failure.
		if pendingFinish != "" {
			trySend(ctx, ch, terminal(pendingFinish))
			return
		}
		trySend(ctx, ch, StreamChunk{Error: err.Error(), Done: true})
		return
	}

	// Always send Done when the stream ends without [DONE] or FinishReason —
	// tool-use-only responses produce empty fullContent but still need to
	// unblock the consumer.
	trySend(ctx, ch, terminal(pendingFinish))
}

type openAIStreamChunk struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []openAIStreamChoice `json:"choices"`
	// Usage is present only on the extra final chunk emitted when the request
	// asked for stream_options.include_usage. That chunk carries an EMPTY
	// choices array, which is why processSSE must read usage before its
	// len(Choices) == 0 guard rather than after.
	Usage *openAIUsage `json:"usage,omitempty"`
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
