package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"memo/internal/agent"
	"memo/internal/livemode"
	"memo/internal/livemode/google"
	"memo/internal/livemode/openai_realtime"
	"memo/internal/logx"
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
		logx.Printf("livemode: active engine %q is not a native realtime engine, using EchoSession", cfg.ActiveEngine)
		return livemode.NewEchoSession()
	}

	engineCfg, ok := a.findLiveModeEngineConfig(engineType)
	if !ok || engineCfg.APIKey == "" || engineCfg.Model == "" {
		logx.Printf("livemode: engine %q not configured (found=%v, apiKey set=%v, model set=%v), using EchoSession", engineType, ok, engineCfg.APIKey != "", engineCfg.Model != "")
		return livemode.NewEchoSession()
	}

	sessionID, err := a.getOrCreateLiveModeChat()
	if err != nil {
		logx.Printf("livemode: getOrCreateLiveModeChat failed: %v, using EchoSession", err)
		return livemode.NewEchoSession()
	}

	// injectFn is set below, once the real client exists. The tool-call
	// handler (built now, since NewClient needs it) must be able to call
	// InjectContext for voice-based permission questions, but the client
	// that owns InjectContext doesn't exist yet — a chicken-and-egg cycle.
	// This forward-reference closure breaks it safely: injectContext is
	// only ever invoked from a tool call the live model itself makes, which
	// can't happen until well after Start() returns and injectFn is set
	// below.
	var injectFn func(string) error
	injectContext := func(text string) error {
		if injectFn == nil {
			return fmt.Errorf("live mode: session not ready yet")
		}
		return injectFn(text)
	}

	tools := a.buildLiveModeToolList(cfg.WorkMode)
	handler := a.buildLiveModeToolCallHandler(cfg.WorkMode, sessionID, injectContext)
	systemPrompt := a.buildLiveModeSystemPrompt(ctx, cfg.WorkMode)

	var session livemode.Session
	switch engineType {
	case livemode.EngineGoogleLive:
		client := google.NewClient(engineCfg.APIKey, engineCfg.Model, systemPrompt, tools, handler, engineCfg.Voice)
		injectFn = client.InjectContext
		session = client
	case livemode.EngineOpenAIRealtime:
		client := openai_realtime.NewClient(engineCfg.APIKey, engineCfg.Model, systemPrompt, tools, handler, engineCfg.Voice)
		injectFn = client.InjectContext
		session = client
	default:
		return livemode.NewEchoSession()
	}
	logx.Printf("livemode: built a real %s session (model=%q, workMode=%q, tools=%d)", engineType, engineCfg.Model, cfg.WorkMode, len(tools))

	// Wraps the real session so a spoken transcript can also resolve a
	// pending voice_prompt permission question, in addition to reaching the
	// Flutter client for normal display — see livemode_session_wrapper.go.
	return a.wrapLiveModeSessionForPermissionRouting(session)
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
// injectContext is the (possibly not-yet-ready, see NewLiveModeSession's
// forward-reference closure) session InjectContext used to ask a
// voice_prompt permission question out loud.
func (a *App) buildLiveModeToolCallHandler(workMode, sessionID string, injectContext func(string) error) livemode.ToolCallHandler {
	autoApprove := a.GetLiveModeConfig().AgentPermissionPolicy == "auto_allow_once"
	buildQuestion, sendQuestion, awaitAnswer := a.liveModeVoicePermissionCallbacks(injectContext)

	if workMode == "standalone" {
		return func(ctx context.Context, name string, args json.RawMessage) (string, error) {
			if a.agentExecutor == nil {
				return "", fmt.Errorf("agent executor not initialized")
			}
			onEvent := func(ev agent.AgentEvent) {
				if ev.Type != agent.EventPermissionRequest {
					return
				}
				// Must run in its own goroutine, never resolved inline
				// here: ExecuteToolCall calls onEvent synchronously,
				// strictly before it registers this request in
				// e.pendingPerms (its very next statement) — resolving on
				// this same goroutine would either lose an immediate
				// autoApprove answer to that ordering, or (for
				// voice_prompt, which blocks for seconds asking and
				// awaiting a spoken answer) deadlock the registration
				// entirely. See resolveLivePermission's doc comment.
				go a.resolveLivePermission(ev, autoApprove, buildQuestion, sendQuestion, awaitAnswer)
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
		// Reported live: without this, the model kept trying to generate
		// more speech while the delegated task was still running (a real
		// task can take real seconds) — it has no natural sense of "I'm
		// waiting on something", producing choppy/repeatedly-interrupted-
		// sounding audio. Tell it explicitly, mirroring the filler-audio
		// trick VoiceModeNotifier already uses for the discrete engines
		// (_playFillerBestEffort, voice_mode_provider.dart) but as a text
		// instruction rather than a pre-recorded clip, since this model
		// generates its own voice rather than playing ours. Best-effort:
		// injectContext can still be unready this early (see
		// NewLiveModeSession's forward-reference closure) without blocking
		// delegation itself.
		if injectContext != nil {
			_ = injectContext(a.t(
				"Az önce delegate_to_main_model'i çağırdın, sonucu bekleniyor. Şimdi TEK kısa bir şey söyle (\"bir saniye, hallediyorum\" gibi) ve gerçek sonuç gelene kadar tekrar konuşmaya çalışma — sessizce bekle.",
				"You just called delegate_to_main_model and the result is pending. Say ONE short acknowledgment now (\"one sec, working on it\") and then stay quiet — don't try to speak again until the real result comes back.",
			))
		}
		ch := a.SendLiveDelegatedMessageStream(ctx, parsed.Instruction)
		return a.drainLiveDelegatedReply(ch, autoApprove, buildQuestion, sendQuestion, awaitAnswer), nil
	}
}

// resolveLivePermission resolves one EventPermissionRequest — shared by
// standalone mode's onEvent (see buildLiveModeToolCallHandler; always
// called in its own goroutine there, for the ordering reason documented at
// that call site). autoApprove mirrors "auto_allow_once": resolved directly
// via HandleAgentPermission rather than touching the shared
// a.agentExecutor.bypassPermissions/autoPermission fields (see
// SendLiveDelegatedMessageStream's doc comment for why that shared state
// must never be touched here). The short retry loop closes the residual
// race between "this goroutine got scheduled" and "ExecuteToolCall's own
// goroutine finished registering the request" — the same shape of race the
// WhatsApp/Telegram self-chat path already tolerates via its SSE-channel
// handoff (selfchat_permission.go), made explicit here since standalone
// mode has no channel handoff to lean on for the same natural delay.
// Otherwise (voice_prompt, the default), mirrors resolveSelfChatPermission
// exactly: ask out loud via sendQuestion, wait up to 45s for a transcribed
// answer, resolve Allow/Deny by isAffirmativeAnswer.
func (a *App) resolveLivePermission(
	ev agent.AgentEvent,
	autoApprove bool,
	buildQuestion func(ev agent.AgentEvent) string,
	sendQuestion func(text string) error,
	awaitAnswer func(ctx context.Context) (string, bool),
) {
	if autoApprove {
		for i := 0; i < 20; i++ {
			if err := a.HandleAgentPermission(ev.RequestID, string(agent.AllowOnce)); err == nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		return
	}

	if err := sendQuestion(buildQuestion(ev)); err != nil {
		logx.Printf("live mode permission: send question error: %v", err)
		_ = a.HandleAgentPermission(ev.RequestID, string(agent.DenyOnce))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	answer, ok := awaitAnswer(ctx)
	if !ok {
		return
	}

	policy := agent.DenyOnce
	if isAffirmativeAnswer(answer) {
		policy = agent.AllowOnce
	}
	_ = a.HandleAgentPermission(ev.RequestID, string(policy))
}

// liveModeVoicePermissionCallbacks are the buildQuestion/sendQuestion/
// awaitAnswer callbacks drainLiveDelegatedReply/resolveLivePermission use
// when AgentPermissionPolicy is "voice_prompt" (the default) rather than
// "auto_allow_once": ask out loud by injecting the question as a text aside
// into the open realtime session (injectContext — see NewLiveModeSession's
// forward-reference closure, which is what makes this safe to call even
// though the client that ultimately serves it doesn't exist yet at the
// point this function is called), then block on awaitLivePermissionAnswer
// for whatever transcript arrives next (routed there by
// routeLiveTranscriptToPermissionAnswer — see livemode_session_wrapper.go).
// If injectContext is still unready (session genuinely never started, or
// this is called before Start() completes), sendQuestion fails, which
// drainSelfChatReply/resolveLivePermission already treat as "deny safely,
// do not hang waiting for an answer that has no way to arrive" — the same
// fail-safe behavior TestDrainSelfChatReply_SendQuestionFailure documents.
func (a *App) liveModeVoicePermissionCallbacks(injectContext func(text string) error) (
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
	sendQuestion = func(text string) error {
		if injectContext == nil {
			return fmt.Errorf("live mode: no active session to ask permission through")
		}
		return injectContext(text)
	}
	awaitAnswer = a.awaitLivePermissionAnswer
	return
}

// awaitLivePermissionAnswer blocks until routeLiveTranscriptToPermissionAnswer
// delivers the next transcript, or ctx is done. See livePermMu/
// livePendingPermAnswerCh's doc comment (app.go) and
// awaitWhatsAppPermissionAnswer, which this mirrors minus the chatJID match
// (Live Mode only ever has one active session at a time).
func (a *App) awaitLivePermissionAnswer(ctx context.Context) (string, bool) {
	answerCh := make(chan string, 1)
	a.livePermMu.Lock()
	a.livePendingPermAnswerCh = answerCh
	a.livePermMu.Unlock()
	defer func() {
		a.livePermMu.Lock()
		if a.livePendingPermAnswerCh == answerCh {
			a.livePendingPermAnswerCh = nil
		}
		a.livePermMu.Unlock()
	}()

	select {
	case ans := <-answerCh:
		return ans, true
	case <-ctx.Done():
		return "", false
	}
}

// routeLiveTranscriptToPermissionAnswer delivers text to an outstanding
// voice_prompt permission question, if one is currently pending — called
// from the session wrapper's event pump for every EventTranscript (see
// livemode_session_wrapper.go). Reports whether a question was pending, for
// symmetry with routeWhatsAppPermissionAnswer's contract; the wrapper
// forwards the transcript event to the outward channel either way; the
// caller's boolean return isn't currently used to suppress display.
func (a *App) routeLiveTranscriptToPermissionAnswer(text string) bool {
	a.livePermMu.Lock()
	answerCh := a.livePendingPermAnswerCh
	a.livePermMu.Unlock()
	if answerCh == nil {
		return false
	}
	select {
	case answerCh <- strings.TrimSpace(text):
	default:
	}
	return true
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
		// Broadened after real-world testing showed the live model doing
		// nothing — not even recalling something from the user's memory —
		// when the original wording only mentioned coding/file/command
		// access as delegation triggers. The memory context above is a
		// one-time snapshot from session start (a generic, non-specific
		// pull — there is no per-turn memory refresh in delegate mode, see
		// docs/plans/PLAN_live_mode_v2.md's Phase 11 note on the deferred
		// mid-session refresh consumer), so anything the user asks that
		// needs real recall — not just "real work" in the coding sense —
		// genuinely requires delegation; the live model has no other way
		// to reach it.
		capability = a.t(
			"Sesli canlı sohbet modundasın. Yukarıdaki bağlam sadece oturum başında alınmış tek seferlik bir hafıza özeti — kullanıcı sana özel bir şey (geçmişte konuştuğunuz bir konu, bir tercih, bir hatırlatma, bir dosya/kod, bir komut) sorduğunda ve yukarıdaki bağlamda gerçek cevabı yoksa, kafandan uydurma — delegate_to_main_model aracını kullanarak ana modele devret (ana model gerçek hafıza aramasını kendisi yapar), sonucu doğal bir şekilde kullanıcıya anlat. Sadece sohbet/görüş sorularında (hava nasıl, nasılsın gibi) kendi başına cevap ver. ÇOK ÖNEMLİ: bir işi 'yaptım', 'hallettim', 'tamamladım' gibi ifadelerle asla söyleme — bunu ancak delegate_to_main_model aracını GERÇEKTEN çağırıp gerçek bir sonuç aldıktan SONRA söyleyebilirsin. Aracı çağırmadan başarı iddia etmek bir yalandır, asla yapma.",
			"You are in live voice mode. The context above is only a one-time memory snapshot taken at session start — when the user asks about something specific (something you discussed before, a preference, a reminder, a file/code, a command) and the answer isn't actually in that context, don't make it up — use the delegate_to_main_model tool to hand it off to the main model (which does a real memory search itself), then narrate the result back naturally. Only answer directly for genuinely casual/conversational questions (how's the weather, how are you, etc.). CRITICAL: never say you 'did it', 'took care of it', or 'finished' unless you actually called delegate_to_main_model and got a real result back first. Claiming success without actually calling the tool is a lie — never do it.",
		)
	}
	return base + "\n\n" + capability
}
