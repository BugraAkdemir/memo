// SPDX-License-Identifier: AGPL-3.0-or-later

package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Verdict is the outcome of one probe.
type Verdict int

const (
	// OK: the endpoint answered conclusively and as expected.
	OK Verdict = iota
	// Drift: the endpoint answered conclusively with something Memo was not
	// built against. This is the one that opens an issue.
	Drift
	// Inconclusive: no conclusive answer (network, 429, 5xx, a bot challenge).
	Inconclusive
)

func (v Verdict) String() string {
	return [...]string{"ok", "DRIFT", "inconclusive"}[v]
}

// Probe is one unauthenticated request and the contract its answer must meet.
type Probe struct {
	Name   string
	Method string
	URL    string
	Header map[string]string
	Body   string
	// Want judges a CONCLUSIVE answer. It returns "" when the contract holds,
	// otherwise a short description of what changed. nil means "any conclusive
	// answer is fine" (pure reachability).
	Want func(status int, body []byte, h http.Header) string
}

// Result is what a probe found.
type Result struct {
	Probe   string
	Verdict Verdict
	Detail  string
}

// retryBackoff is the pause before the single retry of an inconclusive probe. A
// var so tests do not wait.
var retryBackoff = 3 * time.Second

// maxBody bounds what a probe reads; manifests and error bodies are tiny.
const maxBody = 1 << 20

// Run executes p, retrying once if the first attempt was inconclusive, and
// classifies the answer.
func Run(ctx context.Context, c *http.Client, p Probe) Result {
	r := runOnce(ctx, c, p)
	if r.Verdict != Inconclusive {
		return r
	}
	select {
	case <-ctx.Done():
		return r
	case <-time.After(retryBackoff):
	}
	return runOnce(ctx, c, p)
}

func runOnce(ctx context.Context, c *http.Client, p Probe) Result {
	method := p.Method
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, p.URL, strings.NewReader(p.Body))
	if err != nil {
		return Result{p.Name, Drift, "bad probe definition: " + err.Error()}
	}
	req.Header.Set("User-Agent", "memo-upstream-watch")
	for k, v := range p.Header {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		return Result{p.Name, Inconclusive, "network: " + err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))

	if why := inconclusiveReason(resp.StatusCode, resp.Header, body); why != "" {
		return Result{p.Name, Inconclusive, why}
	}
	if p.Want != nil {
		if msg := p.Want(resp.StatusCode, body, resp.Header); msg != "" {
			return Result{p.Name, Drift, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, msg)}
		}
	}
	return Result{p.Name, OK, fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

// inconclusiveReason says why an answer cannot be judged, or "" if it can.
func inconclusiveReason(status int, h http.Header, body []byte) string {
	switch {
	case status == http.StatusTooManyRequests:
		return "rate limited (429)"
	case status >= 500:
		return fmt.Sprintf("server error (%d)", status)
	}
	// A bot-protection challenge is Cloudflare talking, not the vendor's API:
	// it says nothing about whether the contract moved.
	if status == http.StatusForbidden || status == http.StatusServiceUnavailable {
		if strings.EqualFold(h.Get("cf-mitigated"), "challenge") {
			return "bot challenge (cf-mitigated)"
		}
		lb := strings.ToLower(string(body))
		if strings.Contains(lb, "just a moment") || strings.Contains(lb, "cf-chl") || strings.Contains(lb, "attention required") {
			return "bot challenge (Cloudflare page)"
		}
	}
	return ""
}

// ── ready-made expectations ─────────────────────────────────────────────────

// Status expects one of the given HTTP statuses.
func Status(codes ...int) func(int, []byte, http.Header) string {
	return func(status int, _ []byte, _ http.Header) string {
		for _, c := range codes {
			if status == c {
				return ""
			}
		}
		return fmt.Sprintf("want status %v", codes)
	}
}

// JSONError expects one of the statuses AND a JSON object body carrying an
// "error" key — the shape every vendor API uses for a refusal. A 404 HTML page,
// or a 200 where a refusal should be, fails this.
func JSONError(codes ...int) func(int, []byte, http.Header) string {
	st := Status(codes...)
	return func(status int, body []byte, h http.Header) string {
		if msg := st(status, body, h); msg != "" {
			return msg
		}
		var obj map[string]any
		if err := json.Unmarshal(body, &obj); err != nil {
			return "body is not a JSON object: " + truncate(string(body), 80)
		}
		if _, ok := obj["error"]; !ok {
			if _, ok := obj["type"]; !ok {
				return "JSON body has neither an \"error\" nor a \"type\" key: " + truncate(string(body), 80)
			}
		}
		return ""
	}
}

// BodyContains expects status 200 and every given substring in the body.
func BodyContains(subs ...string) func(int, []byte, http.Header) string {
	return func(status int, body []byte, _ http.Header) string {
		if status != http.StatusOK {
			return "want status 200"
		}
		for _, s := range subs {
			if !strings.Contains(string(body), s) {
				return fmt.Sprintf("body lacks %q", s)
			}
		}
		return ""
	}
}

func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
