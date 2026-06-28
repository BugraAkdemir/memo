package app

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/logx"
	"strings"
	"sync"
	"time"

	"memo/internal/agent"
	"memo/internal/api"
	"memo/internal/orchestra"
	"memo/internal/provider"
)

// estimateContentTokens gives a rough token count for a UI display label.
// Word-based (~1.3 tokens/word) is closer than len/4 for Turkish and code.
func estimateContentTokens(s string) int {
	if s == "" {
		return 0
	}
	return int(float64(len(strings.Fields(s))) * 1.3)
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

func (a *App) callAgentStream(ctx context.Context, messages []api.Message, userMsg string) <-chan api.StreamChunk {
	outCh := make(chan api.StreamChunk, 128)

	go func() {
		defer close(outCh)

		pMsgs := make([]provider.Message, len(messages))
		for i, m := range messages {
			pMsgs[i] = provider.Message{Role: m.Role, Content: m.Content}
		}

		sm := a.getSessionManager()
		sessionID := ""
		if sm != nil {
			sessionID = sm.GetActiveID()
		}

		agentRouter, modelName, err := a.resolveAgentProvider()
		if err != nil {
			trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
			return
		}

		projectPath := ""
		if sessionID != "" && sm != nil {
			projectPath = sm.GetProjectPath(sessionID)
		}

		a.agentExecutor.SyncRouter(agentRouter)

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
			trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
			return
		}

		for chunk := range streamCh {
			if chunk.Error != "" {
				trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + chunk.Error, Done: true})
				return
			}

			if chunk.Content != "" {
				fullReply.WriteString(chunk.Content)
				trySend(ctx, outCh, api.StreamChunk{Content: chunk.Content})
			}

			if chunk.Done {
				a.finishStream(start, 0, chunk.FinishReason, fullReply.String(), userMsg, agentEvents)
				trySend(ctx, outCh, api.StreamChunk{Done: true, FinishReason: chunk.FinishReason})
				return
			}
		}

		if fullReply.Len() > 0 {
			a.finishStream(start, 0, "stop", fullReply.String(), userMsg, agentEvents)
			trySend(ctx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
		}
	}()

	return outCh
}

// callAgentWithOrchestra runs when both agent mode and orchestra mode are enabled.
func (a *App) callAgentWithOrchestra(ctx context.Context, messages []api.Message, userMsg string) <-chan api.StreamChunk {
	outCh := make(chan api.StreamChunk, 128)

	go func() {
		defer close(outCh)

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

		sm := a.getSessionManager()
		sessionID := ""
		if sm != nil {
			sessionID = sm.GetActiveID()
		}

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
				trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
				return
			}
		}

		projectPath := ""
		if sessionID != "" && sm != nil {
			projectPath = sm.GetProjectPath(sessionID)
		}

		a.agentExecutor.SyncRouter(agentRouter)

		start := time.Now()
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
				a.finishStream(start, 0, "error", finalContent, userMsg, agentEvents)
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
			a.finishStream(start, 0, finishReason, finalReply, userMsg, agentEvents)
			trySend(ctx, outCh, api.StreamChunk{Done: true, FinishReason: finishReason})
		}

		for chunk := range streamCh {
			if chunk.Error != "" {
				// Persist the chief's synthesis (and any agent text so far) so a
				// mid-agent failure doesn't wipe the answer already on screen.
				if salvage := strings.TrimSpace(finalContent + "\n\n" + agentBuf.String()); salvage != "" {
					a.finishStream(start, 0, "error", salvage, userMsg, agentEvents)
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

func (a *App) callLLMStream(ctx context.Context, messages []api.Message, userMsg, imagePath, filePath string) <-chan api.StreamChunk {
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

			trySend(ctx, outCh, api.StreamChunk{Content: "🎵 **Orchestra Mode Active**\n"})
			trySend(ctx, outCh, api.StreamChunk{Content: fmt.Sprintf("🧙 Şef: %s/%s\n\n", a.orchestraConductor.Config().ChiefType, a.orchestraConductor.Config().ChiefModel)})

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
				a.finishStream(start, 0, "error", fullBufStr, userPrompt)
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

			a.finishStream(start, tokenCount, "stop", finalContent, userPrompt)
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

			providerCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
			defer cancel()

			pMsgs := make([]provider.Message, len(messages))
			for i, m := range messages {
				pMsgs[i] = provider.Message{Role: m.Role, Content: m.Content}
			}

			req := provider.ChatRequest{
				Messages:    pMsgs,
				Temperature: a.cfg.Llama.Temperature,
				TopP:        a.cfg.Llama.TopP,
				MaxTokens:   a.cfg.Llama.MaxTokens,
				Stream:      true,
			}

			ch, err := providerRouter.ChatCompletionStream(providerCtx, req)
			if err != nil {
				logx.Printf("Provider stream error: %v", err)
				a.recordStreamError(userMsg, "⚠️ "+err.Error())
				trySend(providerCtx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
				return
			}

			start := time.Now()
			var fullReply strings.Builder
			tokenCount := 0
			firstTokenLogged := false

		providerLoop:
			for {
				select {
				case <-providerCtx.Done():
					return
				case chunk, ok := <-ch:
					if !ok {
						break providerLoop
					}

					if chunk.Error != "" {
						a.recordStreamError(userMsg, "⚠️ "+chunk.Error)
						trySend(providerCtx, outCh, api.StreamChunk{Error: "⚠️ " + chunk.Error, Done: true})
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
						a.finishStream(start, tokenCount, chunk.FinishReason, fullReply.String(), userMsg)
						trySend(providerCtx, outCh, api.StreamChunk{Done: true, FinishReason: chunk.FinishReason})
						return
					}
				}
			}

			if fullReply.Len() > 0 {
				a.finishStream(start, tokenCount, "stop", fullReply.String(), userMsg)
				trySend(providerCtx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
			} else {
				trySend(providerCtx, outCh, api.StreamChunk{Error: "⚠️ Provider returned empty response", Done: true})
			}
		}()
		return outCh
	}

	// Fallback to local model
	a.clientMu.RLock()
	streamClient := a.client
	a.clientMu.RUnlock()

	if streamClient == nil {
		trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ Yerel model yüklenmemiş. Lütfen bir model başlatın veya API sağlayıcı seçin.", Done: true})
		return outCh
	}

	go func() {
		defer close(outCh)

		streamCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
		defer cancel()

		requestStart := time.Now()
		ch, err := streamClient.ChatCompletionStream(streamCtx, messages, a.cfg.Llama.Temperature, a.cfg.Llama.TopP, a.cfg.Llama.MaxTokens)
		if err != nil {
			logx.Printf("LATENCY llm.stream_error total_ms=%d messages=%d", time.Since(requestStart).Milliseconds(), len(messages))
			logx.Printf("LLM stream error: %v", err)
			a.recordStreamError(userMsg, "⚠️ "+err.Error())
			trySend(streamCtx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
			return
		}
		logx.Printf("LATENCY llm.stream_ready total_ms=%d messages=%d", time.Since(requestStart).Milliseconds(), len(messages))

		start := time.Now()
		var fullReply strings.Builder
		tokenCount := 0
		firstTokenLogged := false

	localLoop:
		for {
			select {
			case <-streamCtx.Done():
				return
			case chunk, ok := <-ch:
				if !ok {
					break localLoop
				}

				if chunk.Error != "" {
					logx.Printf("LATENCY llm.stream_chunk_error total_ms=%d generation_ms=%d tokens=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds(), tokenCount)
					logx.Printf("Stream chunk error: %s", chunk.Error)
					a.recordStreamError(userMsg, "⚠️ "+chunk.Error)
					trySend(streamCtx, outCh, api.StreamChunk{Error: "⚠️ " + chunk.Error, Done: true})
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
					a.finishStream(start, tokenCount, chunk.FinishReason, fullReply.String(), userMsg)
					trySend(streamCtx, outCh, chunk)
					return
				}
			}
		}

		if fullReply.Len() > 0 {
			logx.Printf("LATENCY llm.stream_closed total_ms=%d generation_ms=%d tokens=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds(), tokenCount)
			a.finishStream(start, tokenCount, "stop", fullReply.String(), userMsg)
			trySend(streamCtx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
		} else {
			logx.Printf("LATENCY llm.stream_empty total_ms=%d generation_ms=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds())
			a.recordStreamError(userMsg, "⚠️ Model boş yanıt döndürdü")
			trySend(streamCtx, outCh, api.StreamChunk{Error: "⚠️ Model boş yanıt döndürdü", Done: true})
		}
	}()

	return outCh
}

// trySend sends a chunk to outCh or returns if the context is cancelled.
func trySend(ctx context.Context, outCh chan<- api.StreamChunk, chunk api.StreamChunk) {
	select {
	case outCh <- chunk:
	case <-ctx.Done():
	}
}

// recordStreamError saves an error reply to the session to prevent dangling user
// messages. Called on all stream error paths where finishStream is not invoked.
func (a *App) recordStreamError(userMsg, errReply string) {
	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if !incog {
		sm := a.getSessionManager()
		if sm != nil {
			sm.AddMessage("assistant", errReply, "", "")
		}
	} else {
		a.incognitoMu.Lock()
		a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("assistant", errReply))
		a.incognitoMu.Unlock()
	}
}

func (a *App) finishStream(start time.Time, tokenCount int, finishReason, reply, userMsg string, agentEvents ...[]interface{}) {
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
			sm.AddMessage("assistant", reply, "", "", agentEvents...)
			if len(sm.GetActiveMessages()) == 2 {
				go a.GenerateChatTitle()
			}
		}
		a.saveMemoryAsync(userMsg, reply)
		if a.mood != nil && a.mood.Enabled() {
			go a.updateMoodAsync(userMsg)
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

		req := provider.ChatRequest{
			Messages:    pMsgs,
			Temperature: a.cfg.Llama.Temperature,
			TopP:        a.cfg.Llama.TopP,
			MaxTokens:   a.cfg.Llama.MaxTokens,
		}

		resp, err := providerRouter.ChatCompletion(pctx, req)
		if err != nil {
			logx.Printf("Provider error: %v", err)
			return "⚠️ " + err.Error()
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

	resp, err := llmClient.ChatCompletion(lctx, messages, a.cfg.Llama.Temperature, a.cfg.Llama.TopP, a.cfg.Llama.MaxTokens)
	if err != nil {
		logx.Printf("LATENCY llm.complete total_ms=%d status=error messages=%d", time.Since(start).Milliseconds(), len(messages))
		logx.Printf("LLM error: %v", err)
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
