// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"sync"
	"time"
)

// bgGroup tracks short-lived follow-up work that a chat turn leaves running
// after its reply has been returned (today: the chat-title generation, an LLM
// round trip followed by a write to the session store).
//
// Fire-and-forget is right for the user — the reply must not wait for a title
// — but it means the work can still be in flight when the thing it writes to
// goes away. In production that is shutdown; in tests it is the temp
// directory, whose removal then raced the late session write and failed the
// run with "TempDir RemoveAll cleanup: ... directory not empty" on whichever
// test happened to lose. Tracking the work gives both a way to wait for it.
//
// It is a mutex and a counter rather than a sync.WaitGroup because new work
// may start while someone is already waiting, which a WaitGroup does not allow
// at a zero count.
type bgGroup struct {
	mu   sync.Mutex
	n    int
	idle chan struct{} // closed when n falls back to 0
}

func (g *bgGroup) start() {
	g.mu.Lock()
	if g.n == 0 {
		g.idle = make(chan struct{})
	}
	g.n++
	g.mu.Unlock()
}

func (g *bgGroup) done() {
	g.mu.Lock()
	g.n--
	if g.n == 0 {
		close(g.idle)
	}
	g.mu.Unlock()
}

// wait blocks until no tracked work is running, or the timeout passes. It
// reports whether everything finished.
func (g *bgGroup) wait(timeout time.Duration) bool {
	g.mu.Lock()
	if g.n == 0 {
		g.mu.Unlock()
		return true
	}
	idle := g.idle
	g.mu.Unlock()

	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case <-idle:
		return true
	case <-t.C:
		return false
	}
}

// goBackground runs fn like goRecover (a panic is logged, not fatal) and
// records it so waitBackground can wait for it. Use it for short follow-up
// work that touches state with a lifetime — never for the long-lived loops and
// servers started at startup, which would make the wait never return.
func (a *App) goBackground(label string, fn func()) {
	a.bg.start()
	go func() {
		defer a.bg.done()
		defer recoverPanic(label)
		fn()
	}()
}

// waitBackground waits, up to timeout, for the work started with goBackground.
// It reports whether all of it finished.
func (a *App) waitBackground(timeout time.Duration) bool { return a.bg.wait(timeout) }
