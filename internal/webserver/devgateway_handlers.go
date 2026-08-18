// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"memo/internal/anthropicapi"
	"memo/internal/provider"
)

// handleDevGatewayConfig serves the Settings > Developer tab: GET returns
// the current settings + base URL + model list + token (so the UI can show
// copy-paste-ready values), PUT updates the require-API-key/use-memory
// toggles.
func (s *Server) handleDevGatewayConfig(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		requireAPIKey, useMemory, systemPrompt := s.fullBridge.GetDevGatewayConfig()
		writeJSON(w, map[string]any{
			"require_api_key": requireAPIKey,
			"use_memory":      useMemory,
			"system_prompt":   systemPrompt,
			"token":           s.fullBridge.GetDevGatewayToken(),
		})
	case http.MethodPut:
		var body struct {
			RequireAPIKey bool   `json:"require_api_key"`
			UseMemory     bool   `json:"use_memory"`
			SystemPrompt  string `json:"system_prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetDevGatewayConfig(body.RequireAPIKey, body.UseMemory, body.SystemPrompt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "GET or PUT only", http.StatusMethodNotAllowed)
	}
}

// handleDevGatewayModels lists every "type/model-id" the dev gateway can
// currently route to, for the Settings tab's copyable model list.
func (s *Server) handleDevGatewayModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.fullBridge.ListGatewayModels())
}

// handleDevGatewayLogs returns the in-memory dev-gateway request/response
// log (Developer screen's live log view), oldest first.
func (s *Server) handleDevGatewayLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.fullBridge.GetGatewayLogs())
}

// handleClaudeCodeCLIConnection serves the Developer screen's one-click
// "connect Claude Code CLI" toggle: GET reports the current state, POST
// connects (body carries the base URL the frontend already displays) or
// disconnects (writes/restores env.ANTHROPIC_BASE_URL and
// env.ANTHROPIC_API_KEY in ~/.claude/settings.json — see
// internal/app/claudecodecli.go for why this is CLI-only and never touches
// any separate Claude desktop app).
func (s *Server) handleClaudeCodeCLIConnection(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{"connected": s.fullBridge.GetClaudeCodeCLIConnected()})
	case http.MethodPost:
		var body struct {
			Connect bool   `json:"connect"`
			BaseURL string `json:"base_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		var err error
		if body.Connect {
			if body.BaseURL == "" {
				http.Error(w, `"base_url" is required to connect`, http.StatusBadRequest)
				return
			}
			err = s.fullBridge.ConnectClaudeCodeCLI(body.BaseURL)
		} else {
			err = s.fullBridge.DisconnectClaudeCodeCLI()
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"connected": s.fullBridge.GetClaudeCodeCLIConnected()})
	default:
		http.Error(w, "GET or POST only", http.StatusMethodNotAllowed)
	}
}

// devGatewayAuthOK reports whether an incoming /v1/messages request may
// proceed, independent of remoteAuthMiddleware's listenAddr-gated check —
// the dev gateway's "require API key" setting applies (or doesn't) the same
// way whether Memo is bound to localhost or 0.0.0.0, since arbitrary local
// processes pointing at this port is exactly the scenario the toggle exists
// for. Real Anthropic clients (including Claude Code) send the key as
// `x-api-key`, not `Authorization: Bearer` — checked first, with Bearer as a
// fallback for tools that only support the latter.
func devGatewayAuthOK(r *http.Request, requireAPIKey bool, wantToken string) bool {
	if !requireAPIKey {
		return true
	}
	got := r.Header.Get("x-api-key")
	if got == "" {
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			got = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if wantToken == "" || got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(wantToken), []byte(got)) == 1
}

// handleAnthropicMessages implements POST /v1/messages — the Anthropic
// Messages API, so any tool that only supports ANTHROPIC_BASE_URL (most
// notably Claude Code itself) can point at Memo and use whichever local/
// external model the request's "type/model-id" (e.g. "local/qwen2.5",
// "openai/gpt-4o") selects.
func (s *Server) handleAnthropicMessages(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	requireAPIKey, _, _ := s.fullBridge.GetDevGatewayConfig()
	if !devGatewayAuthOK(r, requireAPIKey, s.fullBridge.GetDevGatewayToken()) {
		anthropicapi.WriteError(w, http.StatusUnauthorized, "missing or invalid x-api-key")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		anthropicapi.WriteError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	anthReq, err := anthropicapi.ParseRequest(body)
	if err != nil {
		anthropicapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if anthReq.Model == "" {
		anthropicapi.WriteError(w, http.StatusBadRequest, `"model" is required (e.g. "local/qwen2.5", "openai/gpt-4o")`)
		return
	}

	chatReq := anthropicapi.ToChatRequest(anthReq)
	lastUserText := lastUserMessageText(chatReq.Messages)
	ctx := r.Context()

	// Recorded into the dev-gateway log (Developer screen) no matter which
	// branch below is taken or how it ends — resolvedModel/replyText/errMsg
	// are filled in as the request proceeds.
	start := time.Now()
	resolvedModel := anthReq.Model
	var replyText, errMsg string
	defer func() {
		s.fullBridge.RecordGatewayLog(resolvedModel, anthReq.Stream, len(chatReq.Tools) > 0, lastUserText, replyText, errMsg, time.Since(start))
	}()

	// A tools-bearing request always goes through the non-streaming
	// DevGatewayChat, regardless of anthReq.Stream — see DevGatewayChat's
	// doc comment for why (no provider's streaming path decodes tool_calls
	// deltas; Memo's own agent pipeline has the same constraint). If the
	// client asked for streaming, the complete response is replayed as an
	// SSE sequence via StreamSSEFromResponse instead of a live channel.
	if len(chatReq.Tools) > 0 {
		resp, rm, err := s.fullBridge.DevGatewayChat(ctx, anthReq.Model, chatReq)
		if err != nil {
			errMsg = err.Error()
			anthropicapi.WriteError(w, http.StatusBadGateway, errMsg)
			return
		}
		resolvedModel = rm
		if resp.Usage == nil {
			resp.Usage = &provider.Usage{
				PromptTokens:     anthropicapi.EstimateTokens(chatReq.Messages),
				CompletionTokens: anthropicapi.EstimateTokens([]provider.Message{{Role: "assistant", Content: resp.Content}}),
			}
		}
		if anthReq.Stream {
			flusher, ok := prepareSSE(w)
			if !ok {
				errMsg = "streaming unsupported"
				anthropicapi.WriteError(w, http.StatusInternalServerError, errMsg)
				return
			}
			replyText = anthropicapi.StreamSSEFromResponse(w, flusher, resolvedModel, resp.Usage.PromptTokens, *resp, "")
			s.fullBridge.MaybeSaveGatewayMemory(lastUserText, replyText)
			return
		}
		replyText = resp.Content
		if err := anthropicapi.WriteNonStream(w, resolvedModel, *resp, ""); err == nil {
			s.fullBridge.MaybeSaveGatewayMemory(lastUserText, resp.Content)
		}
		return
	}

	ch, rm, err := s.fullBridge.DevGatewayChatStream(ctx, anthReq.Model, chatReq)
	if err != nil {
		errMsg = err.Error()
		anthropicapi.WriteError(w, http.StatusBadGateway, errMsg)
		return
	}
	resolvedModel = rm

	if anthReq.Stream {
		flusher, ok := prepareSSE(w)
		if !ok {
			errMsg = "streaming unsupported"
			anthropicapi.WriteError(w, http.StatusInternalServerError, errMsg)
			return
		}
		promptTokens := anthropicapi.EstimateTokens(chatReq.Messages)
		replyText = anthropicapi.StreamSSE(ctx, w, flusher, resolvedModel, promptTokens, ch)
		s.fullBridge.MaybeSaveGatewayMemory(lastUserText, replyText)
		return
	}

	content, finishReason, collectErrMsg := anthropicapi.CollectStream(ctx, ch)
	if collectErrMsg != "" {
		errMsg = collectErrMsg
		anthropicapi.WriteError(w, http.StatusBadGateway, errMsg)
		return
	}
	replyText = content
	// Real per-request token counts aren't available here (provider.StreamChunk
	// carries none) — estimated the same word-count way the streaming path's
	// message_start/message_delta events already are, so a non-streaming
	// caller doesn't just see a hardcoded 0/0.
	resp := provider.ChatResponse{
		Content: content,
		Model:   resolvedModel,
		Usage: &provider.Usage{
			PromptTokens:     anthropicapi.EstimateTokens(chatReq.Messages),
			CompletionTokens: anthropicapi.EstimateTokens([]provider.Message{{Role: "assistant", Content: content}}),
		},
	}
	if err := anthropicapi.WriteNonStream(w, resolvedModel, resp, finishReason); err == nil {
		s.fullBridge.MaybeSaveGatewayMemory(lastUserText, content)
	}
}

// prepareSSE sets the response headers for an SSE stream and returns the
// http.Flusher needed to actually deliver each event as it's written; ok is
// false if the underlying ResponseWriter doesn't support flushing.
func prepareSSE(w http.ResponseWriter) (http.Flusher, bool) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	return flusher, ok
}

// lastUserMessageText returns the last user-role message's text content, for
// the memory-save hook's "what did the user ask" argument.
func lastUserMessageText(messages []provider.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		if s, ok := messages[i].Content.(string); ok {
			return s
		}
		return ""
	}
	return ""
}
