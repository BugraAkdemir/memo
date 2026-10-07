// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"memo/internal/cliproxy"
	"memo/internal/config"
	"memo/internal/provider"
)

var qNow = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func TestParseResetFromError(t *testing.T) {
	cases := []struct {
		name string
		text string
		want time.Time
		ok   bool
	}{
		{"codex resets_at (unix seconds)",
			`{"error":{"type":"usage_limit_reached","message":"The usage limit has been reached","resets_at":` + strconv.FormatInt(qNow.Add(5*time.Hour).Unix(), 10) + `,"resets_in_seconds":999}}`,
			time.Unix(qNow.Add(5*time.Hour).Unix(), 0), true},
		{"codex resets_in_seconds only",
			`status 429: {"error":{"type":"usage_limit_reached","resets_in_seconds":3600}}`,
			qNow.Add(time.Hour), true},
		{"antigravity style duration",
			`RESOURCE_EXHAUSTED: You have exhausted your capacity on this model. Your quota will reset after 2h12m3s.`,
			qNow.Add(2*time.Hour + 12*time.Minute + 3*time.Second), true},
		{"retry in seconds", `Please retry in 45s`, qNow.Add(45 * time.Second), true},
		{"try again in", `rate limited, try again in 30s`, qNow.Add(30 * time.Second), true},
		{"retry after fractional", `retry after 1.5s`, qNow.Add(1500 * time.Millisecond), true},
		{"nothing to read", `status 429: too many requests`, time.Time{}, false},
		{"a stale unix stamp is not a reset", `"resets_at": 1000000000`, time.Time{}, false},
	}
	for _, c := range cases {
		got, ok := parseResetFromError(c.text, qNow)
		if ok != c.ok || (ok && !got.Equal(c.want)) {
			t.Errorf("%s: got %v ok=%v, want %v ok=%v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestQuotaWording(t *testing.T) {
	for _, s := range []string{"quota exceeded", "you've hit your usage limit", "usage_limit_reached", "resource_exhausted",
		"you have exhausted your capacity", "insufficient_quota", "the limit has been reached"} {
		if !quotaWording(strings.ToLower(s)) {
			t.Errorf("%q should read as an allowance running out", s)
		}
	}
	// A momentary rate limit or an overloaded server is not "the allowance ran out".
	for _, s := range []string{"status 429: too many requests", "model is overloaded", "at capacity, try later", "slow down", "internal server error"} {
		if quotaWording(strings.ToLower(s)) {
			t.Errorf("%q must not read as quota", s)
		}
	}
}

// subsActiveApp is an App whose active provider is the Subscriptions provider
// on `model`, with the quota figures set by the test.
func subsActiveApp(t *testing.T, model string, set cliproxy.QuotaSet) *App {
	t.Helper()
	isolatedDataDir(t)
	quotaWarned = map[string]time.Time{}
	seed := provider.NewConfigManager(config.DataPath("providers.json"), nil)
	seed.Set(provider.ProviderConfig{Type: provider.ProviderCustom, Name: subsProviderName, BaseURL: "http://127.0.0.1:1/v1", APIKey: "k", Model: model, Enabled: true})
	seed.Save()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	a := &App{cfg: &config.AppConfig{ActiveProvider: subsProviderName}, lifecycleCtx: ctx}
	a.reinitProviderAndOrchestra()
	a.quotaSource = func() cliproxy.QuotaSet { return set }
	return a
}

func TestQuotaSignalForError_ExhaustedWithTheResetTimeFromTheError(t *testing.T) {
	a := subsActiveApp(t, "gpt-5.5", cliproxy.QuotaSet{Models: map[string]cliproxy.Quota{"gpt-5.5": {Remaining: 0, ResetAt: "2026-10-13T00:00:00Z", Window: "7d"}}})
	// QuotaSignalForError reads the real clock, so the refill time is built from it.
	resetAt := time.Now().Add(3 * time.Hour).Unix()
	sig := a.QuotaSignalForError(`all providers failed: [custom] status 429: {"error":{"type":"usage_limit_reached","resets_at":` + strconv.FormatInt(resetAt, 10) + `}}`)
	if sig == nil {
		t.Fatal("a usage-limit error on a metered model must produce a signal")
	}
	if sig.Kind != "exhausted" || sig.Model != "gpt-5.5" || sig.Provider != subsProviderName {
		t.Errorf("signal = %+v", sig)
	}
	if want := time.Unix(resetAt, 0).UTC().Format(time.RFC3339); sig.ResetAt != want {
		t.Errorf("ResetAt = %q, want the one in the error text (%s) over the snapshot's", sig.ResetAt, want)
	}
	if sig.RemainingPercent != 0 || sig.Window != "7d" {
		t.Errorf("remaining/window = %d/%q", sig.RemainingPercent, sig.Window)
	}
}

func TestQuotaSignalForError_FallsBackToTheSnapshotsResetTime(t *testing.T) {
	a := subsActiveApp(t, "claude-sonnet-4-6", cliproxy.QuotaSet{Models: map[string]cliproxy.Quota{"claude-sonnet-4-6": {Remaining: 0.01, ResetAt: "2026-10-13T00:00:00Z", Window: "7d"}}})
	sig := a.QuotaSignalForError(`status 429: RESOURCE_EXHAUSTED`)
	if sig == nil || sig.ResetAt != "2026-10-13T00:00:00Z" {
		t.Fatalf("signal = %+v, want the snapshot's reset time", sig)
	}
}

func TestQuotaSignalForError_AMomentaryRateLimitIsNotAQuotaCard(t *testing.T) {
	// 80% left and an error with no allowance wording: plain 429, shown as before.
	a := subsActiveApp(t, "claude-sonnet-4-6", cliproxy.QuotaSet{Models: map[string]cliproxy.Quota{"claude-sonnet-4-6": {Remaining: 0.8}}})
	if sig := a.QuotaSignalForError(`status 429: too many requests, please slow down`); sig != nil {
		t.Errorf("a momentary rate limit must not offer to wait for a refill: %+v", sig)
	}
	if sig := a.QuotaSignalForError(`status 503: model is overloaded`); sig != nil {
		t.Errorf("an overloaded server is not quota: %+v", sig)
	}
	if sig := a.QuotaSignalForError(`status 401: invalid api key`); sig != nil {
		t.Errorf("an auth error is not quota: %+v", sig)
	}
	if sig := a.QuotaSignalForError(``); sig != nil {
		t.Errorf("empty error: %+v", sig)
	}
}

func TestQuotaSignalForError_AFigureNearZeroCountsEvenWithoutTheWord(t *testing.T) {
	a := subsActiveApp(t, "m", cliproxy.QuotaSet{Models: map[string]cliproxy.Quota{"m": {Remaining: 0.01, ResetAt: "2026-10-08T00:00:00Z"}}})
	if sig := a.QuotaSignalForError(`status 429: too many requests`); sig == nil || sig.ResetAt == "" {
		t.Errorf("a 429 on a model at 1%% left is the allowance running out: %+v", sig)
	}
}

func TestQuotaSignalForError_OtherProvidersGetNoResetTimeButStillACard(t *testing.T) {
	isolatedDataDir(t)
	seed := provider.NewConfigManager(config.DataPath("providers.json"), nil)
	seed.Set(provider.ProviderConfig{Type: provider.ProviderOpenAI, Name: "My OpenAI", APIKey: "k", Model: "gpt-4o", Enabled: true})
	seed.Save()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &App{cfg: &config.AppConfig{ActiveProvider: "My OpenAI"}, lifecycleCtx: ctx}
	a.reinitProviderAndOrchestra()
	sig := a.QuotaSignalForError(`status 429: You exceeded your current quota, please check your plan and billing details.`)
	if sig == nil || sig.ResetAt != "" || sig.RemainingPercent != -1 {
		t.Errorf("signal = %+v, want an exhausted card with no reset time (the UI then retries on its own schedule)", sig)
	}
}

func TestQuotaLowSignal_WarnsOncePerWindowAtTenPercent(t *testing.T) {
	q := cliproxy.Quota{Remaining: 0.08, ResetAt: "2026-10-13T00:00:00Z", Window: "7d"}
	a := subsActiveApp(t, "claude-sonnet-4-6", cliproxy.QuotaSet{Models: map[string]cliproxy.Quota{"claude-sonnet-4-6": q}})

	sig := a.QuotaLowSignal()
	if sig == nil || sig.Kind != "low" || sig.RemainingPercent != 8 || sig.ResetAt != q.ResetAt {
		t.Fatalf("first warning = %+v", sig)
	}
	if again := a.QuotaLowSignal(); again != nil {
		t.Errorf("the same window must not warn twice: %+v", again)
	}
	// A new window (the allowance refilled and ran low again) warns again.
	a.quotaSource = func() cliproxy.QuotaSet {
		return cliproxy.QuotaSet{Models: map[string]cliproxy.Quota{"claude-sonnet-4-6": {Remaining: 0.05, ResetAt: "2026-10-20T00:00:00Z", Window: "7d"}}}
	}
	if next := a.QuotaLowSignal(); next == nil || next.RemainingPercent != 5 {
		t.Errorf("a new window must warn: %+v", next)
	}
}

func TestQuotaLowSignal_SilentAboveTheThresholdAndForOtherProviders(t *testing.T) {
	a := subsActiveApp(t, "m", cliproxy.QuotaSet{Models: map[string]cliproxy.Quota{"m": {Remaining: 0.11}}})
	if sig := a.QuotaLowSignal(); sig != nil {
		t.Errorf("11%% left must not warn: %+v", sig)
	}
	a.quotaSource = func() cliproxy.QuotaSet {
		return cliproxy.QuotaSet{Models: map[string]cliproxy.Quota{"m": {Remaining: 0.10}}}
	}
	if sig := a.QuotaLowSignal(); sig == nil {
		t.Error("exactly 10% left must warn")
	}
	a.quotaSource = func() cliproxy.QuotaSet { return cliproxy.QuotaSet{} }
	quotaWarned = map[string]time.Time{}
	if sig := a.QuotaLowSignal(); sig != nil {
		t.Errorf("no figure, no warning: %+v", sig)
	}
	a.activeProviderName = "something else"
	a.quotaSource = func() cliproxy.QuotaSet {
		return cliproxy.QuotaSet{Models: map[string]cliproxy.Quota{"m": {Remaining: 0.01}}}
	}
	if sig := a.QuotaLowSignal(); sig != nil {
		t.Errorf("another active provider has no metered allowance: %+v", sig)
	}
}
