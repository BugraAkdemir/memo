// SPDX-License-Identifier: AGPL-3.0-or-later

package geminisub

import (
	"encoding/json"
	"strings"

	"memo/internal/provider"
)

// This file is a deliberate copy — adapted, allowed to diverge — of the
// Gemini wire translation in internal/provider/gemini.go. The Code Assist
// endpoint speaks the same GenerateContent payload as the public Gemini API,
// just wrapped in a {model, project, request:{...}} envelope on the way in
// and a {response:{...}} envelope on the way out, and authenticated with a
// Bearer token instead of an x-goog-api-key header. Keeping this as a copy
// (rather than extracting a shared helper the hot-path gemini provider would
// also depend on) is the isolation the feature is built for: see the package
// doc in geminisub.go.

// ── GenerateContent payload types (mirrors gemini.go) ────────────────────────

type genContentRequest struct {
	Contents          []geminiContent  `json:"contents"`
	GenerationConfig  *geminiGenConfig `json:"generationConfig,omitempty"`
	SystemInstruction *geminiContent   `json:"systemInstruction,omitempty"`
	Tools             []geminiToolDecl `json:"tools,omitempty"`
}

type geminiToolDecl struct {
	FunctionDeclarations []geminiFunctionDecl `json:"functionDeclarations"`
}

type geminiFunctionDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	ID   string          `json:"id,omitempty"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

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

// ── Code Assist envelopes ───────────────────────────────────────────────────

// caGenerateRequest is what :generateContent / :streamGenerateContent expect:
// the normal GenerateContentRequest nested under "request", alongside the
// resolved model id and GCP project from the bootstrap handshake.
type caGenerateRequest struct {
	Model   string           `json:"model"`
	Project string           `json:"project,omitempty"`
	Request genContentRequest `json:"request"`
}

// caGenerateResponse wraps a GenerateContentResponse (both the whole-body
// non-streaming form and each SSE data: line).
type caGenerateResponse struct {
	Response geminiResponse `json:"response"`
}

// ── Request building (mirrors buildGeminiRequest) ────────────────────────────

func buildGenContentRequest(req provider.ChatRequest) genContentRequest {
	out := genContentRequest{
		GenerationConfig: &geminiGenConfig{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxOutputTokens: req.MaxTokens,
		},
	}
	if budget, ok := provider.GeminiThinkingBudgetForLevel(req.EffortLevel); ok {
		out.GenerationConfig.ThinkingConfig = &geminiThinkingConfig{ThinkingBudget: budget}
	}

	var systemText string

	// pipeline.go appends one Message{Role:"tool"} per tool call, so a turn
	// with N parallel calls arrives as N consecutive "tool" messages. Gemini
	// batches the functionResponse parts for one "model" turn into a single
	// following "user" content — buffer them, flush on the next non-tool
	// message or at the end.
	var pending []geminiPart
	flush := func() {
		if len(pending) == 0 {
			return
		}
		out.Contents = append(out.Contents, geminiContent{Role: "user", Parts: pending})
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
		gc := geminiContent{Role: role}
		switch v := m.Content.(type) {
		case string:
			if v != "" {
				gc.Parts = append(gc.Parts, geminiPart{Text: v})
			}
		case []provider.ContentPart:
			for _, part := range v {
				if part.Text != "" {
					gc.Parts = append(gc.Parts, geminiPart{Text: part.Text})
				}
			}
		}
		for _, tc := range m.ToolCalls {
			gc.Parts = append(gc.Parts, geminiPart{
				FunctionCall: &geminiFunctionCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
					Args: tc.Function.Arguments,
				},
			})
		}
		if len(gc.Parts) == 0 {
			continue
		}
		out.Contents = append(out.Contents, gc)
	}
	flush()

	if systemText != "" {
		out.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: strings.TrimSpace(systemText)}},
		}
	}

	out.Tools = toGeminiTools(req.Tools)
	return out
}

func toolNameForCallID(msgs []provider.Message, callID string) string {
	for _, m := range msgs {
		for _, tc := range m.ToolCalls {
			if tc.ID == callID {
				return tc.Function.Name
			}
		}
	}
	return ""
}

func toGeminiTools(defs []provider.ToolDefinition) []geminiToolDecl {
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

// ── Response parsing ────────────────────────────────────────────────────────

// parseGenerateResponse unwraps the Code Assist {response:{...}} envelope and
// flattens the first candidate into a provider.ChatResponse.
func parseGenerateResponse(body []byte, model string) (*provider.ChatResponse, error) {
	var env caGenerateResponse
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	r := env.Response
	if len(r.Candidates) == 0 {
		return &provider.ChatResponse{Model: model}, nil
	}

	var content strings.Builder
	var toolCalls []provider.ToolCall
	for _, part := range r.Candidates[0].Content.Parts {
		if part.Text != "" {
			content.WriteString(part.Text)
		}
		if part.FunctionCall != nil {
			toolCalls = append(toolCalls, provider.ToolCall{
				ID:   part.FunctionCall.ID,
				Type: "function",
				Function: provider.ToolCallFunction{
					Name:      part.FunctionCall.Name,
					Arguments: part.FunctionCall.Args,
				},
			})
		}
	}

	var usage *provider.Usage
	if r.Usage != nil {
		usage = &provider.Usage{
			PromptTokens:     r.Usage.PromptTokenCount,
			CompletionTokens: r.Usage.CandidatesTokenCount,
			TotalTokens:      r.Usage.TotalTokenCount,
		}
	}

	return &provider.ChatResponse{
		Content:   content.String(),
		ToolCalls: toolCalls,
		Model:     model,
		Usage:     usage,
	}, nil
}

// sseEvent is what one Code Assist SSE data: line decodes to for the caller.
type sseEvent struct {
	Text         string
	FinishReason string
}

// parseSSELine unwraps one `data: {...}` line (already stripped of the
// "data: " prefix). ok is false for lines that carry no usable event
// (unparseable, or no candidate) — the caller skips them.
//
// Streaming tool calls are intentionally not surfaced: provider.StreamChunk
// has no field to carry a ToolCall, and every tool-bearing turn in Memo goes
// through the non-streaming ChatCompletion path (the dev gateway forces it,
// and so does the agent pipeline). functionCall parts in a stream are
// therefore dropped here exactly as internal/provider/gemini.go does.
func parseSSELine(data string) (ev sseEvent, ok bool) {
	var env caGenerateResponse
	if err := json.Unmarshal([]byte(data), &env); err != nil {
		return sseEvent{}, false
	}
	r := env.Response
	if len(r.Candidates) == 0 {
		return sseEvent{}, false
	}
	for _, part := range r.Candidates[0].Content.Parts {
		if part.Text != "" {
			ev.Text += part.Text
		}
	}
	ev.FinishReason = r.Candidates[0].FinishReason
	return ev, true
}
