package app

import (
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/sessions"
)

func appendMessageTestApp(t *testing.T) (*App, chan saveTask) {
	t.Helper()
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("sessions.NewManager: %v", err)
	}
	sm.NewChat()
	ch := make(chan saveTask, 4)
	a := &App{
		cfg:          &config.AppConfig{Memory: config.MemoryConfig{MemoryEnabled: true}},
		sessions:     sm,
		memorySaveCh: ch,
	}
	return a, ch
}

// TestAppendMessage_LiveModeTurn_FeedsMemory is the regression test for
// "Live Mode conversations never reach long-term memory": AppendMessage
// (POST /api/messages/append — the transcript-persistence primitive) only
// wrote the bubble to the session transcript. It now also pairs the
// model's transcript with the preceding user utterance and hands the turn
// to saveMemoryAsync, exactly like a typed chat turn.
func TestAppendMessage_LiveModeTurn_FeedsMemory(t *testing.T) {
	a, ch := appendMessageTestApp(t)

	if err := a.AppendMessage("user", "beni yarın 15:00'te toplantı için uyar"); err != nil {
		t.Fatalf("AppendMessage(user): %v", err)
	}
	// The user bubble alone must not enqueue anything — only a completed
	// (user, assistant) pair is a turn worth remembering.
	select {
	case task := <-ch:
		t.Fatalf("user-only AppendMessage enqueued a memory save: %+v", task)
	case <-time.After(50 * time.Millisecond):
	}

	if err := a.AppendMessage("assistant", "tamam, yarın 15:00 için hatırlatıcı oluşturdum"); err != nil {
		t.Fatalf("AppendMessage(assistant): %v", err)
	}
	select {
	case task := <-ch:
		if task.userMsg != "beni yarın 15:00'te toplantı için uyar" {
			t.Errorf("userMsg = %q, want the preceding user utterance", task.userMsg)
		}
		if task.reply != "tamam, yarın 15:00 için hatırlatıcı oluşturdum" {
			t.Errorf("reply = %q, want the model's transcript", task.reply)
		}
	case <-time.After(time.Second):
		t.Fatal("assistant AppendMessage did not enqueue a memory save")
	}
}

// TestAppendMessage_Incognito_DoesNotFeedMemory guards Incognito Mode's
// "nothing persisted, nothing recalled" contract for the Live Mode path —
// the transcript still lands in the (ephemeral) session, but no RAG write
// is enqueued.
func TestAppendMessage_Incognito_DoesNotFeedMemory(t *testing.T) {
	a, ch := appendMessageTestApp(t)
	a.ToggleIncognito(true)

	if err := a.AppendMessage("user", "gizli bir şey soruyorum"); err != nil {
		t.Fatalf("AppendMessage(user): %v", err)
	}
	if err := a.AppendMessage("assistant", "gizli cevabım bu"); err != nil {
		t.Fatalf("AppendMessage(assistant): %v", err)
	}

	select {
	case task := <-ch:
		t.Fatalf("incognito AppendMessage enqueued a memory save: %+v", task)
	case <-time.After(100 * time.Millisecond):
	}
}
