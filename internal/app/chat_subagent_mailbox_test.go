package app

import (
	"context"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/sessions"
	"testing"
	"time"
)

func TestChatSubAgentInterruptedMainTurnNeverAcknowledgesResult(t *testing.T) {
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	chat := sm.NewChat()
	a := &App{cfg: &config.AppConfig{}, sessions: sm, identity: identity.New("Test", "Memo", "casual", "", true)}
	ctx, cancel := context.WithCancel(context.WithValue(withCodeMode(context.Background()), subAgentCompletionKey{}, uint64(8)))
	cancel()
	a.persistInterruptedTurn(ctx, time.Now(), 0, "partial review", "result", chat, &usageMeta{})
	if a.completionWasSaved(chat, 8) {
		t.Fatal("stopped review acknowledged child result")
	}
	success := context.WithValue(withCodeMode(context.Background()), subAgentCompletionKey{}, uint64(9))
	a.finishStream(success, time.Now(), 0, "stop", "accepted", "result", chat, &usageMeta{})
	if !a.completionWasSaved(chat, 9) {
		t.Fatal("successful saved review not acknowledged")
	}
}
