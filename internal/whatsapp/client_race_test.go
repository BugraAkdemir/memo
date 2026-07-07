package whatsapp

import (
	"errors"
	"sync"
	"testing"

	waEvent "go.mau.fi/whatsmeow/types/events"
)

// TestHandleEventConcurrentAccess is a regression test for BUG-C2: handleEvent
// used to write c.qrCodes/c.lastError/c.started/c.reconnecting with no lock,
// while QRCodes()/LastError()/IsReconnecting()/IsConnected() read them under
// startMu — a classic data race. Fixed in 854e04d by taking startMu around
// every handleEvent write. Run with -race to catch any regression.
func TestHandleEventConcurrentAccess(t *testing.T) {
	c := NewClient(Config{})

	events := []any{
		&waEvent.QR{Codes: []string{"code-1", "code-2"}},
		&waEvent.PairSuccess{},
		&waEvent.PairError{Error: errors.New("boom")},
		&waEvent.Connected{},
		&waEvent.Disconnected{},
	}

	var wg sync.WaitGroup
	const iterations = 200

	for _, evt := range events {
		wg.Go(func() {
			for range iterations {
				c.handleEvent(evt)
			}
		})
	}

	for range 4 {
		wg.Go(func() {
			for range iterations {
				_ = c.QRCodes()
				_ = c.LastError()
				_ = c.IsReconnecting()
				_ = c.IsConnected()
				_ = c.IsLoggedIn()
			}
		})
	}

	wg.Wait()
}
