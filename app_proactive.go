// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"encoding/json"
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
	return a.callLLM(dctx, msgs), nil
}

// proactiveEmit surfaces a suggestion: it fires a UI event (desktop/mobile event
// stream) and drops the message into the active session so it appears in chat.
func (a *App) proactiveEmit(p proactive.PendingSuggestion) {
	// emitEvent serializes its payload via fmt.Sprint, so pass a JSON string the
	// frontend can parse directly rather than a Go-map representation.
	if payload, err := json.Marshal(p); err == nil {
		a.emitEvent("proactive_suggestion", string(payload))
	}
	if sm := a.getSessionManager(); sm != nil {
		sm.AddMessage("assistant", p.Message, "", "")
	}
}

// proactiveLevel reports the live proactivity level from config; Off whenever
// proactive learning is disabled.
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

// SetProactiveSettings updates and persists the proactive configuration. The
// engine reads the level live, so no restart is needed.
func (a *App) SetProactiveSettings(enabled bool, level string) error {
	if a.cfg == nil {
		return nil
	}
	a.cfg.Proactive.Enabled = enabled
	a.cfg.Proactive.Level = string(proactive.ParseLevel(level))
	return config.Save(a.cfg)
}

// GetPendingSuggestion returns the in-flight proactive suggestion (for mobile
// polling), or nil when there is none.
func (a *App) GetPendingSuggestion() *proactive.PendingSuggestion {
	if a.proactivePending == nil {
		return nil
	}
	p, err := a.proactivePending.Get()
	if err != nil {
		return nil
	}
	return p
}

// RespondToSuggestion records the user's answer to a pending suggestion. When
// accepted, the suggestion's action is carried out (currently: surface it in
// chat; agent auto-runs are handled by the engine's AutoRunner when configured).
func (a *App) RespondToSuggestion(id, response string) error {
	if a.proactiveEngine == nil {
		return nil
	}
	_, err := a.proactiveEngine.HandleResponse(id, response)
	return err
}

// ListLearnedPatterns exposes the learned patterns for the Settings >
// Learning Profile view (transparency, README §11).
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

// ForgetPattern retires a single learned pattern ("Şu pattern'i unut"). It is
// suppressed so the analyzer will not re-learn it.
func (a *App) ForgetPattern(id string) error {
	if a.observerPatterns == nil {
		return nil
	}
	_, err := a.observerPatterns.Suppress(id)
	return err
}

// ClearLearningData wipes all observations and learned patterns ("Tüm
// verilerimi sil", README §11).
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
