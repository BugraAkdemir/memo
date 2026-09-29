package app

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Plain-chat stream timing (provider and local-model paths in callLLMStream).
//
// Both paths used a single context.WithTimeout(ctx, 300s): a TOTAL deadline
// on the whole turn. A reply that was still arriving token by token was
// therefore cut at the 300th second — exactly what happens in a long
// session, where the prompt is large (slow prefill before the first token)
// and answers get long, and most of all on a slow local model — and the cut
// was then saved as "⏹️ Response stopped." as if the user had pressed stop.
// Found live against a fake provider (250s to first token, then a slow
// stream).
//
// What should end a turn is silence, not length: streamIdleTimeout is how
// long a stream may go without producing anything (including the wait for
// its first token — a CPU prefill of a long session legitimately takes
// minutes), and streamTotalCap is a generous backstop so a stream that
// trickles forever still ends. Vars only so tests can shorten them.
var (
	streamIdleTimeout = 300 * time.Second
	streamTotalCap    = 30 * time.Minute
)

var (
	errStreamIdle     = errors.New("stream idle timeout")
	errStreamTotalCap = errors.New("stream total time cap")
)

// streamWatchdogCtx derives the context a chat stream runs under. touch
// must be called whenever the stream produces something; stop releases the
// timers and must always be called (defer it).
func streamWatchdogCtx(parent context.Context) (ctx context.Context, touch func(), stop func()) {
	ctx, cancel := context.WithCancelCause(parent)
	idle := time.AfterFunc(streamIdleTimeout, func() { cancel(errStreamIdle) })
	capTimer := time.AfterFunc(streamTotalCap, func() { cancel(errStreamTotalCap) })
	touch = func() { idle.Reset(streamIdleTimeout) }
	stop = func() {
		idle.Stop()
		capTimer.Stop()
		cancel(nil)
	}
	return ctx, touch, stop
}

// streamTimeoutMsg reports why a stream's watchdog ended it, as the notice to
// show and save, or "" when it didn't (the turn is still running, or the
// caller's own context ended — the user pressed stop or went away).
func (a *App) streamTimeoutMsg(parent, streamCtx context.Context) string {
	if parent.Err() != nil {
		return ""
	}
	switch cause := context.Cause(streamCtx); {
	case errors.Is(cause, errStreamIdle):
		secs := int(streamIdleTimeout / time.Second)
		return fmt.Sprintf(a.t(
			"⏱️ Model %d saniye boyunca hiçbir şey göndermediği için istek durduruldu. Model çok yavaş ya da takılmış olabilir; tekrar deneyebilirsin.",
			"⏱️ The request was stopped because the model sent nothing for %d seconds. It may be very slow or stuck — you can try again."), secs)
	case errors.Is(cause, errStreamTotalCap):
		mins := int(streamTotalCap / time.Minute)
		return fmt.Sprintf(a.t(
			"⏱️ Yanıt %d dakikalık üst sınıra ulaştığı için durduruldu.",
			"⏱️ The reply was stopped at the %d-minute limit."), mins)
	}
	return ""
}
