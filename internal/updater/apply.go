// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ApplyOptions describes one in-place update of a script-installed Memo.
type ApplyOptions struct {
	Install  string // the directory holding memo-backend and run_memo.sh
	Staged   string // the unpacked new release (its top-level directory)
	Backup   string // where the replaced files are kept until the new one proves itself
	WaitPIDs []int  // processes that must be gone before any file is touched
	// Expect is the version the relaunched backend must report ("V4.7.0").
	Expect string

	WaitTimeout   time.Duration // for WaitPIDs; default 90s
	HealthTimeout time.Duration // for the relaunched backend; default 120s
	PollEvery     time.Duration // default 500ms

	Log func(format string, args ...any)
}

// ApplyDeps are the parts that touch the outside world, so the sequence can be
// tested without launching a real Memo.
type ApplyDeps struct {
	// Relaunch starts Memo.
	Relaunch func() (*Launched, error)
	// Health returns the version the running backend reports.
	Health func(ctx context.Context) (string, error)
	// AliveFn reports whether a pid is still running (default: the OS check).
	AliveFn func(pid int) bool
}

// Launched is a Memo the updater started.
type Launched struct {
	// Stop ends everything it started.
	Stop func()
	// Exited is closed when the launcher process ends. A launcher that ends
	// before the backend ever answered means the new version died on start, and
	// there is no point waiting out the health timeout.
	Exited <-chan struct{}
}

// ErrRolledBack means the new version did not come up and the old one was put
// back (and relaunched). The update did not happen; nothing was lost.
var ErrRolledBack = errors.New("updater: the new version did not start; the previous version was restored")

// rolledBackError carries WHY the new version failed while still matching
// errors.Is(err, ErrRolledBack).
type rolledBackError struct{ cause error }

func (e *rolledBackError) Error() string        { return ErrRolledBack.Error() + " (" + e.cause.Error() + ")" }
func (e *rolledBackError) Is(target error) bool { return target == ErrRolledBack }
func (e *rolledBackError) Unwrap() error        { return e.cause }

// Apply waits for Memo to exit, swaps the new release in, relaunches, and
// checks that the new backend answers with the expected version. Any failure
// after the first file moved restores the previous version and relaunches it.
func Apply(ctx context.Context, o ApplyOptions, d ApplyDeps) error {
	logf := o.Log
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if o.WaitTimeout == 0 {
		o.WaitTimeout = 90 * time.Second
	}
	if o.HealthTimeout == 0 {
		o.HealthTimeout = 120 * time.Second
	}
	if o.PollEvery == 0 {
		o.PollEvery = 500 * time.Millisecond
	}
	alive := d.AliveFn
	if alive == nil {
		alive = pidAlive
	}

	// 1. Nothing may be replaced while the old Memo is still running.
	logf("waiting for pids %v to exit", o.WaitPIDs)
	if err := waitGone(ctx, o.WaitPIDs, alive, o.WaitTimeout, o.PollEvery); err != nil {
		return err
	}

	// 2. Swap.
	logf("swapping %s into %s", o.Staged, o.Install)
	sw, err := PlaceNew(o.Install, o.Staged, o.Backup)
	if err != nil {
		// PlaceNew already undid its partial work. Bring the old Memo back so the
		// user isn't left with nothing running.
		logf("swap failed, nothing changed: %v", err)
		if _, rerr := d.Relaunch(); rerr != nil {
			logf("could not relaunch the previous version: %v", rerr)
		}
		return err
	}

	// 3. Start the new one and wait for it to say who it is.
	logf("relaunching")
	l, err := d.Relaunch()
	if err != nil {
		logf("relaunch failed: %v", err)
		return rollBack(sw, nil, d, logf, err)
	}
	got, herr := waitHealthy(ctx, d.Health, l.Exited, o.Expect, o.HealthTimeout, o.PollEvery)
	if herr != nil {
		logf("new version unhealthy (reported %q): %v", got, herr)
		return rollBack(sw, l, d, logf, herr)
	}

	logf("new version %s is up; discarding the backup", got)
	if err := sw.Commit(); err != nil {
		logf("could not delete the backup (harmless): %v", err)
	}
	return nil
}

func rollBack(sw *Swap, l *Launched, d ApplyDeps, logf func(string, ...any), cause error) error {
	if l != nil && l.Stop != nil {
		l.Stop()
		// Give the half-started process a moment to let go of its files.
		time.Sleep(500 * time.Millisecond)
	}
	if err := sw.Rollback(); err != nil {
		logf("ROLLBACK FAILED: %v", err)
		return fmt.Errorf("updater: update failed (%v) and the rollback also failed: %w", cause, err)
	}
	logf("previous version restored; relaunching the previous version")
	if _, err := d.Relaunch(); err != nil {
		logf("could not relaunch the previous version: %v", err)
	}
	return &rolledBackError{cause: cause}
}

func waitGone(ctx context.Context, pids []int, alive func(int) bool, timeout, every time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		running := false
		for _, p := range pids {
			if p > 0 && alive(p) {
				running = true
				break
			}
		}
		if !running {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("updater: Memo is still running after %v; not touching its files", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(every):
		}
	}
}

func waitHealthy(ctx context.Context, health func(context.Context) (string, error), exited <-chan struct{}, expect string, timeout, every time.Duration) (string, error) {
	if health == nil {
		return "", errors.New("updater: no health check")
	}
	want, err := ParseVersion(expect)
	if err != nil {
		return "", err
	}
	deadline := time.Now().Add(timeout)
	last := ""
	var lastErr error
	for {
		hctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		v, err := health(hctx)
		cancel()
		if err == nil {
			last = v
			if got, perr := ParseVersion(v); perr == nil && got == want {
				return v, nil
			}
			lastErr = fmt.Errorf("backend reports %q, expected %q", v, expect)
		} else {
			lastErr = err
		}
		select {
		case <-exited: // nil channel never fires
			return last, errors.New("the new version stopped before it was ready")
		default:
		}
		if time.Now().After(deadline) {
			return last, fmt.Errorf("updater: new version not healthy after %v: %w", timeout, lastErr)
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(every):
		}
	}
}
