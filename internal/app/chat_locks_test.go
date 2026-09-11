package app

import (
	"testing"

	"memo/internal/api"
)

// TestRunLockedStreamSetup_PanicReleasesLockAndDoesNotPropagate guards Y2: a
// panic in the synchronous work between acquiring a chat's stream lock and
// launching the forwarding goroutine that normally releases it (e.g. a
// nil-map/index-out-of-range on corrupted session data inside
// buildMessagesForSession) used to propagate straight out of
// sendMessageStreamInnerTo/SendMessageWithImageStream/
// SendMessageWithFileStream with the lock still held — nothing else ever
// called release() for that call, so the chat stayed permanently "busy"
// until the backend restarted. runLockedStreamSetup must recover the panic,
// release the lock itself, and report ok=false instead of letting the panic
// escape.
func TestRunLockedStreamSetup_PanicReleasesLockAndDoesNotPropagate(t *testing.T) {
	a := &App{}
	release, ok := a.lockChatStream("chat-1")
	if !ok {
		t.Fatal("expected to acquire the per-chat lock")
	}

	ch, ok := runLockedStreamSetup("test", release, func() <-chan api.StreamChunk {
		panic("boom: corrupted session data")
	})
	if ok {
		t.Fatal("expected ok=false after a panic in fn")
	}
	if ch != nil {
		t.Fatalf("expected a nil channel after a panic, got %v", ch)
	}

	// The lock must be free again — a second attempt on the same chat id
	// must succeed, proving release() actually ran despite the panic.
	release2, ok2 := a.lockChatStream("chat-1")
	if !ok2 {
		t.Fatal("chat lock still held after a panic in the synchronous setup — release() never ran (Y2)")
	}
	release2()
}

// TestRunLockedStreamSetup_NoPanicPassesChannelThroughUnchanged is the
// non-panic counterpart: when fn returns normally, runLockedStreamSetup must
// be a transparent pass-through — same channel, ok=true, release left for
// the caller's own forwarding goroutine to call (not called here).
func TestRunLockedStreamSetup_NoPanicPassesChannelThroughUnchanged(t *testing.T) {
	a := &App{}
	release, ok := a.lockChatStream("chat-2")
	if !ok {
		t.Fatal("expected to acquire the per-chat lock")
	}

	want := make(chan api.StreamChunk)
	ch, ok := runLockedStreamSetup("test", release, func() <-chan api.StreamChunk {
		return want
	})
	if !ok {
		t.Fatal("expected ok=true when fn does not panic")
	}
	if ch != want {
		t.Fatal("expected the channel returned by fn to pass through unchanged")
	}

	// Lock is still held (fn returning normally must not have released it —
	// that's the forwarding goroutine's job, not runLockedStreamSetup's).
	if _, stillLocked := a.lockChatStream("chat-2"); stillLocked {
		t.Fatal("expected the lock to still be held after a non-panicking call")
	}
	release()
}
