// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ─── Swarm (Memo Swarm — multi-machine llama.cpp RPC) ───────────────

func (s *Server) handleSwarmStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	writeJSON(w, s.fullBridge.SwarmStatusSnapshot())
}

func (s *Server) handleSwarmHostCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	var req struct {
		ModelPath string `json:"model_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	code, err := s.fullBridge.HostSwarmCreate(req.ModelPath)
	if err != nil {
		http.Error(w, err.Error(), swarmHTTPStatus(err))
		return
	}
	writeJSON(w, map[string]string{"ok": "true", "room_code": code})
}

// handleSwarmAddWorker is the worker-initiated registration endpoint.
// Exempt from remoteAuthMiddleware's X-Memo-Token check (a joining worker
// has no token) — auth is the room id+secret pair validated inside App.
func (s *Server) handleSwarmAddWorker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	var req struct {
		ID           string `json:"id"`
		Secret       string `json:"secret"`
		MyRPCAddress string `json:"my_rpc_address"`
		Label        string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.ID == "" || req.Secret == "" || req.MyRPCAddress == "" {
		http.Error(w, "id, secret, and my_rpc_address are required", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.HostSwarmAddWorker(req.ID, req.Secret, req.MyRPCAddress, req.Label); err != nil {
		http.Error(w, err.Error(), swarmHTTPStatus(err))
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleSwarmRemoveWorker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	var req struct {
		WorkerID string `json:"worker_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.HostSwarmRemoveWorker(req.WorkerID); err != nil {
		http.Error(w, err.Error(), swarmHTTPStatus(err))
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleSwarmReorderWorkers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	var req struct {
		FromIndex int `json:"from_index"`
		ToIndex   int `json:"to_index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.HostSwarmReorderWorkers(req.FromIndex, req.ToIndex); err != nil {
		http.Error(w, err.Error(), swarmHTTPStatus(err))
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleSwarmSetShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	var req struct {
		WorkerID     string  `json:"worker_id"`
		SharePercent float64 `json:"share_percent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.HostSwarmSetShare(req.WorkerID, req.SharePercent); err != nil {
		http.Error(w, err.Error(), swarmHTTPStatus(err))
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleSwarmStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	var req struct {
		CtxSize int `json:"ctx_size"`
	}
	// Body optional — zero ctx_size lets App fall back to config default.
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := s.fullBridge.HostSwarmStart(req.CtxSize); err != nil {
		http.Error(w, err.Error(), swarmHTTPStatus(err))
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleSwarmStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	if err := s.fullBridge.HostSwarmStop(); err != nil {
		http.Error(w, err.Error(), swarmHTTPStatus(err))
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleSwarmClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	if err := s.fullBridge.HostSwarmClose(); err != nil {
		http.Error(w, err.Error(), swarmHTTPStatus(err))
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleSwarmJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.JoinSwarm(req.Code); err != nil {
		http.Error(w, err.Error(), swarmHTTPStatus(err))
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleSwarmLeave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	if err := s.fullBridge.LeaveSwarm(); err != nil {
		http.Error(w, err.Error(), swarmHTTPStatus(err))
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

// swarmHTTPStatus maps App-layer swarm errors to HTTP codes. Room-secret
// failures are 401 (joining worker auth); Beta-gate is 403; everything else
// is 500 (or 400 for clear client mistakes).
func swarmHTTPStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "invalid room secret"):
		return http.StatusUnauthorized
	case strings.Contains(msg, "beta özelliğidir") || strings.Contains(msg, "Beta"):
		return http.StatusForbidden
	case strings.Contains(msg, "required") ||
		strings.Contains(msg, "not a valid room code") ||
		strings.Contains(msg, "malformed room code") ||
		strings.Contains(msg, "no active room") ||
		strings.Contains(msg, "no workers") ||
		strings.Contains(msg, "model path") ||
		strings.Contains(msg, "model not found"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// isSwarmWorkerAddPath reports whether path is the worker-registration
// endpoint (including the /api/v1/ alias). That path is exempt from
// remoteAuthMiddleware's X-Memo-Token check — a joining worker has no token.
func isSwarmWorkerAddPath(path string) bool {
	return path == "/api/swarm/host/workers/add" ||
		path == "/api/v1/swarm/host/workers/add"
}
