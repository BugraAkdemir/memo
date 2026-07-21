// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"memo/internal/agent/tools"
	"memo/internal/api"
	"memo/internal/calendar"
	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/routine"
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
	a.routineLoop = routine.NewRoutineLoop(st, a.runRoutineGenerate, a.runRoutineDeliver, func(name, data string) {
		a.emitEvent(name, data)
	})
	go a.routineLoop.Start(ctx)
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
		result := a.callLLM(ctx, msgs)
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
	case routine.ContextCalendar, routine.ContextWhatsApp, routine.ContextInsight:
	default:
		contextType = routine.ContextNone
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
			Type:        contextType,
			WhatsAppJID: whatsAppTargetJID,
		},
		DeliveryWhatsApp:  d.DeliveryWhatsApp,
		DeliveryMobile:    d.DeliveryMobile,
		WhatsAppTargetJID: whatsAppTargetJID,
		Language:          language,
		Enabled:           true,
	}
	return a.routineStore.Create(r)
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

// RoutineMobilePayload is what the mobile app polls for to pre-schedule a
// GetRoutinesReadyForMobile returns mobile-delivered routines whose content
// was generated after sinceUnix (seconds), for the phone's poll-driven
// refresh — mirrors the calendar-reminder "poll, refetch, reschedule
// locally" pattern already established in mobile/lib. routine.MobilePayload
// lives in internal/routine (not here) so internal/webserver can reference
// it without importing internal/app.
func (a *App) GetRoutinesReadyForMobile(sinceUnix int64) ([]routine.MobilePayload, error) {
	if a.routineStore == nil {
		return nil, nil
	}
	since := time.Unix(sinceUnix, 0)
	now := time.Now()
	out := make([]routine.MobilePayload, 0)
	for _, r := range a.routineStore.List() {
		if !r.Enabled || !r.DeliveryMobile || r.LastGeneratedContent == "" {
			continue
		}
		if !r.LastGeneratedAt.After(since) {
			continue
		}
		fireTime, err := routine.ParseFireTime(r.Schedule.TimeOfDay, r.Schedule.UTCOffsetMinutes, now)
		if err != nil || fireTime.Before(now) {
			continue
		}
		out = append(out, routine.MobilePayload{
			ID:        r.ID,
			Title:     routineNotificationTitle(r.Language),
			Body:      r.LastGeneratedContent,
			FireAtUTC: fireTime.UTC(),
		})
	}
	return out, nil
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

// routineNotificationTitle is the mobile push-notification title for a
// routine's generated content (BUG-M1) — kept in sync with the
// `routine_fallback` L10n key both clients already carry
// (frontend/lib/core/l10n.dart, mobile/lib/core/l10n.dart).
func routineNotificationTitle(lang string) string {
	if routineLanguageIsEnglish(lang) {
		return "Routine"
	}
	return "Rutin"
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
	reply := a.callLLM(ctx, msgs)
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
	var out strings.Builder
	for chunk := range ch {
		out.WriteString(chunk.Content)
		if chunk.Error != "" {
			return "", fmt.Errorf("routine: agent stream: %s", chunk.Error)
		}
	}
	return out.String(), nil
}

// runRoutineDeliver is the routine.DeliverFn wired into the RoutineLoop —
// mobile delivery has no explicit step here (see RoutineMobilePayload's doc
// comment), only WhatsApp send.
func (a *App) runRoutineDeliver(ctx context.Context, r routine.Routine, content string) error {
	if !r.DeliveryWhatsApp {
		return nil
	}
	if r.WhatsAppTargetJID == "" {
		return fmt.Errorf("routine: no whatsapp target configured")
	}
	_, err := a.WhatsAppSend(ctx, r.WhatsAppTargetJID, content)
	return err
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
