// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"io"
	"net/http"
	"time"

	"memo/internal/openaiapi"
	"memo/internal/provider"
)

// handleOpenAIModels implements GET /v1/models — the OpenAI-compatible
// model-listing endpoint. Many OpenAI-compatible clients (IDE plugins,
// local-AI frontends) call this on connect to populate a model picker
// before ever sending a chat completion; without it they show an empty
// list and never let the user pick a "type/model-id" to send.
func (s *Server) handleOpenAIModels(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	requireAPIKey, _, _ := s.fullBridge.GetDevGatewayConfig()
	if !devGatewayAuthOK(r, requireAPIKey, s.fullBridge.GetDevGatewayToken()) {
		openaiapi.WriteError(w, http.StatusUnauthorized, "missing or invalid API key")
		return
	}
	models := s.fullBridge.ListGatewayModels()
	ids := make([]string, len(models))
	for i, m := range models {
		ids[i] = m.ID
	}
	openaiapi.WriteModelList(w, ids)
}

// handleOpenAIChatCompletions implements POST /v1/chat/completions — the
// OpenAI-compatible sibling of handleAnthropicMessages (POST /v1/messages).
// Shares the exact same gateway plumbing (auth, model routing, memory
// injection, logging) via internal/app's DevGatewayChat/DevGatewayChatStream
// — only the wire format (openaiapi vs anthropicapi) differs.
func (s *Server) handleOpenAIChatCompletions(w http.ResponseWriter, r *http.Request) {
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
		openaiapi.WriteError(w, http.StatusUnauthorized, "missing or invalid API key")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		openaiapi.WriteError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	oaReq, err := openaiapi.ParseRequest(body)
	if err != nil {
		openaiapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if oaReq.Model == "" {
		openaiapi.WriteError(w, http.StatusBadRequest, `"model" is required (e.g. "local/qwen2.5", "openai/gpt-4o")`)
		return
	}

	chatReq := openaiapi.ToChatRequest(oaReq)
	lastUserText := lastUserMessageText(chatReq.Messages)
	ctx := r.Context()

	start := time.Now()
	resolvedModel := oaReq.Model
	var replyText, errMsg string
	defer func() {
		s.fullBridge.RecordGatewayLog(resolvedModel, oaReq.Stream, len(chatReq.Tools) > 0, lastUserText, replyText, errMsg, time.Since(start))
	}()

	// Same constraint as handleAnthropicMessages: a tools-bearing request
	// always goes through the non-streaming DevGatewayChat (no provider's
	// streaming path decodes tool_calls deltas), replayed as SSE afterward
	// if the client asked for streaming.
	if len(chatReq.Tools) > 0 {
		resp, rm, err := s.fullBridge.DevGatewayChat(ctx, oaReq.Model, chatReq)
		if err != nil {
			errMsg = err.Error()
			openaiapi.WriteError(w, http.StatusBadGateway, errMsg)
			return
		}
		resolvedModel = rm
		if resp.Usage == nil {
			resp.Usage = &provider.Usage{
				PromptTokens:     openaiapi.EstimateTokens(chatReq.Messages),
				CompletionTokens: openaiapi.EstimateTokens([]provider.Message{{Role: "assistant", Content: resp.Content}}),
			}
		}
		if oaReq.Stream {
			flusher, ok := prepareSSE(w)
			if !ok {
				errMsg = "streaming unsupported"
				openaiapi.WriteError(w, http.StatusInternalServerError, errMsg)
				return
			}
			replyText = openaiapi.StreamSSEFromResponse(w, flusher, resolvedModel, *resp, "")
			s.fullBridge.MaybeSaveGatewayMemory(lastUserText, replyText)
			return
		}
		replyText = resp.Content
		if err := openaiapi.WriteNonStream(w, resolvedModel, *resp, ""); err == nil {
			s.fullBridge.MaybeSaveGatewayMemory(lastUserText, resp.Content)
		}
		return
	}

	ch, rm, err := s.fullBridge.DevGatewayChatStream(ctx, oaReq.Model, chatReq)
	if err != nil {
		errMsg = err.Error()
		openaiapi.WriteError(w, http.StatusBadGateway, errMsg)
		return
	}
	resolvedModel = rm

	if oaReq.Stream {
		flusher, ok := prepareSSE(w)
		if !ok {
			errMsg = "streaming unsupported"
			openaiapi.WriteError(w, http.StatusInternalServerError, errMsg)
			return
		}
		replyText = openaiapi.StreamSSE(ctx, w, flusher, resolvedModel, ch)
		s.fullBridge.MaybeSaveGatewayMemory(lastUserText, replyText)
		return
	}

	content, finishReason, collectErrMsg := openaiapi.CollectStream(ctx, ch)
	if collectErrMsg != "" {
		errMsg = collectErrMsg
		openaiapi.WriteError(w, http.StatusBadGateway, errMsg)
		return
	}
	replyText = content
	resp := provider.ChatResponse{
		Content: content,
		Model:   resolvedModel,
		Usage: &provider.Usage{
			PromptTokens:     openaiapi.EstimateTokens(chatReq.Messages),
			CompletionTokens: openaiapi.EstimateTokens([]provider.Message{{Role: "assistant", Content: content}}),
		},
	}
	if err := openaiapi.WriteNonStream(w, resolvedModel, resp, finishReason); err == nil {
		s.fullBridge.MaybeSaveGatewayMemory(lastUserText, content)
	}
}
