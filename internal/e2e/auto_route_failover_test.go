package e2e

import (
	"encoding/json"
	"memo/internal/app"
	"memo/internal/config"
	"memo/internal/provider"
	"os"
	"strings"
	"testing"
)

func TestAutoRouteFailoverBeforeOutput(t *testing.T) {
	for _, agentMode := range []bool{false, true} {
		t.Run(map[bool]string{false: "stream", true: "agent"}[agentMode], func(t *testing.T) {
			h := NewHarness(t)
			h.SetAgentEnabled(agentMode)
			h.SetWebSearchEnabled(false)
			fixture := `{"fixture":{"api":"ENDPOINT","models":{"first":{"id":"first","tool_call":true,"limit":{"context":16000},"modalities":{"input":["text"],"output":["text"]}},"backup":{"id":"backup","tool_call":true,"limit":{"context":32000},"modalities":{"input":["text"],"output":["text"]}}}}}`
			if err := os.WriteFile(config.DataPath("model-routing-capabilities.json"), []byte(strings.ReplaceAll(fixture, "ENDPOINT", h.Fake.Srv.URL)), 0600); err != nil {
				t.Fatal(err)
			}
			if err := h.App.UpdateProvider(provider.ProviderConfig{Type: provider.ProviderCustom, Name: "e2e-fake", BaseURL: h.Fake.Srv.URL, APIKey: "test", Model: "first", Enabled: true, Models: []provider.ConfiguredModel{{ID: "first"}, {ID: "backup"}}}); err != nil {
				t.Fatal(err)
			}
			h.Fake.Script = func(_ int, r FakeChatRequest) FakeChatResponse {
				if r.Model == "first" {
					return FakeChatResponse{Status: 503, RawBody: `{"error":{"message":"temporarily unavailable"}}`}
				}
				return FakeChatResponse{Text: "backup answer"}
			}
			chat := h.NewChat()
			h.App.SetActiveProvider(app.AutoRouteProvider)
			events := h.SendMessageStream(chat, "Explain the seasons briefly")
			finalModel := ""
			var reply strings.Builder
			for _, e := range events {
				if e.Error != "" {
					t.Fatalf("fallback failed: %s", e.Error)
				}
				if e.FinishReason == "model_route" {
					var m map[string]any
					json.Unmarshal([]byte(e.Content), &m)
					finalModel, _ = m["model"].(string)
				} else if e.FinishReason == "" {
					reply.WriteString(e.Content)
				}
			}
			if finalModel != "backup" || !strings.Contains(reply.String(), "backup answer") {
				t.Fatalf("model=%s reply=%s", finalModel, reply.String())
			}
			var msgs []struct {
				Role       string `json:"role"`
				RouteModel string `json:"route_model"`
				Content    string `json:"content"`
			}
			decodeInto(t, h.getJSON("/api/messages?chat_id="+chat), &msgs)
			assistants := 0
			for _, m := range msgs {
				if m.Role == "assistant" {
					assistants++
					if m.RouteModel != "backup" {
						t.Fatalf("wrong persisted model: %+v", m)
					}
				}
			}
			if assistants != 1 {
				t.Fatalf("failed attempt left an error bubble: %+v", msgs)
			}
			if report := h.App.ContextReport(chat); report.Model != "backup" || report.Window != 32000 {
				t.Fatalf("wrong fallback gauge: %+v", report)
			}
		})
	}
}
