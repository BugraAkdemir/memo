// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"memo/internal/cliproxy"
	"memo/internal/models"
	"memo/internal/taskloop"
)

// What the chat UI is told about the allowance behind the active model:
//
//   - "exhausted" when a turn failed because the allowance ran out, with the
//     time it refills, so the UI can offer to continue by itself afterwards;
//   - "low" when a turn worked but little is left.
//
// Both are quota questions only for the Subscriptions provider, whose accounts
// have a metered allowance Memo can read. For any other provider only the
// exhausted case can fire, from the error text alone, and without a reset time.

const (
	// quotaLowPercent: at or below this share left, warn once per allowance window.
	quotaLowPercent = 10
	// quotaExhaustedFraction: a figure this low counts as "used up" even when the
	// vendor's error text does not say so in words.
	quotaExhaustedFraction = 0.02
)

var (
	quotaWarnMu sync.Mutex
	quotaWarned = map[string]time.Time{} // window key → when warned
)

// quotaWording reports whether an error text speaks of a usage allowance running
// out, as opposed to a momentary rate limit or an overloaded server. "overloaded"
// and "capacity" alone are NOT quota: waiting for a refill that never comes would
// be the wrong thing to offer.
func quotaWording(lower string) bool {
	for _, w := range []string{
		"quota", "usage limit", "usage_limit", "limit reached", "limit has been reached",
		"resource_exhausted", "resource exhausted", "exceeded your", "exhausted your",
		"out of credits", "insufficient_quota", "billing",
	} {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

var (
	// "resets_at": 1759000000   (Codex)
	resetsAtRe = regexp.MustCompile(`resets?_at"?\s*[:=]\s*"?(\d{9,11})`)
	// "resets_in_seconds": 12345   (Codex)
	resetsInRe = regexp.MustCompile(`resets?_in_seconds"?\s*[:=]\s*"?(\d+)`)
	// "quota will reset after 2h12m3s", "try again in 45s", "retry after 1.5s"
	afterDurationRe = regexp.MustCompile(`(?:reset(?:s)?(?: after| in)?|retry(?: in| after)|try again in|available again in)\s*:?\s*((?:\d+(?:\.\d+)?[hms]\s*)+)`)
)

// parseResetFromError reads a refill time out of a vendor error text. The shapes
// it knows (Codex's resets_at / resets_in_seconds, Antigravity's "reset after
// 2h12m3s") are from the vendors' published behaviour and NOT verified against a
// live exhaustion; when none matches it returns false and the quota snapshot is
// the fallback.
func parseResetFromError(text string, now time.Time) (time.Time, bool) {
	lower := strings.ToLower(text)
	if m := resetsAtRe.FindStringSubmatch(lower); m != nil {
		if sec, err := strconv.ParseInt(m[1], 10, 64); err == nil && sec > now.Unix()-3600 {
			return time.Unix(sec, 0), true
		}
	}
	if m := resetsInRe.FindStringSubmatch(lower); m != nil {
		if sec, err := strconv.ParseInt(m[1], 10, 64); err == nil && sec >= 0 {
			return now.Add(time.Duration(sec) * time.Second), true
		}
	}
	if m := afterDurationRe.FindStringSubmatch(lower); m != nil {
		if d, err := time.ParseDuration(strings.ReplaceAll(strings.TrimSpace(m[1]), " ", "")); err == nil && d > 0 {
			return now.Add(d), true
		}
	}
	return time.Time{}, false
}

// activeSubsModel returns the model and vendor of the active provider when it is
// the Subscriptions provider; ok is false for any other provider.
func (a *App) activeSubsModel() (provider, model, vendor string, ok bool) {
	cfg, found := a.subsMarkerConfig()
	if !found || a.GetActiveProvider() != cfg.Name {
		return "", "", "", false
	}
	vendor = ""
	for _, md := range a.subsManager().CachedModels() {
		if md.ID == cfg.Model {
			vendor = md.OwnedBy
			break
		}
	}
	return cfg.Name, cfg.Model, vendor, true
}

// quotaNow reads the cached quota figures (never waits). A test can stand in for
// the sidecar by setting App.quotaSource.
func (a *App) quotaNow() cliproxy.QuotaSet {
	if a.quotaSource != nil {
		return a.quotaSource()
	}
	return a.subsManager().QuotaSnapshot(context.Background(), 0)
}

// QuotaSignalForError classifies a failed turn's error text. It returns a signal
// only when the failure is the allowance running out; a momentary rate limit, an
// overloaded server or any other error returns nil and is shown as before.
func (a *App) QuotaSignalForError(errText string) *models.QuotaSignal {
	if strings.TrimSpace(errText) == "" || !taskloop.IsRateLimitErr(errors.New(errText)) {
		return nil
	}
	lower := strings.ToLower(errText)
	sig := &models.QuotaSignal{Kind: "exhausted", RemainingPercent: -1}

	var q cliproxy.Quota
	haveQ := false
	if prov, model, vendor, ok := a.activeSubsModel(); ok {
		sig.Provider, sig.Model, sig.Vendor = prov, model, vendor
		// From the cache only: this runs while a turn is ending, never wait here.
		q, haveQ = a.quotaNow().For(model, vendor)
		if haveQ {
			sig.RemainingPercent = int(q.Remaining*100 + 0.5)
			sig.Window = q.Window
		}
	}

	usedUp := haveQ && q.Remaining <= quotaExhaustedFraction
	if !quotaWording(lower) && !usedUp {
		return nil
	}

	now := time.Now()
	if at, ok := parseResetFromError(errText, now); ok {
		sig.ResetAt = at.UTC().Format(time.RFC3339)
	} else if haveQ && q.ResetAt != "" {
		sig.ResetAt = q.ResetAt
	}
	return sig
}

// WarmQuota starts a background refresh of the quota figures when the active
// provider is Subscriptions, so that by the time a turn ends the "running low"
// check reads fresh numbers. It never blocks.
func (a *App) WarmQuota() {
	if _, _, _, ok := a.activeSubsModel(); ok {
		a.quotaNow()
	}
}

// QuotaLowSignal returns a warning when the allowance behind the active model is
// at or below quotaLowPercent — once per allowance window, so a long session does
// not repeat it after every message. nil otherwise.
func (a *App) QuotaLowSignal() *models.QuotaSignal {
	prov, model, vendor, ok := a.activeSubsModel()
	if !ok {
		return nil
	}
	q, have := a.quotaNow().For(model, vendor)
	if !have {
		return nil
	}
	pct := int(q.Remaining*100 + 0.5)
	if pct > quotaLowPercent {
		return nil
	}
	key := vendor + "|" + model + "|" + q.Window + "|" + q.ResetAt
	quotaWarnMu.Lock()
	defer quotaWarnMu.Unlock()
	if _, seen := quotaWarned[key]; seen {
		return nil
	}
	// The map only ever holds one entry per window; drop ones from windows long gone.
	for k, at := range quotaWarned {
		if time.Since(at) > 8*24*time.Hour {
			delete(quotaWarned, k)
		}
	}
	quotaWarned[key] = time.Now()
	return &models.QuotaSignal{
		Kind: "low", Provider: prov, Model: model, Vendor: vendor,
		RemainingPercent: pct, ResetAt: q.ResetAt, Window: q.Window,
	}
}
