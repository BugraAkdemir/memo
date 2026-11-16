// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/cliproxy"
	"memo/internal/config"
)

func seedVendorCred(t *testing.T, providerName, email string) {
	t.Helper()
	dir := config.DataPath("cliproxy", "auth")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := `{"type":"` + providerName + `","email":"` + email + `"}`
	if err := os.WriteFile(filepath.Join(dir, providerName+".json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestIsSubsEntitlementError(t *testing.T) {
	positive := []string{
		"status 403: forbidden",
		"authentication error (status 403): not allowed",
		"⚠️ [custom] status 402: payment required",
		"This account is not allowed to use this model or action. (HTTP 403)",
		"subscription has expired for this account",
		"no active subscription found",
	}
	for _, s := range positive {
		if !isSubsEntitlementError(s) {
			t.Errorf("isSubsEntitlementError(%q) = false, want true", s)
		}
	}

	negative := []string{
		"",
		"status 429: rate limit reached",
		"status 429: You exceeded your current quota, please check your plan",
		"status 401: invalid token",
		"status 500: internal server error",
		"status 404: model not found",
		"dial tcp: connection refused",
	}
	for _, s := range negative {
		if isSubsEntitlementError(s) {
			t.Errorf("isSubsEntitlementError(%q) = true, want false", s)
		}
	}
}

func TestSubsEntitlementEnded_MultiVendorFallsBackAndHidesEndedModels(t *testing.T) {
	fastSubs(t)
	a := syncApp(t)
	seedVendorCred(t, cliproxy.ProviderAntigravity, "ag@example.com")
	seedVendorCred(t, cliproxy.ProviderClaude, "cl@example.com")

	if err := a.syncSubscriptions(context.Background()); err != nil {
		t.Fatalf("syncSubscriptions: %v", err)
	}
	if err := a.SetProviderModel(subsProviderName, "claude-model-a"); err != nil {
		t.Fatalf("SetProviderModel: %v", err)
	}
	a.SetActiveProvider(subsProviderName)

	// Claude hits HTTP 403: Claude models must disappear, Antigravity models stay,
	// active Subscriptions model falls back to Antigravity, and Settings marks
	// Claude as entitlement_ended.
	a.noteProviderFailure(subsProviderName, "claude-model-a", "authentication error (status 403): This account is not allowed")

	if !a.subsManager().IsVendorEnded(cliproxy.ProviderClaude) {
		t.Fatal("expected Claude to be marked ended")
	}
	if a.subsManager().IsVendorEnded(cliproxy.ProviderAntigravity) {
		t.Fatal("Antigravity must remain active")
	}

	models, cur, err := a.ListProviderModels(context.Background(), subsProviderName)
	if err != nil {
		t.Fatalf("ListProviderModels: %v", err)
	}
	for _, m := range models {
		if strings.HasPrefix(m.ID, "claude-") {
			t.Fatalf("ended Claude model %q still present in ListProviderModels", m.ID)
		}
	}
	if len(models) == 0 {
		t.Fatal("expected Antigravity models to remain")
	}
	if !strings.HasPrefix(cur, "antigravity-") {
		t.Fatalf("expected fallback current model to be antigravity-*, got %q", cur)
	}

	gw := a.ListGatewayModels()
	for _, m := range gw {
		if strings.HasPrefix(m.ID, "subs/claude-") {
			t.Fatalf("ended Claude model %q still present in ListGatewayModels", m.ID)
		}
	}

	st := a.SubscriptionsState(context.Background())
	var sawClaudeEnded, sawAGActive bool
	for _, ac := range st.Accounts {
		if ac.Provider == cliproxy.ProviderClaude && ac.EntitlementEnded {
			sawClaudeEnded = true
		}
		if ac.Provider == cliproxy.ProviderAntigravity && !ac.EntitlementEnded {
			sawAGActive = true
		}
	}
	if !sawClaudeEnded || !sawAGActive {
		t.Fatalf("unexpected account states: %+v", st.Accounts)
	}
	for _, m := range st.Models {
		if strings.HasPrefix(m.ID, "claude-") {
			t.Fatalf("ended Claude model %q still present in SubscriptionsState.Models", m.ID)
		}
	}

	// Logout clears Claude's ended state cleanly.
	if err := a.LogoutSubscription(cliproxy.ProviderClaude); err != nil {
		t.Fatalf("LogoutSubscription: %v", err)
	}
	if a.subsManager().IsVendorEnded(cliproxy.ProviderClaude) {
		t.Fatal("expected LogoutSubscription to clear ended state")
	}
}

func TestSubsEntitlementEnded_OnlyVendorDisablesSubscriptionsProvider(t *testing.T) {
	fastSubs(t)
	a := syncApp(t)
	seedVendorCred(t, cliproxy.ProviderClaude, "cl@example.com")

	if err := a.syncSubscriptions(context.Background()); err != nil {
		t.Fatalf("syncSubscriptions: %v", err)
	}
	a.SetActiveProvider(subsProviderName)

	a.noteProviderFailure(subsProviderName, "claude-model-a", "status 403: forbidden")

	if _, ok := a.subsMarkerConfig(); ok {
		t.Fatal("expected Subscriptions provider to be removed when no active vendor remains")
	}
	if got := a.GetActiveProvider(); got != "" {
		t.Fatalf("expected active provider to clear when Subscriptions is removed, got %q", got)
	}

	// FriendlyError for Subscriptions 403 explains removal in both languages.
	for _, lang := range []string{"tr", "en"} {
		fa := friendlyApp(lang)
		fa.SetActiveProvider(subsProviderName)
		got := fa.FriendlyError("⚠️ [custom] authentication error (status 403): forbidden")
		if lang == "tr" && !strings.Contains(got, "kullanım hakkı sona ermiş") {
			t.Errorf("tr FriendlyError = %q", got)
		}
		if lang == "en" && !strings.Contains(got, "usage allowance has ended") {
			t.Errorf("en FriendlyError = %q", got)
		}
	}
}
