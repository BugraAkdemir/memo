package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"memo/internal/provider"
	"memo/internal/subagents"
)

func TestChatSubAgents_InheritSelectedProviderInsteadOfFirstEnabled(t *testing.T) {
	for _, direct := range []bool{false, true} {
		name := "main_tool"
		if direct {
			name = "direct_action"
		}
		t.Run(name, func(t *testing.T) {
			h := NewHarness(t)
			h.SetAgentEnabled(true)
			h.SetAutoPermission(true)
			chat := h.NewAgentChat(t.TempDir())
			var wrongRequests atomic.Int32
			wrong := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				wrongRequests.Add(1)
				http.Error(w, "this provider does not serve the selected model", http.StatusBadRequest)
			}))
			defer wrong.Close()
			if err := h.App.UpdateProvider(provider.ProviderConfig{
				Type: provider.ProviderCustom, Name: "unselected-first", BaseURL: wrong.URL,
				Model: "different-model", Models: []provider.ConfiguredModel{{ID: "different-model"}},
				Priority: 100, Enabled: true,
			}); err != nil {
				t.Fatal(err)
			}
			// The main request succeeds on e2e-fake even though the other provider has
			// higher priority. Children must preserve that restriction and model.
			h.App.SetActiveProvider("e2e-fake")
			children := make(chan FakeChatRequest, 4)
			var spawned atomic.Bool
			h.Fake.Script = func(_ int, req FakeChatRequest) FakeChatResponse {
				if childRequest(req) {
					children <- req
					return FakeChatResponse{Text: "CHILD_ROUTED_CORRECTLY"}
				}
				if len(req.Tools) > 0 && lastSubAgentUser(req) == "SPAWN_ROUTING_CHECK" && spawned.CompareAndSwap(false, true) {
					return FakeChatResponse{ToolCalls: []FakeToolCall{{ID: "spawn-route", Name: "spawn_agent", Arguments: `{"prompt":"CHECK_CHILD_ROUTE","title":"routing-check"}`}}}
				}
				return FakeChatResponse{Text: "MAIN_ROUTED_CORRECTLY"}
			}
			var job subagents.Job
			if direct {
				raw, err := h.App.ChatSubAgentAction(context.Background(), chat, "spawn", json.RawMessage(`{"prompt":"CHECK_CHILD_ROUTE"}`))
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(raw), &job); err != nil {
					t.Fatal(err)
				}
			} else {
				for _, event := range h.SendMessageStream(chat, "SPAWN_ROUTING_CHECK") {
					if event.Error != "" {
						t.Fatalf("main request failed: %s", event.Error)
					}
				}
				var jobs []subagents.Job
				decodeInto(t, h.getJSON("/api/subagents?chat_id="+chat), &jobs)
				if len(jobs) != 1 {
					t.Fatalf("jobs=%d", len(jobs))
				}
				job = jobs[0]
			}
			if job.ProviderName != "e2e-fake" || job.Model != "fake-model" {
				t.Fatalf("child route = %s/%s; want e2e-fake/fake-model", job.ProviderName, job.Model)
			}
			request := awaitSubAgentValue(t, children)
			if request.Model != "fake-model" {
				t.Fatalf("wire model = %s", request.Model)
			}
			awaitSubAgentJob(t, h, chat, func(j subagents.Job) bool { return j.ID == job.ID && j.State == subagents.Awaiting })
			if wrongRequests.Load() != 0 {
				t.Fatalf("sent %d requests to unselected provider", wrongRequests.Load())
			}
		})
	}
}
