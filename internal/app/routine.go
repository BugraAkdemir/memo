// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"memo/internal/agent"
	"memo/internal/agent/tools"
	"memo/internal/api"
	"memo/internal/calendar"
	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/routine"
	"memo/internal/websearch"
	"memo/internal/whatsapp"
)

// initRoutines sets up the scheduled-automation ("routine") system: a JSON
// store plus a one-minute ticker loop that generates and delivers due
// routines. Mirrors initLearning's calendar wiring immediately above it.
func (a *App) initRoutines(ctx context.Context) {
	dir := config.DataPath("routines")
	st, err := routine.NewStore(dir)
	if err != nil {
		logx.Printf("WARN: routine store: %v", err)
		return
	}
	a.routineStore = st
	a.routineLoop = routine.NewRoutineLoop(st, a.runRoutineGenerate, a.runRoutineDeliver)
	loop := a.routineLoop
	goRecover("routineLoop.Start", func() { loop.Start(ctx) })
	logx.Info("Routine system initialized")
}

// buildRoutineDecider returns the Decider used to parse free-text routine
// descriptions, reusing the same a.callLLM path buildLearningDecider uses for
// intent extraction (never call providerRouter directly — see AGENTS.md's
// documented anti-pattern history).
func (a *App) buildRoutineDecider() routine.Decider {
	return func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
		msgs := []api.Message{
			api.NewTextMessage("system", systemPrompt),
			api.NewTextMessage("user", userPrompt),
		}
		result := a.callLLM(ctx, msgs, categoryRoutine)
		if isLLMErrorReply(result) {
			return "", fmt.Errorf("routine: llm call failed: %s", result)
		}
		return result, nil
	}
}

// ListRoutines returns all configured routines.
func (a *App) ListRoutines() []routine.Routine {
	if a.routineStore == nil {
		return nil
	}
	return a.routineStore.List()
}

// ParseRoutineText turns a free-text routine description into a Draft,
// resolving any WhatsApp chat mention to an actual JID — the one piece of
// resolution the routine package itself can't do, since it doesn't depend on
// internal/whatsapp.
func (a *App) ParseRoutineText(ctx context.Context, text string) (routine.Draft, error) {
	extractor := routine.NewExtractor(a.buildRoutineDecider())
	draft, err := extractor.Extract(ctx, text, time.Now())
	if err != nil {
		return routine.Draft{}, err
	}
	return draft, nil
}

// CreateRoutineFromDraft builds and persists a Routine from a (possibly
// user-edited) Draft plus the explicit choices only a human should make:
// whichever WhatsApp JID the hint resolved to, whether unattended tool
// execution is allowed, the client's current UI language ("tr"/"en", see
// Routine.Language's doc comment), and its current UTC offset in minutes
// (see Schedule.UTCOffsetMinutes's doc comment, BUG-M4) — the backend has no
// locale or timezone of its own and this is the only point in the routine
// lifecycle where a client is actually asking to have something created, so
// it's the only place that needs to capture either.
//
// Telegram's target is NOT a parameter here, unlike WhatsApp's: a Telegram
// bot only ever has one legitimate delivery target (its linked owner, see
// Routine.TelegramTargetChatID's doc comment) — resolved internally via
// linkedTelegramOwnerChatID whenever d.DeliveryTelegram is set, so callers
// (both the human-driven Routines-tab flow and CreateRoutineFromChat) never
// need to look it up themselves.
func (a *App) CreateRoutineFromDraft(originalText string, d routine.Draft, whatsAppTargetJID string, autoApproveTools bool, language string, utcOffsetMinutes *int) (*routine.Routine, error) {
	if a.routineStore == nil {
		return nil, fmt.Errorf("routine: store not initialized")
	}

	weekdays := make([]time.Weekday, 0, len(d.Weekdays))
	for _, w := range d.Weekdays {
		weekdays = append(weekdays, time.Weekday(w))
	}

	contextType := routine.ContextSourceType(d.ContextSourceType)
	switch contextType {
	case routine.ContextCalendar, routine.ContextWhatsApp, routine.ContextInsight, routine.ContextWebSearch:
	default:
		contextType = routine.ContextNone
	}

	telegramTargetChatID := int64(0)
	deliveryTelegram := false
	if d.DeliveryTelegram {
		telegramTargetChatID = a.linkedTelegramOwnerChatID()
		deliveryTelegram = telegramTargetChatID != 0
	}

	r := routine.Routine{
		CreatedFromText: originalText,
		Schedule: routine.Schedule{
			TimeOfDay:        d.TimeOfDay,
			Weekdays:         weekdays,
			UTCOffsetMinutes: utcOffsetMinutes,
		},
		Prompt:           d.Prompt,
		AgentMode:        d.NeedsAgentMode,
		AutoApproveTools: d.NeedsAgentMode && autoApproveTools,
		ContextSource: routine.ContextSource{
			Type:           contextType,
			WhatsAppJID:    whatsAppTargetJID,
			WebSearchQuery: d.WebSearchQuery,
		},
		DeliveryWhatsApp:     d.DeliveryWhatsApp,
		WhatsAppTargetJID:    whatsAppTargetJID,
		DeliveryTelegram:     deliveryTelegram,
		TelegramTargetChatID: telegramTargetChatID,
		Language:             language,
		Enabled:              true,
	}
	return a.routineStore.Create(r)
}

// CreateRoutineFromChat is the create_routine agent tool's backing
// implementation (internal/agent/tools/routine.go) — the conversational
// counterpart to CreateRoutineFromDraft, reachable from any agent-enabled
// chat (normal chat, WhatsApp self-chat, Telegram). text is the routine's
// free-text description; the delivery target is deliberately NEVER taken
// from the model — see selfChatSourceFromContext's doc comment for why:
//
//   - Called from WhatsApp/Telegram self-chat: delivery is forced to that
//     exact surface/chat, ignoring whatever the extractor guessed from text.
//   - Called from normal chat (no self-chat source on ctx): delivery
//     defaults to whichever self-chat surfaces are actually connected right
//     now (WhatsApp if logged in, Telegram if a bot is linked) — "smart"
//     defaulting instead of asking the model to pick a target, since it has
//     no legitimate target to pick from in the first place.
//
// AgentMode is whatever the extractor itself inferred from text (e.g. "her
// gün saat 14'te sistemimin durumunu kontrol et" needs real command
// execution, "AI haberlerini getir" doesn't) — no longer forced off.
//
// AutoApproveTools is unconditionally true (only takes effect when
// AgentMode is also true, per CreateRoutineFromDraft's own gating).
// Deliberately NOT tied to the live per-surface /auto-perm setting, despite
// that being the first, more "obviously safe" design tried here — direct
// user feedback after using it: a routine's agent/tool access being at the
// mercy of a live toggle that can silently reset (a fresh self-chat
// session, a forgotten manual flip) is worse than just trusting the
// routine, since the human already reviewed and approved *what the routine
// does* at the moment they asked for it to be created — that's the actual
// approval, not a separate live flag that has to happen to still be on
// whenever the scheduler fires it later. runAgentRoutine's live y/n
// permission-asking machinery (routinePermissionCallbacks) is kept, not
// deleted — it still fires for agent-mode routines created via the
// Routines tab's own UI, which has its own independent, per-routine
// AutoApproveTools toggle (routines_auto_approve) that a human may leave
// off on purpose.
func (a *App) CreateRoutineFromChat(ctx context.Context, text string) (string, error) {
	if a.routineStore == nil {
		return "", fmt.Errorf("rutin sistemi hazır değil")
	}

	draft, err := a.ParseRoutineText(ctx, text)
	if err != nil {
		return "", fmt.Errorf("rutin ayrıştırılamadı: %w", err)
	}

	whatsAppTargetJID, deliveryWhatsApp, deliveryTelegram := a.resolveRoutineDeliveryTarget(ctx)
	draft.DeliveryWhatsApp = deliveryWhatsApp
	draft.DeliveryTelegram = deliveryTelegram

	const autoApproveTools = true

	lang := a.GetUILanguage()
	r, err := a.CreateRoutineFromDraft(text, draft, whatsAppTargetJID, autoApproveTools, lang, nil)
	if err != nil {
		return "", err
	}
	return summarizeCreatedRoutine(r), nil
}

// resolveRoutineDeliveryTarget decides which channel(s)
// CreateRoutineFromChat should enable and, for WhatsApp, which JID — this
// is the actual security boundary described in CreateRoutineFromChat's doc
// comment, split into its own function so it's testable independent of the
// LLM-dependent draft parsing around it. Telegram never needs a returned
// target value: CreateRoutineFromDraft resolves the bot's linked owner
// chat ID itself whenever DeliveryTelegram is set (see its own doc
// comment), since — unlike WhatsApp — there is only ever one legitimate
// value to resolve to.
func (a *App) resolveRoutineDeliveryTarget(ctx context.Context) (whatsAppTargetJID string, deliveryWhatsApp, deliveryTelegram bool) {
	if src, ok := selfChatSourceFromContext(ctx); ok {
		if src.WhatsApp {
			return src.WhatsAppJID, true, false
		}
		if src.Telegram {
			return "", false, true
		}
	}
	whatsAppTargetJID = a.connectedWhatsAppSelfChatJID()
	return whatsAppTargetJID, whatsAppTargetJID != "", a.linkedTelegramOwnerChatID() != 0
}

// connectedWhatsAppSelfChatJID returns the account's own self-chat JID if
// WhatsApp is currently connected and logged in, or "" otherwise.
func (a *App) connectedWhatsAppSelfChatJID() string {
	if a.waClient == nil || !a.waClient.IsConnected() || !a.waClient.IsLoggedIn() {
		return ""
	}
	own := a.waClient.OwnJIDs()
	if len(own) == 0 {
		return ""
	}
	return own[0]
}

// linkedTelegramOwnerChatID returns the Telegram bot's linked owner chat ID
// if one exists, or 0 otherwise.
func (a *App) linkedTelegramOwnerChatID() int64 {
	if a.tgStore == nil {
		return 0
	}
	st := a.tgStore.Get()
	if !st.Linked() {
		return 0
	}
	return st.OwnerChatID
}

// summarizeCreatedRoutine is the create_routine tool's return value — a
// short, model-readable confirmation (not user-facing chat text by itself;
// the LLM turns this into its own reply) of what actually got created,
// since the tool's caller only supplies free text and can't otherwise see
// how it was interpreted (time/weekdays/delivery channel).
func summarizeCreatedRoutine(r *routine.Routine) string {
	days := "her gün"
	if len(r.Schedule.Weekdays) > 0 {
		names := make([]string, len(r.Schedule.Weekdays))
		for i, w := range r.Schedule.Weekdays {
			names[i] = w.String()
		}
		days = strings.Join(names, ", ")
	}
	channels := make([]string, 0, 2)
	if r.DeliveryWhatsApp {
		channels = append(channels, "WhatsApp")
	}
	if r.DeliveryTelegram {
		channels = append(channels, "Telegram")
	}
	channelStr := "hiçbir kanal bağlı değil, bildirim gidemeyecek"
	if len(channels) > 0 {
		channelStr = strings.Join(channels, " + ")
	}
	return fmt.Sprintf("Rutin oluşturuldu: %s saat %s, %s üzerinden gönderilecek. Prompt: %q",
		days, r.Schedule.TimeOfDay, channelStr, r.Prompt)
}

// routineToolAdapter wraps *App to satisfy tools.Routines (name mismatch
// only — CreateRoutineFromChat vs. the interface's CreateRoutine — same
// adapter-for-naming pattern as waToolAdapter in whatsapp.go).
type routineToolAdapter struct{ a *App }

func (r routineToolAdapter) CreateRoutine(ctx context.Context, text string) (string, error) {
	return r.a.CreateRoutineFromChat(ctx, text)
}

// GetRoutine returns a single routine by ID.
func (a *App) GetRoutine(id string) (*routine.Routine, error) {
	if a.routineStore == nil {
		return nil, fmt.Errorf("routine: store not initialized")
	}
	return a.routineStore.Get(id)
}

// UpdateRoutine persists changes to an existing routine (e.g. toggling Enabled).
func (a *App) UpdateRoutine(r routine.Routine) (*routine.Routine, error) {
	if a.routineStore == nil {
		return nil, fmt.Errorf("routine: store not initialized")
	}
	return a.routineStore.Update(r)
}

// DeleteRoutine removes a routine.
func (a *App) DeleteRoutine(id string) error {
	if a.routineStore == nil {
		return fmt.Errorf("routine: store not initialized")
	}
	return a.routineStore.Delete(id)
}

// SyncRoutineUTCOffsets updates every routine's stored UTC offset to match
// minutes — see routine.Store.SyncUTCOffset's doc comment. A no-op (0, nil)
// if the routine system never initialized, matching every other routine
// method's nil-store handling in this file.
func (a *App) SyncRoutineUTCOffsets(minutes int) (int, error) {
	if a.routineStore == nil {
		return 0, nil
	}
	return a.routineStore.SyncUTCOffset(minutes)
}

// runRoutineGenerate is the routine.GenerateFn wired into the RoutineLoop: it
// picks the execution path based on Routine.AgentMode.
func (a *App) runRoutineGenerate(ctx context.Context, r routine.Routine) (string, error) {
	if r.AgentMode {
		return a.runAgentRoutine(ctx, r)
	}
	return a.runSimplePromptRoutine(ctx, r)
}

// routineLanguageIsEnglish reports whether lang (Routine.Language) selects
// the English variant of the backend's own generated routine text — any
// value other than exactly "en" (including the empty string a routine
// created before this field existed always has) defaults to Turkish,
// matching this codebase's Turkish-first convention (see AGENTS.md's
// "Turkish + English mixed user-facing text is intentional" note) rather
// than requiring a migration for old routines.
func routineLanguageIsEnglish(lang string) bool {
	return lang == "en"
}

// routineSystemPrompt is the system message a non-agent routine run sends
// the LLM ahead of its actual prompt — instructs it to reply like a
// standalone notification rather than mid-conversation. Localized per
// Routine.Language (BUG-M1): a routine created under the English UI used to
// still receive this instruction in Turkish, which could bleed into the
// reply's own language.
func routineSystemPrompt(lang string) string {
	if routineLanguageIsEnglish(lang) {
		return "You are now running a task the user scheduled in advance. " +
			"Give a short, natural reply that reads like a standalone message — not like the middle of a conversation."
	}
	return "Bir kullanıcının önceden tanımladığı, zamanlanmış bir görevi şimdi çalıştırıyorsun. " +
		"Kısa, doğal, mesaj gibi okunan bir cevap ver — bir sohbetin ortasındaymış gibi değil, kendi başına bir bildirim gibi."
}

// runSimplePromptRoutine handles the non-agent path: deterministically
// pre-fetch any requested context (calendar agenda / a specific WhatsApp
// chat's recent messages) in Go, then make a single plain LLM call. No tool
// access at all — safe by construction, and doesn't depend on an unattended
// model correctly deciding to call a tool.
func (a *App) runSimplePromptRoutine(ctx context.Context, r routine.Routine) (string, error) {
	var extraContext string
	switch r.ContextSource.Type {
	case routine.ContextCalendar:
		if a.calendarStore != nil {
			now := time.Now()
			start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			end := start.Add(24 * time.Hour)
			events, err := a.calendarStore.List(ctx, start, end)
			if err == nil {
				extraContext = formatEventsForRoutine(events, r.Language)
			}
		}
	case routine.ContextWhatsApp:
		if r.ContextSource.WhatsAppJID != "" {
			msgs, err := a.WhatsAppGetMessages(r.ContextSource.WhatsAppJID, 50)
			if err == nil {
				extraContext = formatWhatsAppMessagesForRoutine(msgs, r.Language)
			}
		}
	case routine.ContextInsight:
		// Reuses GenerateSelfInsight's own context-building (memory.RecentSince +
		// mood.HistorySince) rather than duplicating it here — the routine's
		// user-authored Prompt becomes the *framing* around that pre-synthesized
		// insight, same "deterministic context, one plain LLM call" shape as the
		// calendar/whatsapp cases above, just with the insight already summarized.
		insight, err := a.GenerateSelfInsight(ctx, 0, r.Language)
		if err == nil {
			extraContext = insight
		} else {
			logx.Printf("routine: GenerateSelfInsight: %v", err)
		}
	case routine.ContextWebSearch:
		query := r.ContextSource.WebSearchQuery
		if query == "" {
			query = r.Prompt
		}
		results, err := websearch.Search(ctx, query, 5)
		if err == nil {
			extraContext = websearch.FormatForContext(query, results)
		} else {
			logx.Printf("routine: web search: %v", err)
		}
	}

	prompt := r.Prompt
	if extraContext != "" {
		contextLabel := "Bağlam:"
		if routineLanguageIsEnglish(r.Language) {
			contextLabel = "Context:"
		}
		prompt += "\n\n" + contextLabel + "\n" + extraContext
	}

	msgs := []api.Message{
		api.NewTextMessage("system", routineSystemPrompt(r.Language)),
		api.NewTextMessage("user", prompt),
	}
	reply := a.callLLM(ctx, msgs, categoryRoutine)
	if isLLMErrorReply(reply) {
		return "", fmt.Errorf("routine: llm call failed: %s", reply)
	}
	return reply, nil
}

// runAgentRoutine handles the agent-mode path (e.g. "git pull and report
// status"). It deliberately does NOT call SwitchChat/SetAgentEnabled — that
// pattern (mutating the app's one global active-chat pointer and agent-mode
// flag around a call, with no restore) is the exact anti-pattern
// SendMessageStreamTo's doc comment documents as already found-and-fixed for
// the task loop (internal/app/tasklist.go): forceAgent=true, passed straight
// to sendMessageStreamCore below, already activates tool execution for this
// one call regardless of the global agentEnabled toggle (see routeStream's
// `if forceAgent { agentActive = true }`), and the call already carries its
// own explicit chatID everywhere it's needed — so neither global ever needs
// to move.
//
// It also acquires a.streamMu itself, before touching AutoApproveTools,
// rather than calling sendMessageStreamInnerTo (which only locks internally,
// after the caller has already set any state it wants scoped to the call).
// That gap — set the flag, then attempt the lock — is exactly the window a
// concurrent interactive stream request could win the lock first and
// silently inherit auto-tool-approval it never asked for. Locking here first
// closes that: the flag is only ever true while this call holds streamMu
// exclusively, and thanks to defer's LIFO order below, the restore always
// runs before the unlock, so no other stream can observe the true value
// either before or after this call.
func (a *App) runAgentRoutine(ctx context.Context, r routine.Routine) (string, error) {
	// sessions.Manager.NewAgentChat sets the newly created chat as globally
	// "active" as a side effect of creating it — independent of, and not
	// fixed by, dropping the explicit SwitchChat call above. Capture and
	// restore immediately so this routine's background chat doesn't hijack
	// the user's actual active chat; sendMessageStreamCore below is keyed off
	// the explicit chatID captured here, not the "active" pointer, so
	// restoring it doesn't affect the routine's own stream at all.
	prevActiveChat := a.GetActiveChatID()
	chatID := a.NewAgentChat("")
	if chatID == "" {
		return "", fmt.Errorf("routine: could not create agent chat")
	}
	if prevActiveChat != "" {
		if err := a.SwitchChat(prevActiveChat); err != nil {
			logx.Printf("routine: restore active chat after creating agent chat: %v", err)
		}
	}

	if !a.streamMu.TryLock() {
		return "", fmt.Errorf("routine: busy, another stream already in progress")
	}
	defer a.streamMu.Unlock()

	if r.AutoApproveTools {
		prevAuto := a.GetAgentAutoPermission()
		if err := a.SetAgentAutoPermission(true); err != nil {
			return "", fmt.Errorf("routine: set auto permission: %w", err)
		}
		defer a.SetAgentAutoPermission(prevAuto)
	}

	ch := a.sendMessageStreamCore(ctx, chatID, r.Prompt, true)

	// If r.AutoApproveTools was true, SetAgentAutoPermission(true) above
	// already made the pipeline bypass every permission check silently —
	// no permission_request event is ever emitted in that case. Any event
	// actually seen in this loop therefore genuinely needs a live answer;
	// autoApprove is always false here on purpose (the "auto" branch is
	// already handled upstream, not by this loop).
	sendQuestion, awaitAnswer := a.routinePermissionCallbacks(ctx, r)
	buildQuestion := routinePermissionQuestion(r.Language, r.DeliveryWhatsApp)

	var out strings.Builder
	for chunk := range ch {
		if chunk.FinishReason == "agent_event" {
			var ev agent.AgentEvent
			if err := json.Unmarshal([]byte(chunk.Content), &ev); err == nil && ev.Type == agent.EventPermissionRequest {
				a.resolveSelfChatPermission(ev, false, buildQuestion, sendQuestion, awaitAnswer)
			}
			continue
		}
		out.WriteString(chunk.Content)
		if chunk.Error != "" {
			return "", fmt.Errorf("routine: agent stream: %s", chunk.Error)
		}
	}
	return out.String(), nil
}

// routinePermissionCallbacks wires resolveSelfChatPermission's
// sendQuestion/awaitAnswer to whichever chat surface this routine actually
// delivers to — the scheduled-routine equivalent of
// handleWhatsAppSelfChatMessage/handleTelegramMessage's own callbacks, and
// deliberately reusing the exact same pending-answer plumbing
// (awaitWhatsAppPermissionAnswer/routeWhatsAppPermissionAnswer and their
// Telegram counterparts): both are keyed purely by chat JID/chat ID, with
// no notion of "this call came from an incoming message specifically", so
// the owner's next WhatsApp/Telegram reply routes here exactly the same way
// it would for a live self-chat permission question. If the routine has no
// live delivery channel to ask through at all, sendQuestion fails
// immediately, which resolveSelfChatPermission already treats as a safe
// default: deny.
func (a *App) routinePermissionCallbacks(ctx context.Context, r routine.Routine) (sendQuestion func(string) error, awaitAnswer func(context.Context) (string, bool)) {
	if r.DeliveryWhatsApp && r.WhatsAppTargetJID != "" {
		return func(q string) error {
				_, err := a.WhatsAppSend(ctx, r.WhatsAppTargetJID, q)
				return err
			}, func(waitCtx context.Context) (string, bool) {
				return a.awaitWhatsAppPermissionAnswer(waitCtx, r.WhatsAppTargetJID)
			}
	}
	if r.DeliveryTelegram && r.TelegramTargetChatID != 0 {
		return func(q string) error { return a.TelegramSend(ctx, r.TelegramTargetChatID, q) },
			func(waitCtx context.Context) (string, bool) {
				return a.awaitTelegramPermissionAnswer(waitCtx, r.TelegramTargetChatID)
			}
	}
	return func(string) error { return fmt.Errorf("routine: no chat surface to ask for permission through") },
		func(context.Context) (string, bool) { return "", false }
}

// routinePermissionQuestion builds resolveSelfChatPermission's y/n prompt
// for a routine-triggered permission request, phrased for whichever channel
// it's actually being asked through — see wa_perm_question/tg_perm_question.
func routinePermissionQuestion(lang string, viaWhatsApp bool) func(ev agent.AgentEvent) string {
	return func(ev agent.AgentEvent) string {
		preview := ev.Preview
		if preview == "" {
			preview = ev.ToolName
		}
		if viaWhatsApp {
			return fmt.Sprintf(waT(waLang(lang), "wa_perm_question"), ev.ToolName, preview)
		}
		return fmt.Sprintf(tgT(tgLang(lang), "tg_perm_question"), ev.ToolName, preview)
	}
}

// runRoutineDeliver is the routine.DeliverFn wired into the RoutineLoop —
// fires whichever of WhatsApp/Telegram are enabled, independently: a
// failure on one channel doesn't skip the other, and both failures (if any)
// are reported together via errors.Join rather than one masking the other.
func (a *App) runRoutineDeliver(ctx context.Context, r routine.Routine, content string) error {
	var errs []error
	if r.DeliveryWhatsApp {
		if r.WhatsAppTargetJID == "" {
			errs = append(errs, fmt.Errorf("routine: no whatsapp target configured"))
		} else if _, err := a.WhatsAppSend(ctx, r.WhatsAppTargetJID, content); err != nil {
			errs = append(errs, fmt.Errorf("whatsapp: %w", err))
		}
	}
	if r.DeliveryTelegram {
		if r.TelegramTargetChatID == 0 {
			errs = append(errs, fmt.Errorf("routine: no telegram target configured"))
		} else if err := a.TelegramSend(ctx, r.TelegramTargetChatID, content); err != nil {
			errs = append(errs, fmt.Errorf("telegram: %w", err))
		}
	}
	return errors.Join(errs...)
}

func formatEventsForRoutine(events []calendar.Event, lang string) string {
	if len(events) == 0 {
		if routineLanguageIsEnglish(lang) {
			return "No events on today's calendar."
		}
		return "Bugün için takvimde etkinlik yok."
	}
	var b strings.Builder
	for _, e := range events {
		b.WriteString("- ")
		b.WriteString(e.Title)
		b.WriteString(" (")
		b.WriteString(e.StartTime.Format("15:04"))
		b.WriteString(")\n")
	}
	return b.String()
}

func formatWhatsAppMessagesForRoutine(msgs []whatsapp.Message, lang string) string {
	if len(msgs) == 0 {
		if routineLanguageIsEnglish(lang) {
			return "No new messages in this chat."
		}
		return "Bu sohbette yeni mesaj yok."
	}
	var b strings.Builder
	for _, m := range msgs {
		from := m.SenderName
		if from == "" {
			// Matches GetWhatsAppMessages' own fallback (internal/agent/tools/
			// whatsapp.go) — a contact with no saved display name used to
			// render as a blank sender here (BUG-L5).
			from = tools.PartsBeforeAt(m.SenderJID)
		}
		b.WriteString("[")
		b.WriteString(m.Timestamp.Format("15:04"))
		b.WriteString("] ")
		b.WriteString(from)
		b.WriteString(": ")
		b.WriteString(m.Text)
		b.WriteString("\n")
	}
	return b.String()
}
