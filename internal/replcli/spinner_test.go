package replcli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestTypewriter_RevealsFullText(t *testing.T) {
	var out bytes.Buffer
	typewriter(&out, "Merhaba dünya!")
	if out.String() != "Merhaba dünya!" {
		t.Errorf("got %q, want full text preserved", out.String())
	}
}

func TestTypewriter_EmptyTextIsNoop(t *testing.T) {
	var out bytes.Buffer
	typewriter(&out, "")
	if out.Len() != 0 {
		t.Errorf("expected no output for empty text, got %q", out.String())
	}
}

func TestTypewriter_BoundedDuration(t *testing.T) {
	var out bytes.Buffer
	long := strings.Repeat("a", 5000)
	start := time.Now()
	typewriter(&out, long)
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("typewriter took %v for a long reply, want it capped well under 3s", elapsed)
	}
	if out.String() != long {
		t.Error("long text not fully reproduced")
	}
}

func TestSpinner_StopIsIdempotent(t *testing.T) {
	var out bytes.Buffer
	sp := newSpinner(&out)
	sp.Stop()
	sp.Stop() // must not panic (double close) or block
}

// TestSpinner_StopDoesNotDeadlockWithConcurrentTick is a regression test:
// Stop() used to hold its mutex across a blocking <-doneCh wait, while the
// ticker goroutine's own render tick needs that same mutex (via Label()) to
// make progress back to the loop's stopCh check — a real deadlock window,
// not a data race, so `go test -race` alone would never have caught it.
// Sleeping roughly one tick interval before each Stop() call maximizes the
// odds of landing on the exact race window across many trials; a regression
// hangs the inner goroutine forever, caught here via the outer timeout
// instead of hanging the whole test binary.
func TestSpinner_StopDoesNotDeadlockWithConcurrentTick(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 20 {
			var out bytes.Buffer
			sp := newSpinner(&out)
			time.Sleep(80 * time.Millisecond)
			sp.Stop()
		}
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("spinner.Stop() appears to have deadlocked against a concurrent tick")
	}
}

func TestSpinner_SetLabel_ChangesDisplayedText(t *testing.T) {
	var out bytes.Buffer
	sp := newSpinner(&out)
	defer sp.Stop()

	before := sp.Label()
	sp.SetLabel("⚙ list_directory çalışıyor...")
	after := sp.Label()

	if after == before {
		t.Fatalf("Label() unchanged after SetLabel, got %q both times", after)
	}
	if !strings.Contains(after, "list_directory") {
		t.Errorf("Label() = %q, want it to contain the new text", after)
	}
}
