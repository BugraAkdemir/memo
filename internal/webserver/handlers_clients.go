package webserver

import (
	"encoding/json"
	"net/http"
)

// handleClientRegister attaches a new CLI/GUI client and returns its ID —
// the caller must pass this ID back on every heartbeat and on unregister.
func (s *Server) handleClientRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]string{"client_id": s.bridge.RegisterClient()})
}

// handleClientHeartbeat refreshes a registered client's last-seen time.
// Returns 404 if the ID isn't known (e.g. already pruned as stale), so the
// caller knows to register again instead of heartbeating into the void.
func (s *Server) handleClientHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ClientID string `json:"client_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.bridge.HeartbeatClient(req.ClientID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

// handleClientUnregister is a client's graceful goodbye — if it was the
// last one attached, the backend may shut itself down (see
// internal/app/clients.go's autoShutdown gate).
func (s *Server) handleClientUnregister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ClientID string `json:"client_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	s.bridge.UnregisterClient(req.ClientID)
	writeJSON(w, map[string]string{"status": "ok"})
}
