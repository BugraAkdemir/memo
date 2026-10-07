// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"encoding/json"
	"testing"
	"time"

	"memo/internal/models"
)

// quotaMarkers returns the quota marker events of a stream and their positions.
func quotaMarkers(events []SSEEvent) (markers []SSEEvent, at []int) {
	for i, ev := range events {
		if ev.FinishReason == models.QuotaExhaustedMarker || ev.FinishReason == models.QuotaLowMarker {
			markers = append(markers, ev)
			at = append(at, i)
		}
	}
	return markers, at
}

// A turn that dies because the allowance ran out reaches the app as a
// quota_exhausted marker with the refill time, immediately before the error
// chunk — through the real stream layer, with a real HTTP refusal from the
// provider.
func TestChat_AUsageLimitRefusalCarriesAQuotaMarkerWithTheRefillTime(t *testing.T) {
	h := NewHarness(t)
	h.SetWebSearchEnabled(false)
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		if !req.Stream {
			return FakeChatResponse{Text: "title"}
		}
		return FakeChatResponse{
			Status:  429,
			RawBody: `{"error":{"type":"usage_limit_reached","message":"The usage limit has been reached","resets_in_seconds":3600}}`,
		}
	}
	chatID := h.NewChat()
	before := time.Now()
	events := h.SendMessageStream(chatID, "keep going")

	markers, at := quotaMarkers(events)
	if len(markers) != 1 {
		t.Fatalf("got %d quota markers in %+v, want exactly one", len(markers), events)
	}
	last := len(events) - 1
	if events[last].Error == "" || !events[last].Done {
		t.Fatalf("last event = %+v, want the error that ends the turn", events[last])
	}
	if at[0] != last-1 {
		t.Errorf("the marker sits at %d, want just before the error chunk at %d", at[0], last)
	}
	if markers[0].Done || markers[0].Error != "" {
		t.Errorf("a marker is metadata, not a terminal or error chunk: %+v", markers[0])
	}

	var sig models.QuotaSignal
	if err := json.Unmarshal([]byte(markers[0].Content), &sig); err != nil {
		t.Fatalf("marker payload %q is not a QuotaSignal: %v", markers[0].Content, err)
	}
	if sig.Kind != "exhausted" {
		t.Errorf("kind = %q", sig.Kind)
	}
	reset, err := time.Parse(time.RFC3339, sig.ResetAt)
	if err != nil {
		t.Fatalf("reset_at = %q: %v", sig.ResetAt, err)
	}
	if d := reset.Sub(before); d < 55*time.Minute || d > 65*time.Minute {
		t.Errorf("reset_at is %v from now, want about an hour (resets_in_seconds=3600)", d)
	}
}

// A plain rate limit is not "the allowance ran out": no card, no promise to wait.
func TestChat_APlainRateLimitGetsNoQuotaMarker(t *testing.T) {
	h := NewHarness(t)
	h.SetWebSearchEnabled(false)
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		if !req.Stream {
			return FakeChatResponse{Text: "title"}
		}
		return FakeChatResponse{Status: 429, RawBody: `{"error":{"message":"Too many requests, please slow down"}}`}
	}
	events := h.SendMessageStream(h.NewChat(), "hello")
	if markers, _ := quotaMarkers(events); len(markers) != 0 {
		t.Errorf("a momentary rate limit produced %+v", markers)
	}
	if last := events[len(events)-1]; last.Error == "" {
		t.Errorf("the error itself must still be shown: %+v", last)
	}
}

// And a good turn on a non-metered provider carries no marker at all.
func TestChat_AGoodTurnCarriesNoQuotaMarker(t *testing.T) {
	h := NewHarness(t)
	h.SetWebSearchEnabled(false)
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		return FakeChatResponse{Text: "all fine"}
	}
	events := h.SendMessageStream(h.NewChat(), "hello")
	if markers, _ := quotaMarkers(events); len(markers) != 0 {
		t.Errorf("a clean turn produced %+v", markers)
	}
}
