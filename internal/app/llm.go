package app

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/logx"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"memo/internal/agent"
	"memo/internal/api"
	"memo/internal/orchestra"
	"memo/internal/provider"
	"memo/internal/stats"
)

// estimateContentTokens gives a rough token count for a UI display label.
// Word-based (~1.3 tokens/word) is closer than len/4 for Turkish and code.
func estimateContentTokens(s string) int {
	if s == "" {
		return 0
	}
	return int(float64(len(strings.Fields(s))) * 1.3)
}

// estimateMessagesTokens sums estimateContentTokens across a conversation —
// a rough prompt-token count for usage stats, consistent across every
// callLLMStream/callAgentStream branch regardless of how each builds its
// actual provider request.
func estimateMessagesTokens(messages []api.Message) int {
	total := 0
	for _, m := range messages {
		total += estimateContentTokens(m.GetTextContent())
	}
	return total
}

// usageMeta identifies which provider/model produced a completed turn, for
// the persisted usage-stats store (internal/stats). nil means "don't record"
// — used on degenerate error paths where no model was actually resolved.
type usageMeta struct {
	Provider     string
	Model        string
	PromptTokens int
}

// currentProviderLabel returns the active external provider's name, or
// "local" when none is set (matching resolveAgentProvider's own fallback).
func (a *App) currentProviderLabel() string {
	a.providerMu.RLock()
	name := a.activeProviderName
	a.providerMu.RUnlock()
	if name == "" {
		return "local"
	}
	return name
}

// activeProviderModel looks up the configured model for a given provider
// name, mirroring the lookup resolveAgentProvider does inline for the agent
// pipeline (internal/app/llm.go:54-66).
func (a *App) activeProviderModel(name string) string {
	a.providerMu.RLock()
	cfgMgr := a.providerCfgMgr
	a.providerMu.RUnlock()
	if cfgMgr == nil {
		return ""
	}
	for _, p := range cfgMgr.GetEnabled() {
		if p.Name == name {
			return p.Model
		}
	}
	return ""
}

// localModelName returns the currently loaded local llama.cpp model's name,
// mirroring resolveAgentProvider's own fallback (internal/app/llm.go:72-75).
func (a *App) localModelName() string {
	if a.llamaServer == nil {
		return ""
	}
	status := a.llamaServer.GetStatus()
	if status.ModelName != "" {
		return status.ModelName
	}
	return provider.DefaultModels[provider.ProviderLlamaCPP]
}

// resolveAgentProvider returns the provider router and model name the agent
// pipeline should use. The write lock is held continuously during the nil->
// router creation window to prevent a second goroutine from racing in and
// creating a second router (which would silently replace the first one).
func (a *App) resolveAgentProvider() (*provider.Router, string, error) {
	a.providerMu.Lock()
	activeName := a.activeProviderName
	providerRouter := a.providerRouter
	providerCfgMgr := a.providerCfgMgr

	if activeName != "" && providerRouter == nil && providerCfgMgr != nil {
		if configs := providerCfgMgr.GetEnabled(); len(configs) > 0 {
			a.providerRouter = provider.NewRouter(configs)
			a.providerRouter.SetActiveProvider(activeName)
			providerRouter = a.providerRouter
		}
	}
	a.providerMu.Unlock()

	if activeName != "" {
		if providerRouter == nil || !providerRouter.HasActiveProvider() {
			return nil, "", fmt.Errorf("Agent modu için bir sağlayıcı (provider) yapılandırmadınız. Ayarlar > Sağlayıcılar bölümünde bir API sağlayıcısı ekleyin veya yerel bir model başlatın.")
		}
		if providerCfgMgr == nil {
			return nil, "", fmt.Errorf("provider system not initialized")
		}
		modelName := ""
		for _, p := range providerCfgMgr.GetEnabled() {
			if p.Name == activeName {
				modelName = p.Model
				break
			}
		}
		if modelName == "" {
			for _, p := range providerCfgMgr.GetEnabled() {
				modelName = p.Model
				break
			}
		}
		return providerRouter, modelName, nil
	}

	if a.llamaServer != nil && a.llamaServer.IsRunning() {
		status := a.llamaServer.GetStatus()
		modelName := status.ModelName
		if modelName == "" {
			modelName = provider.DefaultModels[provider.ProviderLlamaCPP]
		}
		cfg := provider.ProviderConfig{
			Type:    provider.ProviderLlamaCPP,
			Name:    "Local (llama.cpp)",
			BaseURL: a.llamaServer.GetBaseURL(),
			Model:   modelName,
			Enabled: true,
		}
		return provider.NewRouter([]provider.ProviderConfig{cfg}), modelName, nil
	}

	return nil, "", fmt.Errorf("Agent modu için bir API sağlayıcısı seçin ya da yerel bir model başlatın (Modeller bölümünden).")
}

// agentRouterFromProviderName builds a single-provider router for the given
// provider name + model, used as the agent's fallback in combined Orchestra+Agent
// mode when no separate active provider is configured.
func (a *App) agentRouterFromProviderName(providerName, model string) (*provider.Router, string, error) {
	a.providerMu.RLock()
	cfgMgr := a.providerCfgMgr
	a.providerMu.RUnlock()
	if cfgMgr == nil {
		return nil, "", fmt.Errorf("no provider config manager")
	}
	enabled := cfgMgr.GetEnabled()
	// Match on Name (the new identifier) first.
	for _, p := range enabled {
		if p.Name == providerName {
			pc := p
			pc.Model = model
			return provider.NewRouter([]provider.ProviderConfig{pc}), model, nil
		}
	}
	// Fall back to Type match for backward compatibility: the Orchestra ChiefType
	// holds a provider *type* string ("claude") both in legacy orchestra.json files
	// and in the current default config, so a Name-only lookup would miss it.
	for _, p := range enabled {
		if string(p.Type) == providerName {
			pc := p
			pc.Model = model
			return provider.NewRouter([]provider.ProviderConfig{pc}), model, nil
		}
	}
	return nil, "", fmt.Errorf("orchestra chief provider %q is not enabled", providerName)
}

func (a *App) callAgentStream(ctx context.Context, messages []api.Message, userMsg, sessionID string) <-chan api.StreamChunk {
	outCh := make(chan api.StreamChunk, 128)

	go func() {
		defer close(outCh)
		defer recoverStreamPanic(ctx, outCh, "callAgentStream")

		pMsgs := make([]provider.Message, len(messages))
		for i, m := range messages {
			pMsgs[i] = provider.Message{Role: m.Role, Content: m.Content}
		}

		agentRouter, modelName, err := a.resolveAgentProvider()
		if err != nil {
			a.recordStreamError(userMsg, "⚠️ "+err.Error(), sessionID)
			trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
			return
		}

		sm := a.getSessionManager()
		projectPath := ""
		if sessionID != "" && sm != nil {
			projectPath = sm.GetProjectPath(sessionID)
		}

		a.agentExecutor.SyncRouter(agentRouter)

		usageMetaVal := usageMeta{Provider: a.currentProviderLabel(), Model: modelName, PromptTokens: estimateMessagesTokens(messages)}

		start := time.Now()
		var fullReply strings.Builder
		var agentEvents []interface{}

		streamCh, err := a.agentExecutor.RunStream(ctx, sessionID, modelName, pMsgs, func(ev agent.AgentEvent) {
			agentEvents = append(agentEvents, ev)
			chunkData, _ := json.Marshal(ev)
			trySend(ctx, outCh, api.StreamChunk{
				Content:      string(chunkData),
				FinishReason: "agent_event",
			})
		}, projectPath)

		if err != nil {
			logx.Printf("Agent error: %v", err)
			a.recordStreamError(userMsg, "⚠️ "+err.Error(), sessionID)
			trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
			return
		}

		for {
			chunk, ok, ctxDone := recvChunk(ctx, streamCh)
			if ctxDone {
				a.recordStreamError(userMsg, "⏹️ Cevap durduruldu.", sessionID)
				trySend(ctx, outCh, api.StreamChunk{Error: "⏹️ Cevap durduruldu.", Done: true})
				return
			}
			if !ok {
				break
			}
			if chunk.Error != "" {
				a.recordStreamError(userMsg, "⚠️ "+chunk.Error, sessionID)
				trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + chunk.Error, Done: true})
				return
			}

			if chunk.Content != "" {
				fullReply.WriteString(chunk.Content)
				trySend(ctx, outCh, api.StreamChunk{Content: chunk.Content})
			}

			if chunk.Done {
				a.finishStream(start, 0, chunk.FinishReason, fullReply.String(), userMsg, sessionID, &usageMetaVal, agentEvents)
				trySend(ctx, outCh, api.StreamChunk{Done: true, FinishReason: chunk.FinishReason})
				return
			}
		}

		if fullReply.Len() > 0 {
			a.finishStream(start, 0, "stop", fullReply.String(), userMsg, sessionID, &usageMetaVal, agentEvents)
			trySend(ctx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
		} else {
			a.recordStreamError(userMsg, "⚠️ Agent boş yanıt döndürdü", sessionID)
		}
	}()

	return outCh
}

// callAgentWithOrchestra runs when both agent mode and orchestra mode are enabled.
func (a *App) callAgentWithOrchestra(ctx context.Context, messages []api.Message, userMsg, sessionID string) <-chan api.StreamChunk {
	outCh := make(chan api.StreamChunk, 128)

	go func() {
		defer close(outCh)
		defer recoverStreamPanic(ctx, outCh, "callAgentWithOrchestra")

		start := time.Now()

		var userPrompt string
		var systemPrompt string
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == "user" {
				if text := messages[i].GetTextContent(); text != "" {
					userPrompt = text
					break
				}
			}
		}
		for _, msg := range messages {
			if msg.Role == "system" {
				if text, ok := msg.Content.(string); ok {
					systemPrompt = text
				}
				break
			}
		}
		if userPrompt == "" {
			trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ No user message found", Done: true})
			return
		}

		conversationCtx := buildConversationContext(messages, userPrompt)
		if systemPrompt != "" {
			conversationCtx = "Sistem talimatları: " + systemPrompt + "\n\n---\n\n" + conversationCtx
		}
		if skillPrompt := a.buildActiveSkillPrompt(); skillPrompt != "" {
			conversationCtx += "\n\n" + skillPrompt
		}

		// Only the chief talks to the user. The preamble, plan, and each
		// specialist's raw output are now process — they live in the right-side
		// activity panel, not the conversation. The saved message holds just the
		// chief's synthesis (+ any agent execution result).
		var fullBuf strings.Builder
		var fullBufMu sync.Mutex
		// outAccum tracks total model output tokens (specialists + chief) for the
		// token counter, even though specialist text isn't shown. Guarded by
		// fullBufMu because parallel tasks invoke the callback concurrently.
		var outAccum int

		// emitActivity streams a structured step to the right-side activity panel.
		// It's additive to the human text — the panel renders these as live cards.
		emitActivity := func(step map[string]interface{}) {
			payload, err := json.Marshal(step)
			if err != nil {
				return
			}
			trySend(ctx, outCh, api.StreamChunk{Content: string(payload), FinishReason: "activity"})
		}

		// Live token counter (Claude-Code style). Input is fixed for the turn;
		// output grows as the orchestra/agent produces text.
		budget := a.apiContextBudget()
		// conversationCtx already embeds systemPrompt (prepended above) — don't
		// add it again or the input count is inflated ~2x.
		inputTokens := estimateContentTokens(conversationCtx)
		emitUsage := func(outputTokens int) {
			payload, err := json.Marshal(map[string]int{
				"input":  inputTokens,
				"output": outputTokens,
				"budget": budget,
			})
			if err != nil {
				return
			}
			trySend(ctx, outCh, api.StreamChunk{Content: string(payload), FinishReason: "usage"})
		}
		emitUsage(0)

		orchestraResult, _, err := a.orchestraConductor.RunWithProgress(ctx, conversationCtx, func(up orchestra.ProgressUpdate) {
			select {
			case <-ctx.Done():
				return
			default:
			}
			switch up.Type {
			case orchestra.ProgressPlan:
				// Process only — the working animation + activity panel cover it.
			case orchestra.ProgressPlanReady:
				// Seed the activity panel with the full plan (all pending).
				// Nothing goes to the conversation — only the chief speaks.
				for i, t := range up.Tasks {
					desc := t.Context
					if desc == "" {
						desc = t.Prompt
					}
					emitActivity(map[string]interface{}{
						"id":       fmt.Sprintf("task-%d", i),
						"kind":     "task",
						"title":    string(t.Role),
						"subtitle": desc,
						"status":   "pending",
					})
				}
			case orchestra.ProgressTaskStart:
				emitActivity(map[string]interface{}{
					"id":     fmt.Sprintf("task-%d", up.Index),
					"kind":   "task",
					"title":  string(up.Role),
					"status": "running",
				})
			case orchestra.ProgressTaskChunk:
				// Specialists work silently — their tokens stream to the chief, not
				// to the user. Nothing emitted here.
			case orchestra.ProgressTaskDone:
				if up.Error != "" {
					emitActivity(map[string]interface{}{
						"id":     fmt.Sprintf("task-%d", up.Index),
						"kind":   "task",
						"title":  string(up.Role),
						"status": "error",
						"detail": up.Error,
					})
				} else {
					emitActivity(map[string]interface{}{
						"id":          fmt.Sprintf("task-%d", up.Index),
						"kind":        "task",
						"title":       string(up.Role),
						"status":      "done",
						"duration_ms": up.DurationMs,
					})
					// Count the specialist's output toward the token meter without
					// ever showing its raw text in the conversation.
					fullBufMu.Lock()
					outAccum += estimateContentTokens(up.Content)
					out := outAccum
					fullBufMu.Unlock()
					emitUsage(out)
				}
			case orchestra.ProgressSynthChunk:
				fullBufMu.Lock()
				fullBuf.WriteString(up.Content)
				fullBufMu.Unlock()
				trySend(ctx, outCh, api.StreamChunk{Content: up.Content})
			}
		})
		if err != nil {
			a.recordStreamError(userMsg, "⚠️ Orchestra hatası: "+err.Error(), sessionID)
			trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ Orchestra hatası: " + err.Error(), Done: true})
			return
		}

		fullBufMu.Lock()
		finalContent := fullBuf.String()
		fullBufMu.Unlock()
		if finalContent == "" {
			finalContent = orchestraResult
		}

		// Token meter: chief synthesis is done — fold it into the running total.
		fullBufMu.Lock()
		outAccum += estimateContentTokens(finalContent)
		fullBufMu.Unlock()
		emitUsage(outAccum)

		pMsgs := make([]provider.Message, len(messages))
		for i, m := range messages {
			pMsgs[i] = provider.Message{Role: m.Role, Content: m.Content}
		}
		pMsgs = append(pMsgs, provider.Message{Role: "assistant", Content: finalContent})

		agentRouter, modelName, err := a.resolveAgentProvider()
		if err != nil {
			// Combined mode: an Orchestra-configured user often has no separate
			// "active provider" set, so the agent half would otherwise fail here.
			// Fall back to the Orchestra chief's provider so the two systems stay
			// connected — Orchestra plans, then the agent executes with the chief.
			ocfg := a.orchestraConductor.Config()
			if r, m, ferr := a.agentRouterFromProviderName(ocfg.ChiefType, ocfg.ChiefModel); ferr == nil {
				agentRouter, modelName = r, m
			} else {
				if strings.TrimSpace(finalContent) != "" {
					a.finishStream(start, 0, "error", finalContent, userMsg, sessionID, nil)
				} else {
					a.recordStreamError(userMsg, "⚠️ "+err.Error(), sessionID)
				}
				trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
				return
			}
		}

		sm := a.getSessionManager()
		projectPath := ""
		if sessionID != "" && sm != nil {
			projectPath = sm.GetProjectPath(sessionID)
		}

		a.agentExecutor.SyncRouter(agentRouter)

		usageMetaVal := usageMeta{Provider: a.currentProviderLabel(), Model: modelName, PromptTokens: inputTokens}

		var agentBuf strings.Builder
		var agentEvents []interface{}

		streamCh, err := a.agentExecutor.RunStream(ctx, sessionID, modelName, pMsgs, func(ev agent.AgentEvent) {
			agentEvents = append(agentEvents, ev)
			chunkData, _ := json.Marshal(ev)
			trySend(ctx, outCh, api.StreamChunk{
				Content:      string(chunkData),
				FinishReason: "agent_event",
			})
		}, projectPath)

		if err != nil {
			// Persist the chief's synthesis before erroring out — otherwise the
			// answer the user already saw is lost on the frontend's refresh.
			if strings.TrimSpace(finalContent) != "" {
				a.finishStream(start, 0, "error", finalContent, userMsg, sessionID, &usageMetaVal, agentEvents)
			}
			trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ Agent hatası: " + err.Error(), Done: true})
			return
		}

		// usedTools reports whether the agent actually executed any tool. If it
		// didn't, its text output is just a redundant second answer on top of the
		// chief's synthesis — exactly the "empty talk" we want to avoid. So we
		// buffer the agent's text and only surface it when real work happened.
		usedTools := func() bool {
			for _, e := range agentEvents {
				if ev, ok := e.(agent.AgentEvent); ok {
					switch ev.Type {
					case agent.EventToolExecuting, agent.EventToolResult, agent.EventToolError:
						return true
					}
				}
			}
			return false
		}

		// finalize assembles the saved reply + closes the stream. agentText is
		// appended only when the agent did tool work.
		finalize := func(finishReason string) {
			agentText := agentBuf.String()
			finalReply := finalContent
			// Show the agent's text when it did real work (tools) OR when the
			// chief produced no answer — otherwise the user would get a blank reply.
			showAgentText := agentText != "" &&
				(usedTools() || strings.TrimSpace(finalContent) == "")
			if showAgentText {
				prefix := "\n\n"
				if strings.TrimSpace(finalContent) == "" {
					prefix = ""
				}
				trySend(ctx, outCh, api.StreamChunk{Content: prefix + agentText})
				finalReply = strings.TrimSpace(finalContent + prefix + agentText)
				emitUsage(outAccum + estimateContentTokens(agentText))
			} else {
				emitUsage(outAccum)
			}
			a.finishStream(start, 0, finishReason, finalReply, userMsg, sessionID, &usageMetaVal, agentEvents)
			trySend(ctx, outCh, api.StreamChunk{Done: true, FinishReason: finishReason})
		}

		for chunk := range streamCh {
			if chunk.Error != "" {
				// Persist the chief's synthesis (and any agent text so far) so a
				// mid-agent failure doesn't wipe the answer already on screen.
				if salvage := strings.TrimSpace(finalContent + "\n\n" + agentBuf.String()); salvage != "" {
					a.finishStream(start, 0, "error", salvage, userMsg, sessionID, &usageMetaVal, agentEvents)
				}
				trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + chunk.Error, Done: true})
				return
			}
			if chunk.Content != "" {
				// Buffer, don't stream live — the decision to show it is made at the
				// end based on whether tools ran.
				agentBuf.WriteString(chunk.Content)
			}
			if chunk.Done {
				finalize(chunk.FinishReason)
				return
			}
		}

		// Stream closed without an explicit Done chunk — finalize anyway so the
		// frontend never hangs waiting for completion.
		finalize("stop")
	}()

	return outCh
}

func (a *App) callLLMStream(ctx context.Context, messages []api.Message, userMsg, imagePath, filePath, sessionID string) <-chan api.StreamChunk {
	outCh := make(chan api.StreamChunk, 128)

	// Orchestra mode takes priority
	a.providerMu.RLock()
	orchEnabled := a.orchestraConductor != nil && a.orchestraConductor.Config().Enabled
	a.providerMu.RUnlock()
	if orchEnabled {
		a.providerMu.RLock()
		if a.activeProviderName != "" {
			logx.Printf("ORCHESTRA: overriding active provider '%s' - orchestra mode uses its own provider configuration", a.activeProviderName)
		}
		a.providerMu.RUnlock()

		go func() {
			defer close(outCh)
			defer recoverStreamPanic(ctx, outCh, "callLLMStream/orchestra")

			var userPrompt string
			var systemPrompt string
			for i := len(messages) - 1; i >= 0; i-- {
				if messages[i].Role == "user" {
					if text := messages[i].GetTextContent(); text != "" {
						userPrompt = text
						break
					}
				}
			}
			for _, msg := range messages {
				if msg.Role == "system" {
					if text, ok := msg.Content.(string); ok {
						systemPrompt = text
					}
					break
				}
			}
			if userPrompt == "" {
				trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ No user message found", Done: true})
				return
			}

			conversationCtx := buildConversationContext(messages, userPrompt)
			if systemPrompt != "" {
				conversationCtx = "Sistem talimatları: " + systemPrompt + "\n\n---\n\n" + conversationCtx
			}

			start := time.Now()
			ocfg := a.orchestraConductor.Config()
			usageMetaVal := usageMeta{Provider: "orchestra", Model: ocfg.ChiefModel, PromptTokens: estimateContentTokens(conversationCtx)}

			trySend(ctx, outCh, api.StreamChunk{Content: "🎵 **Orchestra Mode Active**\n"})
			trySend(ctx, outCh, api.StreamChunk{Content: fmt.Sprintf("🧙 Şef: %s/%s\n\n", ocfg.ChiefType, ocfg.ChiefModel)})

			var fullBuf strings.Builder
			var fullBufMu sync.Mutex

			finalResponse, _, err := a.orchestraConductor.RunWithProgress(ctx, conversationCtx, func(up orchestra.ProgressUpdate) {
				select {
				case <-ctx.Done():
					return
				default:
				}
				switch up.Type {
				case orchestra.ProgressPlan:
					trySend(ctx, outCh, api.StreamChunk{Content: "🧠 **Şef planlıyor...**\n\n"})
				case orchestra.ProgressPlanChunk:
					fullBufMu.Lock()
					fullBuf.WriteString(up.Content)
					fullBufMu.Unlock()
					trySend(ctx, outCh, api.StreamChunk{Content: up.Content})
				case orchestra.ProgressTaskStart:
					trySend(ctx, outCh, api.StreamChunk{Content: up.Content})
				case orchestra.ProgressTaskChunk:
					// Live display only — full content is buffered once on TaskDone.
					trySend(ctx, outCh, api.StreamChunk{Content: up.Content})
				case orchestra.ProgressTaskDone:
					if up.Error != "" {
						chunk := fmt.Sprintf("\n❌ **%s** | %s ⚠️ %s\n\n", up.Role, up.ModelType, up.Error)
						trySend(ctx, outCh, api.StreamChunk{Content: chunk})
					} else {
						if up.Content != "" {
							fullBufMu.Lock()
							fullBuf.WriteString(up.Content)
							fullBufMu.Unlock()
						}
						tokEst := estimateContentTokens(up.Content)
						chunk := fmt.Sprintf("\n✅ **%s** | %s (%dms, ~%d token)\n", up.Role, up.ModelType, up.DurationMs, tokEst)
						trySend(ctx, outCh, api.StreamChunk{Content: chunk})
					}
				case orchestra.ProgressSynthChunk:
					fullBufMu.Lock()
					fullBuf.WriteString(up.Content)
					fullBufMu.Unlock()
					trySend(ctx, outCh, api.StreamChunk{Content: up.Content})
				}
			})
			fullBufMu.Lock()
			fullBufStr := fullBuf.String()
			fullBufMu.Unlock()
			if err != nil {
				a.finishStream(start, 0, "error", fullBufStr, userPrompt, sessionID, &usageMetaVal)
				trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
				return
			}

			finalContent := fullBufStr
			if finalContent == "" {
				finalContent = finalResponse
			}

			tokenCount := 0
			if finalContent != "" {
				tokenCount = len(finalContent) / 4
			}

			a.finishStream(start, tokenCount, "stop", finalContent, userPrompt, sessionID, &usageMetaVal)
			trySend(ctx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
		}()
		return outCh
	}

	// Use external provider only if user explicitly selected one
	a.providerMu.RLock()
	activeName := a.activeProviderName
	providerRouter := a.providerRouter
	a.providerMu.RUnlock()
	if activeName != "" && providerRouter != nil {
		go func() {
			defer close(outCh)
			defer recoverStreamPanic(ctx, outCh, "callLLMStream/external-provider")

			providerCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
			defer cancel()

			pMsgs := make([]provider.Message, len(messages))
			for i, m := range messages {
				pMsgs[i] = provider.Message{Role: m.Role, Content: m.Content}
			}

			a.cfgMu.RLock()
			req := provider.ChatRequest{
				Messages:    pMsgs,
				Temperature: a.cfg.Llama.Temperature,
				TopP:        a.cfg.Llama.TopP,
				MaxTokens:   a.cfg.Llama.MaxTokens,
				Stream:      true,
			}
			a.cfgMu.RUnlock()

			ch, err := providerRouter.ChatCompletionStream(providerCtx, req)
			if err != nil {
				logx.Printf("Provider stream error: %v", err)
				var errMsg string
				if a.providerSwapped(providerRouter) {
					errMsg = modelSwappedMidStreamMsg
				} else {
					errMsg = "⚠️ " + err.Error()
					if hint := a.localModelHint(); hint != "" {
						errMsg += "\n\n" + hint
					}
				}
				a.recordStreamError(userMsg, errMsg, sessionID)
				trySend(providerCtx, outCh, api.StreamChunk{Error: errMsg, Done: true})
				return
			}

			start := time.Now()
			usageMetaVal := usageMeta{Provider: activeName, Model: a.activeProviderModel(activeName), PromptTokens: estimateMessagesTokens(messages)}
			var fullReply strings.Builder
			tokenCount := 0
			firstTokenLogged := false

			for {
				chunk, ok, ctxDone := recvChunk(providerCtx, ch)
				if ctxDone {
					a.recordStreamError(userMsg, "⏹️ Cevap durduruldu.", sessionID)
					trySend(providerCtx, outCh, api.StreamChunk{Error: "⏹️ Cevap durduruldu.", Done: true})
					return
				}
				if !ok {
					break
				}

				if chunk.Error != "" {
					var errMsg string
					if a.providerSwapped(providerRouter) {
						errMsg = modelSwappedMidStreamMsg
					} else {
						errMsg = "⚠️ " + chunk.Error
						if hint := a.localModelHint(); hint != "" {
							errMsg += "\n\n" + hint
						}
					}
					a.recordStreamError(userMsg, errMsg, sessionID)
					trySend(providerCtx, outCh, api.StreamChunk{Error: errMsg, Done: true})
					return
				}

				if chunk.Content != "" {
					if !firstTokenLogged {
						firstTokenLogged = true
						logx.Printf("LATENCY provider.first_token ms=%d", time.Since(start).Milliseconds())
					}
					fullReply.WriteString(chunk.Content)
					tokenCount++
					trySend(providerCtx, outCh, api.StreamChunk{Content: chunk.Content})
				}

				if chunk.Done {
					a.finishStream(start, tokenCount, chunk.FinishReason, fullReply.String(), userMsg, sessionID, &usageMetaVal)
					trySend(providerCtx, outCh, api.StreamChunk{Done: true, FinishReason: chunk.FinishReason})
					return
				}
			}

			if fullReply.Len() > 0 {
				a.finishStream(start, tokenCount, "stop", fullReply.String(), userMsg, sessionID, &usageMetaVal)
				trySend(providerCtx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
			} else {
				errMsg := "⚠️ Provider returned empty response"
				if hint := a.localModelHint(); hint != "" {
					errMsg += "\n\n" + hint
				}
				a.recordStreamError(userMsg, errMsg, sessionID)
				trySend(providerCtx, outCh, api.StreamChunk{Error: errMsg, Done: true})
			}
		}()
		return outCh
	}

	// Fallback to local model
	a.clientMu.RLock()
	streamClient := a.client
	a.clientMu.RUnlock()

	if streamClient == nil {
		// Use direct send (not trySend) so the error is guaranteed to reach
		// the wrapper goroutine even if the parent ctx is already cancelled.
		// Must close outCh here — the wrapper goroutine in sendMessageStreamInner
		// does `for chunk := range innerCh` and would leak + deadlock streamMu
		// on a never-closed channel.
		select {
		case outCh <- api.StreamChunk{Error: "⚠️ Yerel model yüklenmemiş. Lütfen bir model başlatın veya API sağlayıcı seçin.", Done: true}:
		default:
		}
		close(outCh)
		return outCh
	}

	// A real chat message is about to hit the local model, which runs with a
	// single inference slot — preempt any background call (auto fact
	// extraction) still occupying it instead of queueing behind it (BUG_REPORT
	// TD-2).
	a.preemptBackgroundLLM()

	go func() {
		defer close(outCh)
		defer recoverStreamPanic(ctx, outCh, "callLLMStream/local-model")

		streamCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
		defer cancel()

		requestStart := time.Now()
		a.cfgMu.RLock()
		temperature, topP, maxTokens := a.cfg.Llama.Temperature, a.cfg.Llama.TopP, a.cfg.Llama.MaxTokens
		a.cfgMu.RUnlock()
		ch, err := streamClient.ChatCompletionStream(streamCtx, messages, temperature, topP, maxTokens)
		if err != nil {
			logx.Printf("LATENCY llm.stream_error total_ms=%d messages=%d", time.Since(requestStart).Milliseconds(), len(messages))
			logx.Printf("LLM stream error: %v", err)
			errMsg := "⚠️ " + err.Error()
			if a.clientSwapped(streamClient) {
				errMsg = modelSwappedMidStreamMsg
			}
			a.recordStreamError(userMsg, errMsg, sessionID)
			trySend(streamCtx, outCh, api.StreamChunk{Error: errMsg, Done: true})
			return
		}
		logx.Printf("LATENCY llm.stream_ready total_ms=%d messages=%d", time.Since(requestStart).Milliseconds(), len(messages))

		start := time.Now()
		usageMetaVal := usageMeta{Provider: "local", Model: a.localModelName(), PromptTokens: estimateMessagesTokens(messages)}
		var fullReply strings.Builder
		tokenCount := 0
		firstTokenLogged := false

		for {
			chunk, ok, ctxDone := recvChunk(streamCtx, ch)
			if ctxDone {
				a.recordStreamError(userMsg, "⏹️ Cevap durduruldu.", sessionID)
				trySend(streamCtx, outCh, api.StreamChunk{Error: "⏹️ Cevap durduruldu.", Done: true})
				return
			}
			if !ok {
				break
			}

			if chunk.Error != "" {
				logx.Printf("LATENCY llm.stream_chunk_error total_ms=%d generation_ms=%d tokens=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds(), tokenCount)
				logx.Printf("Stream chunk error: %s", chunk.Error)
				errMsg := "⚠️ " + chunk.Error
				if a.clientSwapped(streamClient) {
					errMsg = modelSwappedMidStreamMsg
				}
				a.recordStreamError(userMsg, errMsg, sessionID)
				trySend(streamCtx, outCh, api.StreamChunk{Error: errMsg, Done: true})
				return
			}

			if chunk.Content != "" {
				if !firstTokenLogged {
					firstTokenLogged = true
					logx.Printf("LATENCY llm.first_token total_ms=%d after_stream_ready_ms=%d messages=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds(), len(messages))
				}
				fullReply.WriteString(chunk.Content)
				tokenCount++
				trySend(streamCtx, outCh, chunk)
			}

			if chunk.Done {
				logx.Printf("LATENCY llm.stream_done total_ms=%d generation_ms=%d tokens=%d finish=%s", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds(), tokenCount, chunk.FinishReason)
				a.finishStream(start, tokenCount, chunk.FinishReason, fullReply.String(), userMsg, sessionID, &usageMetaVal)
				trySend(streamCtx, outCh, chunk)
				return
			}
		}

		if fullReply.Len() > 0 {
			logx.Printf("LATENCY llm.stream_closed total_ms=%d generation_ms=%d tokens=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds(), tokenCount)
			a.finishStream(start, tokenCount, "stop", fullReply.String(), userMsg, sessionID, &usageMetaVal)
			trySend(streamCtx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
		} else {
			logx.Printf("LATENCY llm.stream_empty total_ms=%d generation_ms=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds())
			a.recordStreamError(userMsg, "⚠️ Model boş yanıt döndürdü", sessionID)
			trySend(streamCtx, outCh, api.StreamChunk{Error: "⚠️ Model boş yanıt döndürdü", Done: true})
		}
	}()

	return outCh
}

// trySend delivers chunk to outCh, preferring the send over ctx cancellation.
// A plain `select { case outCh <- chunk: case <-ctx.Done(): }` lets Go's
// random tie-breaking between simultaneously-ready cases silently drop the
// chunk — including the final Done:true one — if ctx happens to become Done
// at the exact moment outCh's reader is also ready to receive. The reader
// (handleSendStream) then sees the channel close via the *next* call's
// `close(outCh)` with no final chunk ever written to the SSE response, so
// the client never learns the stream finished and its "sending" UI state
// is stuck forever. outCh is always created with a generous buffer (128) in
// every caller, so the non-blocking attempt below succeeds immediately in
// the overwhelming majority of real cases, sidestepping the race entirely;
// the ctx-aware fallback only matters once that buffer is genuinely full,
// which itself means the reader already stopped consuming.
func trySend(ctx context.Context, outCh chan<- api.StreamChunk, chunk api.StreamChunk) {
	select {
	case outCh <- chunk:
		return
	default:
	}
	select {
	case outCh <- chunk:
	case <-ctx.Done():
	}
}

// recvChunk receives the next chunk from ch, preferring an already-ready
// value over ctx cancellation — the receive-side counterpart to trySend
// above (see its doc comment for the full race explanation). Used by every
// agentLoop/providerLoop/localLoop-style for-select in this file: a plain
// `select { case <-ctx.Done(): return; case chunk, ok := <-ch: ... }` can
// otherwise pick the ctx.Done() branch even when the *final* chunk (the one
// carrying Done:true) is simultaneously ready, discarding it and reporting
// "⏹️ Cevap durduruldu" for a response that had actually already finished
// successfully. Generic because this file streams two different chunk
// types depending on the path: api.StreamChunk (local model,
// streamClient.ChatCompletionStream) and provider.StreamChunk (external
// provider and agent-pipeline paths, providerRouter/agentExecutor).
//
// ok mirrors the standard "receive from a channel" convention: false means
// ch is closed with nothing left to read. ctxDone mirrors ctx.Done(): true
// means neither a value nor a close was immediately available and ctx was
// cancelled first — the only case callers should treat as a genuine
// cancellation.
func recvChunk[T any](ctx context.Context, ch <-chan T) (chunk T, ok bool, ctxDone bool) {
	select {
	case chunk, ok = <-ch:
		return chunk, ok, false
	default:
	}
	select {
	case chunk, ok = <-ch:
		return chunk, ok, false
	case <-ctx.Done():
		var zero T
		return zero, false, true
	}
}

// recoverStreamPanic must be deferred *after* `defer close(outCh)` in every
// streaming goroutine below (defers run LIFO, so this one fires first) — a
// panic anywhere in the LLM/agent/orchestra call stack must not take down
// the whole backend (every other active chat, WhatsApp bridge, calendar
// reminders) along with it, the same reasoning taskloop/engine.go's run()
// already applies to task-list goroutines. Sends a user-visible error chunk
// before outCh is closed, instead of just losing the response silently.
func recoverStreamPanic(ctx context.Context, outCh chan<- api.StreamChunk, label string) {
	if r := recover(); r != nil {
		logx.Printf("PANIC in %s: %v\n%s", label, r, string(debug.Stack()))
		trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ internal error", Done: true})
	}
}

// recordStreamError saves an error reply to the session to prevent dangling user
// messages. Called on all stream error paths where finishStream is not invoked.
func (a *App) recordStreamError(userMsg, errReply, sessionID string) {
	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if !incog {
		sm := a.getSessionManager()
		if sm != nil {
			if sessionID != "" {
				sm.AddMessageToSession(sessionID, "assistant", errReply, "", "")
			} else {
				sm.AddMessage("assistant", errReply, "", "")
			}
		}
	} else {
		a.incognitoMu.Lock()
		a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("assistant", errReply))
		a.incognitoMu.Unlock()
	}
}

// recordUsageEvent persists one completed turn to the usage-stats store.
// Fire-and-forget (called via `go`) — a slow or failing write must never
// hold up or break the chat response it's describing.
func (a *App) recordUsageEvent(meta usageMeta, completionTokens int, durationSecs, tokensPerSecond float64) {
	if a.statsStore == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.statsStore.RecordEvent(ctx, stats.Event{
		Provider:         meta.Provider,
		Model:            meta.Model,
		PromptTokens:     meta.PromptTokens,
		CompletionTokens: completionTokens,
		DurationSecs:     durationSecs,
		TokensPerSecond:  tokensPerSecond,
	}); err != nil {
		logx.Printf("WARN: record usage event: %v", err)
	}
}

func (a *App) finishStream(start time.Time, tokenCount int, finishReason, reply, userMsg, sessionID string, meta *usageMeta, agentEvents ...[]interface{}) {
	duration := time.Since(start).Seconds()
	tps := 0.0
	if duration > 0 && tokenCount > 0 {
		tps = float64(tokenCount) / duration
	}

	a.emitEvent("chat:done", api.StreamChunk{
		Done: true,
		Stats: &api.MessageStats{
			TokensPerSecond:  tps,
			CompletionTokens: tokenCount,
			TotalDuration:    duration,
			StopReason:       finishReason,
		},
	})

	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if !incog {
		sm := a.getSessionManager()
		if sm != nil {
			if sessionID != "" {
				sm.AddMessageToSession(sessionID, "assistant", reply, "", "", agentEvents...)
				if len(sm.GetActiveMessagesForSession(sessionID)) == 2 {
					go a.generateChatTitleForSession(sessionID)
				}
			} else {
				sm.AddMessage("assistant", reply, "", "", agentEvents...)
				if len(sm.GetActiveMessages()) == 2 {
					go a.GenerateChatTitle()
				}
			}
		}
		a.saveMemoryAsync(userMsg, reply)
		if a.mood != nil && a.mood.Enabled() {
			go a.updateMoodAsync(userMsg)
		}
		if meta != nil {
			// tokenCount is 0 on some branches (agent pipeline doesn't count
			// output tokens today) — fall back to estimating from the saved
			// reply so those turns still show up in usage stats.
			completionTokens := tokenCount
			statsTps := tps
			if completionTokens == 0 {
				completionTokens = estimateContentTokens(reply)
				if duration > 0 && completionTokens > 0 {
					statsTps = float64(completionTokens) / duration
				}
			}
			go a.recordUsageEvent(*meta, completionTokens, duration, statsTps)
		}
		// Captured synchronously here, not inside the goroutine — see
		// takeNudgedPattern's doc comment for why that ordering matters.
		if nudged := a.takeNudgedPattern(); nudged != nil {
			go a.checkAmbientNudgeSurfaced(nudged, reply)
		}
	} else {
		a.incognitoMu.Lock()
		a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("assistant", reply))
		a.incognitoMu.Unlock()
	}
}

func (a *App) callLLM(ctx context.Context, messages []api.Message) string {
	// Orchestra mode takes priority
	a.providerMu.RLock()
	orchEnabled := a.orchestraConductor != nil && a.orchestraConductor.Config().Enabled
	a.providerMu.RUnlock()
	if orchEnabled {
		var userPrompt string
		var systemPrompt string
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == "user" {
				if text := messages[i].GetTextContent(); text != "" {
					userPrompt = text
					break
				}
			}
		}
		for _, msg := range messages {
			if msg.Role == "system" {
				if text, ok := msg.Content.(string); ok {
					systemPrompt = text
				}
				break
			}
		}
		if userPrompt == "" {
			return "⚠️ No user message found"
		}
		conversationCtx := buildConversationContext(messages, userPrompt)
		if systemPrompt != "" {
			conversationCtx = "Sistem talimatları: " + systemPrompt + "\n\n---\n\n" + conversationCtx
		}
		octx, cancel := context.WithTimeout(ctx, 300*time.Second)
		defer cancel()
		finalResponse, _, err := a.orchestraConductor.Run(octx, conversationCtx)
		if err != nil {
			return "⚠️ " + err.Error()
		}
		return finalResponse
	}

	// Use external provider only if user explicitly selected one
	a.providerMu.RLock()
	activeName := a.activeProviderName
	providerRouter := a.providerRouter
	a.providerMu.RUnlock()
	if activeName != "" && providerRouter != nil {
		pctx, cancel := context.WithTimeout(ctx, 300*time.Second)
		defer cancel()

		pMsgs := make([]provider.Message, len(messages))
		for i, m := range messages {
			pMsgs[i] = provider.Message{Role: m.Role, Content: m.Content}
		}

		a.cfgMu.RLock()
		req := provider.ChatRequest{
			Messages:    pMsgs,
			Temperature: a.cfg.Llama.Temperature,
			TopP:        a.cfg.Llama.TopP,
			MaxTokens:   a.cfg.Llama.MaxTokens,
		}
		a.cfgMu.RUnlock()

		resp, err := providerRouter.ChatCompletion(pctx, req)
		if err != nil {
			logx.Printf("Provider error: %v", err)
			if a.providerSwapped(providerRouter) {
				return modelSwappedMidStreamMsg
			}
			errMsg := "⚠️ " + err.Error()
			if hint := a.localModelHint(); hint != "" {
				errMsg += "\n\n" + hint
			}
			return errMsg
		}
		return resp.Content
	}

	// Fallback to local model
	lctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	start := time.Now()
	a.clientMu.RLock()
	llmClient := a.client
	a.clientMu.RUnlock()

	if llmClient == nil {
		return "⚠️ Yerel model yüklenmemiş. Lütfen bir model başlatın veya API sağlayıcı seçin."
	}

	a.cfgMu.RLock()
	temperature, topP, maxTokens := a.cfg.Llama.Temperature, a.cfg.Llama.TopP, a.cfg.Llama.MaxTokens
	a.cfgMu.RUnlock()
	resp, err := llmClient.ChatCompletion(lctx, messages, temperature, topP, maxTokens)
	if err != nil {
		logx.Printf("LATENCY llm.complete total_ms=%d status=error messages=%d", time.Since(start).Milliseconds(), len(messages))
		logx.Printf("LLM error: %v", err)
		if a.clientSwapped(llmClient) {
			return modelSwappedMidStreamMsg
		}
		return "⚠️ " + err.Error()
	}
	if len(resp.Choices) == 0 {
		logx.Printf("LATENCY llm.complete total_ms=%d status=empty messages=%d", time.Since(start).Milliseconds(), len(messages))
		return "⚠️ Empty response"
	}

	reply := resp.Choices[0].Message.GetTextContent()
	logx.Printf("LATENCY llm.complete total_ms=%d status=ok messages=%d reply_chars=%d", time.Since(start).Milliseconds(), len(messages), len(reply))
	logx.Printf("<< Reply: %d chars", len(reply))
	return reply
}

// modelSwappedMidStreamMsg replaces a confusing low-level transport error
// (typically "connection refused") with a clear explanation when a request
// fails specifically because the client/provider it started with was
// swapped out from under it — see clientSwapped/providerSwapped.
const modelSwappedMidStreamMsg = "⚠️ Model veya sağlayıcı bu mesaj akarken değiştirildi, bu yüzden tamamlanamadı. Lütfen tekrar deneyin."

// clientSwapped reports whether a.client has changed since streamClient was
// captured under clientMu at the start of a call — i.e. the local model was
// stopped/restarted (StopLocalModel/StartLocalModel) while that call was
// still in flight, so any error it now returns (typically "connection
// refused" against the now-dead server) has an obvious, known cause rather
// than being a genuine, unexplained failure. clientMu/providerMu already
// correctly guard every read/write of a.client/a.providerRouter (verified,
// not a data race — see AGENTS.md's "Data Races" note); this only narrows
// what a stream reports about an error it hits *after* its own copy has
// gone stale.
func (a *App) clientSwapped(streamClient *api.Client) bool {
	a.clientMu.RLock()
	defer a.clientMu.RUnlock()
	return a.client != streamClient
}

// providerSwapped is clientSwapped's counterpart for a.providerRouter (the
// active external API provider changed mid-call).
func (a *App) providerSwapped(router *provider.Router) bool {
	a.providerMu.RLock()
	defer a.providerMu.RUnlock()
	return a.providerRouter != router
}

// beginBackgroundLLMCall derives a cancellable context for a background
// (non-chat) LLM call — currently only auto fact extraction — and registers
// its cancel func so a subsequent real chat turn can preempt it via
// preemptBackgroundLLM. Callers must invoke the returned cleanup func (via
// defer) once the call finishes, successfully or not.
func (a *App) beginBackgroundLLMCall(ctx context.Context) (context.Context, func()) {
	bgCtx, cancel := context.WithCancel(ctx)
	a.bgLLMMu.Lock()
	a.bgLLMCtx = bgCtx
	a.bgLLMCancel = cancel
	a.bgLLMMu.Unlock()
	return bgCtx, func() {
		a.bgLLMMu.Lock()
		// Only clear the registered call if it's still this one — a newer
		// background call may have already replaced it, and this cleanup
		// running late (this call's own goroutine finishing after another
		// one started) must not cancel or clear that newer call's slot.
		if a.bgLLMCtx == bgCtx {
			a.bgLLMCtx = nil
			a.bgLLMCancel = nil
		}
		a.bgLLMMu.Unlock()
		cancel()
	}
}

// preemptBackgroundLLM cancels whatever background LLM call is currently in
// flight (see beginBackgroundLLMCall), so a real user chat message is never
// left queued behind one on llama-server's single local-model inference
// slot (BUG_REPORT TD-2). Cancelling the Go-side context closes the HTTP
// request to llama-server, aborting that generation rather than letting it
// run to completion first. Safe to call unconditionally — a no-op if
// nothing is in flight.
func (a *App) preemptBackgroundLLM() {
	a.bgLLMMu.Lock()
	cancel := a.bgLLMCancel
	a.bgLLMMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// localModelHint returns a user-facing suggestion if a local model is running
// but the active provider failed. Empty string if no hint is applicable.
func (a *App) localModelHint() string {
	if a.llamaServer == nil || !a.llamaServer.IsRunning() {
		return ""
	}
	status := a.llamaServer.GetStatus()
	modelName := status.ModelName
	if modelName == "" {
		modelName = "local"
	}
	return fmt.Sprintf("💡 Yerel modeliniz (%s) çalışıyor. API sağlayıcı yerine yerel modeli kullanmak için /model yazıp Local'i seçin.", modelName)
}
