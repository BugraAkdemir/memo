// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"memo/internal/osbridge"
	"memo/internal/proactive"
)

type oobeCompletionState struct {
	mu          sync.RWMutex
	completed   bool
	completedAt time.Time
	config      *OSOOBECompleteRequest
}

var korteksOOBEState oobeCompletionState

// OSOOBECompleteRequest captures choices made during the KorteksOS First-Boot OOBE wizard.
type OSOOBECompleteRequest struct {
	RingMode        string   `json:"ring_mode"`
	APUBoost        bool     `json:"apu_boost"`
	SelectedModel   string   `json:"selected_model"`
	DistroboxStacks []string `json:"distrobox_stacks"`
	VoiceEnabled    bool     `json:"voice_enabled"`
}

func (s *Server) handleOSDoctorEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload proactive.DoctorEventPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid doctor event payload", http.StatusBadRequest)
		return
	}

	bridge := proactive.DefaultDoctorBridge()
	suggestion := bridge.Ingest(payload)

	writeJSON(w, map[string]any{
		"ok":         true,
		"incidents":  len(payload.Report.Incidents),
		"suggestion": suggestion,
	})
}

func (s *Server) handleOSStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	client := osbridge.NewClient("")
	osMode := os.Getenv("KORTEKS_OS_MODE") == "1"
	if _, err := os.Stat(client.SocketPath); err == nil {
		osMode = true
	}

	ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
	defer cancel()

	var sysdHealth any
	sysdReachable := false
	if rawHealth, err := client.Health(ctx); err == nil {
		sysdReachable = true
		var parsed any
		if json.Unmarshal([]byte(rawHealth), &parsed) == nil {
			sysdHealth = parsed
		} else {
			sysdHealth = rawHealth
		}
	}

	latestReport, hasReport := proactive.DefaultDoctorBridge().Latest()

	korteksOOBEState.mu.RLock()
	oobeDone := korteksOOBEState.completed
	korteksOOBEState.mu.RUnlock()

	if !oobeDone {
		if marker := os.Getenv("KORTEKS_OOBE_MARKER"); marker != "" {
			if _, err := os.Stat(marker); err == nil {
				oobeDone = true
			}
		} else if _, err := os.Stat("/var/lib/korteks/.oobe-complete"); err == nil {
			oobeDone = true
		}
	}

	resp := map[string]any{
		"os_mode":        osMode,
		"oobe_mode":      os.Getenv("KORTEKS_OOBE_MODE") == "1",
		"oobe_complete":  oobeDone,
		"sysd_socket":    client.SocketPath,
		"sysd_reachable": sysdReachable,
		"sysd_health":    sysdHealth,
		"briefing_card":  proactive.DefaultDoctorBridge().FormatDailyBriefingSection(0),
	}
	if hasReport {
		resp["latest_doctor_report"] = latestReport
	}
	writeJSON(w, resp)
}

func (s *Server) handleOSOOBEComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req OSOOBECompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid oobe config json", http.StatusBadRequest)
		return
	}
	if req.RingMode == "" {
		req.RingMode = "hybrid"
	}
	switch req.RingMode {
	case "locked", "hybrid", "autonomous":
	default:
		http.Error(w, "invalid ring_mode (want locked|hybrid|autonomous)", http.StatusBadRequest)
		return
	}

	client := osbridge.NewClient("")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	sysdSynced := false
	if _, err := client.SetMode(ctx, req.RingMode); err == nil {
		sysdSynced = true
	}

	provisioned := make([]string, 0, len(req.DistroboxStacks))
	if sysdSynced {
		for _, stack := range req.DistroboxStacks {
			if stack == "" {
				continue
			}
			res, err := client.Execute(ctx, osbridge.ActionRequest{
				Tool:     "sys_package_env",
				Action:   "distrobox_create",
				Args:     map[string]string{"name": "korteks-" + stack, "template": stack},
				Approved: true,
			})
			if err == nil && res != nil && res.Status == "executed" {
				provisioned = append(provisioned, stack)
			}
		}
	}

	if marker := os.Getenv("KORTEKS_OOBE_MARKER"); marker != "" {
		_ = os.MkdirAll(filepath.Dir(marker), 0o755)
		_ = os.WriteFile(marker, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644)
	}

	cfgCopy := req
	korteksOOBEState.mu.Lock()
	korteksOOBEState.completed = true
	korteksOOBEState.completedAt = time.Now().UTC()
	korteksOOBEState.config = &cfgCopy
	korteksOOBEState.mu.Unlock()

	writeJSON(w, map[string]any{
		"ok":                 true,
		"ring_mode":          req.RingMode,
		"apu_boost":          req.APUBoost,
		"sysd_synced":        sysdSynced,
		"provisioned_stacks": provisioned,
	})
}
