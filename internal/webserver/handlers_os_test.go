// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"memo/internal/proactive"
)

func startMockSysdSocket(t *testing.T) (string, *[]string, *sync.Mutex) {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "sysd.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	var mu sync.Mutex
	var calls []string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "mode": "hybrid"})
	})
	mux.HandleFunc("/v1/mode", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		calls = append(calls, "mode:"+body["mode"])
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"mode": body["mode"]})
	})
	mux.HandleFunc("/v1/execute", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		calls = append(calls, "exec:"+req["tool"].(string)+":"+req["action"].(string))
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "executed", "ring": 1})
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
		_ = os.Remove(sock)
	})
	return sock, &calls, &mu
}

func TestOSHandlers_DoctorStatusAndOOBE(t *testing.T) {
	sock, calls, mu := startMockSysdSocket(t)
	marker := filepath.Join(t.TempDir(), ".oobe-complete")

	t.Setenv("KORTEKS_SYSD_SOCKET", sock)
	t.Setenv("KORTEKS_OS_MODE", "1")
	t.Setenv("KORTEKS_OOBE_MODE", "1")
	t.Setenv("KORTEKS_OOBE_MARKER", marker)

	s := &Server{}

	// 1. POST /api/os/doctor/event
	payload := proactive.DoctorEventPayload{
		Source: "korteks-doctor",
		Report: proactive.DoctorHealthReport{
			Status:    "critical",
			CheckedAt: time.Now().UTC(),
			Incidents: []proactive.DoctorIncident{{
				ID:            "btrfs-io-warning",
				Category:      "btrfs",
				Severity:      "critical",
				Summary:       "Btrfs checksum mismatch detected",
				SuggestedTool: "sys_btrfs_snapshot",
				DetectedAt:    time.Now().UTC(),
			}},
		},
		VRAM: proactive.DoctorVRAMDecision{Action: "keep", Reason: "ok"},
	}
	raw, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/os/doctor/event", bytes.NewReader(raw))
	w := httptest.NewRecorder()
	s.handleOSDoctorEvent(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("doctor event status=%d body=%s", w.Code, w.Body.String())
	}

	// 2. GET /api/os/status
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/os/status", nil)
	wStatus := httptest.NewRecorder()
	s.handleOSStatus(wStatus, reqStatus)
	if wStatus.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", wStatus.Code, wStatus.Body.String())
	}
	var statusResp map[string]any
	if err := json.Unmarshal(wStatus.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if statusResp["os_mode"] != true || statusResp["sysd_reachable"] != true {
		t.Fatalf("unexpected os status: %+v", statusResp)
	}
	if _, ok := statusResp["latest_doctor_report"]; !ok {
		t.Fatalf("expected latest_doctor_report in /api/os/status")
	}

	// 3. POST /api/os/oobe/complete
	oobeBody := `{"ring_mode":"autonomous","apu_boost":true,"selected_model":"Qwen2.5-Coder-7B","distrobox_stacks":["go","python-ai"],"voice_enabled":true}`
	reqOOBE := httptest.NewRequest(http.MethodPost, "/api/os/oobe/complete", bytes.NewBufferString(oobeBody))
	wOOBE := httptest.NewRecorder()
	s.handleOSOOBEComplete(wOOBE, reqOOBE)
	if wOOBE.Code != http.StatusOK {
		t.Fatalf("oobe status=%d body=%s", wOOBE.Code, wOOBE.Body.String())
	}
	var oobeResp map[string]any
	if err := json.Unmarshal(wOOBE.Body.Bytes(), &oobeResp); err != nil {
		t.Fatalf("decode oobe resp: %v", err)
	}
	if oobeResp["sysd_synced"] != true {
		t.Fatalf("expected sysd_synced=true, got %+v", oobeResp)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("expected oobe marker file created: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(*calls) < 3 {
		t.Fatalf("expected mode + 2 distrobox calls on mock sysd, got %v", *calls)
	}
}
