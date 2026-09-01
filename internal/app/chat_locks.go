package app

import (
	"context"
	"sync"
	"time"

	"memo/internal/api"
)

// busyStreamChan returns a closed single-chunk channel carrying a "please
// wait" error — the standard reply when a stream lock can't be taken.
func busyStreamChan(msg string) <-chan api.StreamChunk {
	errCh := make(chan api.StreamChunk, 1)
	errCh <- api.StreamChunk{Error: msg, Done: true}
	close(errCh)
	return errCh
}

// Per-chat streaming serialisation (v4.6.0 Faz A).
//
// The app used to guard every streaming reply with one global mutex
// (a.streamMu). That made a Self-Driving task turn running in its own agent
// chat block the user typing in a different chat, and block Telegram/WhatsApp
// replies — anything that streams waited for whatever held the single lock.
//
// a.streamMu is now an RWMutex acting only as a gate:
//
//   - Ordinary interactive and task streams take it with TryRLock — many
//     chats stream at the same time.
//   - The routine and incognito paths, which flip a *global* flag
//     (SetAgentAutoPermission, a.isIncognito) that would otherwise leak into a
//     concurrent interactive stream, still take it with TryLock (exclusive).
//
// Same-chat serialisation — two turns must never interleave
// user/user/assistant/assistant into one session history — moves to a
// per-chat mutex here, keyed by chat id.

// chatStreamLock returns the per-chat streaming mutex for chatID, creating it
// on first use. The empty string is a valid key (a send with no resolvable
// active chat) and simply shares one lock.
func (a *App) chatStreamLock(chatID string) *sync.Mutex {
	a.chatStreamMu.Lock()
	defer a.chatStreamMu.Unlock()
	if a.chatStreamLocks == nil {
		a.chatStreamLocks = make(map[string]*sync.Mutex)
	}
	l := a.chatStreamLocks[chatID]
	if l == nil {
		l = &sync.Mutex{}
		a.chatStreamLocks[chatID] = l
	}
	return l
}

// lockChatStream takes a shared hold on the global stream gate plus the
// exclusive per-chat lock for chatID. It acquires nothing and returns
// ok=false if either is unavailable — a second stream already running in the
// same chat, or a routine/incognito turn currently holding the gate
// exclusively. release undoes both in the correct order and is safe to call
// exactly once.
func (a *App) lockChatStream(chatID string) (release func(), ok bool) {
	if !a.streamMu.TryRLock() {
		return nil, false
	}
	cl := a.chatStreamLock(chatID)
	if !cl.TryLock() {
		a.streamMu.RUnlock()
		return nil, false
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			cl.Unlock()
			a.streamMu.RUnlock()
		})
	}, true
}

// resolveChatID returns chatID unchanged when set, otherwise the currently
// active chat id. Used by the implicit-active-chat entrypoints so an empty id
// and the explicit active id collapse to the same per-chat lock.
func (a *App) resolveChatID(chatID string) string {
	if chatID != "" {
		return chatID
	}
	if sm := a.getSessionManager(); sm != nil {
		return sm.GetActiveID()
	}
	return ""
}

// busyNotice is the single source of truth for the "previous response still
// running" text. Every stream entrypoint that fails a lock returns exactly
// this string, and the task loop compares against it to tell a busy chat from
// a real provider failure (see buildTaskLoopRunWorker) — so it must not be
// duplicated as a literal anywhere.
func (a *App) busyNotice() string {
	return a.t("⏳ Lütfen önceki cevap tamamlanana kadar bekleyin.", "⏳ Please wait until the previous response finishes.")
}

// chatLockWaitCtxKey marks a stream call that must *queue* behind an in-flight
// turn in the same chat instead of being rejected with busyNotice.
type chatLockWaitCtxKey struct{}

// withChatLockWait marks ctx so sendMessageStreamInnerTo waits for the
// per-chat stream lock rather than failing fast.
//
// Only the Self-Driving worker turn uses it. An interactive caller (the GUI, a
// Telegram reply) still gets the immediate busy notice: a human is waiting on
// that response and a silent multi-minute stall is worse than being told to
// retry. An unattended task turn is the opposite — there is nobody to retry,
// and the launching agent turn is *by construction* still streaming in that
// same chat when the loop starts its first item (start_self_driving_task
// returns while its own turn holds the chat lock), so fail-fast turned every
// item of every task list into an instant "işçi hatası" and the whole list
// into a failure in under two seconds.
func withChatLockWait(ctx context.Context) context.Context {
	return context.WithValue(ctx, chatLockWaitCtxKey{}, true)
}

func chatLockWaitEnabled(ctx context.Context) bool {
	v, _ := ctx.Value(chatLockWaitCtxKey{}).(bool)
	return v
}

// chatLockWaitPoll/chatLockWaitMax bound the queueing above: how often to
// re-try the lock, and how long to keep queueing before giving up and letting
// the caller see the busy notice.
const (
	chatLockWaitPoll = 200 * time.Millisecond
	chatLockWaitMax  = 5 * time.Minute
)

// lockChatStreamWait is lockChatStream that queues instead of failing. It
// polls (sync.Mutex has no context-aware Lock) until both holds are taken,
// ctx is cancelled, or chatLockWaitMax elapses.
func (a *App) lockChatStreamWait(ctx context.Context, chatID string) (release func(), ok bool) {
	deadline := time.Now().Add(chatLockWaitMax)
	for {
		if release, ok := a.lockChatStream(chatID); ok {
			return release, true
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			return nil, false
		}
		select {
		case <-ctx.Done():
			return nil, false
		case <-time.After(chatLockWaitPoll):
		}
	}
}
