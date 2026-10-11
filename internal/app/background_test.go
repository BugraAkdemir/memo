// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"sync/atomic"
	"testing"
	"time"
)

// drainBackground makes the test wait for the follow-up work its App left
// running after a turn (the chat-title generation: an LLM round trip, then a
// write to the session store) before the test's temp directories are removed.
//
// It must be called AFTER the test's last t.TempDir(): cleanups run in reverse
// order of registration, so only then does this wait run first. Without it a
// late title write races the directory removal and the run fails with
// "TempDir RemoveAll cleanup: ... directory not empty" on whichever test
// loses — the flake that kept turning CI red.
func drainBackground(t *testing.T, a *App) {
	t.Helper()
	t.Cleanup(func() {
		if !a.waitBackground(10 * time.Second) {
			t.Errorf("follow-up work started by the app was still running 10s after the test finished")
		}
	})
}

func TestBackground_WaitReturnsOnceTheWorkIsDone(t *testing.T) {
	a := &App{}
	release := make(chan struct{})
	var finished atomic.Bool
	a.goBackground("test", func() {
		<-release
		finished.Store(true)
	})

	if a.waitBackground(30 * time.Millisecond) {
		t.Fatal("waitBackground reported done while the work was still blocked")
	}
	close(release)
	if !a.waitBackground(5 * time.Second) {
		t.Fatal("waitBackground did not return after the work finished")
	}
	if !finished.Load() {
		t.Fatal("waitBackground returned before the work had finished")
	}
}

func TestBackground_NothingRunningIsImmediatelyDone(t *testing.T) {
	a := &App{}
	if !a.waitBackground(time.Millisecond) {
		t.Fatal("waitBackground with nothing started must report done at once")
	}
}

func TestBackground_APanicStillCountsAsFinished(t *testing.T) {
	a := &App{}
	a.goBackground("test", func() { panic("boom") })
	if !a.waitBackground(5 * time.Second) {
		t.Fatal("a panicking task was never marked finished, so every wait would hang")
	}
}

func TestBackground_WorkStartedWhileWaitingIsWaitedForNextTime(t *testing.T) {
	a := &App{}
	first := make(chan struct{})
	a.goBackground("first", func() { <-first })

	second := make(chan struct{})
	waited := make(chan bool, 1)
	go func() { waited <- a.waitBackground(5 * time.Second) }()
	time.Sleep(20 * time.Millisecond)
	a.goBackground("second", func() { <-second }) // joins while a wait is in progress
	close(first)
	close(second)

	if !<-waited {
		t.Fatal("the wait did not finish")
	}
	if !a.waitBackground(5 * time.Second) {
		t.Fatal("work started during a wait left the group unusable")
	}
}
