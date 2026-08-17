// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/observer"
	"memo/internal/proactive"
)

// This file wires the proactive learning engine (docs/learning-system §6–§9)
// into the App: the injected Chief decider, the suggestion emitter, the live
// proactivity level, and the bridge methods the web/mobile layer calls.

// proactiveDecide is the engine's Decider: it asks the main model (Chief),
// routing through orchestra/provider/local exactly like a normal chat turn.
func (a *App) proactiveDecide(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	msgs := []api.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	dctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	reply := a.callLLM(dctx, msgs, categoryProactive)
	if reply == "" || strings.HasPrefix(reply, "⚠️") || strings.HasPrefix(reply, "warning-sign") {
		return "", fmt.Errorf("LLM returned empty or error response")
	}
	return reply, nil
}

// proactiveEmit surfaces a suggestion by firing a UI event and persisting it
// to the pending-suggestion store (already done by the engine before this is
// called — see proactive.Engine.execute); GetPendingSuggestion/the
// "proactive_suggestion" event are the only delivery paths.
//
// This deliberately does NOT write the suggestion into any chat session.
// This is a background, timer-driven engine tick, not a user-initiated
// action — the app has one global "active" chat (see AGENTS.md's
// Concurrency & architecture gotcha), and there is no reliable way from here
// to know whether that's even a chat the user is currently looking at, let
// alone one related to the suggestion. A previous version called
// sm.AddMessage(...), which always writes to whatever session happens to be
// globally active at that instant — including a chat the user is mid-
// conversation in on a completely unrelated topic, or one a task-loop
// automation has temporarily switched to (the exact "automated caller must
// SwitchChat under taskloopRunMu" hazard that gotcha already documents for
// other callers). A suggestion could silently splice itself into the wrong
// conversation's history.
func (a *App) proactiveEmit(p proactive.PendingSuggestion) {
	if payload, err := json.Marshal(p); err == nil {
		a.emitEvent("proactive_suggestion", string(payload))
	}
}

// proactiveLevel reports the live proactivity level from config.
func (a *App) proactiveLevel() proactive.Level {
	if a.cfg == nil || !a.cfg.Proactive.Enabled {
		return proactive.LevelOff
	}
	return proactive.ParseLevel(a.cfg.Proactive.Level)
}

// ─── Bridge methods (web/mobile) ────────────────────────────────

// GetProactiveSettings returns the current proactive configuration.
func (a *App) GetProactiveSettings() config.ProactiveConfig {
	if a.cfg == nil {
		return config.ProactiveConfig{Level: "off"}
	}
	return a.cfg.Proactive
}

// SetProactiveSettings updates and persists the proactive configuration.
func (a *App) SetProactiveSettings(enabled bool, level string) error {
	if a.cfg == nil {
		return nil
	}
	a.cfg.Proactive.Enabled = enabled
	a.cfg.Proactive.Level = string(proactive.ParseLevel(level))
	return config.Save(a.cfg)
}

// GetPendingSuggestion returns the in-flight proactive suggestion, or nil.
// Deliberately excludes Action == ActionAmbient: those are the whole point
// of not being a separate banner/notification (see
// internal/app/proactive_ambient.go) — the model already surfaced it inline
// in the conversation itself, so showing it again here as a popup would be
// exactly the redundant interruption ambient nudging exists to avoid. Only
// callers that resolve suggestions programmatically (checkAmbientNudgeOutcome)
// go through PendingStore.Get directly, bypassing this filter.
func (a *App) GetPendingSuggestion() *proactive.PendingSuggestion {
	if a.proactivePending == nil {
		return nil
	}
	p, err := a.proactivePending.Get()
	if err != nil || p == nil || p.Action == proactive.ActionAmbient {
		return nil
	}
	return p
}

// RespondToSuggestion records the user's answer to a pending suggestion.
func (a *App) RespondToSuggestion(id, response string) error {
	if a.proactiveEngine == nil {
		return nil
	}
	_, err := a.proactiveEngine.HandleResponse(id, response)
	return err
}

// ListLearnedPatterns exposes the learned patterns for the Settings > Learning Profile view.
func (a *App) ListLearnedPatterns() []observer.TimePattern {
	if a.observerPatterns == nil {
		return nil
	}
	patterns, err := a.observerPatterns.Load()
	if err != nil {
		return nil
	}
	return patterns
}

// ForgetPattern retires a single learned pattern.
func (a *App) ForgetPattern(id string) error {
	if a.observerPatterns == nil {
		return nil
	}
	_, err := a.observerPatterns.Suppress(id)
	return err
}

// ClearLearningData wipes all observations and learned patterns.
func (a *App) ClearLearningData() error {
	if a.observerStore != nil {
		if err := a.observerStore.ClearAll(); err != nil {
			return err
		}
	}
	if a.observerPatterns != nil {
		if err := a.observerPatterns.Clear(); err != nil {
			return err
		}
	}
	if a.proactivePending != nil {
		if err := a.proactivePending.Clear(); err != nil {
			return err
		}
	}
	return nil
}
