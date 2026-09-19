package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleBrowserSessionNavigate_RoundTrip(t *testing.T) {
	var gotURL string
	bridge := &swarmStubBridge{
		navigateBrowserSession: func(ctx context.Context, rawURL string) ([]byte, string, error) {
			gotURL = rawURL
			return []byte{0x89, 0x50, 0x4E, 0x47}, "https://example.com/", nil
		},
	}
	s := &Server{fullBridge: bridge}

	req := httptest.NewRequest(http.MethodPost, "/api/browser/session/navigate",
		strings.NewReader(`{"url":"https://example.com"}`))
	rec := httptest.NewRecorder()
	s.handleBrowserSessionNavigate(rec, req)

	if gotURL != "https://example.com" {
		t.Errorf("bridge received url=%q, want https://example.com", gotURL)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp browserSessionActionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response did not decode: %v (body=%s)", err, rec.Body.String())
	}
	if resp.URL != "https://example.com/" {
		t.Errorf("resp.URL = %q, want https://example.com/", resp.URL)
	}
	if resp.ScreenshotBase64 == "" {
		t.Error("resp.ScreenshotBase64 is empty, want the base64-encoded PNG")
	}
	if resp.Error != "" {
		t.Errorf("resp.Error = %q, want empty on success", resp.Error)
	}
}

func TestHandleBrowserSessionNavigate_MissingURL_Returns400(t *testing.T) {
	s := &Server{fullBridge: &swarmStubBridge{}}
	req := httptest.NewRequest(http.MethodPost, "/api/browser/session/navigate", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	s.handleBrowserSessionNavigate(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for a missing url", rec.Code)
	}
}

func TestHandleBrowserSessionNavigate_GetNotAllowed(t *testing.T) {
	s := &Server{fullBridge: &swarmStubBridge{}}
	req := httptest.NewRequest(http.MethodGet, "/api/browser/session/navigate", nil)
	rec := httptest.NewRecorder()
	s.handleBrowserSessionNavigate(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405 for GET", rec.Code)
	}
}

func TestHandleBrowserSessionNavigate_BridgeError_ReturnsErrorField(t *testing.T) {
	bridge := &swarmStubBridge{
		navigateBrowserSession: func(ctx context.Context, rawURL string) ([]byte, string, error) {
			return nil, "", errors.New("no active browser session")
		},
	}
	s := &Server{fullBridge: bridge}
	req := httptest.NewRequest(http.MethodPost, "/api/browser/session/navigate",
		strings.NewReader(`{"url":"https://example.com"}`))
	rec := httptest.NewRecorder()
	s.handleBrowserSessionNavigate(rec, req)

	// A bridge-level failure (browser not installed, launch failed) is
	// reported as a 200 with an Error field, not an HTTP error status — the
	// pane needs to show the message inline, this isn't a transport failure.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (errors are reported in the body)", rec.Code)
	}
	var resp browserSessionActionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response did not decode: %v", err)
	}
	if resp.Error == "" {
		t.Error("resp.Error is empty, want the bridge's error message")
	}
}

func TestHandleBrowserSessionClick_PassesCoordinates(t *testing.T) {
	var gotX, gotY float64
	bridge := &swarmStubBridge{
		clickBrowserSession: func(ctx context.Context, x, y float64) ([]byte, string, error) {
			gotX, gotY = x, y
			return []byte{1, 2, 3}, "https://example.com/", nil
		},
	}
	s := &Server{fullBridge: bridge}
	req := httptest.NewRequest(http.MethodPost, "/api/browser/session/click",
		strings.NewReader(`{"x":120.5,"y":340}`))
	rec := httptest.NewRecorder()
	s.handleBrowserSessionClick(rec, req)

	if gotX != 120.5 || gotY != 340 {
		t.Errorf("bridge received (x,y) = (%v, %v), want (120.5, 340)", gotX, gotY)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleBrowserSessionScroll_PassesDeltas(t *testing.T) {
	var gotDx, gotDy int
	bridge := &swarmStubBridge{
		scrollBrowserSession: func(ctx context.Context, dx, dy int) ([]byte, string, error) {
			gotDx, gotDy = dx, dy
			return []byte{1}, "", nil
		},
	}
	s := &Server{fullBridge: bridge}
	req := httptest.NewRequest(http.MethodPost, "/api/browser/session/scroll",
		strings.NewReader(`{"dx":0,"dy":600}`))
	rec := httptest.NewRecorder()
	s.handleBrowserSessionScroll(rec, req)

	if gotDx != 0 || gotDy != 600 {
		t.Errorf("bridge received (dx,dy) = (%d, %d), want (0, 600)", gotDx, gotDy)
	}
}

func TestHandleBrowserSessionClose_CallsBridge(t *testing.T) {
	called := false
	bridge := &swarmStubBridge{
		closeBrowserSession: func() error {
			called = true
			return nil
		},
	}
	s := &Server{fullBridge: bridge}
	req := httptest.NewRequest(http.MethodPost, "/api/browser/session/close", nil)
	rec := httptest.NewRecorder()
	s.handleBrowserSessionClose(rec, req)

	if !called {
		t.Error("CloseBrowserSession was never called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleBrowserSessionStatus_ReportsActiveAndURL(t *testing.T) {
	bridge := &swarmStubBridge{
		browserSessionStatus: func(ctx context.Context) (bool, string) {
			return true, "https://example.com/"
		},
	}
	s := &Server{fullBridge: bridge}
	req := httptest.NewRequest(http.MethodGet, "/api/browser/session/status", nil)
	rec := httptest.NewRecorder()
	s.handleBrowserSessionStatus(rec, req)

	var resp struct {
		Active bool   `json:"active"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response did not decode: %v", err)
	}
	if !resp.Active || resp.URL != "https://example.com/" {
		t.Errorf("resp = %+v, want active=true url=https://example.com/", resp)
	}
}

func TestHandleBrowserSessionStatus_NoFullBridge_ReportsInactive(t *testing.T) {
	s := &Server{fullBridge: nil}
	req := httptest.NewRequest(http.MethodGet, "/api/browser/session/status", nil)
	rec := httptest.NewRecorder()
	s.handleBrowserSessionStatus(rec, req)

	var resp struct {
		Active bool `json:"active"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response did not decode: %v", err)
	}
	if resp.Active {
		t.Error("resp.Active = true with no fullBridge configured, want false")
	}
}
