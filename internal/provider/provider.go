package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ProviderType enumerates supported LLM providers.
type ProviderType string

const (
	ProviderOpenAI     ProviderType = "openai"
	ProviderGemini     ProviderType = "gemini"
	ProviderGrok       ProviderType = "grok"
	ProviderGroq       ProviderType = "groq"
	ProviderClaude     ProviderType = "claude"
	ProviderOpenRouter ProviderType = "openrouter"
	ProviderOllama     ProviderType = "ollama"
	ProviderLlamaCPP   ProviderType = "llama.cpp"
	// ProviderOpenCodeZen is opencode.ai's pay-as-you-go model gateway (some
	// models are free). ProviderOpenCodeGo is their subscription-based gateway.
	// Both are OpenAI-compatible endpoints, so models are listed dynamically
	// via ListModels rather than typed in by hand.
	ProviderOpenCodeZen ProviderType = "opencode-zen"
	ProviderOpenCodeGo  ProviderType = "opencode-go"
	// ProviderClaudeCodeCLI shells out to the user's locally installed Claude
	// Code CLI (`claude`) instead of making an HTTP call — implemented in
	// internal/agentcli, not this package (see RegisterConstructor below for
	// why). Unlike every ProviderType above, this is a real coding agent with
	// its own file/shell execution, not a stateless chat-completion API.
	ProviderClaudeCodeCLI ProviderType = "claude-code-cli"
	// ProviderCodexCLI is the same idea as ProviderClaudeCodeCLI but for
	// OpenAI's `codex` CLI — implemented in internal/agentcli.
	ProviderCodexCLI ProviderType = "codex-cli"
	// ProviderCustom is any OpenAI-compatible endpoint the user points at via a
	// custom Base URL (self-hosted, proxies, providers we don't list natively).
	ProviderCustom ProviderType = "custom"
)

// Provider is the common interface all LLM providers must implement.
type Provider interface {
	Name() ProviderType
	DisplayName() string
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
	ListModels(ctx context.Context) ([]string, error)
}

// ChatRequest consolidates all parameters for a chat completion call.
type ChatRequest struct {
	Model       string
	Messages    []Message
	Tools       []ToolDefinition
	Temperature float64
	TopP        float64
	MaxTokens   int
	Stream      bool

	// ResumeSessionID and WorkDir are consumed only by CLI-backed providers
	// (internal/agentcli) that shell out to an external coding agent instead
	// of making an HTTP call — every other provider ignores them. A CLI
	// agent keeps its own multi-turn history keyed by ResumeSessionID, so
	// callers send just the latest message rather than the full transcript
	// this struct's Messages field carries for HTTP providers.
	ResumeSessionID string
	WorkDir         string
}

// Message represents a single message in a conversation.
type Message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
}

func TextMessage(role, text string) Message {
	return Message{Role: role, Content: text}
}

// ContentPart supports multimodal content (text + images).
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"`
}

// ToolDefinition is a tool that the LLM may call, defined in JSON Schema format
// (OpenAI tool calling standard). This struct is marshaled verbatim into the
// "tools" array of the actual request sent to the provider's API — it must
// only ever contain fields from that standard, nothing app-internal.
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ChatResponse is the non-streaming response from a provider.
type ChatResponse struct {
	Content   string
	ToolCalls []ToolCall
	Usage     *Usage
	Model     string
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk is a single chunk in a streaming response.
type StreamChunk struct {
	Content      string `json:"content"`
	Thinking     string `json:"thinking,omitempty"`
	Done         bool   `json:"done"`
	Error        string `json:"error,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`

	// CLISessionID is set by CLI-backed providers (internal/agentcli) once
	// the underlying process reports its own session id — the caller
	// persists it (per Memo chat) to pass back as ChatRequest.ResumeSessionID
	// on the next message. Empty for every HTTP provider.
	CLISessionID string `json:"cli_session_id,omitempty"`
}

// trySend delivers chunk to ch, preferring the send over ctx cancellation. A
// plain `select { case ch <- chunk: case <-ctx.Done(): }` lets Go's random
// tie-breaking between simultaneously-ready select cases silently drop the
// chunk — including the final Done:true one — if ctx becomes Done at the
// exact moment ch's reader is also ready to receive. Every processSSE-style
// producer in this package creates ch with a generous buffer (128), so the
// non-blocking attempt below succeeds immediately in the overwhelming
// majority of real cases, sidestepping the race entirely; the ctx-aware
// fallback only matters once that buffer is genuinely full, which itself
// means the reader already stopped consuming. Mirrors trySend in
// internal/app/llm.go, one layer further upstream in the same pipeline.
func trySend(ctx context.Context, ch chan<- StreamChunk, chunk StreamChunk) {
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

// ProviderConfig holds configuration for a single provider instance.
type ProviderConfig struct {
	Type        ProviderType `json:"type"`
	Name        string       `json:"name"`
	APIKey      string       `json:"api_key,omitempty"`
	BaseURL     string       `json:"base_url,omitempty"`
	Model       string       `json:"model"`
	Enabled     bool         `json:"enabled"`
	Priority    int          `json:"priority"`
	Temperature float64      `json:"temperature,omitempty"`
	TopP        float64      `json:"top_p,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	// ContextTokens is the model's context-window size, used to budget how much
	// chat history is packed into each request. 0 = use a sensible default.
	ContextTokens int `json:"context_tokens,omitempty"`

	// Connection check result (not persisted)
	Connected bool   `json:"connected,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (c ProviderConfig) Validate() error {
	if c.Type == "" {
		return fmt.Errorf("provider type is required")
	}
	if c.Model == "" {
		return fmt.Errorf("model is required for %s", c.Type)
	}
	// A custom provider has no default endpoint — without a Base URL its requests
	// would go nowhere and fail with an opaque "unsupported protocol scheme".
	if c.Type == ProviderCustom && strings.TrimSpace(c.BaseURL) == "" {
		return fmt.Errorf("base URL is required for a custom provider")
	}
	return nil
}

// ErrProviderNotAvailable is returned when no provider is configured/enabled.
var ErrProviderNotAvailable = fmt.Errorf("no provider configured or all providers failed")

// ErrRateLimited is returned when a provider returns a rate limit error.
var ErrRateLimited = fmt.Errorf("provider rate limited")

// ErrTimeout is returned when a provider request times out.
var ErrTimeout = fmt.Errorf("provider request timed out")

// ProviderError wraps provider-specific errors with context.
type ProviderError struct {
	Provider ProviderType
	Err      error
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("[%s] %v", e.Provider, e.Err)
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// ExtractErrorMessage pulls the human-readable message out of a provider's
// JSON error body — {"error": {"message": "...", ...}}, the shape OpenAI-
// compatible APIs, Claude, and Gemini all use — falling back to the raw body
// if it isn't in that shape or the message field is empty. Without this, a
// failed request surfaced its entire raw JSON error body (nested braces,
// "param":null, duplicate "type"/"code" fields) straight through
// Router.ChatCompletionStream's "all providers failed: %w" wrapping and
// into the chat UI verbatim — unreadable to a non-technical user trying to
// tell what actually went wrong.
func ExtractErrorMessage(body []byte) string {
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error.Message != "" {
		return parsed.Error.Message
	}
	return string(body)
}

// DefaultBaseURLs returns default base URLs for known provider types.
func DefaultBaseURL(p ProviderType) string {
	switch p {
	case ProviderOpenAI:
		return "https://api.openai.com/v1"
	case ProviderGemini:
		return "https://generativelanguage.googleapis.com/v1beta"
	case ProviderGrok:
		return "https://api.x.ai/v1"
	case ProviderGroq:
		return "https://api.groq.com/openai/v1"
	case ProviderClaude:
		return "https://api.anthropic.com/v1"
	case ProviderOpenRouter:
		return "https://openrouter.ai/api/v1"
	case ProviderOllama:
		return "http://127.0.0.1:11434/v1"
	case ProviderLlamaCPP:
		return "http://127.0.0.1:8081/v1"
	case ProviderOpenCodeZen:
		return "https://opencode.ai/zen/v1"
	case ProviderOpenCodeGo:
		return "https://opencode.ai/zen/go/v1"
	default:
		return ""
	}
}

var DefaultModels = map[ProviderType]string{
	ProviderOpenAI:        "gpt-4o",
	ProviderGemini:        "gemini-2.0-flash",
	ProviderGrok:          "grok-2",
	ProviderGroq:          "openai/gpt-oss-20b",
	ProviderClaude:        "claude-sonnet-4-20250514",
	ProviderOpenRouter:    "openai/gpt-4o",
	ProviderOllama:        "llama3",
	ProviderLlamaCPP:      "local-model",
	ProviderClaudeCodeCLI: "claude-code",
	ProviderCodexCLI:      "codex",
}

func init() {
	// Validate that DefaultBaseURL returns a value for all known types
	for _, pt := range []ProviderType{ProviderOpenAI, ProviderGemini, ProviderGrok, ProviderGroq, ProviderClaude, ProviderOpenRouter, ProviderOllama, ProviderLlamaCPP, ProviderOpenCodeZen, ProviderOpenCodeGo} {
		if DefaultBaseURL(pt) == "" {
			panic(fmt.Sprintf("missing default base URL for %s", pt))
		}
	}
}

// ConstructorFunc builds a Provider for one ProviderType. Used by
// RegisterConstructor below for provider implementations that would
// otherwise create an import cycle with this package.
type ConstructorFunc func(ProviderConfig) (Provider, error)

var externalConstructors = map[ProviderType]ConstructorFunc{}

// RegisterConstructor makes NewProvider able to build the given provider
// type via fn. Exists for provider implementations that need types from
// this package (Provider, ChatRequest, StreamChunk, ...) but can't be
// imported directly from here without an import cycle — internal/agentcli
// (CLI-backed providers: ClaudeCodeCLI) is the reference user, registering
// itself from its own init(). Call from an init() in the implementing
// package; Go guarantees that runs before any NewProvider call reaches this
// map, for any package actually linked in (a blank import is enough).
func RegisterConstructor(pt ProviderType, fn ConstructorFunc) {
	externalConstructors[pt] = fn
}

// NewProvider creates a provider by type.
func NewProvider(cfg ProviderConfig) (Provider, error) {
	switch cfg.Type {
	case ProviderOpenAI, ProviderCustom:
		return newOpenAIProvider(cfg)
	case ProviderGemini:
		return newGeminiProvider(cfg)
	case ProviderGrok:
		return newGrokProvider(cfg)
	case ProviderGroq:
		return newGroqProvider(cfg)
	case ProviderClaude:
		return newClaudeProvider(cfg)
	case ProviderOpenRouter:
		return newOpenRouterProvider(cfg)
	case ProviderOllama:
		return newOllamaProvider(cfg)
	case ProviderLlamaCPP:
		return newLlamaCPPProvider(cfg)
	case ProviderOpenCodeZen:
		return newOpenCodeZenProvider(cfg)
	case ProviderOpenCodeGo:
		return newOpenCodeGoProvider(cfg)
	default:
		if fn, ok := externalConstructors[cfg.Type]; ok {
			return fn(cfg)
		}
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.Type)
	}
}

// authHeader sets the Authorization header for the given provider.
func authHeader(pt ProviderType, apiKey string) string {
	switch pt {
	case ProviderClaude:
		return "x-api-key " + apiKey
	default:
		return "Bearer " + apiKey
	}
}
