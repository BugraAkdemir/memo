package app

import (
	"context"
	"sync"

	"memo/internal/api"
)

// chatTurnQueue is the main agent's FIFO, shared by user turns and child
// completions. Reserving happens before launching the goroutine, so a later
// arrival cannot overtake a turn whose goroutine has not been scheduled yet.
type chatTurnQueue struct {
	mu    sync.Mutex
	lanes map[string][]*chatTurnTicket
}
type chatTurnTicket struct {
	owner *chatTurnQueue
	chat  string
	ready chan struct{}
	once  sync.Once
}

func (q *chatTurnQueue) reserve(chat string) *chatTurnTicket {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.lanes == nil {
		q.lanes = make(map[string][]*chatTurnTicket)
	}
	t := &chatTurnTicket{owner: q, chat: chat, ready: make(chan struct{})}
	q.lanes[chat] = append(q.lanes[chat], t)
	if len(q.lanes[chat]) == 1 {
		close(t.ready)
	}
	return t
}
func (t *chatTurnTicket) wait(ctx context.Context) bool {
	if ctx.Err() != nil {
		t.release()
		return false
	}
	select {
	case <-ctx.Done():
		t.release()
		return false
	case <-t.ready:
		if ctx.Err() != nil {
			t.release()
			return false
		}
		return true
	}
}
func (t *chatTurnTicket) release() {
	t.once.Do(func() {
		q := t.owner
		q.mu.Lock()
		defer q.mu.Unlock()
		lane := q.lanes[t.chat]
		for i, v := range lane {
			if v == t {
				lane = append(lane[:i], lane[i+1:]...)
				if len(lane) == 0 {
					delete(q.lanes, t.chat)
				} else {
					q.lanes[t.chat] = lane
					if i == 0 {
						close(lane[0].ready)
					}
				}
				return
			}
		}
	})
}

type chatTurnQueuedKey struct{}

func (a *App) queueChatAgentTurn(ctx context.Context, chatID, userMsg string, forceAgent bool) <-chan api.StreamChunk {
	return a.queueChatStream(ctx, chatID, func(ctx context.Context) <-chan api.StreamChunk {
		return a.sendMessageStreamInnerTo(ctx, chatID, userMsg, forceAgent)
	})
}

// queueChatStream reserves synchronously and returns immediately while a prior
// main-agent turn holds this chat. Callers capture the target chat before entry.
func (a *App) queueChatStream(ctx context.Context, chatID string, start func(context.Context) <-chan api.StreamChunk) <-chan api.StreamChunk {
	ticket := a.subAgentTurns.reserve(a.resolveChatID(chatID))
	out := make(chan api.StreamChunk, 128)
	go func() {
		defer close(out)
		defer ticket.release()
		defer recoverStreamPanic(ctx, out, "queueChatStream")
		if !ticket.wait(ctx) {
			return
		}
		ctx = context.WithValue(withChatLockWait(ctx), chatTurnQueuedKey{}, true)
		for chunk := range start(ctx) {
			select {
			case out <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func (a *App) lockAttachmentChatStream(ctx context.Context, chatID string) (func(), bool) {
	if ctx.Value(chatTurnQueuedKey{}) == true {
		return a.lockChatStreamWait(ctx, chatID)
	}
	return a.lockChatStream(chatID)
}
