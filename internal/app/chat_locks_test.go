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

// TestGetStreamingChatIDs_ReflectsHeldLocksOnly is the chat sidebar's
// "still working" indicator's backing signal: a chat currently holding its
// stream lock must be reported, a chat that never streamed or whose stream
// already released must not, and the shared empty-string key (sends with
// no resolvable active chat) must never leak out as a fake chat id.
func TestGetStreamingChatIDs_ReflectsHeldLocksOnly(t *testing.T) {
	a := &App{}

	releaseA, ok := a.lockChatStream("chat-a")
	if !ok {
		t.Fatal("expected to acquire chat-a's lock")
	}
	releaseB, ok := a.lockChatStream("chat-b")
	if !ok {
		t.Fatal("expected to acquire chat-b's lock")
	}
	// chat-c: lock briefly then release, same as a turn that already
	// finished — must not show up as streaming.
	releaseC, ok := a.lockChatStream("chat-c")
	if !ok {
		t.Fatal("expected to acquire chat-c's lock")
	}
	releaseC()
	// The shared empty-key lock, from a send with no resolvable active
	// chat — must never surface as a chat id.
	releaseEmpty, ok := a.lockChatStream("")
	if !ok {
		t.Fatal("expected to acquire the empty-key lock")
	}

	ids := a.GetStreamingChatIDs()
	got := map[string]bool{}
	for _, id := range ids {
		got[id] = true
	}
	if !got["chat-a"] || !got["chat-b"] {
		t.Fatalf("GetStreamingChatIDs() = %v, want chat-a and chat-b (both still locked)", ids)
	}
	if got["chat-c"] {
		t.Fatalf("GetStreamingChatIDs() = %v, want chat-c absent (its lock was released)", ids)
	}
	if got[""] {
		t.Fatalf("GetStreamingChatIDs() = %v, want the empty-key lock never reported as a chat id", ids)
	}
	if len(ids) != 2 {
		t.Fatalf("GetStreamingChatIDs() returned %d ids, want exactly 2: %v", len(ids), ids)
	}

	releaseA()
	releaseB()
	releaseEmpty()

	if ids := a.GetStreamingChatIDs(); len(ids) != 0 {
		t.Fatalf("GetStreamingChatIDs() after releasing everything = %v, want empty", ids)
	}
}
