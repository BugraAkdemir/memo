package webserver

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type twinEventFixture struct{ fakeTwinBridge }

func (*twinEventFixture) GetEvents() []map[string]string {
	return []map[string]string{
		{"seq": "1", "name": "other:event", "data": `{"secret":"private"}`},
		{"seq": "2", "name": "autonomy:action_pending", "data": `{"id":"one","state":"pending","code":12,"body":"private body","recipient":"private recipient"}`},
		{"seq": "3", "name": "autonomy:action_executed", "data": `{"id":"one","state":"sent","code":12}`},
	}
}
func TestTwinApprovalSSEFiltersPrivateDataAndResumes(t *testing.T) {
	s := New(&twinEventFixture{})
	server := httptest.NewServer(http.HandlerFunc(s.handleTwinEvents))
	defer server.Close()
	for _, last := range []string{"", "2", "999999"} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
		if last != "" {
			req.Header.Set("Last-Event-ID", last)
		}
		resp, err := server.Client().Do(req)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		if resp.Header.Get("Content-Type") != "text/event-stream" {
			t.Fatal(resp.Header)
		}
		scan := bufio.NewScanner(resp.Body)
		name := ""
		found := false
		for scan.Scan() {
			line := scan.Text()
			if strings.HasPrefix(line, "event: ") {
				name = strings.TrimPrefix(line, "event: ")
			}
			if strings.HasPrefix(line, "data: ") {
				var data map[string]any
				if err = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &data); err != nil {
					t.Fatal(err)
				}
				if data["id"] != "one" || data["body"] != nil || data["recipient"] != nil || data["secret"] != nil {
					t.Fatal(data)
				}
				want := "autonomy:action_pending"
				if last == "2" {
					want = "autonomy:action_executed"
				}
				if name != want {
					t.Fatal(name, want)
				}
				found = true
				break
			}
		}
		resp.Body.Close()
		cancel()
		if !found {
			t.Fatal("no SSE change", scan.Err())
		}
	}
	req := httptest.NewRequest("GET", "/api/twin/events", nil)
	req.Header.Set("Last-Event-ID", "invalid")
	w := httptest.NewRecorder()
	s.handleTwinEvents(w, req)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	s.handleTwinEvents(w, httptest.NewRequest("POST", "/api/twin/events", nil))
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}
