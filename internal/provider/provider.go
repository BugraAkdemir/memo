package provider

import (
	"context"
	"encoding/json"
	"fmt"
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
}

// Message represents a single message in a conversation.
type Message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
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
// (OpenAI tool calling standard).
type ToolDefinition struct {
	Type       string       `json:"type"`
	Function   ToolFunction `json:"function"`
	Danger     string       `json:"danger,omitempty"`
}

type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
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
}

// ProviderConfig holds configuration for a single provider instance.
type ProviderConfig struct {
	Type       ProviderType `json:"type"`
	Name       string       `json:"name"`
	APIKey     string       `json:"api_key,omitempty"`
	BaseURL    string       `json:"base_url,omitempty"`
	Model      string       `json:"model"`
	Enabled    bool         `json:"enabled"`
	Priority   int          `json:"priority"`
	Temperature float64     `json:"temperature,omitempty"`
	TopP       float64      `json:"top_p,omitempty"`
	MaxTokens  int          `json:"max_tokens,omitempty"`
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
	default:
		return ""
	}
}

var DefaultModels = map[ProviderType]string{
	ProviderOpenAI:     "gpt-4o",
	ProviderGemini:     "gemini-2.0-flash",
	ProviderGrok:       "grok-2",
	ProviderGroq:       "openai/gpt-oss-20b",
	ProviderClaude:     "claude-sonnet-4-20250514",
	ProviderOpenRouter: "openai/gpt-4o",
	ProviderOllama:     "llama3",
	ProviderLlamaCPP:   "local-model",
}

func init() {
	// Validate that DefaultBaseURL returns a value for all known types
	for _, pt := range []ProviderType{ProviderOpenAI, ProviderGemini, ProviderGrok, ProviderGroq, ProviderClaude, ProviderOpenRouter, ProviderOllama, ProviderLlamaCPP} {
		if DefaultBaseURL(pt) == "" {
			panic(fmt.Sprintf("missing default base URL for %s", pt))
		}
	}
}

// NewProvider creates a provider by type.
func NewProvider(cfg ProviderConfig) (Provider, error) {
	switch cfg.Type {
	case ProviderOpenAI:
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
	default:
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
