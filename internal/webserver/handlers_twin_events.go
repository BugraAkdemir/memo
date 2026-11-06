package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Owner-only SSE notifications reuse the existing sequenced event ring. A
// reconnect fetches authoritative activity; this bounded stream is a change
// signal, never a durable message queue or a source of private draft contents.
func (s *Server) handleTwinEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	source, ok := s.bridge.(interface{ GetEvents() []map[string]string })
	if !ok && s.fullBridge != nil {
		source = s.fullBridge
		ok = true
	}
	if !ok {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}
	var cursor uint64
	if raw := r.Header.Get("Last-Event-ID"); raw != "" {
		var err error
		cursor, err = strconv.ParseUint(raw, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	if _, err := fmt.Fprint(w, ": ready\n\n"); err != nil {
		return
	}
	flusher.Flush()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	first := true
	for {
		events := source.GetEvents()
		if first {
			first = false
			var latest uint64
			for _, event := range events {
				n, _ := strconv.ParseUint(event["seq"], 10, 64)
				if n > latest {
					latest = n
				}
			}
			if cursor > latest {
				cursor = 0
			}
		}
		for _, event := range events {
			seq, err := strconv.ParseUint(event["seq"], 10, 64)
			if err != nil || seq <= cursor {
				continue
			}
			cursor = seq
			if event["name"] != "autonomy:action_pending" && event["name"] != "autonomy:action_executed" {
				continue
			}
			// Re-encode an allow-list even if another emitter adds private fields later.
			var signal struct {
				ID    string `json:"id"`
				State string `json:"state"`
				Code  int64  `json:"code,omitempty"`
			}
			if err = json.Unmarshal([]byte(event["data"]), &signal); err != nil || signal.ID == "" {
				continue
			}
			raw, err := json.Marshal(signal)
			if err != nil {
				continue
			}
			if _, err = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", seq, event["name"], raw); err != nil {
				return
			}
			flusher.Flush()
		}
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
		case <-ping.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
