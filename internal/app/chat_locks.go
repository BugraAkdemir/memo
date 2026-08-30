package app

import (
	"sync"

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
