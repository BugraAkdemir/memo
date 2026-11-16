// SPDX-License-Identifier: AGPL-3.0-or-later

package cliproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProviderForOwner(t *testing.T) {
	cases := map[string]string{
		"antigravity": ProviderAntigravity,
		"Antigravity": ProviderAntigravity,
		"claude":      ProviderClaude,
		"anthropic":   ProviderClaude,
		"codex":       ProviderCodex,
		"openai":      ProviderCodex,
		"":            "",
		"unknown":     "",
	}
	for in, want := range cases {
		if got := ProviderForOwner(in); got != want {
			t.Errorf("ProviderForOwner(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEntitlementEnded_PersistsAndFiltersModels(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)

	if !m.MarkVendorEnded(ProviderClaude, EndedSourceInference, "status 403") {
		t.Fatal("expected MarkVendorEnded to report newly marked")
	}
	if m.MarkVendorEnded(ProviderClaude, EndedSourceInference, "status 403 again") {
		t.Fatal("expected second MarkVendorEnded to report already marked")
	}
	if !m.IsVendorEnded(ProviderClaude) {
		t.Fatal("expected Claude to be marked ended")
	}
	if m.IsVendorEnded(ProviderAntigravity) {
		t.Fatal("Antigravity must not be marked ended")
	}

	// Reloading from disk keeps the ended state.
	reloaded := New(dir)
	if !reloaded.IsVendorEnded(ProviderClaude) {
		t.Fatal("expected ended state to survive Manager reload")
	}

	// Inference source must not be cleared by ClearQuotaEnded.
	if reloaded.ClearQuotaEnded(ProviderClaude) {
		t.Fatal("ClearQuotaEnded must not clear an inference-sourced ended marker")
	}
	if !reloaded.IsVendorEnded(ProviderClaude) {
		t.Fatal("Claude must still be ended after ClearQuotaEnded")
	}

	raw := []Model{
		{ID: "claude-opus-4-5", OwnedBy: "anthropic"},
		{ID: "claude-sonnet-4-6", OwnedBy: "claude"},
		{ID: "gpt-oss-120b-medium", OwnedBy: "antigravity"},
		{ID: "gpt-5.4", OwnedBy: "openai"},
	}
	filtered := reloaded.FilterActiveModels(raw)
	if len(filtered) != 2 {
		t.Fatalf("got %d models after filtering, want 2: %+v", len(filtered), filtered)
	}
	if filtered[0].ID != "gpt-oss-120b-medium" || filtered[1].ID != "gpt-5.4" {
		t.Fatalf("unexpected filtered models: %+v", filtered)
	}

	if !reloaded.ClearVendorEnded(ProviderClaude) {
		t.Fatal("expected ClearVendorEnded to remove Claude")
	}
	if reloaded.IsVendorEnded(ProviderClaude) {
		t.Fatal("Claude should no longer be ended after ClearVendorEnded")
	}
}

func TestEntitlementEnded_DisabledAccountsOnDiskCountAsEnded(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if err := os.MkdirAll(m.authDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.authDir(), "claude-a.json"),
		[]byte(`{"type":"claude","email":"a@example.com","disabled":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if !m.IsVendorEnded(ProviderClaude) {
		t.Fatal("expected disabled Claude account to report ended")
	}
	// Adding a second enabled account makes the vendor usable again.
	if err := os.WriteFile(filepath.Join(m.authDir(), "claude-b.json"),
		[]byte(`{"type":"claude","email":"b@example.com","disabled":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if m.IsVendorEnded(ProviderClaude) {
		t.Fatal("vendor with an enabled account must not report ended")
	}
}

func TestEntitlementEnded_Quota403MarksAndQuota200Clears(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if err := os.MkdirAll(m.authDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.authDir(), "claude.json"),
		[]byte(`{"type":"claude","email":"c@example.com","access_token":"tok"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	status := http.StatusForbidden
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = w.Write([]byte(`{"five_hour":{"utilization":12.0,"resets_at":"2026-11-16T20:00:00Z"}}`))
		} else {
			_, _ = w.Write([]byte(`{"error":"forbidden"}`))
		}
	}))
	defer srv.Close()

	oldClaudeURL := claudeUsageURL
	claudeUsageURL = srv.URL
	defer func() { claudeUsageURL = oldClaudeURL }()

	_, tried, _ := m.accountQuotas(context.Background())
	if !tried {
		t.Fatal("expected accountQuotas to probe claude")
	}
	if !m.IsVendorEnded(ProviderClaude) {
		t.Fatal("expected 403 from usage endpoint to mark Claude ended")
	}

	// Once usage returns 200 with valid windows, RecheckVendor clears it and keeps it cleared.
	status = http.StatusOK
	m.RecheckVendor(context.Background(), ProviderClaude)
	// Wait briefly in case QuotaSnapshot ran asynchronously; RecheckVendor waits up to 8s.
	deadline := time.Now().Add(2 * time.Second)
	for m.IsVendorEnded(ProviderClaude) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if m.IsVendorEnded(ProviderClaude) {
		t.Fatal("expected 200 OK from usage endpoint to clear quota-ended marker")
	}
}
