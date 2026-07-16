// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"encoding/json"
	"testing"
	"time"

	"memo/internal/proactive"
	"memo/internal/sessions"
)

// TestProactiveEmit_DoesNotTouchActiveChat is a regression test for a real
// bug: proactiveEmit used to call sm.AddMessage(...), which always writes to
// whatever chat happens to be the single globally-active one (see
// AGENTS.md's Concurrency & architecture gotcha) — but the proactive engine
// is a background, timer-driven ticker with no relationship to what the user
// is currently doing, so a suggestion could silently splice itself into an
// unrelated, in-progress conversation. proactiveEmit must only fire the
// "proactive_suggestion" event, never touch any chat's message history.
func TestProactiveEmit_DoesNotTouchActiveChat(t *testing.T) {
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	activeID := sm.GetActiveID()
	sm.AddMessage("user", "completely unrelated ongoing conversation", "", "")
	before := sm.GetActiveMessages()

	a := &App{sessions: sm, events: &eventRing{}}

	a.proactiveEmit(proactive.PendingSuggestion{
		ID:        "sugg-1",
		Message:   "hey, want to start coding now?",
		PatternID: "declared:21:00",
		Action:    proactive.ActionSuggest,
		CreatedAt: time.Now(),
	})

	after := sm.GetActiveMessages()
	if len(after) != len(before) {
		t.Fatalf("active chat %s message count changed from %d to %d — proactiveEmit wrote into the active session",
			activeID, len(before), len(after))
	}

	events := a.GetEvents()
	if len(events) != 1 || events[0]["name"] != "proactive_suggestion" {
		t.Fatalf("expected exactly one proactive_suggestion event, got %+v", events)
	}
	var payload proactive.PendingSuggestion
	if err := json.Unmarshal([]byte(events[0]["data"]), &payload); err != nil {
		t.Fatalf("event payload not valid JSON: %v", err)
	}
	if payload.ID != "sugg-1" || payload.Message != "hey, want to start coding now?" {
		t.Errorf("event payload = %+v, want the suggestion passed in", payload)
	}
}
