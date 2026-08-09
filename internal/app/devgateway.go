// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"fmt"
	"strings"

	"memo/internal/config"
	"memo/internal/memory"
	"memo/internal/models"
	"memo/internal/provider"
	"memo/internal/remoteauth"
)

// gatewayToolsUnsupportedTypes lists provider types whose internal/provider
// implementation doesn't decode/encode Tools/ToolCalls at all yet (a
// pre-existing gap, not introduced by the dev gateway) — every other type
// shares internal/provider's openAIProvider, which already has full
// ChatCompletion (non-streaming) tool support, the same path Memo's own
// agent pipeline uses. Checked before routing a tools-bearing request so the
// caller gets a clear error instead of tools silently vanishing.
var gatewayToolsUnsupportedTypes = map[provider.ProviderType]bool{
	provider.ProviderGemini: true,
	provider.ProviderClaude: true,
	provider.ProviderOllama: true,
}

// GetDevGatewayConfig returns the dev gateway's settings.
func (a *App) GetDevGatewayConfig() (requireAPIKey, useMemory bool) {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.DevGateway.RequireAPIKey, a.cfg.DevGateway.UseMemory
}

// SetDevGatewayConfig updates and persists the dev gateway's settings.
func (a *App) SetDevGatewayConfig(requireAPIKey, useMemory bool) error {
	a.cfgMu.Lock()
	a.cfg.DevGateway.RequireAPIKey = requireAPIKey
	a.cfg.DevGateway.UseMemory = useMemory
	cfg := a.cfg
	a.cfgMu.Unlock()
	return config.Save(cfg)
}

// GetDevGatewayToken returns the dev gateway's own API key (see
// config.DevGatewayConfig.Token), generating and persisting one first if
// none exists yet. Unlike remote access itself, the dev gateway doesn't
// require LAN/ngrok/Tailscale mode to be turned on to have a key — "require
// API key" is an independent, local-or-remote check.
func (a *App) GetDevGatewayToken() string {
	a.cfgMu.Lock()
	if a.cfg.DevGateway.Token == "" {
		a.cfg.DevGateway.Token = remoteauth.GenerateDeviceToken()
	}
	token := a.cfg.DevGateway.Token
	cfg := a.cfg
	a.cfgMu.Unlock()
	config.Save(cfg)
	return token
}

// ListGatewayModels enumerates every model currently reachable through the
// dev gateway: the local model (if a llama.cpp server is running) plus every
// enabled external provider, each labeled "type/model-id".
func (a *App) ListGatewayModels() []models.GatewayModel {
	var out []models.GatewayModel

	if a.llamaServer != nil && a.llamaServer.IsRunning() {
		status := a.llamaServer.GetStatus()
		modelName := status.ModelName
		if modelName == "" {
			modelName = provider.DefaultModels[provider.ProviderLlamaCPP]
		}
		out = append(out, models.GatewayModel{ID: "local/" + modelName, Type: "local"})
	}

	a.providerMu.RLock()
	cfgMgr := a.providerCfgMgr
	a.providerMu.RUnlock()
	if cfgMgr != nil {
		for _, p := range cfgMgr.GetEnabled() {
			out = append(out, models.GatewayModel{ID: string(p.Type) + "/" + p.Model, Type: string(p.Type)})
		}
	}
	return out
}

// resolveGatewayProvider returns a ready-to-call provider.Provider for typ,
// plus the resolved "type/model-id" label. "local" builds the same
// llama.cpp provider.Provider (llamaCPPProvider, sharing openAIProvider's
// full ChatCompletion/Tools support) that resolveAgentProvider itself uses
// for Memo's own agent mode — not the lower-level internal/api.Client, which
// has no Tools support at all. Anything else is matched against the Type of
// an *enabled* configured provider — if more than one provider shares that
// type, the first enabled match wins (no per-request disambiguation; see
// AGENTS.md).
func (a *App) resolveGatewayProvider(typ, modelID string) (provider.Provider, provider.ProviderType, string, error) {
	if typ == "local" {
		if a.llamaServer == nil || !a.llamaServer.IsRunning() {
			return nil, "", "", fmt.Errorf("no local model loaded")
		}
		cfg := provider.ProviderConfig{
			Type:    provider.ProviderLlamaCPP,
			Name:    "Local (llama.cpp)",
			BaseURL: a.llamaServer.GetBaseURL(),
			Model:   modelID,
		}
		prov, err := provider.NewProvider(cfg)
		if err != nil {
			return nil, "", "", err
		}
		return prov, provider.ProviderLlamaCPP, "local/" + modelID, nil
	}

	a.providerMu.RLock()
	cfgMgr := a.providerCfgMgr
	a.providerMu.RUnlock()
	if cfgMgr == nil {
		return nil, "", "", fmt.Errorf("no provider configured")
	}
	var matched *provider.ProviderConfig
	for _, p := range cfgMgr.GetEnabled() {
		if strings.EqualFold(string(p.Type), typ) {
			pc := p
			matched = &pc
			break
		}
	}
	if matched == nil {
		return nil, "", "", fmt.Errorf("no enabled provider configured for type %q", typ)
	}
	matched.Model = modelID

	prov, err := provider.NewProvider(*matched)
	if err != nil {
		return nil, "", "", err
	}
	return prov, matched.Type, typ + "/" + modelID, nil
}

// DevGatewayChatStream is the /v1/messages HTTP handler's entry point for
// requests with no tools — a plain streaming (or streamed-then-buffered)
// text exchange. modelSpec is the caller-supplied "type/model-id" (e.g.
// "local/qwen2.5", "openai/gpt-4o"); see resolveGatewayProvider for how it
// resolves.
//
// When the dev gateway's "use memory" setting is on, req.Messages gets a
// RAG memory block prepended to its system message before the call — this
// mutates req.Messages, it does not return a copy.
func (a *App) DevGatewayChatStream(ctx context.Context, modelSpec string, req provider.ChatRequest) (<-chan provider.StreamChunk, string, error) {
	typ, modelID, err := splitGatewayModelSpec(modelSpec)
	if err != nil {
		return nil, "", err
	}

	if a.devGatewayMemoryEnabled() && a.GetMemoryEnabled() {
		req.Messages = a.injectGatewayMemory(ctx, req.Messages)
	}

	prov, _, resolvedModel, err := a.resolveGatewayProvider(typ, modelID)
	if err != nil {
		return nil, "", err
	}
	req.Model = modelID

	ch, err := prov.ChatCompletionStream(ctx, req)
	if err != nil {
		return nil, "", err
	}
	return ch, resolvedModel, nil
}

// DevGatewayChat is the /v1/messages HTTP handler's entry point for
// tools-bearing requests — always a non-streaming round trip, since Memo's
// own agent pipeline (internal/agent/pipeline.go) already only ever gets
// tool calls this way; no provider's streaming path decodes tool_calls
// deltas. The caller (handleAnthropicMessages) replays the buffered
// response as an SSE sequence itself when the client asked for streaming —
// see anthropicapi.StreamSSEFromResponse.
//
// Returns a clear error, rather than silently dropping the tools, when
// modelSpec resolves to a provider type that doesn't support Tools/ToolCalls
// at all yet (gemini/claude/ollama — see gatewayToolsUnsupportedTypes).
func (a *App) DevGatewayChat(ctx context.Context, modelSpec string, req provider.ChatRequest) (*provider.ChatResponse, string, error) {
	typ, modelID, err := splitGatewayModelSpec(modelSpec)
	if err != nil {
		return nil, "", err
	}

	if a.devGatewayMemoryEnabled() && a.GetMemoryEnabled() {
		req.Messages = a.injectGatewayMemory(ctx, req.Messages)
	}

	prov, resolvedType, resolvedModel, err := a.resolveGatewayProvider(typ, modelID)
	if err != nil {
		return nil, "", err
	}
	if len(req.Tools) > 0 && gatewayToolsUnsupportedTypes[resolvedType] {
		return nil, "", fmt.Errorf("tool calling isn't supported yet for provider type %q through the dev gateway", typ)
	}
	req.Model = modelID

	resp, err := prov.ChatCompletion(ctx, req)
	if err != nil {
		return nil, "", err
	}
	return resp, resolvedModel, nil
}

// MaybeSaveGatewayMemory saves a completed dev-gateway turn to RAG memory —
// but only when the gateway's "use memory" setting is on. It never creates a
// visible chat session (saveMemoryAsync doesn't touch sessions), matching
// the explicit design decision that gateway traffic stays out of chat
// history even when memory is enabled for it.
func (a *App) MaybeSaveGatewayMemory(userMsg, reply string) {
	if !a.devGatewayMemoryEnabled() {
		return
	}
	a.saveMemoryAsync(userMsg, reply)
}

func (a *App) devGatewayMemoryEnabled() bool {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.DevGateway.UseMemory
}

// splitGatewayModelSpec parses a "type/model-id" gateway model string.
func splitGatewayModelSpec(modelSpec string) (typ, modelID string, err error) {
	parts := strings.SplitN(modelSpec, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf(`model must be "type/model-id" (e.g. "local/qwen2.5", "openai/gpt-4o"), got %q`, modelSpec)
	}
	return parts[0], parts[1], nil
}

// injectGatewayMemory prepends a RAG memory block (built from the last user
// message) to req's leading system message — or adds one if there wasn't
// one — mirroring how regular chat folds retrieveMemory's results into the
// system prompt (internal/app/chat.go), but WITHOUT the rest of
// BuildSystemPrompt's persona/capability announcements: an external tool
// like Claude Code supplies its own system prompt and shouldn't have Memo's
// identity/capabilities injected into it, only the user's own remembered
// facts.
func (a *App) injectGatewayMemory(ctx context.Context, messages []provider.Message) []provider.Message {
	var lastUserText string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		if s, ok := messages[i].Content.(string); ok {
			lastUserText = s
		}
		break
	}
	if lastUserText == "" {
		return messages
	}

	results := a.retrieveMemory(ctx, lastUserText)
	block := strings.TrimSpace(memory.FormatMemoriesForPrompt(results))
	return mergeMemoryBlock(messages, block)
}

// mergeMemoryBlock prepends block to messages' leading system message (or
// adds one if there wasn't one). Pure and side-effect-free so it's directly
// unit-testable without a real memory store. A blank block is a no-op.
func mergeMemoryBlock(messages []provider.Message, block string) []provider.Message {
	if block == "" {
		return messages
	}

	if len(messages) > 0 && messages[0].Role == "system" {
		if s, ok := messages[0].Content.(string); ok {
			out := make([]provider.Message, len(messages))
			copy(out, messages)
			out[0] = provider.Message{Role: "system", Content: strings.TrimSpace(s + "\n" + block)}
			return out
		}
	}

	out := make([]provider.Message, 0, len(messages)+1)
	out = append(out, provider.Message{Role: "system", Content: block})
	out = append(out, messages...)
	return out
}
