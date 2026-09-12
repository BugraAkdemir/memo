// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"testing"
)

// TestGatewayModel_JSONFieldNames guards the wire contract with the Flutter
// Developer screen's model picker (id/type) — a struct-tag typo here would
// compile fine and silently break the frontend's json.decode instead.
func TestGatewayModel_JSONFieldNames(t *testing.T) {
	b, err := json.Marshal(GatewayModel{ID: "local/qwen2.5", Type: "llamacpp"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if m["id"] != "local/qwen2.5" {
		t.Errorf("got id=%v, want %q", m["id"], "local/qwen2.5")
	}
	if m["type"] != "llamacpp" {
		t.Errorf("got type=%v, want %q", m["type"], "llamacpp")
	}
}

// TestGatewayLogEntry_JSONFieldNames guards the same wire contract for the
// Developer screen's live gateway-traffic log, including the two
// omitempty fields (response_preview, error) that must stay absent rather
// than render as an empty string in the UI when there's nothing to show.
func TestGatewayLogEntry_JSONFieldNames(t *testing.T) {
	b, err := json.Marshal(GatewayLogEntry{
		Seq:            1,
		Timestamp:      "2026-09-12T00:00:00Z",
		Model:          "local/qwen2.5",
		Stream:         true,
		HasTools:       false,
		RequestPreview: "hi",
		DurationMs:     42,
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for _, want := range []string{"seq", "timestamp", "model", "stream", "has_tools", "request_preview", "duration_ms"} {
		if _, ok := m[want]; !ok {
			t.Errorf("missing expected JSON field %q in %v", want, m)
		}
	}
	if _, ok := m["response_preview"]; ok {
		t.Errorf("response_preview should be omitted when empty, got %v", m["response_preview"])
	}
	if _, ok := m["error"]; ok {
		t.Errorf("error should be omitted when empty, got %v", m["error"])
	}
}
