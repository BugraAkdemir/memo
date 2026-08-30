package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"memo/internal/provider"
)

// streamIdleTimeout is the "no new token for this long → give up" budget for a
// Self-Driving planner / coder / escalator LLM stream. Total run time stays
// unbounded: a plan that streams for hours is fine as long as tokens keep
// coming. Configurable via TaskLoopConfig.StreamIdleTimeoutSec (default 300).
func (a *App) streamIdleTimeout() time.Duration {
	a.cfgMu.RLock()
	s := a.cfg.TaskLoop.StreamIdleTimeoutSec
	a.cfgMu.RUnlock()
	if s <= 0 {
		s = 300
	}
	return time.Duration(s) * time.Second
}

// drainStreamIdle reads streamCh into a single string. It aborts via cancel
// only when no chunk has arrived for `idle` (a stalled/hung stream); a stream
// that keeps producing tokens runs for as long as it likes. A chunk carrying
// an Error ends the read with that error. idle <= 0 disables the guard.
func drainStreamIdle(streamCh <-chan provider.StreamChunk, cancel context.CancelFunc, idle time.Duration) (string, error) {
	var sb strings.Builder

	if idle <= 0 {
		for chunk := range streamCh {
			if chunk.Error != "" {
				return sb.String(), fmt.Errorf("%s", chunk.Error)
			}
			sb.WriteString(chunk.Content)
		}
		return sb.String(), nil
	}

	timer := time.NewTimer(idle)
	defer timer.Stop()
	for {
		select {
		case chunk, ok := <-streamCh:
			if !ok {
				return sb.String(), nil
			}
			if chunk.Error != "" {
				return sb.String(), fmt.Errorf("%s", chunk.Error)
			}
			sb.WriteString(chunk.Content)
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(idle)
		case <-timer.C:
			cancel()
			// Let the producer goroutine finish rather than leak it.
			go func() {
				for range streamCh {
				}
			}()
			return sb.String(), fmt.Errorf("taskloop: LLM stream idle for %s with no new token", idle)
		}
	}
}
