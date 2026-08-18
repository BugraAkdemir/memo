// SPDX-License-Identifier: AGPL-3.0-or-later

// Package openaiapi implements the server side of OpenAI's Chat Completions
// wire format (POST /v1/chat/completions) — the sibling of
// internal/anthropicapi (POST /v1/messages) for the dev gateway
// (Settings > Developer). Any tool that only knows how to speak the
// OpenAI-compatible API shape (most local-AI clients, IDE plugins, and
// scripts default to this rather than Anthropic's) can point its base URL
// at Memo instead, with Memo translating the request to whichever backend
// (local llama.cpp model or a configured external provider) the caller's
// "type/model-id" selects — same routing internal/app's dev gateway
// (DevGatewayChat/DevGatewayChatStream) already does for the Anthropic path.
//
// Unlike Anthropic's content-block wire shape, provider.Message/ToolCall/
// ToolDefinition are already OpenAI-shaped (Memo's own external-provider
// client code in internal/provider/openai.go talks this same format), so
// the request/response translation here is close to a direct field copy —
// no content-block reassembly like anthropicapi.toProviderMessages needs.
package openaiapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"memo/internal/provider"
)

// Request is an incoming POST /v1/chat/completions body.
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// Message is one turn. Content is captured raw because OpenAI accepts both
// a plain string and an array of content parts (multimodal-style
// `[{"type":"text","text":"..."}, ...]`) — resolved by extractText, which
// keeps only text parts (images aren't supported by this gateway).
type Message struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall      `json:"tool_calls,omitempty"`
}

// Tool is one entry in the request's "tools" array — already the exact
// shape provider.ToolDefinition expects.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ToolCall mirrors provider.ToolCall on the wire.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ParseRequest decodes an OpenAI Chat Completions request body.
func ParseRequest(body []byte) (Request, error) {
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		return Request{}, fmt.Errorf("openaiapi: parse request: %w", err)
	}
	return req, nil
}

// extractText pulls plain text out of a message's content field, whether
// it's a bare JSON string (the overwhelmingly common case) or an array of
// content parts. Non-text parts (image_url etc.) are silently skipped.
func extractText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var sb strings.Builder
		for _, p := range parts {
			if p.Type != "text" || p.Text == "" {
				continue
			}
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(p.Text)
		}
		return sb.String()
	}
	return ""
}

func toProviderToolCalls(calls []ToolCall) []provider.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]provider.ToolCall, len(calls))
	for i, c := range calls {
		out[i] = provider.ToolCall{
			ID:   c.ID,
			Type: c.Type,
			Function: provider.ToolCallFunction{
				Name:      c.Function.Name,
				Arguments: c.Function.Arguments,
			},
		}
	}
	return out
}

func toProviderTools(tools []Tool) []provider.ToolDefinition {
	if len(tools) == 0 {
		return nil
	}
	out := make([]provider.ToolDefinition, len(tools))
	for i, t := range tools {
		out[i] = provider.ToolDefinition{
			Type: t.Type,
			Function: provider.ToolFunction{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			},
		}
	}
	return out
}

// ToChatRequest converts a parsed OpenAI request into Memo's internal
// provider.ChatRequest. The caller fills in Model separately (after
// resolving the "type/model-id" the request's Model field named).
func ToChatRequest(req Request) provider.ChatRequest {
	messages := make([]provider.Message, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = provider.Message{
			Role:       m.Role,
			Content:    extractText(m.Content),
			ToolCallID: m.ToolCallID,
			ToolCalls:  toProviderToolCalls(m.ToolCalls),
		}
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	return provider.ChatRequest{
		Messages:    messages,
		Tools:       toProviderTools(req.Tools),
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   maxTokens,
	}
}

// newID mints an opaque, OpenAI-shaped completion ID.
func newID() string {
	return "chatcmpl-" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

// EstimateTokens gives a rough word-count-based token estimate — mirrors
// anthropicapi.EstimateTokens (this package stays independent of
// internal/app for the same import-cycle reason, see
// internal/models/devgateway.go's doc comment).
func EstimateTokens(messages []provider.Message) int {
	total := 0
	for _, m := range messages {
		if s, ok := m.Content.(string); ok {
			total += int(float64(len(strings.Fields(s))) * 1.3)
		}
	}
	return total
}

// CollectStream drains ch into a single accumulated response — for
// non-streaming (stream:false) requests. Mirrors anthropicapi.CollectStream.
func CollectStream(ctx context.Context, ch <-chan provider.StreamChunk) (content, finishReason, errMsg string) {
	var sb strings.Builder
	for {
		select {
		case chunk, ok := <-ch:
			if !ok {
				return sb.String(), finishReason, errMsg
			}
			if chunk.Error != "" {
				return sb.String(), finishReason, chunk.Error
			}
			sb.WriteString(chunk.Content)
			if chunk.Done {
				finishReason = chunk.FinishReason
				return sb.String(), finishReason, ""
			}
		case <-ctx.Done():
			return sb.String(), finishReason, ctx.Err().Error()
		}
	}
}

// WriteError writes an OpenAI-shaped error response
// ({"error":{"message":...,"type":...}}).
func WriteError(w http.ResponseWriter, status int, message string) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    "invalid_request_error",
			"code":    nil,
		},
	})
}

// finishReasonOrDefault maps a provider.StreamChunk/ChatResponse finish
// reason to OpenAI's vocabulary, defaulting to "stop" — the overwhelmingly
// common case. A response carrying tool calls always reports "tool_calls"
// regardless of the backend's own finish reason, since that's what OpenAI
// clients key off of to know a turn ended in a tool call.
func finishReasonOrDefault(finishReason string, hasToolCalls bool) string {
	if hasToolCalls {
		return "tool_calls"
	}
	switch finishReason {
	case "length":
		return "length"
	case "", "stop":
		return "stop"
	default:
		return "stop"
	}
}

func toWireToolCalls(calls []provider.ToolCall) []map[string]any {
	if len(calls) == 0 {
		return nil
	}
	out := make([]map[string]any, len(calls))
	for i, tc := range calls {
		out[i] = map[string]any{
			"index": i,
			"id":    tc.ID,
			"type":  "function",
			"function": map[string]any{
				"name": tc.Function.Name,
				// Embedded as json.RawMessage, NOT string(...) — Arguments'
				// raw bytes are already a JSON-encoded string (OpenAI's wire
				// format for function.arguments: a string whose *content* is
				// the argument object's text, e.g. the byte sequence
				// `"{\"location\":\"SF\"}"`). RawMessage.MarshalJSON passes
				// those bytes through unchanged, so they land on the wire
				// exactly once-encoded. Converting to a Go string first and
				// letting json.Marshal re-encode it would escape the quotes
				// a second time, corrupting the payload — this is exactly
				// the invariant internal/anthropicapi's own
				// anthropicInputToOpenAIArguments/openAIArgumentsToJSONText
				// helpers exist to preserve when translating between wire
				// formats; here both ends already agree, so no translation
				// is needed, just a direct embed.
				"arguments": tc.Function.Arguments,
			},
		}
	}
	return out
}

// WriteNonStream writes a complete (non-streaming) OpenAI Chat Completions
// response for a finished provider.ChatResponse, including any tool_calls
// and the corresponding finish_reason.
func WriteNonStream(w http.ResponseWriter, model string, resp provider.ChatResponse, finishReason string) error {
	promptTokens, completionTokens := 0, 0
	if resp.Usage != nil {
		promptTokens = resp.Usage.PromptTokens
		completionTokens = resp.Usage.CompletionTokens
	}
	message := map[string]any{"role": "assistant"}
	// OpenAI sets content to null (not "") when a turn is pure tool calls
	// with no accompanying text.
	if resp.Content != "" || len(resp.ToolCalls) == 0 {
		message["content"] = resp.Content
	} else {
		message["content"] = nil
	}
	if toolCalls := toWireToolCalls(resp.ToolCalls); toolCalls != nil {
		message["tool_calls"] = toolCalls
	}
	body := map[string]any{
		"id":      newID(),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]any{
			{
				"index":         0,
				"message":       message,
				"finish_reason": finishReasonOrDefault(finishReason, len(resp.ToolCalls) > 0),
			},
		},
		"usage": map[string]int{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      promptTokens + completionTokens,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(body)
}

// StreamSSE drains ch, translating Memo's internal provider.StreamChunk
// stream into OpenAI's SSE chunk sequence: a leading role-only delta, one
// delta per content chunk, a closing delta carrying finish_reason, then a
// literal "data: [DONE]" line. Unlike Anthropic's SSE (which pairs an
// `event:` line with each `data:` line), OpenAI clients dispatch purely on
// the JSON payload shape, so every line here is a bare `data: {...}`.
//
// Returns the full accumulated reply text — the caller (internal/app's dev
// gateway) needs it to optionally save the turn to RAG memory once the
// stream finishes.
func StreamSSE(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, model string, ch <-chan provider.StreamChunk) string {
	id := newID()
	created := time.Now().Unix()

	writeChunk := func(delta map[string]any, finishReason *string) {
		payload, err := json.Marshal(map[string]any{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model,
			"choices": []map[string]any{
				{"index": 0, "delta": delta, "finish_reason": finishReason},
			},
		})
		if err != nil {
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	}

	writeChunk(map[string]any{"role": "assistant", "content": ""}, nil)

	finishReason := ""
	var fullContent strings.Builder

	for {
		var chunk provider.StreamChunk
		var ok bool
		select {
		case chunk, ok = <-ch:
		default:
			select {
			case <-ctx.Done():
				chunk, ok = provider.StreamChunk{}, false
			case chunk, ok = <-ch:
			}
		}
		if !ok {
			break
		}
		if chunk.Error != "" {
			payload, _ := json.Marshal(map[string]any{"error": map[string]string{"message": chunk.Error, "type": "api_error"}})
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
			return fullContent.String()
		}
		if chunk.Content != "" {
			fullContent.WriteString(chunk.Content)
			writeChunk(map[string]any{"content": chunk.Content}, nil)
		}
		if chunk.Done {
			finishReason = chunk.FinishReason
			break
		}
	}

	fr := finishReasonOrDefault(finishReason, false)
	writeChunk(map[string]any{}, &fr)
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()

	return fullContent.String()
}

// StreamSSEFromResponse emits the same OpenAI SSE chunk sequence as
// StreamSSE, but from an already-complete provider.ChatResponse rather than
// draining a live channel — used for tool-calling requests, which (like the
// Anthropic path) are always resolved non-streaming internally and replayed
// as SSE only if the caller asked for streaming. Each tool call is emitted
// as one whole delta rather than incrementally, same simplification
// anthropicapi.StreamSSEFromResponse makes.
//
// Returns the accumulated text (never tool call content) for the same
// memory-save use as StreamSSE.
func StreamSSEFromResponse(w http.ResponseWriter, flusher http.Flusher, model string, resp provider.ChatResponse, finishReason string) string {
	id := newID()
	created := time.Now().Unix()

	writeChunk := func(delta map[string]any, fr *string) {
		payload, err := json.Marshal(map[string]any{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model,
			"choices": []map[string]any{
				{"index": 0, "delta": delta, "finish_reason": fr},
			},
		})
		if err != nil {
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	}

	writeChunk(map[string]any{"role": "assistant", "content": ""}, nil)

	if resp.Content != "" {
		writeChunk(map[string]any{"content": resp.Content}, nil)
	}
	for i, tc := range resp.ToolCalls {
		writeChunk(map[string]any{
			"tool_calls": []map[string]any{
				{
					"index": i,
					"id":    tc.ID,
					"type":  "function",
					"function": map[string]any{
						"name": tc.Function.Name,
						// See toWireToolCalls' comment — embedded as
						// json.RawMessage, not stringified, to avoid
						// double-encoding the already JSON-string-shaped
						// wire value.
						"arguments": tc.Function.Arguments,
					},
				},
			},
		}, nil)
	}

	fr := finishReasonOrDefault(finishReason, len(resp.ToolCalls) > 0)
	writeChunk(map[string]any{}, &fr)
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()

	return resp.Content
}

// modelListEntry is one item in GET /v1/models' "data" array.
type modelListEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// WriteModelList writes the GET /v1/models response — the OpenAI-compatible
// model-listing endpoint many clients probe on connect to populate a model
// picker, before ever calling /v1/chat/completions.
func WriteModelList(w http.ResponseWriter, modelIDs []string) error {
	now := time.Now().Unix()
	data := make([]modelListEntry, len(modelIDs))
	for i, id := range modelIDs {
		data[i] = modelListEntry{ID: id, Object: "model", Created: now, OwnedBy: "memo"}
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data})
}
