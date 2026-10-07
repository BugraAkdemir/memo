// SPDX-License-Identifier: AGPL-3.0-or-later

package cliproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// QuotaSet is everything known about how much allowance is left.
type QuotaSet struct {
	// Models: per model id (Antigravity reports a figure per model).
	Models map[string]Quota
	// Vendors: per owned_by ("openai" = the Codex account, "anthropic" = the
	// Claude account). These vendors meter the ACCOUNT, in time windows (5 hours,
	// 7 days), not each model, so one figure covers every model of that vendor.
	Vendors map[string]Quota
}

// For returns the figure for one model: its own when the vendor reports per
// model, else its vendor's account-wide one.
func (s QuotaSet) For(model, ownedBy string) (Quota, bool) {
	if q, ok := s.Models[model]; ok {
		return q, true
	}
	q, ok := s.Vendors[ownedBy]
	return q, ok
}

// Endpoints the vendors' own apps use to show a usage meter. Vars so tests can
// point them at a local server. Neither is a documented, supported API: if one
// changes the figure simply disappears (see ErrQuotaUnavailable).
var (
	codexUsageURL  = "https://chatgpt.com/backend-api/wham/usage"
	claudeUsageURL = "https://api.anthropic.com/api/oauth/usage"
)

const (
	quotaSnapshotMaxAge = 10 * time.Minute // older than this is worse than nothing
	quotaFetchTimeout   = 8 * time.Second
)

type quotaSnapshot struct {
	mu sync.Mutex
	// Each source keeps its own last good figure and the time it was read: one
	// vendor failing (an expired token, an outage) must not erase another's, and
	// a figure that keeps failing to refresh ages out on its own.
	models   map[string]Quota
	modelsAt time.Time
	vendors  map[string]Quota
	vendorAt map[string]time.Time
	tried    time.Time     // last attempt start
	failed   bool          // the last attempt got nothing at all
	busy     chan struct{} // non-nil while a refresh runs; closed when it ends
}

// QuotaSnapshot returns the quota figures without making the caller wait on
// the vendors. A fresh snapshot is returned as is; otherwise one background
// refresh is started (at most one at a time) and the previous figures — possibly
// none — are returned, unless maxWait > 0, in which case it waits that long for
// the refresh to land first. Opening the model picker used to block on this
// network call (about a second, up to several on a slow link) and freeze the UI.
func (m *Manager) QuotaSnapshot(ctx context.Context, maxWait time.Duration) QuotaSet {
	sn := &m.snap
	sn.mu.Lock()
	ttl := quotaTTL
	if sn.failed {
		ttl = quotaFailTTL
	}
	stale := sn.tried.IsZero() || time.Since(sn.tried) >= ttl
	if stale && sn.busy == nil {
		sn.busy = make(chan struct{})
		sn.tried = time.Now()
		go m.refreshSnapshot(sn.busy)
	}
	wait := sn.busy
	sn.mu.Unlock()

	if stale && maxWait > 0 && wait != nil {
		t := time.NewTimer(maxWait)
		defer t.Stop()
		select {
		case <-wait:
		case <-t.C:
		case <-ctx.Done():
		}
	}

	sn.mu.Lock()
	defer sn.mu.Unlock()
	out := QuotaSet{}
	if time.Since(sn.modelsAt) <= quotaSnapshotMaxAge {
		out.Models = sn.models
	}
	for v, q := range sn.vendors {
		if time.Since(sn.vendorAt[v]) <= quotaSnapshotMaxAge {
			if out.Vendors == nil {
				out.Vendors = map[string]Quota{}
			}
			out.Vendors[v] = q
		}
	}
	return out
}

// resetQuotaSnapshot forgets the cached figures (sign-in or sign-out changed
// whose allowance this is).
func (m *Manager) resetQuotaSnapshot() {
	m.snap.mu.Lock()
	m.snap.models, m.snap.modelsAt = nil, time.Time{}
	m.snap.vendors, m.snap.vendorAt = nil, nil
	m.snap.tried, m.snap.failed = time.Time{}, false
	m.snap.mu.Unlock()
}

func (m *Manager) refreshSnapshot(done chan struct{}) {
	defer func() {
		m.snap.mu.Lock()
		m.snap.busy = nil
		m.snap.mu.Unlock()
		close(done)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), quotaFetchTimeout+2*time.Second)
	defer cancel()

	var (
		wg                         sync.WaitGroup
		mu                         sync.Mutex
		models                     map[string]Quota
		modelsErr                  error
		vendors                    map[string]Quota
		vendorsErr                 error
		vendorsTried, modelsTried_ bool
	)
	for _, a := range m.Accounts() {
		if a.Provider == ProviderAntigravity && !a.Disabled {
			modelsTried_ = true
		}
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		if !modelsTried_ {
			return
		}
		q, err := m.Quotas(ctx)
		mu.Lock()
		models, modelsErr = q, err
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		q, tried, err := m.accountQuotas(ctx)
		mu.Lock()
		vendors, vendorsTried, vendorsErr = q, tried, err
		mu.Unlock()
	}()
	wg.Wait()

	now := time.Now()
	m.snap.mu.Lock()
	defer m.snap.mu.Unlock()
	gotAny := false
	if modelsTried_ && modelsErr == nil {
		m.snap.models, m.snap.modelsAt, gotAny = models, now, true
	}
	if !modelsTried_ {
		m.snap.models, m.snap.modelsAt = nil, time.Time{}
	}
	if vendorsTried && vendorsErr == nil {
		if m.snap.vendors == nil {
			m.snap.vendors, m.snap.vendorAt = map[string]Quota{}, map[string]time.Time{}
		}
		for v, q := range vendors {
			m.snap.vendors[v], m.snap.vendorAt[v] = q, now
		}
		if len(vendors) > 0 {
			gotAny = true
		}
	}
	if !vendorsTried {
		m.snap.vendors, m.snap.vendorAt = nil, nil
	}
	// Retry soon only when there was something to ask and nothing came back.
	m.snap.failed = (modelsTried_ || vendorsTried) && !gotAny
}

// accountQuotas reads the account-wide allowance of the signed-in Codex and
// Claude accounts. tried reports whether there was such an account to ask.
func (m *Manager) accountQuotas(ctx context.Context) (out map[string]Quota, tried bool, err error) {
	out = map[string]Quota{}
	var firstErr error
	for _, a := range m.Accounts() {
		if a.Disabled || (a.Provider != ProviderCodex && a.Provider != ProviderClaude) {
			continue
		}
		tried = true
		raw, rerr := os.ReadFile(a.file)
		if rerr != nil {
			continue
		}
		var cred struct {
			AccessToken string `json:"access_token"`
			AccountID   string `json:"account_id"`
		}
		if json.Unmarshal(raw, &cred) != nil || cred.AccessToken == "" {
			continue
		}
		var q Quota
		var ok bool
		var ownedBy string
		var qerr error
		switch a.Provider {
		case ProviderCodex:
			ownedBy = "openai"
			q, ok, qerr = fetchUsage(ctx, codexUsageURL, cred.AccessToken, map[string]string{"chatgpt-account-id": cred.AccountID}, parseCodexUsage)
		case ProviderClaude:
			ownedBy = "anthropic"
			q, ok, qerr = fetchUsage(ctx, claudeUsageURL, cred.AccessToken, map[string]string{"anthropic-beta": "oauth-2025-04-20"}, parseClaudeUsage)
		}
		if qerr != nil && firstErr == nil {
			firstErr = qerr
		}
		if ok {
			out[ownedBy] = q
		}
	}
	// A vendor that failed simply has no entry in out: the caller keeps its
	// previous figure. An error is reported only when NOTHING could be read.
	if len(out) == 0 && firstErr != nil {
		return nil, tried, firstErr
	}
	return out, tried, nil
}

func fetchUsage(ctx context.Context, url, token string, extra map[string]string, parse func([]byte, time.Time) (Quota, bool)) (Quota, bool, error) {
	cctx, cancel := context.WithTimeout(ctx, quotaFetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	if err != nil {
		return Quota{}, false, ErrQuotaUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "memo")
	for k, v := range extra {
		if v != "" {
			req.Header.Set(k, v)
		}
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return Quota{}, false, fmt.Errorf("%w: %v", ErrQuotaUnavailable, redactErr(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Quota{}, false, fmt.Errorf("%w: status %d", ErrQuotaUnavailable, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Quota{}, false, ErrQuotaUnavailable
	}
	q, ok := parse(body, time.Now())
	if !ok {
		return Quota{}, false, fmt.Errorf("%w: unexpected response", ErrQuotaUnavailable)
	}
	return q, true, nil
}

// mostLimiting folds several windows into the one that is closest to empty.
type windowFigure struct {
	remaining float64
	resetAt   time.Time
	label     string
}

func mostLimiting(ws []windowFigure) (Quota, bool) {
	if len(ws) == 0 {
		return Quota{}, false
	}
	best := ws[0]
	for _, w := range ws[1:] {
		if w.remaining < best.remaining {
			best = w
		}
	}
	q := Quota{Remaining: clamp01(best.remaining), Window: best.label}
	if !best.resetAt.IsZero() {
		q.ResetAt = best.resetAt.UTC().Format(time.RFC3339)
	}
	q.Windows = windowsByLength(ws)
	return q, true
}

// windowsByLength keeps one figure per window length (the tightest when several
// share a length — Claude meters the weekly window per model family too) and
// orders them shortest first. Windows with no label are dropped: without a
// length there is nothing to call them.
func windowsByLength(ws []windowFigure) []QuotaWindow {
	best := map[string]windowFigure{}
	for _, w := range ws {
		if w.label == "" {
			continue
		}
		if cur, ok := best[w.label]; !ok || w.remaining < cur.remaining {
			best[w.label] = w
		}
	}
	out := make([]QuotaWindow, 0, len(best))
	for label, w := range best {
		qw := QuotaWindow{Label: label, Remaining: clamp01(w.remaining)}
		if !w.resetAt.IsZero() {
			qw.ResetAt = w.resetAt.UTC().Format(time.RFC3339)
		}
		out = append(out, qw)
	}
	sort.Slice(out, func(i, j int) bool { return windowMinutes(out[i].Label) < windowMinutes(out[j].Label) })
	return out
}

// windowMinutes is windowLabel's inverse, for ordering ("5h" < "7d").
func windowMinutes(label string) int {
	if len(label) < 2 {
		return 0
	}
	n, err := strconv.Atoi(label[:len(label)-1])
	if err != nil {
		return 0
	}
	switch label[len(label)-1] {
	case 'h':
		return n * 60
	case 'd':
		return n * 60 * 24
	}
	return n
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func firstKey(m map[string]any, keys ...string) (any, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v, true
		}
	}
	return nil, false
}

// windowLabel renders a window length: 300 min → "5h", 10080 min → "7d".
func windowLabel(minutes float64) string {
	switch {
	case minutes <= 0:
		return ""
	case minutes < 60*24:
		return fmt.Sprintf("%dh", int(minutes/60+0.5))
	default:
		return fmt.Sprintf("%dd", int(minutes/(60*24)+0.5))
	}
}

// parseCodexUsage reads ChatGPT's usage meter: a rate_limit object with a
// primary and a secondary window, each {used_percent, window length, reset}. The
// field spellings vary between the websocket event and the usage probe (the
// sidecar's own parser accepts both), so both are accepted here.
func parseCodexUsage(body []byte, now time.Time) (Quota, bool) {
	var root map[string]any
	if json.Unmarshal(body, &root) != nil {
		return Quota{}, false
	}
	obj := root
	if v, ok := firstKey(root, "rate_limit", "rate_limits", "rateLimit"); ok {
		if m, ok := v.(map[string]any); ok {
			obj = m
		}
	}
	var ws []windowFigure
	for key, v := range obj {
		lk := strings.ToLower(key)
		if !strings.HasPrefix(lk, "primary") && !strings.HasPrefix(lk, "secondary") {
			continue
		}
		w, ok := v.(map[string]any)
		if !ok {
			continue
		}
		usedV, ok := firstKey(w, "used_percent", "usedPercent")
		used, isNum := asFloat(usedV)
		if !ok || !isNum || used < 0 || used > 100 {
			continue
		}
		fig := windowFigure{remaining: 1 - used/100}
		if mv, ok := firstKey(w, "window_minutes", "windowMinutes"); ok {
			if mins, ok := asFloat(mv); ok {
				fig.label = windowLabel(mins)
			}
		} else if sv, ok := firstKey(w, "limit_window_seconds", "limitWindowSeconds"); ok {
			if secs, ok := asFloat(sv); ok {
				fig.label = windowLabel(secs / 60)
			}
		}
		if av, ok := firstKey(w, "reset_after_seconds", "resetAfterSeconds"); ok {
			if secs, ok := asFloat(av); ok && secs >= 0 {
				fig.resetAt = now.Add(time.Duration(secs) * time.Second)
			}
		}
		if rv, ok := firstKey(w, "reset_at", "resetAt"); ok {
			if at, ok := asFloat(rv); ok && at > 0 {
				fig.resetAt = time.Unix(int64(at), 0)
			}
		}
		ws = append(ws, fig)
	}
	return mostLimiting(ws)
}

// parseClaudeUsage reads Anthropic's OAuth usage meter: {"five_hour": {
// "utilization": 12.0, "resets_at": "…"}, "seven_day": {…}, …}; a window that is
// null (not applicable to the plan) is skipped.
func parseClaudeUsage(body []byte, _ time.Time) (Quota, bool) {
	var root map[string]any
	if json.Unmarshal(body, &root) != nil {
		return Quota{}, false
	}
	labels := map[string]string{"five_hour": "5h", "seven_day": "7d", "seven_day_opus": "7d", "seven_day_sonnet": "7d"}
	var ws []windowFigure
	for key, label := range labels {
		w, ok := root[key].(map[string]any)
		if !ok {
			continue
		}
		used, ok := asFloat(w["utilization"])
		if !ok || used < 0 {
			continue
		}
		fig := windowFigure{remaining: 1 - used/100, label: label}
		if s, ok := w["resets_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				fig.resetAt = t
			}
		}
		ws = append(ws, fig)
	}
	return mostLimiting(ws)
}
