package e2e

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCalendarChangesStreamFollowsExternalAddAndDelete(t *testing.T) {
	h := NewHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, h.BaseURL+"/api/calendar/changes", nil)
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("SSE status %d", resp.StatusCode)
	}
	reader := bufio.NewReader(resp.Body)
	changed := func() {
		t.Helper()
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(line, "data: ") {
				if !strings.Contains(line, `"changed":true`) {
					t.Fatalf("wrong signal: %s", line)
				}
				return
			}
		}
	}
	changed() // first snapshot signal
	var event struct {
		ID string `json:"id"`
	}
	decodeInto(t, h.postJSON("/api/calendar/events", map[string]any{"title": "Outside calendar tab", "start_time": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}), &event)
	changed()
	del, _ := http.NewRequest(http.MethodDelete, h.BaseURL+"/api/calendar/events/"+event.ID, nil)
	removed, err := h.client.Do(del)
	if err != nil {
		t.Fatal(err)
	}
	removed.Body.Close()
	if removed.StatusCode != 204 {
		t.Fatalf("delete=%d", removed.StatusCode)
	}
	changed()
}
