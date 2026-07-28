package replcli

import (
	"fmt"
	"io"
	"memo/internal/logx"
	"sync"
	"time"
)

// spinner animates a "düşünüyor..." indicator in place while waiting for
// the first byte of a response. Agent mode's backend call is not truly
// streamed (the whole reply arrives as one chunk once the model — and any
// tool calls — finish), so without this the terminal would sit blank and
// look frozen between sending a message and the reply appearing.
type spinner struct {
	out     io.Writer
	stopCh  chan struct{}
	doneCh  chan struct{}
	stopped bool
	mu      sync.Mutex
	label   string // guarded by mu; read by the ticker goroutine, written by SetLabel
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func newSpinner(out io.Writer) *spinner {
	s := &spinner{out: out, stopCh: make(chan struct{}), doneCh: make(chan struct{}), label: dim(t("spinner_thinking"))}
	go func() {
		defer close(s.doneCh)
		defer logx.Recover("replcli.spinner")
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-s.stopCh:
				fmt.Fprint(s.out, "\r\033[K")
				return
			case <-ticker.C:
				fmt.Fprintf(s.out, "\r%s %s", dim(spinnerFrames[i%len(spinnerFrames)]), s.Label())
				i++
			}
		}
	}()
	return s
}

// Label returns the spinner's current status text — thread-safe against a
// concurrent SetLabel call from the turn's own goroutine.
func (s *spinner) Label() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.label
}

// SetLabel changes what the spinner shows next to its animated frame. Used
// to reflect live agent tool-call activity ("⚙ list_directory çalışıyor...",
// then "✓ list_directory tamamlandı", then the next tool, and so on) in
// place, instead of each step leaving its own permanent line in the
// terminal scrollback — a multi-step agent turn used to print one line per
// tool call, which made an otherwise ordinary request look like a wall of
// noise once the model needed 4-5 tool calls to get something done.
func (s *spinner) SetLabel(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.label = text
}

// Stop is safe to call more than once — only the first call has any effect.
func (s *spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.stopped = true
	close(s.stopCh)
	<-s.doneCh
}

// typewriter reveals text in small batches with a short delay between each,
// simulating live token streaming for the terminal REPL. The total reveal
// time is capped (via a length-scaled batch size) so long replies don't
// take ages to finish printing.
func typewriter(out io.Writer, text string) {
	runes := []rune(text)
	if len(runes) == 0 {
		return
	}
	const ticks = 80
	const tickDelay = 12 * time.Millisecond

	batch := max(len(runes)/ticks, 1)
	for i := 0; i < len(runes); i += batch {
		end := min(i+batch, len(runes))
		fmt.Fprint(out, string(runes[i:end]))
		time.Sleep(tickDelay)
	}
}
