package e2e

import (
	"encoding/json"
	"fmt"
	"memo/internal/app"
	"memo/internal/config"
	"memo/internal/provider"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAutoRouteChangesModelsWithSharedHistory(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(false)
	h.SetWebSearchEnabled(false)
	// A fresh published snapshot keeps this test credential-free and offline.
	fixture := `{"fixture":{"api":"ENDPOINT","models":{"general":{"id":"general","tool_call":true,"limit":{"context":16384},"modalities":{"input":["text"],"output":["text"]}},"coder":{"id":"coder","tool_call":true,"description":"coding","limit":{"context":16384},"modalities":{"input":["text"],"output":["text"]}},"long":{"id":"long","tool_call":true,"limit":{"context":100000},"modalities":{"input":["text"],"output":["text"]}},"not-added":{"id":"not-added","limit":{"context":1000000},"modalities":{"input":["text"],"output":["text"]}}}}}`
	if err := os.WriteFile(config.DataPath("model-routing-capabilities.json"), []byte(strings.ReplaceAll(fixture, "ENDPOINT", h.Fake.Srv.URL)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := h.App.UpdateProvider(provider.ProviderConfig{Type: provider.ProviderCustom, Name: "e2e-fake", BaseURL: h.Fake.Srv.URL, APIKey: "test", Model: "general", Enabled: true, Models: []provider.ConfiguredModel{{ID: "general"}, {ID: "coder"}, {ID: "long"}}}); err != nil {
		t.Fatal(err)
	}
	h.Fake.Script = func(_ int, r FakeChatRequest) FakeChatResponse {
		return FakeChatResponse{Text: "answer from " + r.Model}
	}
	chat := h.NewChat()
	h.App.SetActiveProvider(app.AutoRouteProvider)
	for i, turn := range []struct{ message, model string }{{"Remember this conversation marker: FIRST-TURN", "general"}, {"Write code for a button", "coder"}, {"Examine this file: " + strings.Repeat("x", 60000), "long"}} {
		events := h.SendMessageStream(chat, turn.message)
		seen := false
		for _, ev := range events {
			if ev.FinishReason == "model_route" {
				var m map[string]any
				if json.Unmarshal([]byte(ev.Content), &m) != nil {
					t.Fatal("invalid model route marker")
				}
				if m["model"] != turn.model {
					t.Fatalf("turn %d routed to %v, want %s", i, m, turn.model)
				}
				seen = true
			}
		}
		if !seen {
			t.Fatal("no live attribution")
		}
		if h.App.GetActiveProvider() != app.AutoRouteProvider {
			t.Fatal("request mutated user selection")
		}
	}
	reqs := h.Fake.Requests()
	var last FakeChatRequest
	for _, r := range reqs {
		if r.Model == "long" && strings.Contains(string(r.Raw), "Examine this file") {
			last = r
		}
	}
	if !strings.Contains(string(last.Raw), "FIRST-TURN") || !strings.Contains(string(last.Raw), "answer from general") || !strings.Contains(string(last.Raw), "answer from coder") {
		t.Fatalf("shared history lost: %s", last.Raw)
	}
	if last.Model != "long" {
		t.Fatalf("long input routed to %s", last.Model)
	}
	resp := h.getJSON("/api/messages?chat_id=" + chat)
	var messages []struct {
		Role       string `json:"role"`
		RouteModel string `json:"route_model"`
	}
	decodeInto(t, resp, &messages)
	var got []string
	for _, m := range messages {
		if m.Role == "assistant" {
			got = append(got, m.RouteModel)
		}
	}
	if strings.Join(got, ",") != "general,coder,long" {
		t.Fatalf("persisted attribution: %v", got)
	}
	report := h.App.ContextReport(chat)
	if report.Model != "long" || report.Window != 100000 {
		t.Fatalf("context gauge used wrong routed model: %+v", report)
	}
	h.Fake.ImageModels = map[string]bool{"draw": true}
	if err := h.App.SetImageConfig(config.ImageConfig{AutoRoute: true, DefaultProvider: "e2e-fake", DefaultModel: "draw"}); err != nil {
		t.Fatal(err)
	}
	imageEvents := h.SendMessageStream(chat, "Draw a picture of a cat")
	drawn := false
	for _, ev := range imageEvents {
		if ev.FinishReason == "model_route" && strings.Contains(ev.Content, `"model":"draw"`) {
			drawn = true
		}
	}
	if !drawn || len(h.Fake.ImageRequests()) != 1 {
		t.Fatal("explicit image did not use and identify its image model")
	}
	h.App.SetActiveProvider("e2e-fake")
	manual := h.SendMessageStream(chat, "Manual selection remains general")
	for _, ev := range manual {
		if ev.FinishReason == "model_route" {
			t.Fatal("manual mode was auto routed")
		}
	}
	found := false
	for _, r := range h.Fake.Requests() {
		if r.Model == "general" && strings.Contains(string(r.Raw), "Manual selection remains general") {
			found = true
		}
	}
	if !found {
		t.Fatal("auto route changed saved model selection")
	}

}

func TestAutoRouteDecisionInvalidAndFailedChoicesFallBack(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(false)
	h.SetWebSearchEnabled(false)
	fixture := `{"fixture":{"models":{"general":{"id":"general","tool_call":true,"limit":{"context":16384},"modalities":{"input":["text"],"output":["text"]}},"long":{"id":"long","tool_call":true,"limit":{"context":100000},"modalities":{"input":["text"],"output":["text"]}}}}}`
	if err := os.WriteFile(config.DataPath("model-routing-capabilities.json"), []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	if err := h.App.UpdateProvider(provider.ProviderConfig{Type: provider.ProviderCustom, Name: "e2e-fake", BaseURL: h.Fake.Srv.URL, APIKey: "test", Model: "general", Enabled: true, Models: []provider.ConfiguredModel{{ID: "general"}, {ID: "long"}}}); err != nil {
		t.Fatal(err)
	}
	if err := h.App.UpdateDecisionSettings(true, "e2e-fake", "decision"); err != nil {
		t.Fatal(err)
	}
	var choices atomic.Int32
	h.Fake.Script = func(_ int, r FakeChatRequest) FakeChatResponse {
		if r.Model == "decision" {
			switch choices.Add(1) {
			case 1:
				return FakeChatResponse{Text: "1"}
			case 2:
				return FakeChatResponse{Text: "999"}
			default:
				return FakeChatResponse{Status: 400, RawBody: "classifier unavailable"}
			}
		}
		return FakeChatResponse{Text: "answer from " + r.Model}
	}
	h.App.SetActiveProvider(app.AutoRouteProvider)
	chat := h.NewChat()
	for i, want := range []string{"long", "general", "general"} {
		events := h.SendMessageStream(chat, fmt.Sprintf("Ordinary request %d", i))
		matched := false
		for _, e := range events {
			if e.Error != "" {
				t.Fatal(e.Error)
			}
			if e.FinishReason == "model_route" {
				var r map[string]any
				_ = json.Unmarshal([]byte(e.Content), &r)
				if r["model"] != want {
					t.Fatalf("invalid decision fallback: %v, want %s", r, want)
				}
				matched = true
			}
		}
		if !matched {
			t.Fatal("no routing attribution")
		}
	}
	if choices.Load() != 3 {
		t.Fatalf("classifier repeated unexpectedly: %d", choices.Load())
	}
}
