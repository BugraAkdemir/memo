package webserver

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"memo/internal/api"
	"memo/internal/models"
)

// feed sends chunks into a channel the way a chat turn does, then closes it.
func feed(chunks ...api.StreamChunk) <-chan api.StreamChunk {
	ch := make(chan api.StreamChunk, len(chunks))
	for _, c := range chunks {
		ch <- c
	}
	close(ch)
	return ch
}

func drain(t *testing.T, ch <-chan api.StreamChunk) []api.StreamChunk {
	t.Helper()
	var out []api.StreamChunk
	deadline := time.After(3 * time.Second)
	for {
		select {
		case c, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, c)
		case <-deadline:
			t.Fatal("the stream never closed")
		}
	}
}

func TestWithQuotaSignals_MarkerGoesJustBeforeTheErrorChunk(t *testing.T) {
	b := &swarmStubBridge{quotaForError: func(errText string) *models.QuotaSignal {
		return &models.QuotaSignal{Kind: "exhausted", Model: "gpt-5.5", RemainingPercent: 0, ResetAt: "2026-10-13T00:00:00Z"}
	}}
	s := &Server{fullBridge: b}
	got := drain(t, s.withQuotaSignals(context.Background(), feed(
		api.StreamChunk{Content: "partial "},
		api.StreamChunk{Error: "⚠️ status 429: usage limit reached", Done: true},
	)))
	if len(got) != 3 {
		t.Fatalf("got %d chunks, want text + marker + error: %+v", len(got), got)
	}
	if got[0].Content != "partial " {
		t.Errorf("ordinary text must pass through untouched: %+v", got[0])
	}
	if got[1].FinishReason != models.QuotaExhaustedMarker {
		t.Fatalf("second chunk = %+v, want the quota_exhausted marker", got[1])
	}
	var sig models.QuotaSignal
	if err := json.Unmarshal([]byte(got[1].Content), &sig); err != nil || sig.Model != "gpt-5.5" || sig.ResetAt != "2026-10-13T00:00:00Z" {
		t.Errorf("marker payload = %q (%v)", got[1].Content, err)
	}
	if got[1].Done || got[1].Error != "" {
		t.Error("a marker is metadata: it must be neither terminal nor an error")
	}
	if got[2].Error == "" || !got[2].Done {
		t.Errorf("the original error chunk must still end the stream: %+v", got[2])
	}
}

func TestWithQuotaSignals_LowWarningGoesBeforeTheCleanDone(t *testing.T) {
	b := &swarmStubBridge{quotaLow: func() *models.QuotaSignal {
		return &models.QuotaSignal{Kind: "low", RemainingPercent: 8}
	}}
	s := &Server{fullBridge: b}
	got := drain(t, s.withQuotaSignals(context.Background(), feed(
		api.StreamChunk{Content: "answer"},
		api.StreamChunk{Done: true, FinishReason: "stop"},
	)))
	if len(got) != 3 || got[1].FinishReason != models.QuotaLowMarker {
		t.Fatalf("got %+v, want text + quota_low marker + done", got)
	}
	if got[2].FinishReason != "stop" || !got[2].Done {
		t.Errorf("the terminal chunk must keep its own finish reason: %+v", got[2])
	}
}

func TestWithQuotaSignals_NoSignalMeansNoExtraChunks(t *testing.T) {
	s := &Server{fullBridge: &swarmStubBridge{}} // both hooks answer nil
	in := []api.StreamChunk{{Content: "a"}, {Error: "⚠️ status 500", Done: true}}
	got := drain(t, s.withQuotaSignals(context.Background(), feed(in...)))
	if len(got) != 2 || got[0] != in[0] || got[1] != in[1] {
		t.Errorf("an unremarkable stream was altered: %+v", got)
	}
}

func TestWithQuotaSignals_ARealErrorIsNeverReportedAsLow(t *testing.T) {
	// An error chunk is also terminal: it must get the exhausted check only.
	lowCalled := false
	b := &swarmStubBridge{quotaLow: func() *models.QuotaSignal { lowCalled = true; return &models.QuotaSignal{Kind: "low"} }}
	s := &Server{fullBridge: b}
	drain(t, s.withQuotaSignals(context.Background(), feed(api.StreamChunk{Error: "boom", Done: true})))
	if lowCalled {
		t.Error("the low-quota check must run only after a clean finish")
	}
}

func TestWithQuotaSignals_WarmsTheQuotaOnceAndHandlesNilParts(t *testing.T) {
	b := &swarmStubBridge{}
	s := &Server{fullBridge: b}
	drain(t, s.withQuotaSignals(context.Background(), feed(api.StreamChunk{Done: true})))
	if b.warmed != 1 {
		t.Errorf("WarmQuota ran %d times, want once per turn", b.warmed)
	}
	if ch := (&Server{}).withQuotaSignals(context.Background(), nil); ch != nil {
		t.Error("a nil stream stays nil")
	}
}

func TestWithQuotaSignals_ADisconnectedClientDoesNotLeakTheGoroutine(t *testing.T) {
	b := &swarmStubBridge{}
	s := &Server{fullBridge: b}
	ctx, cancel := context.WithCancel(context.Background())
	src := make(chan api.StreamChunk)
	out := s.withQuotaSignals(ctx, src)
	cancel()
	// Nobody reads `out` and the source keeps producing: the helper must give up.
	go func() {
		defer func() { _ = recover() }()
		for i := 0; i < 300; i++ {
			src <- api.StreamChunk{Content: "x"}
		}
		close(src)
	}()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case _, ok := <-out:
			if !ok {
				return // closed: the goroutine exited
			}
		case <-deadline:
			t.Fatal("the quota helper never stopped after the client went away")
		}
	}
}

// The quota card needs the vendor's own wording and reset time, so the error is
// classified on the RAW text — the person then reads the friendly rewrite.
func TestWithQuotaSignals_ClassifiesTheRawErrorThenShowsTheFriendlyOne(t *testing.T) {
	var classified string
	b := &swarmStubBridge{
		quotaForError: func(errText string) *models.QuotaSignal {
			classified = errText
			return nil
		},
		friendly: func(raw string) string { return "friendly" },
	}
	s := &Server{fullBridge: b}
	got := drain(t, s.withQuotaSignals(context.Background(), feed(
		api.StreamChunk{Error: "⚠️ [custom] status 429: usage limit reached (resets in 1h0m0s)", Done: true},
	)))
	if classified != "⚠️ [custom] status 429: usage limit reached (resets in 1h0m0s)" {
		t.Errorf("the quota check saw %q, want the raw error text (it parses the reset time out of it)", classified)
	}
	if len(got) != 1 || got[0].Error != "friendly" {
		t.Errorf("client got %+v, want the friendly rewrite", got)
	}
}

func TestWithFriendlyErrors_RewritesOnlyTheErrorChunk(t *testing.T) {
	b := &swarmStubBridge{friendly: func(raw string) string { return "friendly" }}
	s := &Server{fullBridge: b}
	got := drain(t, s.withFriendlyErrors(context.Background(), feed(
		api.StreamChunk{Content: "text with status 500 in it"},
		api.StreamChunk{Error: "⚠️ raw", Done: true},
	)))
	if len(got) != 2 || got[0].Content != "text with status 500 in it" || got[1].Error != "friendly" || !got[1].Done {
		t.Errorf("got %+v", got)
	}
}
