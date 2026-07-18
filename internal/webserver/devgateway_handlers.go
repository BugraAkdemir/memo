// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"

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
		requireAPIKey, useMemory := s.fullBridge.GetDevGatewayConfig()
		writeJSON(w, map[string]any{
			"require_api_key": requireAPIKey,
			"use_memory":      useMemory,
			"token":           s.fullBridge.GetDevGatewayToken(),
		})
	case http.MethodPut:
		var body struct {
			RequireAPIKey bool `json:"require_api_key"`
			UseMemory     bool `json:"use_memory"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetDevGatewayConfig(body.RequireAPIKey, body.UseMemory); err != nil {
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

	requireAPIKey, _ := s.fullBridge.GetDevGatewayConfig()
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
	ch, resolvedModel, err := s.fullBridge.DevGatewayChatStream(ctx, anthReq.Model, chatReq)
	if err != nil {
		anthropicapi.WriteError(w, http.StatusBadGateway, err.Error())
		return
	}

	if anthReq.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			anthropicapi.WriteError(w, http.StatusInternalServerError, "streaming unsupported")
			return
		}
		promptTokens := anthropicapi.EstimateTokens(chatReq.Messages)
		reply := anthropicapi.StreamSSE(ctx, w, flusher, resolvedModel, promptTokens, ch)
		s.fullBridge.MaybeSaveGatewayMemory(lastUserText, reply)
		return
	}

	content, finishReason, errMsg := anthropicapi.CollectStream(ctx, ch)
	if errMsg != "" {
		anthropicapi.WriteError(w, http.StatusBadGateway, errMsg)
		return
	}
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
