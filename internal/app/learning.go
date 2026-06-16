// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"memo/internal/api"
	"memo/internal/calendar"
	"memo/internal/config"
	"memo/internal/intent"
	"memo/internal/provider"
)

// initLearning sets up the intent extractor and calendar system. Called from
// Start() after the observer and proactive engine are ready.
func (a *App) initLearning(ctx context.Context) {
	// Calendar store
	calDir := config.DataPath("calendar")
	cs, err := calendar.NewStore(calDir)
	if err != nil {
		log.Printf("WARN: calendar store: %v", err)
	} else {
		a.calendarStore = cs
		a.calendarRemind = calendar.NewReminderLoop(cs, a.calendarLeadFn, func(name, data string) {
			a.emitEvent(name, data)
		})
		go a.calendarRemind.Start(ctx)
		log.Println("Calendar system initialized")
	}

	// Intent extractor — uses same Decider as proactive engine.
	a.learningMu.Lock()
	a.intentExtractor = intent.NewExtractor(a.buildLearningDecider())
	a.learningMu.Unlock()
	log.Println("Intent extractor initialized")
}

// buildLearningDecider returns the Decider for both the intent extractor and
// (optionally) the proactive engine. When single model mode is enabled it calls
// the chosen model directly; otherwise it routes through Orchestra/callLLM.
func (a *App) buildLearningDecider() intent.Decider {
	cfg := a.cfg.Learning
	return intent.BuildDecider(
		cfg,
		func(ctx context.Context, prompt string) (string, error) {
			msgs := []api.Message{
				{Role: "user", Content: prompt},
			}
			result := a.callLLM(ctx, msgs)
			return result, nil
		},
		func(ctx context.Context, modelID, system, user string) (string, error) {
			return a.callSingleModel(ctx, modelID, system, user)
		},
	)
}

// callSingleModel calls a specific model by ID, bypassing Orchestra and the
// active provider selection. Used by single model mode in the learning system.
func (a *App) callSingleModel(ctx context.Context, modelID, systemPrompt, userPrompt string) (string, error) {
	a.providerMu.RLock()
	router := a.providerRouter
	a.providerMu.RUnlock()

	if router == nil {
		return "", fmt.Errorf("no provider router available")
	}

	msgs := []provider.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	req := provider.ChatRequest{
		Model:       modelID,
		Messages:    msgs,
		Temperature: a.cfg.Llama.Temperature,
		TopP:        a.cfg.Llama.TopP,
		MaxTokens:   512,
	}

	// Bound the call so a hung provider can't leak the goroutine indefinitely.
	cctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	resp, err := router.ChatCompletion(cctx, req)
	if err != nil {
		return "", fmt.Errorf("single model call: %w", err)
	}
	return resp.Content, nil
}

// calendarLeadFn returns the live reminder lead time from config.
func (a *App) calendarLeadFn() int {
	if a.cfg == nil {
		return 30
	}
	return a.cfg.Calendar.ReminderLeadMinutes
}

// processMessageIntent runs the intent extraction pipeline for a message.
// It is called asynchronously from WhatsApp and chat handlers.
func (a *App) processMessageIntent(text string, source intent.Source, contact string, ts time.Time) {
	a.learningMu.RLock()
	extractor := a.intentExtractor
	a.learningMu.RUnlock()
	if extractor == nil {
		return
	}

	result, err := extractor.Extract(a.lifecycleCtx, text, source, contact, ts)
	if err != nil {
		log.Printf("intent: extract: %v", err)
		return
	}
	if !result.HasIntent {
		return
	}

	// Cancellation: delete the matching calendar event instead of creating one.
	if result.IsCancellation {
		a.cancelCalendarFromIntent(result)
		return
	}

	// Record in observer for habit/pattern learning.
	if a.observerRecorder != nil {
		a.observerRecorder.RecordIntent(
			result.Summary,
			string(result.Source),
			result.ContactName,
			result.IsHabit,
			result.HabitTime,
			ts,
		)
	}

	// Add to calendar when a specific event time was resolved. If the user has
	// disabled time guessing, skip events whose time the model only inferred.
	if result.IsCalendarEvent && result.EventTime != nil && a.calendarStore != nil {
		if !result.TimeExplicit && a.cfg.Calendar.DisableTimeGuess {
			log.Printf("calendar: skipped %q — vague time and guessing disabled", result.Summary)
			return
		}
		event, err := calendar.AddFromIntent(a.lifecycleCtx, a.calendarStore, result)
		if err != nil {
			log.Printf("calendar: add from intent: %v", err)
			return
		}
		payload, _ := json.Marshal(calendar.AddedPayload{ID: event.ID, Title: event.Title})
		a.emitEvent("calendar:added", string(payload))
		log.Printf("calendar: added %q at %s (source=%s)", event.Title, event.StartTime.Format("2006-01-02 15:04"), event.Source)
	}
}

// cancelCalendarFromIntent deletes calendar events that match a cancellation
// intent. It looks within a few hours of the referenced time and matches by
// title; if no title is given it removes a lone event in that window.
func (a *App) cancelCalendarFromIntent(r intent.IntentResult) {
	if a.calendarStore == nil || r.EventTime == nil {
		return
	}
	t := *r.EventTime
	events, err := a.calendarStore.List(a.lifecycleCtx, t.Add(-3*time.Hour), t.Add(3*time.Hour))
	if err != nil {
		log.Printf("calendar: cancel list: %v", err)
		return
	}
	if len(events) == 0 {
		return
	}

	title := strings.ToLower(strings.TrimSpace(r.EventTitle))
	var toDelete []calendar.Event
	if title == "" {
		// No title to match on: only safe to delete when there's exactly one.
		if len(events) == 1 {
			toDelete = events
		}
	} else {
		for _, e := range events {
			et := strings.ToLower(e.Title)
			if strings.Contains(et, title) || strings.Contains(title, et) {
				toDelete = append(toDelete, e)
			}
		}
	}

	for _, e := range toDelete {
		if err := a.calendarStore.Delete(a.lifecycleCtx, e.ID); err != nil {
			log.Printf("calendar: cancel delete %s: %v", e.ID, err)
			continue
		}
		payload, _ := json.Marshal(calendar.AddedPayload{ID: e.ID, Title: e.Title})
		a.emitEvent("calendar:removed", string(payload))
		log.Printf("calendar: cancelled %q at %s", e.Title, e.StartTime.Format("2006-01-02 15:04"))
	}
}

// UpdateLearningSettings saves learning config and rebuilds the intent extractor
// Decider so the change takes effect without restarting.
func (a *App) UpdateLearningSettings(singleModelEnabled bool, modelID string) error {
	if a.cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	a.cfg.Learning.SingleModelEnabled = singleModelEnabled
	a.cfg.Learning.ModelID = modelID
	if err := config.Save(a.cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	// Rebuild decider with new settings.
	a.learningMu.Lock()
	if a.intentExtractor != nil {
		a.intentExtractor = intent.NewExtractor(a.buildLearningDecider())
	}
	a.learningMu.Unlock()
	return nil
}

// GetLearningSettings returns the current learning configuration.
func (a *App) GetLearningSettings() config.LearningConfig {
	if a.cfg == nil {
		return config.LearningConfig{}
	}
	return a.cfg.Learning
}

// GetCalendarSettings returns the current calendar configuration.
func (a *App) GetCalendarSettings() config.CalendarConfig {
	if a.cfg == nil {
		return config.CalendarConfig{ReminderLeadMinutes: 30}
	}
	return a.cfg.Calendar
}

// UpdateCalendarSettings saves calendar config.
func (a *App) UpdateCalendarSettings(leadMinutes int, disableTimeGuess bool) error {
	if a.cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	if leadMinutes <= 0 {
		leadMinutes = 30
	}
	a.cfg.Calendar.ReminderLeadMinutes = leadMinutes
	a.cfg.Calendar.DisableTimeGuess = disableTimeGuess
	return config.Save(a.cfg)
}

// ListCalendarEvents returns events within the given time range.
func (a *App) ListCalendarEvents(from, to time.Time) ([]calendar.Event, error) {
	if a.calendarStore == nil {
		return nil, fmt.Errorf("calendar not initialized")
	}
	return a.calendarStore.List(a.lifecycleCtx, from, to)
}

// AddCalendarEvent creates a manual calendar event.
func (a *App) AddCalendarEvent(title string, startTime time.Time, description string) (calendar.Event, error) {
	if a.calendarStore == nil {
		return calendar.Event{}, fmt.Errorf("calendar not initialized")
	}
	e := calendar.Event{
		Title:       title,
		StartTime:   startTime,
		Description: description,
		Source:      calendar.SourceManual,
	}
	return a.calendarStore.Add(a.lifecycleCtx, e)
}

// DeleteCalendarEvent removes an event by ID.
func (a *App) DeleteCalendarEvent(id string) error {
	if a.calendarStore == nil {
		return fmt.Errorf("calendar not initialized")
	}
	return a.calendarStore.Delete(a.lifecycleCtx, id)
}
