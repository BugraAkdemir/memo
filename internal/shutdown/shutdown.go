// Package shutdown provides a single process-wide, cross-platform way for
// any part of the backend to ask main() to begin a graceful shutdown.
//
// It exists because the previous mechanism — self-signaling via
// os.Process.Signal(os.Interrupt), used by both the client-registry
// auto-shutdown (internal/app/clients.go) and the POST /api/shutdown HTTP
// handler (internal/webserver/handlers_flutter.go) — is a silent no-op on
// Windows: Go's os package only implements Process.Signal for os.Kill
// there; any other signal, including os.Interrupt, returns
// syscall.EWINDOWS, which every caller here discarded. A same-process
// channel send has no such platform gap.
package shutdown

// ch is process-wide by design: there is exactly one backend App per OS
// process, same as the os.Signal channel main() already selects on.
var ch = make(chan struct{}, 1)

// Request asks main()'s signal-wait loop to begin the graceful shutdown
// sequence — the same effect an external SIGINT/SIGTERM has. Safe to call
// more than once; extra requests after the first are dropped, not queued.
func Request() {
	select {
	case ch <- struct{}{}:
	default:
	}
}

// Requested returns the channel main() selects on alongside its OS signal
// channel.
func Requested() <-chan struct{} {
	return ch
}
