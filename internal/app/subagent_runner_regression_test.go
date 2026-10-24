package app

import (
	"context"
	"encoding/json"
	"io"
	"memo/internal/provider"
	"memo/internal/taskloop"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestResolveSubAgentSpecs_UsesOwningTask(t *testing.T) {
	a := newSelfHealApp(t, "other", "owner")
	// Only a different task is in the map. The owner is carried by context.
	a.seedTaskRunConfig("other-list", "other")
	owner := &taskRunConfig{model: "owner-picked-model", effortLevel: "low"}
	specs := a.resolveSubAgentSpecs(withTaskRunConfig(context.Background(), owner), "implement one item", "")
	for _, s := range specs {
		if s.Model != owner.model {
			t.Fatalf("role %s got model %q, want owning task %q", s.Role, s.Model, owner.model)
		}
	}
}

func TestDrainStreamIdleUsage_KeepsUsageOnFailure(t *testing.T) {
	for _, idle := range []time.Duration{0, time.Second} {
		ch := make(chan provider.StreamChunk, 1)
		ch <- provider.StreamChunk{Error: "HTTP 429 quota exceeded", Content: "partial", Usage: &provider.Usage{PromptTokens: 1826050, CachedPromptTokens: 1715493, CompletionTokens: 26559}}
		close(ch)
		out, u, err := drainStreamIdleUsage(ch, func() {}, idle)
		if err == nil || !strings.Contains(err.Error(), "429") {
			t.Fatalf("lost error: %v", err)
		}
		if out != "partial" || u == nil || u.PromptTokens != 1826050 || u.CachedPromptTokens != 1715493 {
			t.Fatalf("paid failed attempt lost: out=%q usage=%+v", out, u)
		}
	}
}

func TestTaskSubAgentRunner_ForwardsEffortAndRecordsPaidFailure(t *testing.T) {
	for _, roleModel := range []string{"fake-model", "different-role-model"} {
		t.Run(roleModel, func(t *testing.T) {
			fp := &fakeProvider{}
			fp.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				fp.mu.Lock()
				fp.requests = append(fp.requests, raw)
				n := len(fp.requests)
				fp.mu.Unlock()
				if n == 1 {
					writeFakeSSE(w, respToolCall("fake-model", "c1", "read_file", map[string]any{"path": "go.mod"}, 1500, 30))
					return
				}
				http.Error(w, "quota exceeded", http.StatusTooManyRequests)
			}))
			t.Cleanup(fp.srv.Close)
			h := newLongSessionApp(t, fp, false)
			trc := &taskRunConfig{exec: h.a.agentExecutor, model: "fake-model", effortLevel: "low"}
			_, err := (&appSubAgentRunner{a: h.a}).Run(withTaskRunConfig(context.Background(), trc), taskloop.SubAgentSpec{Role: taskloop.SubRoleCoder, Model: roleModel, SystemPrompt: "complete item", Task: "read go.mod"}, true)
			if err == nil {
				t.Fatal("expected quota failure after paid first iteration")
			}
			for _, raw := range fp.reqBodies() {
				var req map[string]any
				if err := json.Unmarshal([]byte(raw), &req); err != nil {
					t.Fatal(err)
				}
				wantEffort := any("low")
				if roleModel != trc.model {
					wantEffort = nil
				}
				if req["reasoning_effort"] != wantEffort {
					t.Fatalf("task effort was dropped: %s", raw)
				}
			}
			row := h.waitUsageCategory(t, categoryTaskSubagent)
			if row.PromptTokens != 1500 || row.CompletionTokens != 30 {
				t.Fatalf("paid failed turn usage=%+v", row)
			}
		})
	}
}
