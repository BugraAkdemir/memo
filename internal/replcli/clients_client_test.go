package replcli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestClient_RegisterHeartbeatUnregister_HTTPWiring(t *testing.T) {
	var gotHeartbeatID, gotUnregisterID string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/clients/register", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"client_id": "client-xyz"})
	})
	mux.HandleFunc("/api/clients/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		gotHeartbeatID = body["client_id"]
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/clients/unregister", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		gotUnregisterID = body["client_id"]
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(srv.URL)
	id, err := c.RegisterClient(context.Background())
	if err != nil || id != "client-xyz" {
		t.Fatalf("RegisterClient() = %q, %v, want %q, nil", id, err, "client-xyz")
	}
	if err := c.HeartbeatClient(context.Background(), id); err != nil {
		t.Fatalf("HeartbeatClient() error = %v", err)
	}
	if gotHeartbeatID != id {
		t.Errorf("heartbeat client_id = %q, want %q", gotHeartbeatID, id)
	}
	if err := c.UnregisterClient(context.Background(), id); err != nil {
		t.Fatalf("UnregisterClient() error = %v", err)
	}
	if gotUnregisterID != id {
		t.Errorf("unregister client_id = %q, want %q", gotUnregisterID, id)
	}
}

// newClientTrackingTestServer wires up newTestServer's endpoints plus the
// client-registry ones, recording every register/unregister call — used to
// prove Run() actually attaches and detaches (repl.go's wiring), not just
// that the Client methods work in isolation.
func newClientTrackingTestServer(t *testing.T, sseLines []string) (srv *httptest.Server, registered *[]string, unregistered *[]string) {
	t.Helper()
	var mu sync.Mutex
	var regCalls, unregCalls []string

	base, _ := newTestServer(t, sseLines)
	baseHandler := base.Config.Handler
	base.Close() // only wanted its mux wiring; serve our own combined mux instead

	mux := http.NewServeMux()
	mux.Handle("/", baseHandler)
	mux.HandleFunc("/api/clients/register", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		id := "client-1"
		regCalls = append(regCalls, id)
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]string{"client_id": id})
	})
	mux.HandleFunc("/api/clients/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/clients/unregister", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		unregCalls = append(unregCalls, body["client_id"])
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	srv = httptest.NewServer(mux)
	return srv, &regCalls, &unregCalls
}

func TestRun_RegistersAndUnregistersClient(t *testing.T) {
	srv, registered, unregistered := newClientTrackingTestServer(t, []string{
		`data: {"content":"ok","done":true,"finish_reason":"stop"}`,
	})
	defer srv.Close()

	in := strings.NewReader("selam\n/exit\n")
	var out bytes.Buffer

	if err := Run(srv.URL, "/tmp/project", in, &out, false); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(*registered) != 1 {
		t.Fatalf("register calls = %v, want exactly 1", *registered)
	}
	if len(*unregistered) != 1 || (*unregistered)[0] != (*registered)[0] {
		t.Fatalf("unregister calls = %v, want [%q]", *unregistered, (*registered)[0])
	}
}

// TestRun_InvokesOnClientRegisteredCallback is the regression guard for
// BUG-M3 (SIGTERM skips the CLI's unregister): main.go can't wait for Run's
// own goroutine to run its deferred UnregisterClient on an external signal
// (the goroutine is left blocked on stdin and abandoned when the process
// exits), so it needs the client ID up front to send the goodbye itself.
// This proves the callback that hands it over actually fires, with the
// right ID, right after registration succeeds.
func TestRun_InvokesOnClientRegisteredCallback(t *testing.T) {
	srv, registered, _ := newClientTrackingTestServer(t, []string{
		`data: {"content":"ok","done":true,"finish_reason":"stop"}`,
	})
	defer srv.Close()

	in := strings.NewReader("selam\n/exit\n")
	var out bytes.Buffer

	var gotID string
	callCount := 0
	if err := Run(srv.URL, "/tmp/project", in, &out, false, func(id string) {
		callCount++
		gotID = id
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if callCount != 1 {
		t.Fatalf("onClientRegistered called %d times, want exactly 1", callCount)
	}
	if len(*registered) != 1 || gotID != (*registered)[0] {
		t.Fatalf("callback got id %q, want %q (the id RegisterClient actually returned)", gotID, (*registered)[0])
	}
}
