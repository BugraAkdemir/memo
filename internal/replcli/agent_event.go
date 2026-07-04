package replcli

import "encoding/json"

// AgentEvent is the payload carried inside a StreamChunk whose FinishReason
// is "agent_event". It mirrors frontend/lib/models/agent.dart's AgentEvent
// so the terminal REPL and the Flutter app interpret the same wire format.
type AgentEvent struct {
	Type        string          `json:"type"`
	RequestID   string          `json:"request_id"`
	Tool        string          `json:"tool"`
	Args        json.RawMessage `json:"args"`
	Result      string          `json:"result"`
	Error       string          `json:"error"`
	DangerLevel string          `json:"danger_level"`
	DurationMs  int             `json:"duration_ms"`
	Content     string          `json:"content"`
	Preview     string          `json:"preview"`
}
