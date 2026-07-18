package models

// GatewayModel is one selectable "type/model-id" entry for the Developer
// screen's local API gateway (e.g. "local/qwen2.5", "openai/gpt-4o").
type GatewayModel struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// GatewayLogEntry is one recorded /v1/messages request/response, shown in
// the Developer screen's live log so a user can watch traffic pass through
// the gateway (e.g. while wiring up Claude Code) without reading backend logs.
type GatewayLogEntry struct {
	Seq             uint64 `json:"seq"`
	Timestamp       string `json:"timestamp"` // RFC3339
	Model           string `json:"model"`
	Stream          bool   `json:"stream"`
	HasTools        bool   `json:"has_tools"`
	RequestPreview  string `json:"request_preview"`
	ResponsePreview string `json:"response_preview,omitempty"`
	Error           string `json:"error,omitempty"`
	DurationMs      int64  `json:"duration_ms"`
}
