package app

import (
	"context"
	"memo/internal/api"
	"memo/internal/provider"
	"testing"
	"time"
)

func TestEligibleRouteCandidates(t *testing.T) {
	yes, no := true, false
	general := routeCandidate{cfg: provider.ProviderConfig{Name: "api", Model: "general"}, window: 16000, tools: &yes}
	coder := routeCandidate{cfg: provider.ProviderConfig{Name: "api", Model: "coder"}, window: 16000, code: true, tools: &yes}
	long := routeCandidate{cfg: provider.ProviderConfig{Name: "subscription", Model: "long"}, window: 100000, tools: &yes}
	vision := routeCandidate{cfg: provider.ProviderConfig{Name: "local", Model: "vision"}, window: 8000, vision: true, tools: &no}
	unknown := routeCandidate{cfg: provider.ProviderConfig{Name: "api", Model: "unknown"}}
	cs := []routeCandidate{general, coder, long, vision, unknown}
	for _, tc := range []struct {
		name                string
		need                int
		vision, tools, code bool
		want                string
	}{
		{"general", 12000, false, true, false, "general"},
		{"coding", 12000, false, true, true, "coder"},
		{"large file", 30000, false, true, true, "long"},
		{"too large uses largest for compaction", 200000, false, true, false, "long"},
		{"vision", 5000, true, false, false, "vision"},
		{"vision tools unavailable", 5000, true, true, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := eligibleRouteCandidates(cs, tc.need, tc.vision, tc.tools, tc.code)
			if tc.want == "" {
				if len(got) > 0 {
					t.Fatal("ineligible model selected")
				}
				return
			}
			if len(got) == 0 || got[0].cfg.Model != tc.want {
				t.Fatalf("got %+v, want %s", got, tc.want)
			}
		})
	}
	if got := eligibleRouteCandidates([]routeCandidate{unknown}, 10000, false, false, false); len(got) != 1 || got[0].window != 0 {
		t.Fatal("unknown-only offline fallback invented capacity")
	}
}

func TestAutoRouteSetupReturnsBeforeModelLoads(t *testing.T) {
	a := &App{activeProviderName: AutoRouteProvider}
	ready := make(chan struct{})
	defer close(ready)
	ch, ok := a.runLockedAutoRouteSetup(context.Background(), "cold model", func() {}, func() <-chan api.StreamChunk {
		<-ready
		out := make(chan api.StreamChunk, 1)
		out <- api.StreamChunk{Done: true}
		close(out)
		return out
	})
	if !ok {
		t.Fatal("setup failed")
	}
	select {
	case chunk := <-ch:
		if chunk.FinishReason != "heartbeat" {
			t.Fatalf("unexpected initial chunk: %+v", chunk)
		}
	case <-time.After(time.Second):
		t.Fatal("SSE headers blocked by cold load")
	}
}
