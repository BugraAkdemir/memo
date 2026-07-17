// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"fmt"
	"strings"
	"time"

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
// whichever WhatsApp JID the hint resolved to, and whether unattended tool
// execution is allowed.
func (a *App) CreateRoutineFromDraft(originalText string, d routine.Draft, whatsAppTargetJID string, autoApproveTools bool) (*routine.Routine, error) {
	if a.routineStore == nil {
		return nil, fmt.Errorf("routine: store not initialized")
	}

	weekdays := make([]time.Weekday, 0, len(d.Weekdays))
	for _, w := range d.Weekdays {
		weekdays = append(weekdays, time.Weekday(w))
	}

	contextType := routine.ContextSourceType(d.ContextSourceType)
	switch contextType {
	case routine.ContextCalendar, routine.ContextWhatsApp:
	default:
		contextType = routine.ContextNone
	}

	r := routine.Routine{
		CreatedFromText: originalText,
		Schedule: routine.Schedule{
			TimeOfDay: d.TimeOfDay,
			Weekdays:  weekdays,
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
		Enabled:           true,
	}
	return a.routineStore.Create(r)
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
		fireTime, err := routine.ParseFireTime(r.Schedule.TimeOfDay, now)
		if err != nil || fireTime.Before(now) {
			continue
		}
		out = append(out, routine.MobilePayload{
			ID:        r.ID,
			Title:     "Rutin",
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

const routineSystemPrompt = "Bir kullanıcının önceden tanımladığı, zamanlanmış bir görevi şimdi çalıştırıyorsun. " +
	"Kısa, doğal, mesaj gibi okunan bir cevap ver — bir sohbetin ortasındaymış gibi değil, kendi başına bir bildirim gibi."

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
				extraContext = formatEventsForRoutine(events)
			}
		}
	case routine.ContextWhatsApp:
		if r.ContextSource.WhatsAppJID != "" {
			msgs, err := a.WhatsAppGetMessages(r.ContextSource.WhatsAppJID, 50)
			if err == nil {
				extraContext = formatWhatsAppMessagesForRoutine(msgs)
			}
		}
	}

	prompt := r.Prompt
	if extraContext != "" {
		prompt += "\n\nBağlam:\n" + extraContext
	}

	msgs := []api.Message{
		api.NewTextMessage("system", routineSystemPrompt),
		api.NewTextMessage("user", prompt),
	}
	reply := a.callLLM(ctx, msgs)
	if isLLMErrorReply(reply) {
		return "", fmt.Errorf("routine: llm call failed: %s", reply)
	}
	return reply, nil
}

// runAgentRoutine handles the agent-mode path (e.g. "git pull and report
// status"): it runs the exact same in-process App methods main.go's `-p
// --auto-allow` CLI mode uses for unattended tool execution
// (NewAgentChat/SwitchChat/SetAgentEnabled/the stream-sending path), scoping
// permission auto-approval to just this one call via SetAgentAutoPermission
// (save/restore) rather than the persistent, global AllowForever policy —
// only this routine's own single serialized stream (see streamMu in chat.go)
// is affected, never a concurrent interactive chat.
func (a *App) runAgentRoutine(ctx context.Context, r routine.Routine) (string, error) {
	chatID := a.NewAgentChat("")
	if chatID == "" {
		return "", fmt.Errorf("routine: could not create agent chat")
	}
	if err := a.SwitchChat(chatID); err != nil {
		return "", fmt.Errorf("routine: switch chat: %w", err)
	}
	if err := a.SetAgentEnabled(true); err != nil {
		return "", fmt.Errorf("routine: enable agent mode: %w", err)
	}

	if r.AutoApproveTools {
		prevAuto := a.GetAgentAutoPermission()
		if err := a.SetAgentAutoPermission(true); err != nil {
			return "", fmt.Errorf("routine: set auto permission: %w", err)
		}
		defer a.SetAgentAutoPermission(prevAuto)
	}

	ch := a.sendMessageStreamInnerTo(ctx, chatID, r.Prompt, true)
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

func formatEventsForRoutine(events []calendar.Event) string {
	if len(events) == 0 {
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

func formatWhatsAppMessagesForRoutine(msgs []whatsapp.Message) string {
	if len(msgs) == 0 {
		return "Bu sohbette yeni mesaj yok."
	}
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString("[")
		b.WriteString(m.Timestamp.Format("15:04"))
		b.WriteString("] ")
		b.WriteString(m.SenderName)
		b.WriteString(": ")
		b.WriteString(m.Text)
		b.WriteString("\n")
	}
	return b.String()
}
