package app

import (
	"context"
	"encoding/json"
	"errors"
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
	Category     string
	PromptTokens int
}

// Usage event categories — see stats.Event.Category's doc comment. Every
// call site that constructs a usageMeta (streaming) or calls callLLM
// (single-shot) picks exactly one of these, so the Stats tab's category
// breakdown can show "which kind of call is spending my tokens," not just
// which model/provider.
const (
	categoryChat           = "chat"
	categoryAgent          = "agent"
	categoryFactExtraction = "fact_extraction"
	categoryDream          = "dream"
	categoryConsolidation  = "consolidation"
	categoryMemoryImport   = "memory_import"
	categoryMood           = "mood"
	categoryTitle          = "title"
	categoryLearning       = "learning"
	categoryRoutine        = "routine"
	categoryProactive      = "proactive"
	categoryInsight        = "insight"
	categoryWebSearch      = "web_search"
)

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

// activeProviderEffortLevel mirrors activeProviderModel above, for
// provider.ProviderConfig.EffortLevel — deliberately per-provider-config,
// not a single global setting like Temperature/TopP (a.cfg.Llama.*) are
// today, since the valid values (and even the request shape) differ by
// vendor. See provider/effort.go's package doc comment.
func (a *App) activeProviderEffortLevel(name string) string {
	a.providerMu.RLock()
	cfgMgr := a.providerCfgMgr
	a.providerMu.RUnlock()
	if cfgMgr == nil {
		return ""
	}
	for _, p := range cfgMgr.GetEnabled() {
		if p.Name == name {
			return p.EffortLevel
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

// resolveAgentProvider returns the provider router, model name, and
// resolved EffortLevel (see activeProviderEffortLevel's doc comment; empty
// for the local llama.cpp fallback, which has no such stored setting) the
// agent pipeline should use. The write lock is held continuously during
// the nil->router creation window to prevent a second goroutine from
// racing in and creating a second router (which would silently replace the
// first one).
func (a *App) resolveAgentProvider() (*provider.Router, string, string, error) {
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
			return nil, "", "", errors.New(a.t("Agent modu için bir sağlayıcı (provider) yapılandırmadınız. Ayarlar > Sağlayıcılar bölümünde bir API sağlayıcısı ekleyin veya yerel bir model başlatın.", "No provider configured for Agent mode. Add an API provider under Settings > Providers or start a local model."))
		}
		if providerCfgMgr == nil {
			return nil, "", "", fmt.Errorf("provider system not initialized")
		}
		modelName := ""
		effortLevel := ""
		for _, p := range providerCfgMgr.GetEnabled() {
			if p.Name == activeName {
				modelName = p.Model
				effortLevel = p.EffortLevel
				break
			}
		}
		if modelName == "" {
			for _, p := range providerCfgMgr.GetEnabled() {
				modelName = p.Model
				effortLevel = p.EffortLevel
				break
			}
		}
		return providerRouter, modelName, effortLevel, nil
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
		return provider.NewRouter([]provider.ProviderConfig{cfg}), modelName, "", nil
	}

	return nil, "", "", errors.New(a.t("Agent modu için bir API sağlayıcısı seçin ya da yerel bir model başlatın (Modeller bölümünden).", "Select an API provider for Agent mode or start a local model (from the Models section)."))
}

// agentRouterFromProviderName builds a single-provider router for the given
// provider name + model, used as the agent's fallback in combined Orchestra+Agent
// mode when no separate active provider is configured. Also returns that
// provider's own configured EffortLevel (pc keeps every field from the
// matched config except Model, which the caller overrides).
func (a *App) agentRouterFromProviderName(providerName, model string) (*provider.Router, string, string, error) {
	a.providerMu.RLock()
	cfgMgr := a.providerCfgMgr
	a.providerMu.RUnlock()
	if cfgMgr == nil {
		return nil, "", "", fmt.Errorf("no provider config manager")
	}
	enabled := cfgMgr.GetEnabled()
	// Match on Name (the new identifier) first.
	for _, p := range enabled {
		if p.Name == providerName {
			pc := p
			pc.Model = model
			return provider.NewRouter([]provider.ProviderConfig{pc}), model, pc.EffortLevel, nil
		}
	}
	// Fall back to Type match for backward compatibility: the Orchestra ChiefType
	// holds a provider *type* string ("claude") both in legacy orchestra.json files
	// and in the current default config, so a Name-only lookup would miss it.
	for _, p := range enabled {
		if string(p.Type) == providerName {
			pc := p
			pc.Model = model
			return provider.NewRouter([]provider.ProviderConfig{pc}), model, pc.EffortLevel, nil
		}
	}
	return nil, "", "", fmt.Errorf("orchestra chief provider %q is not enabled", providerName)
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

		agentRouter, modelName, effortLevel, err := a.resolveAgentProvider()
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

		usageMetaVal := usageMeta{Provider: a.currentProviderLabel(), Model: modelName, Category: categoryAgent, PromptTokens: estimateMessagesTokens(messages)}

		start := time.Now()
		agentEvents := &agentEventLog{}

		streamCh, err := a.agentExecutor.RunStream(ctx, sessionID, modelName, effortLevel, pMsgs, func(ev agent.AgentEvent) {
			agentEvents.add(ev)
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

		a.drainAgentStream(ctx, streamCh, outCh, start, userMsg, sessionID, &usageMetaVal, agentEvents)
	}()

	return outCh
}

// agentEventLog accumulates agent tool-execution events across the lifetime
// of one callAgentStream call. Two goroutines touch it: the pipeline's own
// worker goroutine appends to it (via the onEvent callback passed to
// agent.Executor.RunStream, which starts that goroutine and returns
// immediately — RunStream does not block until the pipeline finishes), while
// callAgentStream's goroutine reads it once the stream completes, to hand
// the full event list to finishStream for session recording. A plain
// `var agentEvents []interface{}` (the previous shape) raced on exactly
// that: append vs. read with no synchronization (caught by
// TestSendMessage_AgentModeOn_SendsToolDefinitions under -race, the first
// test to actually drive a real agent pipeline run to completion). It was
// also silently wrong even ignoring the race — the read happened once,
// immediately after RunStream returned (i.e. right as the pipeline started,
// before any tool call could have run), and that stale near-empty snapshot
// is what got passed into drainAgentStream and ultimately persisted as the
// turn's agent-event history, regardless of how many tool calls actually
// ran before the stream later finished. snapshot() must be called only
// once the stream is actually done (drainAgentStream's two finishStream
// call sites), not at RunStream's return time.
type agentEventLog struct {
	mu     sync.Mutex
	events []interface{}
}

func (l *agentEventLog) add(ev interface{}) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.events = append(l.events, ev)
	l.mu.Unlock()
}

// snapshot returns a defensive copy — the caller (finishStream, eventually
// sessions.Manager) must not share backing storage with a slice this log
// might still be appending to concurrently.
func (l *agentEventLog) snapshot() []interface{} {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.events) == 0 {
		return nil
	}
	out := make([]interface{}, len(l.events))
	copy(out, l.events)
	return out
}

// drainAgentStream reads streamCh (the agent pipeline's own output) until it
// closes or ctx is cancelled, forwarding content to outCh and finishing the
// turn (finishStream + a terminal Done/Error chunk) on every exit path.
//
// Extracted out of callAgentStream's closure so this exact fan-in logic is
// directly unit-testable against a hand-built streamCh, without needing a
// real agent.Executor/Pipeline round trip — see
// TestDrainAgentStream_EmptyChannelSendsTerminalChunk (BUG_REPORT.md SF-5):
// every current Pipeline.RunStream exit path already sends a terminal chunk
// before closing its channel, so the bug this guards against (streamCh
// closing with zero chunks ever received) isn't reachable through today's
// real pipeline — this is deliberate defense-in-depth against a future
// change to that pipeline (or a different agent.Executor implementation)
// introducing exactly that gap, matching the same "every branch sends a
// terminal chunk" rule already enforced in internal/agentcli's own
// ChatCompletionStream implementations.
func (a *App) drainAgentStream(ctx context.Context, streamCh <-chan provider.StreamChunk, outCh chan<- api.StreamChunk, start time.Time, userMsg, sessionID string, usageMetaVal *usageMeta, agentEvents *agentEventLog) {
	var fullReply strings.Builder
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
			a.finishStream(ctx, start, 0, chunk.FinishReason, fullReply.String(), userMsg, sessionID, usageMetaVal, agentEvents.snapshot())
			trySend(ctx, outCh, api.StreamChunk{Done: true, FinishReason: chunk.FinishReason})
			return
		}
	}

	if fullReply.Len() > 0 {
		a.finishStream(ctx, start, 0, "stop", fullReply.String(), userMsg, sessionID, usageMetaVal, agentEvents.snapshot())
		trySend(ctx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
	} else {
		a.recordStreamError(userMsg, a.t("⚠️ Agent boş yanıt döndürdü", "⚠️ Agent returned an empty response"), sessionID)
		trySend(ctx, outCh, api.StreamChunk{Error: a.t("⚠️ Agent boş yanıt döndürdü", "⚠️ Agent returned an empty response"), Done: true})
	}
}

// callWebSearchAgentStream runs a single-tool (web_search only) agent
// pipeline for plain (non-agent) chat when the web-search toggle is on —
// see agent.NewWebSearchExecutor's doc comment for why. It replaces the old
// design (buildMessagesForSession used to blindly run websearch.Search on
// every single message, injecting results into the system prompt whether or
// not the message actually needed them, and only skipped this when agent
// mode was also on — see BUG_REPORT/handoff history for the two separate
// user reports this caused: results built from the raw, un-distilled user
// message, and the search running "indiscriminately" even for a bare
// greeting). Here the model gets exactly one tool via native
// function-calling in the SAME completion request that produces its answer
// — it decides per message, at zero extra network/LLM cost when it decides
// not to search, same shape full agent mode already uses for its whole
// toolset.
//
// Deliberately does not forward agent_event chunks to outCh (unlike
// callAgentStream) and does not record them into agentEvents either — full
// agent mode's tool-badge UI (_AgentStatusBar/_AgentStatusBadge in
// chat_message_list.dart) is intentionally not part of this mode's UX; the
// only visible signal is the existing "Webde aranıyor..." typing-line
// status (streamingStatusProvider), fired here exactly when the tool
// actually starts executing — not, like the old design's pre-emptive
// chunk in SendMessageStream, unconditionally on every message regardless
// of whether a search happens.
func (a *App) callWebSearchAgentStream(ctx context.Context, messages []api.Message, userMsg, sessionID string) <-chan api.StreamChunk {
	outCh := make(chan api.StreamChunk, 128)

	go func() {
		defer close(outCh)
		defer recoverStreamPanic(ctx, outCh, "callWebSearchAgentStream")

		pMsgs := make([]provider.Message, len(messages))
		for i, m := range messages {
			pMsgs[i] = provider.Message{Role: m.Role, Content: m.Content}
		}

		agentRouter, modelName, effortLevel, err := a.resolveAgentProvider()
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

		a.webSearchExecutor.SyncRouter(agentRouter)

		usageMetaVal := usageMeta{Provider: a.currentProviderLabel(), Model: modelName, Category: categoryWebSearch, PromptTokens: estimateMessagesTokens(messages)}

		start := time.Now()
		agentEvents := &agentEventLog{}

		streamCh, err := a.webSearchExecutor.RunStream(ctx, sessionID, modelName, effortLevel, pMsgs, func(ev agent.AgentEvent) {
			if ev.Type == agent.EventToolExecuting && ev.ToolName == "web_search" {
				trySend(ctx, outCh, api.StreamChunk{FinishReason: "status", Content: "web_search"})
			}
		}, projectPath)

		if err != nil {
			logx.Printf("Web search agent error: %v", err)
			a.recordStreamError(userMsg, "⚠️ "+err.Error(), sessionID)
			trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
			return
		}

		a.drainAgentStream(ctx, streamCh, outCh, start, userMsg, sessionID, &usageMetaVal, agentEvents)
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
			// Active-skill instructions are already part of systemPrompt by
			// the time messages reaches here (baked in by
			// buildMessagesForSession, helpers.go) — appending
			// buildActiveSkillPrompt() again here would duplicate them for
			// every real caller of this function, which all build messages
			// that way.
			conversationCtx = "Sistem talimatları: " + systemPrompt + "\n\n---\n\n" + conversationCtx
		}

		// Only the chief talks to the user with its own words. Plan/task
		// activity lives in the right-side activity panel; each task's real
		// tool activity (permission dialogs, tool results) streams as normal
		// agent_event chunks, exactly like a non-Orchestra agent chat turn.
		var fullBuf strings.Builder
		var fullBufMu sync.Mutex
		// outAccum tracks total model output tokens (specialists + chief) for
		// the token counter. Guarded by fullBufMu because parallel tasks
		// invoke the callback concurrently.
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

		// Resolved once, up front, so every task's specialist executes real
		// tool calls through the same agent (provider/model/sandbox) this
		// chat is actually configured with — see AgentTaskRunner's and
		// RunAgentTasks's doc comments for why tasks need this instead of a
		// plain ChatCompletion.
		agentRouter, modelName, effortLevel, err := a.resolveAgentProvider()
		if err != nil {
			// Combined mode: an Orchestra-configured user often has no separate
			// "active provider" set, so the agent half would otherwise fail here.
			// Fall back to the Orchestra chief's provider so the two systems stay
			// connected.
			ocfg := a.orchestraConductor.Config()
			if r, m, el, ferr := a.agentRouterFromProviderName(ocfg.ChiefType, ocfg.ChiefModel); ferr == nil {
				agentRouter, modelName, effortLevel = r, m, el
			} else {
				a.recordStreamError(userMsg, "⚠️ "+err.Error(), sessionID)
				trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
				return
			}
		}
		a.agentExecutor.SyncRouter(agentRouter)

		sm := a.getSessionManager()
		projectPath := ""
		if sessionID != "" && sm != nil {
			projectPath = sm.GetProjectPath(sessionID)
		}

		pMsgs := make([]provider.Message, len(messages))
		for i, m := range messages {
			pMsgs[i] = provider.Message{Role: m.Role, Content: m.Content}
		}

		var agentEventsMu sync.Mutex
		var agentEvents []interface{}

		// agentRunner gives each task's specialist real tool access (file,
		// command, web) via the same pipeline a direct agent chat uses —
		// sandboxed and permission-gated, so a live permission dialog still
		// appears exactly like it would outside Orchestra. taskPrompt
		// becomes one more user turn on top of the real conversation
		// history (pMsgs), never a fabricated assistant claim about what
		// happened — that's what caused the self-fulfilling "I don't have
		// tool access" failures this replaces (see RunAgentTasks).
		agentRunner := func(taskCtx context.Context, taskPrompt string, onEvent func(string)) (string, error) {
			taskMsgs := append(append([]provider.Message{}, pMsgs...), provider.Message{Role: "user", Content: taskPrompt})
			streamCh, err := a.agentExecutor.RunStream(taskCtx, sessionID, modelName, effortLevel, taskMsgs, func(ev agent.AgentEvent) {
				agentEventsMu.Lock()
				agentEvents = append(agentEvents, ev)
				agentEventsMu.Unlock()
				if chunkData, merr := json.Marshal(ev); merr == nil {
					onEvent(string(chunkData))
				}
			}, projectPath)
			if err != nil {
				return "", err
			}
			var sb strings.Builder
			for chunk := range streamCh {
				if chunk.Error != "" {
					return sb.String(), fmt.Errorf("%s", chunk.Error)
				}
				sb.WriteString(chunk.Content)
				if chunk.Done {
					break
				}
			}
			return sb.String(), nil
		}

		orchestraResult, _, err := a.orchestraConductor.RunAgentTasks(ctx, conversationCtx, func(up orchestra.ProgressUpdate) {
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
				// agentRunner forwards each real agent.AgentEvent here as raw
				// JSON — pass it straight through as a normal agent_event
				// chunk so permission dialogs and tool-activity render live,
				// exactly like a non-Orchestra agent chat turn.
				if up.Content != "" {
					trySend(ctx, outCh, api.StreamChunk{Content: up.Content, FinishReason: "agent_event"})
				}
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
		}, agentRunner)
		if err != nil {
			a.recordStreamError(userMsg, a.t("⚠️ Orchestra hatası: ", "⚠️ Orchestra error: ")+err.Error(), sessionID)
			trySend(ctx, outCh, api.StreamChunk{Error: a.t("⚠️ Orchestra hatası: ", "⚠️ Orchestra error: ") + err.Error(), Done: true})
			return
		}

		fullBufMu.Lock()
		finalContent := fullBuf.String()
		fullBufMu.Unlock()
		if finalContent == "" {
			finalContent = orchestraResult
		}

		fullBufMu.Lock()
		outAccum += estimateContentTokens(finalContent)
		fullBufMu.Unlock()
		emitUsage(outAccum)

		usageMetaVal := usageMeta{Provider: a.currentProviderLabel(), Model: modelName, Category: categoryAgent, PromptTokens: inputTokens}
		agentEventsMu.Lock()
		finalEvents := agentEvents
		agentEventsMu.Unlock()
		a.finishStream(ctx, start, 0, "stop", finalContent, userMsg, sessionID, &usageMetaVal, finalEvents)
		trySend(ctx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
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
			usageMetaVal := usageMeta{Provider: "orchestra", Model: ocfg.ChiefModel, Category: categoryChat, PromptTokens: estimateContentTokens(conversationCtx)}

			trySend(ctx, outCh, api.StreamChunk{Content: "🎵 **Orchestra Mode Active**\n"})
			trySend(ctx, outCh, api.StreamChunk{Content: fmt.Sprintf(a.t("🧙 Şef: %s/%s\n\n", "🧙 Chief: %s/%s\n\n"), ocfg.ChiefType, ocfg.ChiefModel)})

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
					trySend(ctx, outCh, api.StreamChunk{Content: a.t("🧠 **Şef planlıyor...**\n\n", "🧠 **Chief is planning...**\n\n")})
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
				a.finishStream(ctx, start, 0, "error", fullBufStr, userPrompt, sessionID, &usageMetaVal)
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

			a.finishStream(ctx, start, tokenCount, "stop", finalContent, userPrompt, sessionID, &usageMetaVal)
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
				EffortLevel: a.activeProviderEffortLevel(activeName),
			}
			a.cfgMu.RUnlock()

			ch, err := providerRouter.ChatCompletionStream(providerCtx, req)
			if err != nil {
				logx.Printf("Provider stream error: %v", err)
				var errMsg string
				if a.providerSwapped(providerRouter) {
					errMsg = a.modelSwappedMidStreamMsg()
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
			usageMetaVal := usageMeta{Provider: activeName, Model: a.activeProviderModel(activeName), Category: categoryChat, PromptTokens: estimateMessagesTokens(messages)}
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
						errMsg = a.modelSwappedMidStreamMsg()
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
					a.finishStream(ctx, start, tokenCount, chunk.FinishReason, fullReply.String(), userMsg, sessionID, &usageMetaVal)
					trySend(providerCtx, outCh, api.StreamChunk{Done: true, FinishReason: chunk.FinishReason})
					return
				}
			}

			if fullReply.Len() > 0 {
				a.finishStream(ctx, start, tokenCount, "stop", fullReply.String(), userMsg, sessionID, &usageMetaVal)
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
		case outCh <- api.StreamChunk{Error: a.t("⚠️ Yerel model yüklenmemiş. Lütfen bir model başlatın veya API sağlayıcı seçin.", "⚠️ Local model not loaded. Start a model or select an API provider."), Done: true}:
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
				errMsg = a.modelSwappedMidStreamMsg()
			}
			a.recordStreamError(userMsg, errMsg, sessionID)
			trySend(streamCtx, outCh, api.StreamChunk{Error: errMsg, Done: true})
			return
		}
		logx.Printf("LATENCY llm.stream_ready total_ms=%d messages=%d", time.Since(requestStart).Milliseconds(), len(messages))

		start := time.Now()
		usageMetaVal := usageMeta{Provider: "local", Model: a.localModelName(), Category: categoryChat, PromptTokens: estimateMessagesTokens(messages)}
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
					errMsg = a.modelSwappedMidStreamMsg()
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
				a.finishStream(ctx, start, tokenCount, chunk.FinishReason, fullReply.String(), userMsg, sessionID, &usageMetaVal)
				trySend(streamCtx, outCh, chunk)
				return
			}
		}

		if fullReply.Len() > 0 {
			logx.Printf("LATENCY llm.stream_closed total_ms=%d generation_ms=%d tokens=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds(), tokenCount)
			a.finishStream(ctx, start, tokenCount, "stop", fullReply.String(), userMsg, sessionID, &usageMetaVal)
			trySend(streamCtx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
		} else {
			logx.Printf("LATENCY llm.stream_empty total_ms=%d generation_ms=%d", time.Since(requestStart).Milliseconds(), time.Since(start).Milliseconds())
			a.recordStreamError(userMsg, a.t("⚠️ Model boş yanıt döndürdü", "⚠️ Model returned an empty response"), sessionID)
			trySend(streamCtx, outCh, api.StreamChunk{Error: a.t("⚠️ Model boş yanıt döndürdü", "⚠️ Model returned an empty response"), Done: true})
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
		Category:         meta.Category,
		PromptTokens:     meta.PromptTokens,
		CompletionTokens: completionTokens,
		DurationSecs:     durationSecs,
		TokensPerSecond:  tokensPerSecond,
	}); err != nil {
		logx.Printf("WARN: record usage event: %v", err)
	}
}

// memoryUsedCtxKey carries how many memories buildMessagesForSession
// retrieved for the current turn from where it's produced
// (sendMessageStreamCore, chat.go) to where it's consumed (finishStream,
// below) — riding along on the ctx that already flows unchanged through
// routeStream/callLLMStream/callAgentStream rather than adding a parameter
// to each of them for a value only the two endpoints care about.
type memoryUsedCtxKey struct{}

func (a *App) finishStream(ctx context.Context, start time.Time, tokenCount int, finishReason, reply, userMsg, sessionID string, meta *usageMeta, agentEvents ...[]interface{}) {
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
				if memUsed, ok := ctx.Value(memoryUsedCtxKey{}).(int); ok && memUsed > 0 {
					sm.SetLastMessageMemoryUsed(sessionID, memUsed)
				}
				if len(sm.GetActiveMessagesForSession(sessionID)) == 2 {
					goRecover("generateChatTitleForSession", func() { a.generateChatTitleForSession(sessionID) })
				}
			} else {
				sm.AddMessage("assistant", reply, "", "", agentEvents...)
				if len(sm.GetActiveMessages()) == 2 {
					goRecover("GenerateChatTitle", func() { a.GenerateChatTitle() })
				}
			}
		}
		a.saveMemoryAsync(userMsg, reply)
		if a.mood != nil && a.mood.Enabled() {
			goRecover("updateMoodAsync", func() { a.updateMoodAsync(userMsg) })
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
			goRecover("recordUsageEvent", func() { a.recordUsageEvent(*meta, completionTokens, duration, statsTps) })
		}
		// Captured synchronously here, not inside the goroutine — see
		// takeNudgedPattern's doc comment for why that ordering matters.
		if nudged := a.takeNudgedPattern(); nudged != nil {
			goRecover("checkAmbientNudgeSurfaced", func() { a.checkAmbientNudgeSurfaced(nudged, reply) })
		}
	} else {
		a.incognitoMu.Lock()
		a.incognitoMessages = append(a.incognitoMessages, api.NewTextMessage("assistant", reply))
		a.incognitoMu.Unlock()
	}
}

// callLLM is the shared single-completion helper behind every background/
// utility LLM call in this codebase (chat titles, Dream, fact extraction,
// mood, learning, routines, proactive checks, memory import/consolidation —
// see the category* constants above). category tags the resulting usage
// event so the Stats tab's category breakdown can show which of these is
// actually spending tokens — required, not defaulted, so a new call site
// can't silently land as miscategorized or unrecorded.
func (a *App) callLLM(ctx context.Context, messages []api.Message, category string) string {
	start := time.Now()

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
		// callLLM is a single-completion helper (chat titles, routine
		// parsing, memory summaries, proactive checks — see its callers) —
		// it must not go through Run/RunWithProgress, which forces the
		// chief into the plan→execute→synthesize workflow and its
		// {"tasks":[...]} contract. See RunSingle's doc comment for the two
		// live-reproduced failures that caused (a routine request failing
		// with "chief returned no tasks", and a plain chat title taking 3+
		// minutes because it silently ran the full pipeline).
		octx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		finalResponse, err := a.orchestraConductor.RunSingle(octx, systemPrompt, userPrompt)
		if err != nil {
			return "⚠️ " + err.Error()
		}
		// Orchestra's RunSingle exposes no usage info — estimate both sides
		// the same way the orchestra branch of callLLMStream already does.
		a.recordCallLLMUsage(start, "orchestra", "", category, estimateContentTokens(systemPrompt+" "+userPrompt), estimateContentTokens(finalResponse))
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
			EffortLevel: a.activeProviderEffortLevel(activeName),
		}
		a.cfgMu.RUnlock()

		resp, err := providerRouter.ChatCompletion(pctx, req)
		if err != nil {
			logx.Printf("Provider error: %v", err)
			if a.providerSwapped(providerRouter) {
				return a.modelSwappedMidStreamMsg()
			}
			errMsg := "⚠️ " + err.Error()
			if hint := a.localModelHint(); hint != "" {
				errMsg += "\n\n" + hint
			}
			return errMsg
		}
		model := resp.Model
		if model == "" {
			model = a.activeProviderModel(activeName)
		}
		promptTokens, completionTokens := estimateMessagesTokens(messages), estimateContentTokens(resp.Content)
		if resp.Usage != nil {
			promptTokens, completionTokens = resp.Usage.PromptTokens, resp.Usage.CompletionTokens
		}
		a.recordCallLLMUsage(start, activeName, model, category, promptTokens, completionTokens)
		return resp.Content
	}

	// Fallback to local model
	lctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	a.clientMu.RLock()
	llmClient := a.client
	a.clientMu.RUnlock()

	if llmClient == nil {
		return a.t("⚠️ Yerel model yüklenmemiş. Lütfen bir model başlatın veya API sağlayıcı seçin.", "⚠️ Local model not loaded. Start a model or select an API provider.")
	}

	a.cfgMu.RLock()
	temperature, topP, maxTokens := a.cfg.Llama.Temperature, a.cfg.Llama.TopP, a.cfg.Llama.MaxTokens
	a.cfgMu.RUnlock()
	resp, err := llmClient.ChatCompletion(lctx, messages, temperature, topP, maxTokens)
	if err != nil {
		logx.Printf("LATENCY llm.complete total_ms=%d status=error messages=%d", time.Since(start).Milliseconds(), len(messages))
		logx.Printf("LLM error: %v", err)
		if a.clientSwapped(llmClient) {
			return a.modelSwappedMidStreamMsg()
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
	promptTokens, completionTokens := estimateMessagesTokens(messages), estimateContentTokens(reply)
	if resp.Usage.PromptTokens > 0 || resp.Usage.CompletionTokens > 0 {
		promptTokens, completionTokens = resp.Usage.PromptTokens, resp.Usage.CompletionTokens
	}
	a.recordCallLLMUsage(start, "local", a.localModelName(), category, promptTokens, completionTokens)
	return reply
}

// recordCallLLMUsage is callLLM's shared recording tail for all three
// branches — fired in its own goroutine (matching finishStream's own
// recordUsageEvent call) so a slow/failing stats write never adds latency
// to the already-synchronous callLLM return path. Skips incognito sessions
// entirely, same privacy invariant finishStream applies to the streaming
// path — token *counts* are still session content in aggregate, not just
// message text.
func (a *App) recordCallLLMUsage(start time.Time, provider, model, category string, promptTokens, completionTokens int) {
	a.incognitoMu.RLock()
	incog := a.isIncognito
	a.incognitoMu.RUnlock()
	if incog {
		return
	}
	duration := time.Since(start).Seconds()
	tps := 0.0
	if duration > 0 && completionTokens > 0 {
		tps = float64(completionTokens) / duration
	}
	goRecover("recordUsageEvent", func() {
		a.recordUsageEvent(usageMeta{Provider: provider, Model: model, Category: category, PromptTokens: promptTokens}, completionTokens, duration, tps)
	})
}

// modelSwappedMidStreamMsg replaces a confusing low-level transport error
// (typically "connection refused") with a clear explanation when a request
// fails specifically because the client/provider it started with was
// swapped out from under it — see clientSwapped/providerSwapped.
func (a *App) modelSwappedMidStreamMsg() string {
	return a.t("⚠️ Model veya sağlayıcı bu mesaj akarken değiştirildi, bu yüzden tamamlanamadı. Lütfen tekrar deneyin.", "⚠️ The model or provider changed while this message was streaming, so it could not finish. Please try again.")
}

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
	return fmt.Sprintf(a.t("💡 Yerel modeliniz (%s) çalışıyor. API sağlayıcı yerine yerel modeli kullanmak için /model yazıp Local'i seçin.", "💡 Your local model (%s) is running. To use the local model instead of an API provider, type /model and select Local."), modelName)
}
