// SPDX-License-Identifier: AGPL-3.0-or-later

package proactive

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDoctorBridge_IngestIncidentAndPendingStore(t *testing.T) {
	store := NewPendingStore(filepath.Join(t.TempDir(), "pending.json"))
	bridge := NewDoctorBridge(store)

	sugg := bridge.Ingest(DoctorEventPayload{
		Source: "korteks-doctor",
		Report: DoctorHealthReport{
			Status:      "degraded",
			FailedUnits: []string{"docker.service"},
			CheckedAt:   time.Now().UTC(),
			Incidents: []DoctorIncident{
				{
					ID:            "systemd-docker.service",
					Category:      "systemd",
					Severity:      "warn",
					Summary:       "Failed systemd service detected: docker.service",
					SuggestedTool: "sys_systemd_manage",
					SuggestedArgs: `{"action":"restart","unit":"docker.service"}`,
					DetectedAt:    time.Now().UTC(),
				},
			},
		},
		VRAM: DoctorVRAMDecision{Action: "keep", Reason: "VRAM headroom healthy"},
	})

	if sugg == nil {
		t.Fatalf("expected proactive suggestion for warning incident")
	}
	if sugg.Action != ActionSuggest {
		t.Fatalf("expected ActionSuggest, got %q", sugg.Action)
	}
	if !strings.Contains(sugg.Message, "docker.service") {
		t.Fatalf("expected unit in message, got %q", sugg.Message)
	}

	gotPending, err := store.Get()
	if err != nil || gotPending == nil {
		t.Fatalf("expected pending suggestion persisted, err=%v", err)
	}

	brief := bridge.FormatDailyBriefingSection(4)
	if !strings.Contains(brief, "Btrfs Pre-Memo Snapshot Sayısı:** 4") {
		t.Fatalf("expected snapshot count in briefing, got %s", brief)
	}
	if !strings.Contains(brief, "docker.service") {
		t.Fatalf("expected incident details in briefing, got %s", brief)
	}
}

func TestDoctorBridge_HealthyReturnsNilAndVRAMUnloadNotifies(t *testing.T) {
	bridge := NewDoctorBridge(nil)

	if sugg := bridge.Ingest(DoctorEventPayload{
		Report: DoctorHealthReport{Status: "healthy"},
		VRAM:   DoctorVRAMDecision{Action: "keep", Reason: "ok"},
	}); sugg != nil {
		t.Fatalf("expected nil suggestion when healthy, got %+v", sugg)
	}

	vramSugg := bridge.Ingest(DoctorEventPayload{
		VRAM: DoctorVRAMDecision{
			Action:      "unload",
			Reason:      "GPU/VRAM usage at 92.0%; unloading local LLM",
			BusyProcess: "blender",
		},
	})
	if vramSugg == nil || vramSugg.Action != ActionNotify {
		t.Fatalf("expected ActionNotify for VRAM unload, got %+v", vramSugg)
	}
}
