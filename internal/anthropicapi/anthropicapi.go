// SPDX-License-Identifier: AGPL-3.0-or-later

// Package anthropicapi implements the server side of Anthropic's Messages
// API wire format (https://docs.anthropic.com/en/api/messages) — the mirror
// image of internal/provider/claude.go, which implements the *client* side
// (Memo calling out to the real Anthropic API). This lets any tool that only
// knows how to speak to Anthropic (most notably Claude Code itself, via
// ANTHROPIC_BASE_URL) point at Memo instead, with Memo translating the
// request to whichever backend (local llama.cpp model or a configured
// external provider) the caller's "type/model-id" selects.
//
// Known v1 limitation: only text content blocks are read/written. Anthropic
// tool_use/tool_result blocks (and the "tools" field on a request) are not
// translated — a request that relies on Claude Code's tool-calling will
// still get a plain text reply, just without the backend ever seeing the
// tool definitions. Full bidirectional tool-schema translation (Anthropic's
// tool format <-> OpenAI's function-calling format used by most other
// providers) is a substantially bigger, separate piece of work.
package anthropicapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"memo/internal/provider"
)

// Request is an incoming POST /v1/messages body.
type Request struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	Messages    []Message       `json:"messages"`
	System      json.RawMessage `json:"system,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

// Message is one turn. Content is either a plain string or an array of
// content blocks (Anthropic accepts both) — captured raw and resolved by
// extractText.
type Message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// contentBlock is the subset of Anthropic's content-block schema this
// package understands (text only — see package doc).
type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ParseRequest decodes an Anthropic Messages API request body.
func ParseRequest(body []byte) (Request, error) {
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		return Request{}, fmt.Errorf("anthropicapi: parse request: %w", err)
	}
	return req, nil
}

// extractText pulls plain text out of an Anthropic content field, whether
// it's a bare JSON string or an array of content blocks. Non-text blocks
// (image, tool_use, tool_result) are silently skipped — see package doc.
func extractText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var sb strings.Builder
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(b.Text)
			}
		}
		return sb.String()
	}
	return ""
}

// ToChatRequest converts a parsed Anthropic request into Memo's internal
// provider.ChatRequest. The caller fills in Model separately (after
// resolving the "type/model-id" the request's Model field named) and Stream
// to match its own transport decision.
func ToChatRequest(req Request) provider.ChatRequest {
	var messages []provider.Message
	if systemText := extractText(req.System); systemText != "" {
		messages = append(messages, provider.Message{Role: "system", Content: systemText})
	}
	for _, m := range req.Messages {
		messages = append(messages, provider.Message{Role: m.Role, Content: extractText(m.Content)})
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	return provider.ChatRequest{
		Messages:    messages,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   maxTokens,
	}
}

// newMessageID mints an opaque, Anthropic-shaped message ID. Claude Code
// doesn't validate its format, just that it's present and unique per turn.
func newMessageID() string {
	return "msg_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

// EstimateTokens gives a rough word-count-based token estimate for a
// request's messages — used for the message_start event's initial
// input_tokens figure in streaming responses, where the real count isn't
// known until the backend replies. Mirrors the same word-count heuristic
// internal/app/llm.go's estimateContentTokens already uses for its own live
// counter; this package stays independent of internal/app (import-cycle
// reasons — see internal/models/devgateway.go's doc comment) so it can't
// just call that function directly.
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
// non-streaming (stream:false) requests, where the caller wants one
// complete answer instead of an SSE sequence.
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

// WriteError writes an Anthropic-shaped error response
// ({"type":"error","error":{...}}) — used for request-validation and
// backend-routing failures that happen before any streaming has started.
func WriteError(w http.ResponseWriter, status int, message string) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(map[string]any{
		"type": "error",
		"error": map[string]string{
			"type":    "invalid_request_error",
			"message": message,
		},
	})
}

// stopReason maps a provider.StreamChunk/ChatResponse finish reason to
// Anthropic's stop_reason vocabulary. Defaults to "end_turn" — the
// overwhelmingly common case and a safe fallback for reasons this package
// doesn't recognize.
func stopReason(finishReason string) string {
	switch finishReason {
	case "length":
		return "max_tokens"
	case "stop", "":
		return "end_turn"
	default:
		return "end_turn"
	}
}

// WriteNonStream writes a complete (non-streaming) Anthropic Messages API
// response for a finished provider.ChatResponse.
func WriteNonStream(w http.ResponseWriter, model string, resp provider.ChatResponse, finishReason string) error {
	inputTokens, outputTokens := 0, 0
	if resp.Usage != nil {
		inputTokens = resp.Usage.PromptTokens
		outputTokens = resp.Usage.CompletionTokens
	}
	body := map[string]any{
		"id":            newMessageID(),
		"type":          "message",
		"role":          "assistant",
		"content":       []map[string]any{{"type": "text", "text": resp.Content}},
		"model":         model,
		"stop_reason":   stopReason(finishReason),
		"stop_sequence": nil,
		"usage": map[string]int{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(body)
}

// StreamSSE drains ch, translating Memo's internal provider.StreamChunk
// stream into Anthropic's SSE event sequence: message_start ->
// content_block_start -> repeated content_block_delta -> content_block_stop
// -> message_delta -> message_stop. Unlike OpenAI-style SSE (bare
// `data: {...}` lines), each Anthropic event carries both an `event: <type>`
// line and a `data: {...}` line — Claude Code's SDK dispatches on the event
// name, not just the payload shape.
//
// Returns the full accumulated reply text — the caller (internal/app's dev
// gateway) needs it to optionally save the turn to RAG memory after the
// stream finishes, without re-deriving it from the wire.
func StreamSSE(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, model string, promptTokens int, ch <-chan provider.StreamChunk) string {
	writeEvent := func(eventType string, data map[string]any) {
		payload, err := json.Marshal(data)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, payload)
		flusher.Flush()
	}

	msgID := newMessageID()
	writeEvent("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            msgID,
			"type":          "message",
			"role":          "assistant",
			"content":       []any{},
			"model":         model,
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         map[string]int{"input_tokens": promptTokens, "output_tokens": 0},
		},
	})
	writeEvent("content_block_start", map[string]any{
		"type":          "content_block_start",
		"index":         0,
		"content_block": map[string]any{"type": "text", "text": ""},
	})

	outputTokens := 0
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
			writeEvent("error", map[string]any{
				"type":  "error",
				"error": map[string]string{"type": "api_error", "message": chunk.Error},
			})
			return fullContent.String()
		}
		if chunk.Content != "" {
			outputTokens += len(strings.Fields(chunk.Content))
			fullContent.WriteString(chunk.Content)
			writeEvent("content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]string{"type": "text_delta", "text": chunk.Content},
			})
		}
		if chunk.Done {
			finishReason = chunk.FinishReason
			break
		}
	}

	writeEvent("content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": 0,
	})
	writeEvent("message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": stopReason(finishReason), "stop_sequence": nil},
		"usage": map[string]int{"output_tokens": outputTokens},
	})
	writeEvent("message_stop", map[string]any{
		"type": "message_stop",
	})

	return fullContent.String()
}
