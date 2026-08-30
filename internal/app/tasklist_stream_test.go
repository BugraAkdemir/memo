package app

import (
	"strings"
	"testing"
	"time"

	"memo/internal/provider"
)

func TestDrainStreamIdle_HappyPath(t *testing.T) {
	ch := make(chan provider.StreamChunk, 3)
	ch <- provider.StreamChunk{Content: "hel"}
	ch <- provider.StreamChunk{Content: "lo"}
	close(ch)

	out, err := drainStreamIdle(ch, func() {}, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != "hello" {
		t.Fatalf("out = %q", out)
	}
}

func TestDrainStreamIdle_ErrorChunk(t *testing.T) {
	ch := make(chan provider.StreamChunk, 2)
	ch <- provider.StreamChunk{Content: "part"}
	ch <- provider.StreamChunk{Error: "boom"}
	close(ch)

	out, err := drainStreamIdle(ch, func() {}, time.Second)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("want boom error, got out=%q err=%v", out, err)
	}
}

func TestDrainStreamIdle_AbortsOnStall_ButKeepsFlowingStreamAlive(t *testing.T) {
	// A stream that keeps producing every 20ms must NOT be aborted by a 60ms
	// idle budget, even well past that budget in total.
	ch := make(chan provider.StreamChunk)
	go func() {
		for i := 0; i < 10; i++ {
			ch <- provider.StreamChunk{Content: "x"}
			time.Sleep(20 * time.Millisecond)
		}
		close(ch)
	}()
	out, err := drainStreamIdle(ch, func() {}, 60*time.Millisecond)
	if err != nil {
		t.Fatalf("a steadily-flowing stream was aborted: %v", err)
	}
	if out != "xxxxxxxxxx" {
		t.Fatalf("out = %q", out)
	}

	// A stream that goes silent past the budget IS aborted, and cancel fires.
	ch2 := make(chan provider.StreamChunk)
	go func() {
		ch2 <- provider.StreamChunk{Content: "start"}
		// then never send again
	}()
	cancelled := make(chan struct{}, 1)
	out2, err2 := drainStreamIdle(ch2, func() { cancelled <- struct{}{} }, 50*time.Millisecond)
	if err2 == nil || !strings.Contains(err2.Error(), "idle") {
		t.Fatalf("want idle-abort error, got %v", err2)
	}
	if out2 != "start" {
		t.Fatalf("partial output not returned: %q", out2)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("cancel() was not called on idle abort")
	}
}

func TestDrainStreamIdle_ZeroDisablesGuard(t *testing.T) {
	ch := make(chan provider.StreamChunk)
	go func() {
		time.Sleep(80 * time.Millisecond) // longer than any small budget
		ch <- provider.StreamChunk{Content: "late"}
		close(ch)
	}()
	out, err := drainStreamIdle(ch, func() { t.Error("cancel called with guard disabled") }, 0)
	if err != nil || out != "late" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
