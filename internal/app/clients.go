package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"memo/internal/logx"
	"memo/internal/shutdown"
)

// clientStaleAfter and clientSweepEvery govern how quickly a client that
// stopped heartbeating without an explicit goodbye (a killed process, a
// Flutter window closed with no reliable close hook — see cmd/replcli's
// heartbeat loop and chat_provider.dart's connectionStatusProvider on the
// Flutter side) is noticed. staleAfter is ~3x the 30s heartbeat interval
// both clients use, so one missed tick never causes a false disconnect.
const (
	clientStaleAfter = 90 * time.Second
	clientSweepEvery = 15 * time.Second
)

// clientRegistry tracks which processes (the terminal CLI, the Flutter GUI)
// are currently attached to this backend. It exists so a backend spawned
// on demand for a terminal session (main.go, no --headless) can shut itself
// down once nothing needs it anymore, instead of dying the moment whichever
// specific process happened to start it exits — a GUI opened via /gui must
// not crash just because the terminal that started the backend exits, and
// vice versa.
//
// sawClient plus autoShutdown together gate the actual self-shutdown: a
// plain `--headless` backend run as a standalone service (remote access,
// WhatsApp bridge) never has autoShutdown set, so registry activity here is
// harmless bookkeeping and never triggers a shutdown, regardless of clients
// coming and going. Only main.go's own on-demand backend spawn passes
// --auto-shutdown.
type clientRegistry struct {
	mu           sync.Mutex
	clients      map[string]time.Time // client ID -> last heartbeat
	sawClient    bool                 // true once any client has ever registered
	autoShutdown bool
}

// EnableAutoShutdown arms the "no clients left → shut down" behavior. Call
// only from a backend main.go spawned specifically to serve a CLI/GUI
// session on demand — never for a standalone `--headless` run.
func (a *App) EnableAutoShutdown() {
	a.clients.mu.Lock()
	a.clients.autoShutdown = true
	a.clients.mu.Unlock()
}

// RegisterClient records a newly attached client and returns its ID.
func (a *App) RegisterClient() string {
	id := randomClientID()
	a.clients.mu.Lock()
	if a.clients.clients == nil {
		a.clients.clients = make(map[string]time.Time)
	}
	a.clients.clients[id] = time.Now()
	a.clients.sawClient = true
	n := len(a.clients.clients)
	a.clients.mu.Unlock()
	logx.Printf("client attached: %s (%d active)", id, n)
	return id
}

// HeartbeatClient refreshes clientID's last-seen time. Returns an error if
// the ID isn't known (e.g. already pruned as stale) so the caller
// re-registers instead of heartbeating into the void.
func (a *App) HeartbeatClient(clientID string) error {
	a.clients.mu.Lock()
	defer a.clients.mu.Unlock()
	if _, ok := a.clients.clients[clientID]; !ok {
		return fmt.Errorf("unknown client %q", clientID)
	}
	a.clients.clients[clientID] = time.Now()
	return nil
}

// UnregisterClient removes clientID immediately (a graceful goodbye) and, if
// that was the last attached client and auto-shutdown is armed, triggers the
// same shutdown path /api/shutdown uses.
func (a *App) UnregisterClient(clientID string) {
	a.clients.mu.Lock()
	delete(a.clients.clients, clientID)
	n := len(a.clients.clients)
	shouldShutdown := n == 0 && a.clients.sawClient && a.clients.autoShutdown
	a.clients.mu.Unlock()
	logx.Printf("client detached: %s (%d active)", clientID, n)
	if shouldShutdown {
		a.selfShutdownIfIdle("last client disconnected")
	}
}

// startClientSweep prunes clients that stopped heartbeating without an
// explicit unregister. Tied to lifecycleCtx like Startup's other background
// loops, so it stops itself on shutdown.
func (a *App) startClientSweep(ctx context.Context) {
	ticker := time.NewTicker(clientSweepEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.sweepStaleClients()
		}
	}
}

func (a *App) sweepStaleClients() {
	cutoff := time.Now().Add(-clientStaleAfter)
	a.clients.mu.Lock()
	for id, last := range a.clients.clients {
		if last.Before(cutoff) {
			delete(a.clients.clients, id)
			logx.Printf("client %s timed out, removed", id)
		}
	}
	n := len(a.clients.clients)
	shouldShutdown := n == 0 && a.clients.sawClient && a.clients.autoShutdown
	a.clients.mu.Unlock()
	if shouldShutdown {
		a.selfShutdownIfIdle("all clients timed out")
	}
}

// selfShutdownSignal is the actual self-signal call, factored into a
// swappable var (same technique as shutdownForceExit in shutdown.go) so
// tests can exercise the "no clients left" decision without triggering a
// real process-wide shutdown request.
//
// There's an inherent gap between this decision (registry was empty) and
// the signal actually being handled in main.go — a new client (e.g. /gui
// spawning right as the last CLI session disconnects) can register in that
// window. main.go's signal-wait loop re-checks HasActiveClients() right
// before committing to shut down and ignores a stale signal if one showed
// up, so this call itself doesn't need to be perfectly race-free.
var selfShutdownSignal = shutdown.Request

// selfShutdownIfIdle requests the same graceful shutdown
// webserver.handleShutdown does on an explicit POST /api/shutdown, so
// main()'s existing deferred Shutdown chain runs — no client is left
// attached to justify staying up.
func (a *App) selfShutdownIfIdle(reason string) {
	logx.Printf("no clients attached (%s) — shutting down", reason)
	selfShutdownSignal()
}

// HasActiveClients reports whether any client is currently registered. Used
// to re-verify a self-shutdown decision right before it's acted on — see the
// race note on selfShutdownSignal.
func (a *App) HasActiveClients() bool {
	a.clients.mu.Lock()
	defer a.clients.mu.Unlock()
	return len(a.clients.clients) > 0
}

func randomClientID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("client-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
