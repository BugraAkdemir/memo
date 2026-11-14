// SPDX-License-Identifier: AGPL-3.0-or-later

package proactive

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const maxDoctorHistory = 50

// DoctorIncident mirrors doctor.Incident sent by korteks-doctor.
type DoctorIncident struct {
	ID            string    `json:"id"`
	Category      string    `json:"category"` // systemd, pipewire, gpu, btrfs, oom
	Severity      string    `json:"severity"` // warn, critical
	Summary       string    `json:"summary"`
	LogExcerpt    string    `json:"log_excerpt,omitempty"`
	SuggestedTool string    `json:"suggested_tool,omitempty"`
	SuggestedArgs string    `json:"suggested_args,omitempty"`
	DetectedAt    time.Time `json:"detected_at"`
}

// DoctorHealthReport mirrors doctor.HealthReport from korteks-doctor.
type DoctorHealthReport struct {
	Status      string           `json:"status"` // healthy, degraded, critical
	FailedUnits []string         `json:"failed_units,omitempty"`
	Incidents   []DoctorIncident `json:"incidents"`
	CheckedAt   time.Time        `json:"checked_at"`
}

// DoctorVRAMDecision mirrors gpu.ArbiterDecision from korteks-doctor.
type DoctorVRAMDecision struct {
	Action      string `json:"action"` // keep, swap-to-light, unload
	Reason      string `json:"reason"`
	FreeVRAMMB  int    `json:"free_vram_mb,omitempty"`
	BusyProcess string `json:"busy_process,omitempty"`
}

// DoctorEventPayload is the HTTP JSON body POSTed to /api/os/doctor/event.
type DoctorEventPayload struct {
	Source string             `json:"source"`
	Report DoctorHealthReport `json:"report"`
	VRAM   DoctorVRAMDecision `json:"vram"`
	SentAt time.Time          `json:"sent_at"`
}

// DoctorBridge receives OS anomaly reports from korteks-doctor, turns them
// into proactive remediation suggestions, and feeds Daily Briefing summaries.
type DoctorBridge struct {
	mu      sync.RWMutex
	history []DoctorEventPayload
	pending *PendingStore
}

var (
	defaultDoctorBridgeOnce sync.Once
	defaultDoctorBridgeInst *DoctorBridge
)

// DefaultDoctorBridge returns the process-wide DoctorBridge singleton.
func DefaultDoctorBridge() *DoctorBridge {
	defaultDoctorBridgeOnce.Do(func() {
		defaultDoctorBridgeInst = NewDoctorBridge(nil)
	})
	return defaultDoctorBridgeInst
}

// NewDoctorBridge creates a DoctorBridge backed by an optional PendingStore.
func NewDoctorBridge(pending *PendingStore) *DoctorBridge {
	return &DoctorBridge{pending: pending}
}

// Ingest stores a health event from korteks-doctor and returns a proactive
// suggestion when actionable warnings, critical incidents, or VRAM pressure occur.
func (b *DoctorBridge) Ingest(payload DoctorEventPayload) *PendingSuggestion {
	if payload.SentAt.IsZero() {
		payload.SentAt = time.Now().UTC()
	}
	if payload.Source == "" {
		payload.Source = "korteks-doctor"
	}

	b.mu.Lock()
	b.history = append(b.history, payload)
	if len(b.history) > maxDoctorHistory {
		b.history = append([]DoctorEventPayload(nil), b.history[len(b.history)-maxDoctorHistory:]...)
	}
	b.mu.Unlock()

	suggestion := buildDoctorSuggestion(payload)
	if suggestion != nil && b.pending != nil {
		_ = b.pending.Set(*suggestion)
	}
	return suggestion
}

// Latest returns the most recently ingested korteks-doctor event, if any.
func (b *DoctorBridge) Latest() (DoctorEventPayload, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.history) == 0 {
		return DoctorEventPayload{}, false
	}
	return b.history[len(b.history)-1], true
}

// History returns a shallow copy of recent korteks-doctor events (oldest first).
func (b *DoctorBridge) History() []DoctorEventPayload {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]DoctorEventPayload, len(b.history))
	copy(out, b.history)
	return out
}

// FormatDailyBriefingSection builds a Markdown summary card suitable for
// Memo's Daily Briefing based on recent korteks-doctor telemetry and Btrfs snapshots.
func (b *DoctorBridge) FormatDailyBriefingSection(snapshotCount int) string {
	latest, ok := b.Latest()
	var sb strings.Builder
	sb.WriteString("### KorteksOS Sistem Sağlığı & Btrfs Snapshot Özeti\n")
	if !ok {
		sb.WriteString(fmt.Sprintf("- **Durum:** Sağlıklı (korteks-doctor aktif olay bildirmedi)\n- **Btrfs Pre-Memo Snapshot Sayısı:** %d\n", snapshotCount))
		return sb.String()
	}

	incCount := len(latest.Report.Incidents)
	statusLabel := "Sağlıklı"
	if incCount > 0 {
		statusLabel = fmt.Sprintf("%d anomali tespit edildi (%s)", incCount, latest.Report.Status)
	}
	sb.WriteString(fmt.Sprintf("- **Durum:** %s\n", statusLabel))
	sb.WriteString(fmt.Sprintf("- **Btrfs Pre-Memo Snapshot Sayısı:** %d (`@memo-vault` izole)\n", snapshotCount))
	if latest.VRAM.Action != "" && latest.VRAM.Action != "keep" {
		sb.WriteString(fmt.Sprintf("- **VRAM Arbiter:** `%s` (%s)\n", latest.VRAM.Action, latest.VRAM.Reason))
	}
	for _, inc := range latest.Report.Incidents {
		rem := inc.SuggestedTool
		if rem == "" {
			rem = "sys_btrfs_snapshot veya sys_systemd_manage ile incele"
		} else if inc.SuggestedArgs != "" {
			rem = rem + " " + inc.SuggestedArgs
		}
		sb.WriteString(fmt.Sprintf("- **[%s] %s (%s):** %s — *Önerilen Araç:* `%s`\n",
			strings.ToUpper(inc.Severity), inc.ID, inc.Category, inc.Summary, rem))
	}
	return sb.String()
}

func buildDoctorSuggestion(payload DoctorEventPayload) *PendingSuggestion {
	var top *DoctorIncident
	for i := range payload.Report.Incidents {
		inc := &payload.Report.Incidents[i]
		if top == nil || severityRank(inc.Severity) > severityRank(top.Severity) {
			top = inc
		}
	}

	if top == nil && (payload.VRAM.Action == "" || payload.VRAM.Action == "keep") {
		return nil
	}

	var msg string
	var patternID string
	action := ActionSuggest

	if top != nil {
		patternID = "os_doctor:" + top.Category + ":" + top.ID
		rem := "Son pre-memo snapshot'ı kontrol edip kurtarmamı ister misin?"
		if top.SuggestedTool != "" {
			rem = fmt.Sprintf("%s(%s)", top.SuggestedTool, top.SuggestedArgs)
		}
		msg = fmt.Sprintf("[Korteks Doctor - %s] %s (%s): %s. Önerilen adım: %s",
			strings.ToUpper(top.Severity), top.ID, top.Category, top.Summary, rem)
		if strings.EqualFold(top.Severity, "critical") {
			action = ActionNotify
		}
	} else {
		patternID = "os_vram:" + payload.VRAM.Action
		msg = fmt.Sprintf("[VRAM Arbiter] Bellek baskısı nedeniyle %s kararı alındı (%s).",
			payload.VRAM.Action, payload.VRAM.Reason)
		action = ActionNotify
	}

	return &PendingSuggestion{
		ID:        uuid.NewString(),
		Message:   msg,
		PatternID: patternID,
		Action:    action,
		CreatedAt: time.Now().UTC(),
	}
}

func severityRank(sev string) int {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "critical":
		return 3
	case "warn", "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}
