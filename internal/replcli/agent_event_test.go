package replcli

import (
	"encoding/json"
	"testing"
)

func TestAgentEvent_FromJSON_FullPayload(t *testing.T) {
	raw := `{
		"type": "permission_request",
		"request_id": "req-123",
		"tool": "run_command",
		"args": {"command": "rm -rf /tmp/x"},
		"danger_level": "high"
	}`

	var ev AgentEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if ev.Type != "permission_request" {
		t.Errorf("Type = %q, want permission_request", ev.Type)
	}
	if ev.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want req-123", ev.RequestID)
	}
	if ev.Tool != "run_command" {
		t.Errorf("Tool = %q, want run_command", ev.Tool)
	}
	if ev.DangerLevel != "high" {
		t.Errorf("DangerLevel = %q, want high", ev.DangerLevel)
	}
	if string(ev.Args) != `{"command": "rm -rf /tmp/x"}` {
		t.Errorf("Args = %s, want raw JSON passthrough", ev.Args)
	}
}

func TestAgentEvent_FromJSON_ToolResult(t *testing.T) {
	raw := `{"type":"tool_result","tool":"read_file","result":"ok"}`

	var ev AgentEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if ev.Type != "tool_result" || ev.Tool != "read_file" || ev.Result != "ok" {
		t.Errorf("got %+v", ev)
	}
	if ev.RequestID != "" {
		t.Errorf("RequestID should be empty when absent, got %q", ev.RequestID)
	}
}
