package app

import (
	"context"
	"encoding/json"
	"fmt"

	"memo/internal/agent"
	"memo/internal/livemode"
	"memo/internal/livemode/google"
	"memo/internal/livemode/openai_realtime"
	"memo/internal/memory"
)

// NewLiveModeSession builds the livemode.Session for whatever engine is
// currently active — the single place that decides between a real
// google.Client/openai_realtime.Client and the livemode.EchoSession
// fallback, and (Phase 10) wires each real client's delegate_to_main_model/
// standalone tool list, tool-call handling, and session-start system
// prompt. See docs/plans/PLAN_live_mode_v2.md §4/§4b/§5.1.
//
// Falls back to EchoSession whenever a real session can't be built —
// unconfigured/misconfigured native engine, or "local"/"elevenlabs"/
// "custom" (which never reach this at all in practice, per the locked-in
// design: their voice loop is the discrete SynthesizeSpeech/TranscribeAudio
// path, not this WS session) — so a session always opens rather than
// failing outright, the same contract handleLiveModeSession's callers
// (Phase 6-8) already expect.
func (a *App) NewLiveModeSession(ctx context.Context) livemode.Session {
	cfg := a.GetLiveModeConfig()
	engineType := livemode.EngineType(cfg.ActiveEngine)
	if engineType != livemode.EngineGoogleLive && engineType != livemode.EngineOpenAIRealtime {
		return livemode.NewEchoSession()
	}

	engineCfg, ok := a.findLiveModeEngineConfig(engineType)
	if !ok || engineCfg.APIKey == "" || engineCfg.Model == "" {
		return livemode.NewEchoSession()
	}

	sessionID, err := a.getOrCreateLiveModeChat()
	if err != nil {
		return livemode.NewEchoSession()
	}

	tools := a.buildLiveModeToolList(cfg.WorkMode)
	handler := a.buildLiveModeToolCallHandler(cfg.WorkMode, sessionID)
	systemPrompt := a.buildLiveModeSystemPrompt(ctx, cfg.WorkMode)

	switch engineType {
	case livemode.EngineGoogleLive:
		return google.NewClient(engineCfg.APIKey, engineCfg.Model, systemPrompt, tools, handler)
	case livemode.EngineOpenAIRealtime:
		return openai_realtime.NewClient(engineCfg.APIKey, engineCfg.Model, systemPrompt, tools, handler)
	default:
		return livemode.NewEchoSession()
	}
}

func (a *App) findLiveModeEngineConfig(t livemode.EngineType) (livemode.EngineConfig, bool) {
	a.liveModeMu.RLock()
	cfgMgr := a.liveModeEngineCfgMgr
	a.liveModeMu.RUnlock()
	if cfgMgr == nil {
		return livemode.EngineConfig{}, false
	}
	return cfgMgr.Get(t)
}

// buildLiveModeToolList returns the tool declarations a live session
// advertises: WorkMode "standalone" gets the same full tool registry
// agent mode's own ChatCompletion loop uses (translated from
// agent.ToolRegistry.ToOpenAITools' provider.ToolDefinition shape into
// livemode.ToolSpec — both are already name/description/JSON-schema, no
// real translation needed); every other WorkMode (including the
// unconfigured/empty default, which config.DefaultConfig seeds as
// "delegate") gets exactly one: delegate_to_main_model.
func (a *App) buildLiveModeToolList(workMode string) []livemode.ToolSpec {
	if workMode != "standalone" {
		return []livemode.ToolSpec{livemode.DelegateToolSpec()}
	}
	if a.agentExecutor == nil {
		return nil
	}
	defs := a.agentExecutor.Registry().ToOpenAITools()
	specs := make([]livemode.ToolSpec, 0, len(defs))
	for _, d := range defs {
		specs = append(specs, livemode.ToolSpec{Name: d.Function.Name, Description: d.Function.Description, Parameters: d.Function.Parameters})
	}
	return specs
}

// buildLiveModeToolCallHandler returns the livemode.ToolCallHandler a
// client's runToolCall invokes for every tool call the live model itself
// decides to make — routing to SendLiveDelegatedMessageStream/
// drainLiveDelegatedReply for "delegate", or straight to
// agent.Executor.ExecuteToolCall for "standalone". sessionID (the Live
// Mode background chat) is used for standalone's audit-log entries, the
// same sessionID RunStream would pass for a normal agent-mode turn.
func (a *App) buildLiveModeToolCallHandler(workMode, sessionID string) livemode.ToolCallHandler {
	autoApprove := a.GetLiveModeConfig().AgentPermissionPolicy == "auto_allow_once"

	if workMode == "standalone" {
		return func(ctx context.Context, name string, args json.RawMessage) (string, error) {
			if a.agentExecutor == nil {
				return "", fmt.Errorf("agent executor not initialized")
			}
			onEvent := func(ev agent.AgentEvent) {
				// autoApprove mirrors "auto_allow_once": resolve the
				// request synchronously, in the same callback ExecuteToolCall
				// invokes it from, rather than touching the shared
				// a.agentExecutor.bypassPermissions/autoPermission fields
				// (see SendLiveDelegatedMessageStream's doc comment for why
				// that shared state must never be touched here).
				if ev.Type == agent.EventPermissionRequest && autoApprove {
					_ = a.HandleAgentPermission(ev.RequestID, string(agent.AllowOnce))
				}
			}
			return a.agentExecutor.ExecuteToolCall(ctx, sessionID, name, args, onEvent)
		}
	}

	return func(ctx context.Context, name string, args json.RawMessage) (string, error) {
		if name != livemode.DelegateToolName {
			return "", fmt.Errorf("unknown tool: %s", name)
		}
		var parsed struct {
			Instruction string `json:"instruction"`
		}
		if err := json.Unmarshal(args, &parsed); err != nil || parsed.Instruction == "" {
			return "", fmt.Errorf("delegate_to_main_model: missing instruction")
		}
		ch := a.SendLiveDelegatedMessageStream(ctx, parsed.Instruction)
		buildQuestion, sendQuestion, awaitAnswer := a.liveModeVoicePermissionCallbacks()
		return a.drainLiveDelegatedReply(ch, autoApprove, buildQuestion, sendQuestion, awaitAnswer), nil
	}
}

// liveModeVoicePermissionCallbacks are the buildQuestion/sendQuestion/
// awaitAnswer callbacks drainLiveDelegatedReply uses when
// AgentPermissionPolicy is "voice_prompt" (the default) rather than
// "auto_allow_once". The real "ask out loud through the open realtime
// session, transcribe the spoken answer" implementation is Phase 12 —
// until then, sendQuestion fails on purpose, which drainSelfChatReply
// (what drainLiveDelegatedReply wraps) already treats as "deny safely, do
// not hang waiting for an answer that has no way to arrive" — exactly the
// same fail-safe behavior TestDrainSelfChatReply_SendQuestionFailure
// documents.
func (a *App) liveModeVoicePermissionCallbacks() (
	buildQuestion func(ev agent.AgentEvent) string,
	sendQuestion func(text string) error,
	awaitAnswer func(ctx context.Context) (string, bool),
) {
	buildQuestion = func(ev agent.AgentEvent) string {
		return a.t(
			fmt.Sprintf("İzin gerekiyor: %q aracını çalıştırmak istiyorum. Onaylıyor musun?", ev.ToolName),
			fmt.Sprintf("Permission needed: I'd like to run the %q tool. Do you approve?", ev.ToolName),
		)
	}
	sendQuestion = func(string) error {
		return fmt.Errorf("voice-based permission prompting is not implemented yet")
	}
	awaitAnswer = func(context.Context) (string, bool) { return "", false }
	return
}

// buildLiveModeSystemPrompt builds a native realtime session's one-time,
// session-start system instruction — see docs/plans/PLAN_live_mode_v2.md
// §5.1. Reuses identity.BuildSystemPrompt (the same persona/mood/memory
// logic every normal chat turn's system prompt goes through) rather than
// duplicating it, called with agentEnabled=false/webSearchEnabled=false so
// its own tool-capability text (written for the ana model's direct
// tool-calling shape) is suppressed, then appends a live-mode-specific
// capability paragraph describing this session's *actual* tools honestly:
// standalone has full tool access, delegate has exactly one
// (delegate_to_main_model). Memory retrieval uses a broad, recency-biased
// query rather than a specific user message — there isn't one yet at
// session start, the same gap a fresh chat's very first turn has to
// tolerate.
func (a *App) buildLiveModeSystemPrompt(ctx context.Context, workMode string) string {
	if a.identity == nil {
		return ""
	}
	var memories []memory.MemoryResult
	if a.GetMemoryEnabled() {
		memories = a.retrieveMemory(ctx, a.t("güncel bağlam", "current context"))
	}
	base := a.identity.BuildSystemPrompt(memories, false, false, false, a.whatsappReachable(), a.telegramReachable())

	var capability string
	if workMode == "standalone" {
		capability = a.t(
			"Sesli canlı sohbet modundasın. Elindeki araçları (dosya/komut erişimi dahil) doğrudan kendin kullanabilirsin.",
			"You are in live voice mode. You can use your available tools (including file/command access) directly, yourself.",
		)
	} else {
		capability = a.t(
			"Sesli canlı sohbet modundasın. Kod yazma veya dosya/komut gerektiren gerçek işleri kendin yapamazsın — kullanıcı senden böyle bir şey istediğinde delegate_to_main_model aracını kullanarak ana modele devret, sonucu doğal bir şekilde kullanıcıya anlat.",
			"You are in live voice mode. You cannot do real work yourself (coding, file/command access) — when the user asks for that, use the delegate_to_main_model tool to hand it off to the main model, then narrate the result back naturally.",
		)
	}
	return base + "\n\n" + capability
}
