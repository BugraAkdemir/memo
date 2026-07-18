package models

// GatewayModel is one selectable "type/model-id" entry for the Settings >
// Developer tab's local API gateway (e.g. "local/qwen2.5", "openai/gpt-4o").
type GatewayModel struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}
