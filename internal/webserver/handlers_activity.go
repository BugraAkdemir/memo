package webserver

import "net/http"

// GET /api/mascot/activity — coarse app-wide "what is Memo doing right
// now" signal, polled by the standalone desktop mascot window, which has
// no chat context of its own to read a per-chat SSE stream from. See
// models.ActivityStatus's doc comment for what it does and doesn't cover.
func (s *Server) handleMascotActivity(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusMethodNotAllowed)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.fullBridge.GetActivityStatus())
}
