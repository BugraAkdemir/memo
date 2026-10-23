package sessions

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// HasPersistedCompletion reads the saved file, not the in-memory session.
// Mailbox ACK must never discard a durable child result merely because a
// failed session write left its marker in memory.
func (m *Manager) HasPersistedCompletion(sessionID string, id uint64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.sessions[sessionID] == nil {
		return false
	}
	b, err := os.ReadFile(filepath.Join(m.dir, sessionID+".json"))
	if err != nil {
		return false
	}
	var s Session
	if json.Unmarshal(b, &s) != nil {
		return false
	}
	for _, msg := range s.Messages {
		if msg.Role != "assistant" {
			continue
		}
		for _, event := range msg.AgentEvents {
			b, err := json.Marshal(event)
			if err != nil {
				continue
			}
			var marker struct {
				Type string `json:"type"`
				ID   uint64 `json:"completion_id"`
			}
			if json.Unmarshal(b, &marker) == nil && marker.Type == "subagent_completion" && marker.ID == id {
				return true
			}
		}
	}
	return false
}
