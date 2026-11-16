// SPDX-License-Identifier: AGPL-3.0-or-later

package cliproxy

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// EndedSourceInference records that an actual model request was rejected
	// with 403/402/entitlement-ended. Kept until explicit recheck, logout, or login.
	EndedSourceInference = "inference"
	// EndedSourceQuota records that the vendor's usage/quota probe returned 403/402.
	EndedSourceQuota = "quota"
)

// EndedVendor remembers why a subscription vendor's models were hidden.
type EndedVendor struct {
	Source string `json:"source"`
	Reason string `json:"reason,omitempty"`
	At     string `json:"at"`
}

// ProviderForOwner maps a model's owned_by field from the sidecar's /v1/models
// into the matching login provider key (antigravity, claude, codex).
// Returns "" when ownedBy is empty or unknown.
func ProviderForOwner(ownedBy string) string {
	switch strings.ToLower(strings.TrimSpace(ownedBy)) {
	case ProviderAntigravity:
		return ProviderAntigravity
	case ProviderClaude, "anthropic":
		return ProviderClaude
	case ProviderCodex, "openai":
		return ProviderCodex
	default:
		return ""
	}
}

func (m *Manager) endedPath() string { return filepath.Join(m.dir, "ended.json") }

func (m *Manager) loadEnded() {
	if m.dir == "" {
		return
	}
	b, err := os.ReadFile(m.endedPath())
	if err != nil {
		return
	}
	var loaded map[string]EndedVendor
	if json.Unmarshal(b, &loaded) != nil || len(loaded) == 0 {
		return
	}
	m.endedMu.Lock()
	m.ended = loaded
	m.endedMu.Unlock()
}

func (m *Manager) saveEndedLocked() {
	if m.dir == "" {
		return
	}
	if len(m.ended) == 0 {
		_ = os.Remove(m.endedPath())
		return
	}
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return
	}
	b, err := json.Marshal(m.ended)
	if err != nil {
		return
	}
	_ = os.WriteFile(m.endedPath(), b, 0o600)
}

// MarkVendorEnded marks provider (antigravity|claude|codex) as having no usable
// usage entitlement. Returns true if the provider was newly marked ended.
func (m *Manager) MarkVendorEnded(provider, source, reason string) bool {
	if !validProvider(provider) {
		return false
	}
	if source == "" {
		source = EndedSourceInference
	}
	m.endedMu.Lock()
	defer m.endedMu.Unlock()
	if m.ended == nil {
		m.ended = map[string]EndedVendor{}
	}
	prev, existed := m.ended[provider]
	// Never downgrade an inference rejection into a quota-probe one so a
	// background usage poll returning 200 cannot silently re-expose broken models.
	if existed && prev.Source == EndedSourceInference && source == EndedSourceQuota {
		return false
	}
	m.ended[provider] = EndedVendor{
		Source: source,
		Reason: strings.TrimSpace(reason),
		At:     time.Now().UTC().Format(time.RFC3339),
	}
	m.saveEndedLocked()
	return !existed
}

// ClearVendorEnded removes any ended marker for provider (e.g. on logout, new
// login, or an explicit user recheck).
func (m *Manager) ClearVendorEnded(provider string) bool {
	m.endedMu.Lock()
	defer m.endedMu.Unlock()
	if _, ok := m.ended[provider]; !ok {
		return false
	}
	delete(m.ended, provider)
	m.saveEndedLocked()
	return true
}

// ClearQuotaEnded removes a quota-probe ended marker once the usage endpoint
// answers 200 OK with valid allowance figures. Inference-sourced rejections are
// preserved until explicit recheck, logout, or fresh sign-in.
func (m *Manager) ClearQuotaEnded(provider string) bool {
	m.endedMu.Lock()
	defer m.endedMu.Unlock()
	cur, ok := m.ended[provider]
	if !ok || cur.Source != EndedSourceQuota {
		return false
	}
	delete(m.ended, provider)
	m.saveEndedLocked()
	return true
}

// IsVendorEnded reports whether provider's usage entitlement is currently
// marked ended or all of its signed-in credentials are disabled on disk.
func (m *Manager) IsVendorEnded(provider string) bool {
	if !validProvider(provider) {
		return false
	}
	m.endedMu.Lock()
	_, marked := m.ended[provider]
	m.endedMu.Unlock()
	if marked {
		return true
	}
	count, disabled := 0, 0
	for _, a := range m.Accounts() {
		if a.Provider == provider {
			count++
			if a.Disabled {
				disabled++
			}
		}
	}
	return count > 0 && disabled == count
}

// FilterActiveModels returns only models whose owning vendor is not ended or
// disabled. Models with an unknown owner are kept.
func (m *Manager) FilterActiveModels(list []Model) []Model {
	if len(list) == 0 {
		return nil
	}
	out := make([]Model, 0, len(list))
	for _, md := range list {
		if p := ProviderForOwner(md.OwnedBy); p != "" && m.IsVendorEnded(p) {
			continue
		}
		out = append(out, md)
	}
	return out
}

// RecheckVendor clears the ended marker for provider (or all providers when
// provider is "") and forces a fresh quota probe. If the vendor still rejects
// the usage check with 403/402, it will immediately be marked ended again.
func (m *Manager) RecheckVendor(ctx context.Context, provider string) {
	if provider == "" {
		for _, p := range Providers {
			m.ClearVendorEnded(p)
		}
	} else {
		m.ClearVendorEnded(provider)
	}
	m.invalidateModels()
	m.QuotaSnapshot(ctx, 8*time.Second)
}
