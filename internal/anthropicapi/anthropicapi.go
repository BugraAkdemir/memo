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
// Tool calling (tool_use/tool_result blocks, the "tools" field) is
// translated to/from Memo's internal OpenAI-shaped provider.ToolDefinition/
// ToolCall — but only for backends that already support it there: every
// provider type built on top of internal/provider's shared openAIProvider
// (openai, custom, local llama.cpp, groq, openrouter, grok, opencode-zen,
// opencode-go). gemini/claude/ollama's provider implementations don't
// implement Tools/ToolCalls at all yet (a pre-existing gap, not introduced
// here) — internal/app's dev gateway rejects a tools-bearing request routed
// to one of those with a clear error rather than silently dropping the
// tools.
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
	Tools       []Tool          `json:"tools,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

// Tool is one entry in the request's "tools" array. InputSchema is already
// JSON Schema — the same shape OpenAI's function-calling "parameters" field
// expects, so it passes through unchanged.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Message is one turn. Content is either a plain string or an array of
// content blocks (Anthropic accepts both) — captured raw and resolved by
// toProviderMessages/extractText.
type Message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// contentBlock is the subset of Anthropic's content-block schema this
// package understands: text, tool_use (an assistant message echoing back a
// tool call it made earlier in the conversation), and tool_result (the
// user-role message carrying that tool's output back to the model).
type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`

	// tool_use (assistant)
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result (user)
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"` // string or []contentBlock, text-only extracted
}

// ParseRequest decodes an Anthropic Messages API request body.
func ParseRequest(body []byte) (Request, error) {
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		return Request{}, fmt.Errorf("anthropicapi: parse request: %w", err)
	}
	return req, nil
}

// anthropicInputToOpenAIArguments converts an Anthropic tool_use block's
// `input` — a genuine JSON object on the wire — into the JSON-string-encoded
// form OpenAI's tool-calling wire format requires for a tool call's
// `function.arguments` field (e.g. `"arguments": "{\"location\":\"SF\"}"`,
// not an embedded raw object). provider.ToolCallFunction.Arguments is
// marshaled verbatim by internal/provider/openai.go's toOpenAIMessages, so
// getting this encoding right here is what makes a Claude-Code-echoed prior
// tool_use round-trip correctly to an OpenAI-compatible backend.
func anthropicInputToOpenAIArguments(input json.RawMessage) json.RawMessage {
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}
	encoded, err := json.Marshal(string(input))
	if err != nil {
		return json.RawMessage(`"{}"`)
	}
	return json.RawMessage(encoded)
}

// openAIArgumentsToJSONText is the inverse of anthropicInputToOpenAIArguments
// — it extracts the plain JSON object text out of an OpenAI-style tool
// call's function.arguments field for re-emission as Anthropic's tool_use
// input (a real object, not a string). Falls back to treating raw as
// already-plain JSON text if it isn't itself a JSON string, in case a
// non-standard backend sends the object directly.
func openAIArgumentsToJSONText(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

// extractText pulls plain text out of an Anthropic content field, whether
// it's a bare JSON string or an array of content blocks. Non-text blocks
// (image, tool_use, tool_result) are silently skipped — used for the
// system field and for tool_result's own nested content field, neither of
// which carry tool_use/tool_result blocks themselves.
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

// toProviderMessages converts Anthropic's messages array into Memo's
// internal provider.Message list. A message with only text collapses to one
// provider.Message as before; an assistant message with tool_use blocks
// becomes one provider.Message carrying both any text and .ToolCalls
// (OpenAI's shape — one assistant turn, possibly several parallel calls); a
// user message's tool_result blocks each become their own separate
// Role:"tool" provider.Message (OpenAI expects tool results as standalone
// messages, not nested inside a user turn) — any of that same user message's
// own text is emitted as one more trailing user message, since Anthropic
// allows a tool_result and plain user text to share one message.
func toProviderMessages(messages []Message) []provider.Message {
	var out []provider.Message
	for _, m := range messages {
		var asString string
		if err := json.Unmarshal(m.Content, &asString); err == nil {
			out = append(out, provider.Message{Role: m.Role, Content: asString})
			continue
		}

		var blocks []contentBlock
		if err := json.Unmarshal(m.Content, &blocks); err != nil {
			continue
		}

		var text strings.Builder
		var toolCalls []provider.ToolCall
		for _, b := range blocks {
			switch b.Type {
			case "text":
				if b.Text == "" {
					continue
				}
				if text.Len() > 0 {
					text.WriteString("\n")
				}
				text.WriteString(b.Text)
			case "tool_use":
				toolCalls = append(toolCalls, provider.ToolCall{
					ID:   b.ID,
					Type: "function",
					Function: provider.ToolCallFunction{
						Name:      b.Name,
						Arguments: anthropicInputToOpenAIArguments(b.Input),
					},
				})
			case "tool_result":
				out = append(out, provider.Message{
					Role:       "tool",
					Content:    extractText(b.Content),
					ToolCallID: b.ToolUseID,
				})
			}
		}

		if len(toolCalls) > 0 {
			out = append(out, provider.Message{Role: m.Role, Content: text.String(), ToolCalls: toolCalls})
		} else if text.Len() > 0 {
			out = append(out, provider.Message{Role: m.Role, Content: text.String()})
		}
	}
	return out
}

// toProviderTools converts Anthropic's tool schema to Memo's internal
// OpenAI-shaped provider.ToolDefinition — a near-direct field mapping,
// since Anthropic's input_schema already is the JSON Schema OpenAI's
// "parameters" field expects.
func toProviderTools(tools []Tool) []provider.ToolDefinition {
	if len(tools) == 0 {
		return nil
	}
	out := make([]provider.ToolDefinition, len(tools))
	for i, t := range tools {
		out[i] = provider.ToolDefinition{
			Type: "function",
			Function: provider.ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		}
	}
	return out
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
	messages = append(messages, toProviderMessages(req.Messages)...)

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
// doesn't recognize. Ignored in favor of "tool_use" whenever the response
// actually carries tool calls — see responseStopReason.
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

// responseStopReason is stopReason plus the one case it can't decide from
// finishReason alone: Anthropic clients (Claude Code included) key off
// stop_reason == "tool_use" to know a turn ended in a tool call rather than
// a finished answer, regardless of what finish reason the backend reported.
func responseStopReason(resp provider.ChatResponse, finishReason string) string {
	if len(resp.ToolCalls) > 0 {
		return "tool_use"
	}
	return stopReason(finishReason)
}

// toAnthropicContentBlocks builds the Anthropic content-block array for a
// completed response: a leading text block (if any — a model can narrate
// before calling a tool) followed by one tool_use block per tool call.
func toAnthropicContentBlocks(resp provider.ChatResponse) []map[string]any {
	var blocks []map[string]any
	if resp.Content != "" {
		blocks = append(blocks, map[string]any{"type": "text", "text": resp.Content})
	}
	for _, tc := range resp.ToolCalls {
		var input any
		if err := json.Unmarshal([]byte(openAIArgumentsToJSONText(tc.Function.Arguments)), &input); err != nil {
			input = map[string]any{}
		}
		blocks = append(blocks, map[string]any{
			"type":  "tool_use",
			"id":    tc.ID,
			"name":  tc.Function.Name,
			"input": input,
		})
	}
	if blocks == nil {
		// Anthropic responses always carry a content array, even if empty —
		// never omit the field entirely.
		blocks = []map[string]any{}
	}
	return blocks
}

// WriteNonStream writes a complete (non-streaming) Anthropic Messages API
// response for a finished provider.ChatResponse, including any tool_use
// blocks and the corresponding stop_reason.
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
		"content":       toAnthropicContentBlocks(resp),
		"model":         model,
		"stop_reason":   responseStopReason(resp, finishReason),
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

// StreamSSEFromResponse emits the same Anthropic SSE event sequence as
// StreamSSE, but from an already-complete provider.ChatResponse rather than
// draining a live channel — used for tool-calling requests, which Memo's own
// agent pipeline (internal/agent/pipeline.go) already only ever handles via
// non-streaming ChatCompletion (no provider's streaming path decodes
// tool_calls deltas at all, only the non-streaming one does). Each block's
// content_block_delta carries the whole block in one piece — real Anthropic
// tool_use streams break the JSON across many input_json_delta chunks, but
// clients reconstruct the argument string by concatenation either way, so
// one big chunk is equally valid, just not chunked for a "typing" feel.
//
// Returns the accumulated text (never tool call content) for the same
// memory-save use as StreamSSE.
func StreamSSEFromResponse(w http.ResponseWriter, flusher http.Flusher, model string, promptTokens int, resp provider.ChatResponse, finishReason string) string {
	writeEvent := func(eventType string, data map[string]any) {
		payload, err := json.Marshal(data)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, payload)
		flusher.Flush()
	}

	outputTokens := 0
	if resp.Usage != nil {
		outputTokens = resp.Usage.CompletionTokens
	}
	writeEvent("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            newMessageID(),
			"type":          "message",
			"role":          "assistant",
			"content":       []any{},
			"model":         model,
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         map[string]int{"input_tokens": promptTokens, "output_tokens": 0},
		},
	})

	index := 0
	if resp.Content != "" {
		writeEvent("content_block_start", map[string]any{
			"type":          "content_block_start",
			"index":         index,
			"content_block": map[string]any{"type": "text", "text": ""},
		})
		writeEvent("content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": index,
			"delta": map[string]string{"type": "text_delta", "text": resp.Content},
		})
		writeEvent("content_block_stop", map[string]any{"type": "content_block_stop", "index": index})
		index++
	}

	for _, tc := range resp.ToolCalls {
		writeEvent("content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": index,
			"content_block": map[string]any{
				"type":  "tool_use",
				"id":    tc.ID,
				"name":  tc.Function.Name,
				"input": map[string]any{},
			},
		})
		writeEvent("content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": index,
			"delta": map[string]string{"type": "input_json_delta", "partial_json": openAIArgumentsToJSONText(tc.Function.Arguments)},
		})
		writeEvent("content_block_stop", map[string]any{"type": "content_block_stop", "index": index})
		index++
	}

	writeEvent("message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": responseStopReason(resp, finishReason), "stop_sequence": nil},
		"usage": map[string]int{"output_tokens": outputTokens},
	})
	writeEvent("message_stop", map[string]any{"type": "message_stop"})

	return resp.Content
}
