package livemode

import (
	"context"
	"sync"
	"time"

	"memo/internal/logx"
)

// reconnectInitialDelay/reconnectMaxDelay mirror
// internal/telegram/client.go's pollLoop backoff (1s -> 30s cap, doubling)
// — the same lost-connection resilience pattern this codebase already
// trusts elsewhere, applied here for the first time to Live Mode. vars
// (not consts), same trick google.Client's SessionBaseURL uses, so tests
// can shrink them instead of taking real wall-clock seconds per retry.
var (
	reconnectInitialDelay = 1 * time.Second
	reconnectMaxDelay     = 30 * time.Second
)

// SessionFactory builds one fresh, unstarted Session for a single
// connection attempt. A ReconnectingSession calls it again for every
// (re)connect — neither google.Client nor openai_realtime.Client support
// being Start()ed twice on the same instance, and a fresh websocket needs
// the whole setup message re-sent regardless.
type SessionFactory func() Session

// ReconnectingSession wraps a SessionFactory and transparently redials
// with exponential backoff whenever the current inner session's Events()
// channel closes for a reason other than this wrapper's own Close()/ctx
// cancellation — closing the gap that neither the Google Live nor OpenAI
// Realtime client had any reconnect logic at all: a single dropped
// websocket (Wi-Fi blip, provider idle timeout) ended the whole Live Mode
// session permanently, with no automatic recovery, leaving the user to
// notice their mic had gone silent and manually restart.
//
// Trade-off, stated plainly: this resumes the AUDIO TRANSPORT, not the
// provider's conversation state. Neither engine's Live API exposes a
// resume token, so a reconnect opens a genuinely new session server-side
// (fresh setup message, same system prompt/tools/model) — the model can
// "forget" the last few seconds of context around the drop. That is still
// strictly better than the prior behavior (the session simply died), and
// matches how a human phone call would recover from a dropped line: pick
// back up, don't pretend the gap didn't happen.
type ReconnectingSession struct {
	factory SessionFactory
	events  chan SessionEvent

	ctx    context.Context
	cancel context.CancelFunc

	mu    sync.Mutex
	inner Session

	closeOnce sync.Once
	stopped   chan struct{} // closed by supervise() on its way out for good
}

// NewReconnectingSession creates a ready-to-Start ReconnectingSession.
// factory must be safe to call repeatedly (each call should build a fresh
// client bound to the same engine config).
func NewReconnectingSession(factory SessionFactory) *ReconnectingSession {
	return &ReconnectingSession{
		factory: factory,
		events:  make(chan SessionEvent, 16),
		stopped: make(chan struct{}),
	}
}

func (r *ReconnectingSession) Start(ctx context.Context) error {
	r.ctx, r.cancel = context.WithCancel(ctx)

	inner := r.factory()
	if err := inner.Start(r.ctx); err != nil {
		r.cancel()
		return err
	}
	r.setInner(inner)

	logx.GoRecover("livemode.ReconnectingSession.supervise", r.supervise)
	return nil
}

// supervise forwards events from whatever the current inner session is,
// and — when its Events() channel closes for a reason other than this
// session ending — redials via factory with backoff and keeps going. Owns
// closing r.events exactly once, on the way out for good.
func (r *ReconnectingSession) supervise() {
	defer close(r.stopped)
	defer close(r.events)

	current := r.getInner()
	backoff := reconnectInitialDelay
	for {
		// A bare `for ev := range current.Events()` would block forever on
		// a channel that's simply open with nothing to forward — it has no
		// way to also observe ctx.Done() concurrently, so Close() would
		// never be noticed until the inner session's channel happens to
		// close on its own. Select on both explicitly instead.
	forward:
		for {
			select {
			case ev, ok := <-current.Events():
				if !ok {
					break forward
				}
				select {
				case r.events <- ev:
				case <-r.ctx.Done():
					return
				}
			case <-r.ctx.Done():
				return
			}
		}

		if r.ctx.Err() != nil {
			// Close()/context cancellation — a clean, intentional end, not
			// a drop to recover from.
			return
		}

		logx.Printf("livemode: session dropped unexpectedly, reconnecting (backoff=%s)", backoff)

		var next Session
		for {
			select {
			case <-time.After(backoff):
			case <-r.ctx.Done():
				return
			}
			if backoff < reconnectMaxDelay {
				backoff *= 2
				if backoff > reconnectMaxDelay {
					backoff = reconnectMaxDelay
				}
			}

			candidate := r.factory()
			if err := candidate.Start(r.ctx); err != nil {
				logx.Printf("livemode: reconnect attempt failed, retrying in %s: %v", backoff, err)
				continue
			}
			next = candidate
			break
		}

		logx.Printf("livemode: reconnected successfully")
		r.setInner(next)
		current = next
		backoff = reconnectInitialDelay
	}
}

func (r *ReconnectingSession) getInner() Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inner
}

func (r *ReconnectingSession) setInner(s Session) {
	r.mu.Lock()
	r.inner = s
	r.mu.Unlock()
}

func (r *ReconnectingSession) SendAudio(pcm []byte) error {
	return r.getInner().SendAudio(pcm)
}

func (r *ReconnectingSession) InjectContext(text string) error {
	return r.getInner().InjectContext(text)
}

func (r *ReconnectingSession) Events() <-chan SessionEvent { return r.events }

// closeWaitTimeout bounds how long Close() waits for supervise() to
// actually observe ctx cancellation and return, so a wedged reconnect
// attempt (a factory/Start() call that never returns) can't hang the
// caller forever — matching webserver.Server.Stop()'s bounded-Shutdown
// pattern for the same reason.
const closeWaitTimeout = 3 * time.Second

func (r *ReconnectingSession) Close() error {
	var err error
	r.closeOnce.Do(func() {
		if r.cancel != nil {
			r.cancel()
		}
		err = r.getInner().Close()
		select {
		case <-r.stopped:
		case <-time.After(closeWaitTimeout):
			logx.Printf("livemode: ReconnectingSession.Close timed out waiting for supervise() to exit")
		}
	})
	return err
}

var _ Session = (*ReconnectingSession)(nil)
