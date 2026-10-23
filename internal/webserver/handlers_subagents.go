package webserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"memo/internal/subagents"
)

type chatSubAgentBridge interface {
	ListChatSubAgents(string) ([]subagents.Job, error)
	GetChatSubAgent(string, string) (subagents.Job, error)
	ChatSubAgentAction(context.Context, string, string, json.RawMessage) (string, error)
	GetChatSubAgentSettings() map[string]int
	UpdateChatSubAgentSettings(int) error
}

func (s *Server) handleChatSubAgents(w http.ResponseWriter, r *http.Request) {
	bridge, ok := s.bridge.(chatSubAgentBridge)
	if !ok {
		http.Error(w, "unavailable", http.StatusNotImplemented)
		return
	}
	chat := r.URL.Query().Get("chat_id")
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", 405)
		return
	}
	if chat == "" {
		http.Error(w, "chat_id required", 400)
		return
	}
	jobs, err := bridge.ListChatSubAgents(chat)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if jobs == nil {
		jobs = []subagents.Job{}
	}
	writeJSON(w, jobs)
}
func (s *Server) handleChatSubAgentByID(w http.ResponseWriter, r *http.Request) {
	bridge, ok := s.bridge.(chatSubAgentBridge)
	if !ok {
		http.Error(w, "unavailable", 501)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/subagents/")
	parts := strings.Split(rest, "/")
	if len(parts) < 1 || parts[0] == "" || len(parts) > 2 {
		http.Error(w, "invalid sub-agent path", 400)
		return
	}
	id := parts[0]
	if r.Method == http.MethodGet && len(parts) == 1 {
		j, err := bridge.GetChatSubAgent(r.URL.Query().Get("chat_id"), id)
		if err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		writeJSON(w, j)
		return
	}
	if r.Method != http.MethodPost || len(parts) != 2 {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		ChatID string `json:"chat_id"`
		Prompt string `json:"prompt"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024))
	dec.DisallowUnknownFields()
	if dec.Decode(&req) != nil || req.ChatID == "" {
		http.Error(w, "valid chat_id required", 400)
		return
	}
	action := parts[1]
	switch action {
	case "send", "cancel", "close", "merge", "resume":
	default:
		http.Error(w, "unknown action", 400)
		return
	}
	args, _ := json.Marshal(map[string]string{"id": id, "prompt": req.Prompt})
	result, err := bridge.ChatSubAgentAction(r.Context(), req.ChatID, action, args)
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(result))
}
func (s *Server) handleChatSubAgentSettings(w http.ResponseWriter, r *http.Request) {
	bridge, ok := s.bridge.(chatSubAgentBridge)
	if !ok {
		http.Error(w, "unavailable", 501)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, bridge.GetChatSubAgentSettings())
	case http.MethodPut:
		var req struct {
			MaxParallel int `json:"max_parallel"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		dec.DisallowUnknownFields()
		if dec.Decode(&req) != nil {
			http.Error(w, "invalid settings", 400)
			return
		}
		if err := bridge.UpdateChatSubAgentSettings(req.MaxParallel); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, bridge.GetChatSubAgentSettings())
	default:
		http.Error(w, "method not allowed", 405)
	}
}
