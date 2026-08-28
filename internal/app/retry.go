package app

import (
	"context"
	"time"
)

// retryWithBackoff calls fn immediately and then repeatedly until it
// returns true or ctx is cancelled, sleeping an exponentially growing delay
// (doubling, capped at maxDelay) between attempts. Returns true if fn
// eventually succeeded, false if ctx was cancelled first.
//
// Used by the Telegram / WhatsApp startup connect paths: a single attempt
// at boot fails whenever the network/DNS isn't up yet (common right after a
// reboot — "dial tcp: lookup ...: i/o timeout", getMe "context deadline
// exceeded"), and the bridge would then stay dead until a manual reconnect.
func retryWithBackoff(ctx context.Context, initialDelay, maxDelay time.Duration, fn func() bool) bool {
	delay := initialDelay
	for {
		if fn() {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(delay):
		}
		if delay < maxDelay {
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}
}
