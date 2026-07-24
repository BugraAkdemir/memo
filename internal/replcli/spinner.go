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
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func newSpinner(out io.Writer) *spinner {
	s := &spinner{out: out, stopCh: make(chan struct{}), doneCh: make(chan struct{})}
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
				fmt.Fprintf(s.out, "\r%s", dim(spinnerFrames[i%len(spinnerFrames)]+" "+t("spinner_thinking")))
				i++
			}
		}
	}()
	return s
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
