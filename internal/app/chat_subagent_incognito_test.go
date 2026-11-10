package app

import (
	"context"
	"testing"

	"memo/internal/sessions"
	"memo/internal/subagents"
)

// Incognito keeps no sub-agent records and cannot start any, so listing them
// must answer "none", not fail: the failure showed every Incognito chat the
// "Alt ajanlar yüklenemedi" (could not load sub-agents) button (seen live,
// 2026-11-10). Everything that reads or acts on one record stays refused.
func TestListChatSubAgents_IncognitoIsEmptyNotAnError(t *testing.T) {
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m, err := subagents.Open(t.TempDir(), 1, func(context.Context, subagents.Job, func(subagents.Event)) (subagents.RunResult, error) {
		return subagents.RunResult{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown(context.Background())
	a := &App{sessions: sm, chatSubAgents: m}
	chatID := sm.GetActiveID()

	a.isIncognito = true
	jobs, err := a.ListChatSubAgents(chatID)
	if err != nil {
		t.Fatalf("ListChatSubAgents in Incognito = %v, want an empty list", err)
	}
	if jobs == nil || len(jobs) != 0 {
		t.Fatalf("ListChatSubAgents in Incognito = %v, want []", jobs)
	}
	if _, err := a.GetChatSubAgent(chatID, "any"); err == nil {
		t.Error("reading one sub-agent record in Incognito must still be refused")
	}

	if _, err := a.ListChatSubAgents("no-such-chat"); err == nil {
		t.Error("an unknown chat must still be an error")
	}

	a.isIncognito = false
	if _, err := a.ListChatSubAgents(chatID); err != nil {
		t.Fatalf("ListChatSubAgents outside Incognito = %v", err)
	}
}
