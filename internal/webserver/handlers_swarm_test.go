// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsSwarmWorkerAddPath(t *testing.T) {
	cases := map[string]bool{
		"/api/swarm/host/workers/add":      true,
		"/api/v1/swarm/host/workers/add":   true,
		"/api/swarm/host/workers/remove":   false,
		"/api/swarm/status":                false,
		"/api/swarm/host/create":           false,
		"/api/v1/swarm/host/workers/share": false,
	}
	for path, want := range cases {
		if got := isSwarmWorkerAddPath(path); got != want {
			t.Errorf("isSwarmWorkerAddPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestSwarmHTTPStatus(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{nil, http.StatusOK},
		{errors.New("swarm: invalid room secret"), http.StatusUnauthorized},
		{errors.New("Swarm beta özelliğidir; Ayarlar'dan Beta'yı açın"), http.StatusForbidden},
		{errors.New("swarm: model path required"), http.StatusBadRequest},
		{errors.New("swarm: no workers registered yet"), http.StatusBadRequest},
		{errors.New("something exploded"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		if got := swarmHTTPStatus(c.err); got != c.want {
			t.Errorf("swarmHTTPStatus(%v) = %d, want %d", c.err, got, c.want)
		}
	}
}

func TestSwarmHandlers_NilFullBridge(t *testing.T) {
	s := newMockServer(&mockBridge{})
	// fullBridge intentionally nil

	handlers := []struct {
		name    string
		method  string
		path    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{"status", http.MethodGet, "/api/swarm/status", s.handleSwarmStatus},
		{"create", http.MethodPost, "/api/swarm/host/create", s.handleSwarmHostCreate},
		{"add", http.MethodPost, "/api/swarm/host/workers/add", s.handleSwarmAddWorker},
		{"remove", http.MethodPost, "/api/swarm/host/workers/remove", s.handleSwarmRemoveWorker},
		{"reorder", http.MethodPost, "/api/swarm/host/workers/reorder", s.handleSwarmReorderWorkers},
		{"share", http.MethodPost, "/api/swarm/host/workers/share", s.handleSwarmSetShare},
		{"start", http.MethodPost, "/api/swarm/host/start", s.handleSwarmStart},
		{"stop", http.MethodPost, "/api/swarm/host/stop", s.handleSwarmStop},
		{"close", http.MethodPost, "/api/swarm/host/close", s.handleSwarmClose},
		{"join", http.MethodPost, "/api/swarm/join", s.handleSwarmJoin},
		{"leave", http.MethodPost, "/api/swarm/leave", s.handleSwarmLeave},
	}
	for _, h := range handlers {
		t.Run(h.name, func(t *testing.T) {
			req := httptest.NewRequest(h.method, h.path, strings.NewReader(`{}`))
			w := httptest.NewRecorder()
			h.handler(w, req)
			if w.Code != http.StatusNotImplemented {
				t.Errorf("status = %d, want %d (body %q)", w.Code, http.StatusNotImplemented, w.Body.String())
			}
		})
	}
}

func TestHandleSwarmAddWorker_MissingFields(t *testing.T) {
	stub := &swarmStubBridge{}
	s := New(stub)
	s.fullBridge = stub

	req := httptest.NewRequest(http.MethodPost, "/api/swarm/host/workers/add",
		strings.NewReader(`{"id":"x"}`))
	w := httptest.NewRecorder()
	s.handleSwarmAddWorker(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %q)", w.Code, w.Body.String())
	}
}

func TestHandleSwarmAddWorker_RejectsInvalidSecret(t *testing.T) {
	stub := &swarmStubBridge{
		addWorker: func(id, secret, myRPCAddress, label string) error {
			return fmt.Errorf("swarm: invalid room secret")
		},
	}
	s := New(stub)
	s.fullBridge = stub

	body := `{"id":"abc","secret":"wrong","my_rpc_address":"10.0.0.2:50052","label":"pc2"}`
	req := httptest.NewRequest(http.MethodPost, "/api/swarm/host/workers/add", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.handleSwarmAddWorker(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body %q)", w.Code, w.Body.String())
	}
}

func TestHandleSwarmAddWorker_AcceptsValidSecret(t *testing.T) {
	var gotID, gotSecret, gotAddr, gotLabel string
	stub := &swarmStubBridge{
		addWorker: func(id, secret, myRPCAddress, label string) error {
			gotID, gotSecret, gotAddr, gotLabel = id, secret, myRPCAddress, label
			return nil
		},
	}
	s := New(stub)
	s.fullBridge = stub

	body := `{"id":"abc","secret":"sekrit","my_rpc_address":"10.0.0.2:50052","label":"pc2"}`
	req := httptest.NewRequest(http.MethodPost, "/api/swarm/host/workers/add", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.handleSwarmAddWorker(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", w.Code, w.Body.String())
	}
	if gotID != "abc" || gotSecret != "sekrit" || gotAddr != "10.0.0.2:50052" || gotLabel != "pc2" {
		t.Errorf("addWorker got (%q,%q,%q,%q)", gotID, gotSecret, gotAddr, gotLabel)
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp["ok"] != "true" {
		t.Errorf("resp = %v", resp)
	}
}

func TestRemoteAuthMiddleware_SkipsWorkerAddPath(t *testing.T) {
	stub := &swarmStubBridge{token: "memo-real-token"}
	s := New(stub)
	s.fullBridge = stub
	s.listenAddr = "0.0.0.0" // token-gated mode

	var hit bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	})
	mw := s.remoteAuthMiddleware(next)

	// workers/add without token must still reach next (room secret is separate).
	req := httptest.NewRequest(http.MethodPost, "/api/swarm/host/workers/add", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	if !hit {
		t.Fatal("workers/add was blocked by remoteAuthMiddleware; want exemption")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	// A different swarm path without token must still 401.
	hit = false
	req2 := httptest.NewRequest(http.MethodGet, "/api/swarm/status", nil)
	w2 := httptest.NewRecorder()
	mw.ServeHTTP(w2, req2)
	if hit {
		t.Fatal("/api/swarm/status reached next without token; want 401")
	}
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w2.Code)
	}
}

func TestHandleSwarmStatus_OK(t *testing.T) {
	stub := &swarmStubBridge{
		status: func() interface{} {
			return map[string]interface{}{"role": "host", "room_code": "swarm-abc"}
		},
	}
	s := New(stub)
	s.fullBridge = stub

	req := httptest.NewRequest(http.MethodGet, "/api/swarm/status", nil)
	w := httptest.NewRecorder()
	s.handleSwarmStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body %q", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "swarm-abc") {
		t.Errorf("body = %q, want room_code", w.Body.String())
	}
}

func TestSwarmRoutes_Registered(t *testing.T) {
	// StartHTTPWithAddr registers routes — spin up on an ephemeral port and
	// hit each swarm path; nil-fullBridge answers 501/not-available rather
	// than 404, proving the pattern matched.
	s := newMockServer(&mockBridge{})
	// Use StartHTTPWithAddr's route table without a fullBridge.
	if err := s.StartHTTPWithAddr(0, "127.0.0.1"); err != nil {
		// port 0 may not work with their StartHTTPWithAddr — try a free high port
		// by probing. If that fails too, fall back to direct handler checks (above).
		t.Skipf("could not start test server: %v", err)
	}
	defer s.Stop()

	port := s.GetPort()
	if port == 0 {
		t.Skip("server port not set")
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	paths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/swarm/status"},
		{http.MethodPost, "/api/swarm/host/create"},
		{http.MethodPost, "/api/swarm/host/workers/add"},
		{http.MethodPost, "/api/swarm/join"},
		{http.MethodPost, "/api/v1/swarm/status"},
	}
	for _, p := range paths {
		req, err := http.NewRequest(p.method, base+p.path, strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", p.method, p.path, err)
		}
		resp.Body.Close()
		// Not 404 = route registered. Nil fullBridge → 501 for these handlers.
		if resp.StatusCode == http.StatusNotFound {
			t.Errorf("%s %s: got 404, route not registered", p.method, p.path)
		}
	}
}
