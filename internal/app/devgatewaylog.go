// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"sync"
	"time"

	"memo/internal/models"
	"memo/internal/truncate"
)

// gatewayLogCapacity caps the in-memory dev-gateway log. Separate from the
// shared AppEvent ring (internal/app/app.go's eventRing, 64 slots shared
// across every kind of app event at much higher frequency) so a busy chat
// session elsewhere in the app can't evict gateway log entries the user
// opened the Developer screen specifically to watch.
const gatewayLogCapacity = 200

// gatewayLog is a fixed-capacity, append-only (oldest-dropped) log of
// dev-gateway requests, guarded by its own mutex.
type gatewayLog struct {
	mu      sync.Mutex
	entries []models.GatewayLogEntry
	nextSeq uint64
}

func (l *gatewayLog) record(entry models.GatewayLogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.nextSeq++
	entry.Seq = l.nextSeq
	entry.Timestamp = time.Now().Format(time.RFC3339)
	l.entries = append(l.entries, entry)
	if len(l.entries) > gatewayLogCapacity {
		l.entries = l.entries[len(l.entries)-gatewayLogCapacity:]
	}
}

func (l *gatewayLog) snapshot() []models.GatewayLogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]models.GatewayLogEntry, len(l.entries))
	copy(out, l.entries)
	return out
}

// RecordGatewayLog appends one dev-gateway request/response entry — called
// by the /v1/messages handler (internal/webserver/devgateway_handlers.go)
// after every request, success or failure, streaming or not.
func (a *App) RecordGatewayLog(modelSpec string, stream, hasTools bool, requestText, responseText, errMsg string, duration time.Duration) {
	a.devGatewayLog.record(models.GatewayLogEntry{
		Model:           modelSpec,
		Stream:          stream,
		HasTools:        hasTools,
		RequestPreview:  truncate.Text(requestText, 200),
		ResponsePreview: truncate.Text(responseText, 200),
		Error:           errMsg,
		DurationMs:      duration.Milliseconds(),
	})
}

// GetGatewayLogs returns every currently retained dev-gateway log entry,
// oldest first.
func (a *App) GetGatewayLogs() []models.GatewayLogEntry {
	return a.devGatewayLog.snapshot()
}
