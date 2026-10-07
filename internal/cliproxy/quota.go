// SPDX-License-Identifier: AGPL-3.0-or-later

package cliproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"memo/internal/logx"
)

// Quota is how much of a model's allowance is left. The sidecar itself only
// tracks "exceeded / cooling down", never a percentage; the percentage comes from
// the vendor's own catalogue call, which is what the Antigravity app shows too.
type Quota struct {
	// Remaining is the fraction left, 0..1.
	Remaining float64 `json:"remaining"`
	// ResetAt is when the allowance refills (RFC 3339), when the vendor says.
	ResetAt string `json:"reset_at,omitempty"`
	// Window names the allowance window this figure is for when the vendor has
	// several ("5h", "7d"): the most limiting one is reported.
	Window string `json:"window,omitempty"`
	// Windows lists every allowance window the vendor reports, shortest first,
	// one per window length (the tightest figure when a vendor splits one length
	// by model family). Remaining/ResetAt/Window above are the most limiting of
	// them; this is for a view that wants the whole picture (the chat's context
	// popover shows the 5-hour and the weekly meter side by side). Empty for a
	// source that has a single figure (Antigravity, per model).
	Windows []QuotaWindow `json:"windows,omitempty"`
}

// QuotaWindow is one allowance window of a Quota.
type QuotaWindow struct {
	// Label is the window length ("5h", "7d").
	Label string `json:"label"`
	// Remaining is the fraction left, 0..1.
	Remaining float64 `json:"remaining"`
	// ResetAt is when this window refills (RFC 3339), when the vendor says.
	ResetAt string `json:"reset_at,omitempty"`
}

// Antigravity's catalogue endpoint — the same call the sidecar makes for its own
// model list, with the same client identity string. Vars so tests can point them
// at a local server.
var (
	antigravityQuotaURL = "https://daily-cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels"
	antigravityQuotaUA  = "antigravity/hub/2.9.1 linux/amd64"
	quotaTTL            = 30 * time.Second
)

// ErrQuotaUnavailable means no quota could be read; the models list is still
// fine without it.
var ErrQuotaUnavailable = errors.New("cliproxy: quota unavailable")

type quotaMemo struct {
	at  time.Time
	val map[string]Quota
	err error // a failure is remembered too (shorter), so a poll does not hammer the vendor
}

const quotaFailTTL = 30 * time.Second

// Quotas returns the remaining allowance per model id for the signed-in
// Antigravity account (Claude and Codex expose no such figure to a third-party
// client, so they simply have no entry). Cached briefly; a failure is not cached.
//
// The access token is read from the credential file here and sent only to the
// vendor's own endpoint; it never reaches the API, the logs or the UI.
func (m *Manager) Quotas(ctx context.Context) (map[string]Quota, error) {
	m.modelsMu.Lock()
	if q := m.quotaMem; q != nil {
		ttl := quotaTTL
		if q.err != nil {
			ttl = quotaFailTTL
		}
		if time.Since(q.at) < ttl {
			out, err := q.val, q.err
			m.modelsMu.Unlock()
			return out, err
		}
	}
	m.modelsMu.Unlock()
	out, err := m.fetchQuotas(ctx)
	m.modelsMu.Lock()
	m.quotaMem = &quotaMemo{at: time.Now(), val: out, err: err}
	m.modelsMu.Unlock()
	if err != nil && !errors.Is(err, context.Canceled) {
		logx.Printf("cliproxy: quota not available: %v", err)
	}
	return out, err
}

func (m *Manager) fetchQuotas(ctx context.Context) (map[string]Quota, error) {
	var acct *Account
	for _, a := range m.Accounts() {
		if a.Provider == ProviderAntigravity && !a.Disabled {
			a := a
			acct = &a
			break
		}
	}
	if acct == nil {
		return nil, ErrQuotaUnavailable
	}
	raw, err := os.ReadFile(acct.file)
	if err != nil {
		return nil, ErrQuotaUnavailable
	}
	var cred struct {
		AccessToken string `json:"access_token"`
		ProjectID   string `json:"project_id"`
	}
	if json.Unmarshal(raw, &cred) != nil || cred.AccessToken == "" {
		return nil, ErrQuotaUnavailable
	}

	body, _ := json.Marshal(map[string]string{"project": cred.ProjectID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, antigravityQuotaURL, bytes.NewReader(body))
	if err != nil {
		return nil, ErrQuotaUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cred.AccessToken)
	req.Header.Set("User-Agent", antigravityQuotaUA)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQuotaUnavailable, redactErr(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrQuotaUnavailable, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, ErrQuotaUnavailable
	}
	out, ok := parseQuotas(data)
	if !ok {
		return nil, fmt.Errorf("%w: unexpected response", ErrQuotaUnavailable)
	}
	return out, nil
}

// parseQuotas reads {"models": {"<id>": {"quotaInfo": {"remainingFraction": f,
// "resetTime": "..."}}}}. Models without quotaInfo are left out.
func parseQuotas(body []byte) (map[string]Quota, bool) {
	var parsed struct {
		Models map[string]struct {
			QuotaInfo *struct {
				RemainingFraction *float64 `json:"remainingFraction"`
				ResetTime         string   `json:"resetTime"`
			} `json:"quotaInfo"`
		} `json:"models"`
	}
	if json.Unmarshal(body, &parsed) != nil || parsed.Models == nil {
		return nil, false
	}
	out := make(map[string]Quota, len(parsed.Models))
	for id, md := range parsed.Models {
		if md.QuotaInfo == nil {
			continue
		}
		f := 0.0
		if md.QuotaInfo.RemainingFraction != nil {
			f = *md.QuotaInfo.RemainingFraction
		}
		// An absent fraction with a reset time means "used up until then".
		if md.QuotaInfo.RemainingFraction == nil && md.QuotaInfo.ResetTime == "" {
			continue
		}
		if f < 0 {
			f = 0
		}
		if f > 1 {
			f = 1
		}
		out[strings.TrimSpace(id)] = Quota{Remaining: f, ResetAt: md.QuotaInfo.ResetTime}
	}
	return out, true
}

// redactErr keeps a transport error short and free of URLs/headers.
func redactErr(err error) string {
	var ue interface{ Unwrap() error }
	if errors.As(err, &ue) && ue.Unwrap() != nil {
		return ue.Unwrap().Error()
	}
	return "request failed"
}
