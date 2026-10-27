// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memo/internal/telemetry"
)

// telemetryStub is a bridge that also supports telemetry. The base stub does
// not, which is how the "not available" answer is tested.
type telemetryStub struct {
	*swarmStubBridge
	status   telemetry.Status
	preview  *telemetry.PreviewResult
	consent  []bool
	resets   int
	deletes  int
	failWith error
}

func (b *telemetryStub) TelemetryStatus() telemetry.Status { return b.status }
func (b *telemetryStub) SetTelemetryConsent(yes bool) error {
	b.consent = append(b.consent, yes)
	return b.failWith
}
func (b *telemetryStub) TelemetryPreview(context.Context) (*telemetry.PreviewResult, error) {
	return b.preview, b.failWith
}
func (b *telemetryStub) TelemetryResetID(context.Context) error { b.resets++; return b.failWith }
func (b *telemetryStub) TelemetryDelete(context.Context) error  { b.deletes++; return b.failWith }

func newTelemetryServer(stub *telemetryStub) *Server {
	s := New(stub)
	s.fullBridge = stub
	return s
}

// serve calls the handler registered for path directly (the mux itself is built inside StartHTTPWithAddr);
// who may reach it is checked by the restricted-user test in admin_gate_test.go.
func serve(s *Server, method, path, body string) *httptest.ResponseRecorder {
	handlers := map[string]http.HandlerFunc{
		"/api/telemetry/status":   s.handleTelemetryStatus,
		"/api/telemetry/preview":  s.handleTelemetryPreview,
		"/api/telemetry/consent":  s.handleTelemetryConsent,
		"/api/telemetry/reset-id": s.handleTelemetryResetID,
		"/api/telemetry/delete":   s.handleTelemetryDelete,
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	w := httptest.NewRecorder()
	handlers[path](w, req)
	return w
}

func TestTelemetryStatus_ReportsTheCurrentAnswer(t *testing.T) {
	stub := &telemetryStub{swarmStubBridge: &swarmStubBridge{}, status: telemetry.Status{Enabled: true, Consent: telemetry.ConsentYes, Build: "official", Endpoint: "https://tel.memocpp.com", DeviceID: "0f3c9a1e"}}
	w := serve(newTelemetryServer(stub), "GET", "/api/telemetry/status", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if _, stale := got["needs_ask"]; stale || got["enabled"] != true || got["consent"] != "yes" || got["build"] != "official" || got["device_id"] != "0f3c9a1e" {
		t.Fatalf("body = %s", w.Body)
	}
}

func TestTelemetryPreview_ReturnsTheExactBytesAsAString(t *testing.T) {
	raw := `{"schema":1,"day":"2026-10-26"}`
	stub := &telemetryStub{swarmStubBridge: &swarmStubBridge{}, preview: &telemetry.PreviewResult{Day: "2026-10-26", JSON: []byte(raw)}}
	w := serve(newTelemetryServer(stub), "GET", "/api/telemetry/preview", "")
	var got struct {
		Day   string `json:"day"`
		Empty bool   `json:"empty"`
		JSON  string `json:"json"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || w.Code != 200 {
		t.Fatalf("%d %s %v", w.Code, w.Body, err)
	}
	if got.JSON != raw || got.Day != "2026-10-26" || got.Empty {
		t.Fatalf("preview = %+v", got)
	}
}

func TestTelemetryPreview_AQuietDayIsEmptyNotAnError(t *testing.T) {
	stub := &telemetryStub{swarmStubBridge: &swarmStubBridge{}, preview: &telemetry.PreviewResult{Empty: true}}
	w := serve(newTelemetryServer(stub), "GET", "/api/telemetry/preview", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"empty":true`) || !strings.Contains(w.Body.String(), `"json":""`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestTelemetryConsent_RecordsTheAnswerAndReturnsTheNewStatus(t *testing.T) {
	stub := &telemetryStub{swarmStubBridge: &swarmStubBridge{}}
	s := newTelemetryServer(stub)
	if w := serve(s, "POST", "/api/telemetry/consent", `{"share":true}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w := serve(s, "POST", "/api/telemetry/consent", `{"share":false}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if len(stub.consent) != 2 || !stub.consent[0] || stub.consent[1] {
		t.Fatalf("answers = %v", stub.consent)
	}
}

func TestTelemetryConsent_RefusesAnythingThatIsNotAnExplicitYesOrNo(t *testing.T) {
	stub := &telemetryStub{swarmStubBridge: &swarmStubBridge{}}
	s := newTelemetryServer(stub)
	for _, body := range []string{``, `{}`, `{"share":"yes"}`, `not json`, `{"share":true,"extra":1}`, `[]`} {
		if w := serve(s, "POST", "/api/telemetry/consent", body); w.Code != http.StatusBadRequest {
			t.Errorf("body %q: status %d, want 400", body, w.Code)
		}
	}
	if len(stub.consent) != 0 {
		t.Fatalf("an unclear request recorded an answer: %v", stub.consent)
	}
}

func TestTelemetryResetAndDelete_CallTheBridge(t *testing.T) {
	stub := &telemetryStub{swarmStubBridge: &swarmStubBridge{}}
	s := newTelemetryServer(stub)
	if w := serve(s, "POST", "/api/telemetry/reset-id", ""); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w := serve(s, "POST", "/api/telemetry/delete", ""); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if stub.resets != 1 || stub.deletes != 1 {
		t.Fatalf("resets=%d deletes=%d", stub.resets, stub.deletes)
	}
}

func TestTelemetry_ABridgeFailureIsAnErrorNotAFalseSuccess(t *testing.T) {
	stub := &telemetryStub{swarmStubBridge: &swarmStubBridge{}, failWith: errors.New("disk full")}
	s := newTelemetryServer(stub)
	for _, c := range []struct{ method, path, body string }{
		{"POST", "/api/telemetry/consent", `{"share":true}`}, {"POST", "/api/telemetry/reset-id", ""}, {"POST", "/api/telemetry/delete", ""}, {"GET", "/api/telemetry/preview", ""},
	} {
		if w := serve(s, c.method, c.path, c.body); w.Code != http.StatusInternalServerError {
			t.Errorf("%s %s: status %d, want 500", c.method, c.path, w.Code)
		}
	}
}

func TestTelemetry_WrongMethodsAreRefused(t *testing.T) {
	s := newTelemetryServer(&telemetryStub{swarmStubBridge: &swarmStubBridge{}})
	for _, c := range []struct{ method, path string }{
		{"POST", "/api/telemetry/status"}, {"POST", "/api/telemetry/preview"}, {"GET", "/api/telemetry/consent"}, {"GET", "/api/telemetry/reset-id"}, {"GET", "/api/telemetry/delete"}, {"PUT", "/api/telemetry/consent"},
	} {
		if w := serve(s, c.method, c.path, "{}"); w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: status %d, want 405", c.method, c.path, w.Code)
		}
	}
}

func TestTelemetry_ABridgeWithoutTelemetrySaysSo(t *testing.T) {
	stub := &swarmStubBridge{}
	s := New(stub)
	s.fullBridge = stub
	for _, c := range []struct{ method, path string }{{"GET", "/api/telemetry/status"}, {"GET", "/api/telemetry/preview"}, {"POST", "/api/telemetry/consent"}} {
		if w := serve(s, c.method, c.path, `{"share":true}`); w.Code != http.StatusNotImplemented {
			t.Errorf("%s %s: status %d, want 501", c.method, c.path, w.Code)
		}
	}
}
