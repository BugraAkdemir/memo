package webserver

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
)

// Direct (non-agent) interactive-browser-session endpoints — BrowserPane's
// own URL bar, click-on-screenshot, scroll-wheel, and close button call
// these, bypassing the agent tool/permission-request pipeline entirely
// (nothing to ask permission for when the user is driving it themselves).
// Response shape is deliberately uniform across navigate/click/scroll: a
// base64 PNG plus the tab's resolved URL, so the frontend can update its
// pane from a single round trip without a second fetch.

type browserSessionActionResponse struct {
	ScreenshotBase64 string `json:"screenshot_base64,omitempty"`
	URL              string `json:"url,omitempty"`
	Error            string `json:"error,omitempty"`
}

func (s *Server) handleBrowserSessionNavigate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		http.Error(w, "bad json (url required)", http.StatusBadRequest)
		return
	}
	shot, resolvedURL, err := s.fullBridge.NavigateBrowserSession(r.Context(), req.URL)
	if err != nil {
		writeJSON(w, browserSessionActionResponse{Error: err.Error()})
		return
	}
	writeJSON(w, browserSessionActionResponse{
		ScreenshotBase64: base64.StdEncoding.EncodeToString(shot),
		URL:              resolvedURL,
	})
}

func (s *Server) handleBrowserSessionClick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	shot, resolvedURL, err := s.fullBridge.ClickBrowserSession(r.Context(), req.X, req.Y)
	if err != nil {
		writeJSON(w, browserSessionActionResponse{Error: err.Error()})
		return
	}
	writeJSON(w, browserSessionActionResponse{
		ScreenshotBase64: base64.StdEncoding.EncodeToString(shot),
		URL:              resolvedURL,
	})
}

func (s *Server) handleBrowserSessionScroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Dx int `json:"dx"`
		Dy int `json:"dy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	shot, resolvedURL, err := s.fullBridge.ScrollBrowserSession(r.Context(), req.Dx, req.Dy)
	if err != nil {
		writeJSON(w, browserSessionActionResponse{Error: err.Error()})
		return
	}
	writeJSON(w, browserSessionActionResponse{
		ScreenshotBase64: base64.StdEncoding.EncodeToString(shot),
		URL:              resolvedURL,
	})
}

func (s *Server) handleBrowserSessionClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusMethodNotAllowed)
		return
	}
	if err := s.fullBridge.CloseBrowserSession(); err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleBrowserSessionStatus(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		writeJSON(w, map[string]any{"active": false})
		return
	}
	active, url := s.fullBridge.BrowserSessionStatus(r.Context())
	writeJSON(w, map[string]any{"active": active, "url": url})
}
