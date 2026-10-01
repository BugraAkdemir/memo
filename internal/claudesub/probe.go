// SPDX-License-Identifier: AGPL-3.0-or-later

package claudesub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"memo/internal/logx"

	"golang.org/x/oauth2"
)

// Capabilities is what an account can actually do, measured rather than
// assumed.
//
// Anthropic applies an allowlist-style entitlement gate to subscription-OAuth
// traffic, and it is not one rule but a growing set of them: the Claude Code
// identity block (settled), temperature (settled), the request body's shape
// (settled, and the source of the least explicable failures), and possibly
// transport-level fingerprinting. The set has widened at least three times in
// 2026. Rather than guess which rules apply to which plan, this sends one tiny
// request per capability class and reports what came back.
//
// The alternative — shipping a feature whose failure mode is a bare 429 with no
// rate-limit headers — is exactly the confusion anthropics/claude-code#87420
// reports: an entitlement refusal that looks like a quota wall, producing
// false back-offs on healthy accounts and a user-facing message that says
// "slow down" when the truth is "not allowed".
type Capabilities struct {
	// Plain: a minimal non-streaming request. Almost always true — Haiku is
	// exempt from the entitlement gate entirely.
	Plain bool
	// Tools: the same request with one trivial tool declared. This is the
	// class that most decides whether agent mode works at all, since every
	// tool-carrying turn in Memo goes through the non-streaming ChatCompletion
	// path.
	Tools bool
	// Thinking: adaptive thinking plus an effort level, i.e. what the effort
	// picker produces.
	Thinking bool
	// OneMContext: the account serves a 1M window when asked for one. Without
	// the context-1m beta this is silently false: the request succeeds and the
	// window is 200K anyway, so the only way to know is to send the beta and
	// see whether it is accepted.
	OneMContext bool
	// EntitlementBlocked is the important negative result: a refusal that
	// carries no rate-limit headers. It is NOT a quota problem and must never
	// be reported as one.
	EntitlementBlocked bool
	// Detail is a short human-readable explanation for the UI and the log.
	Detail string
	// Model is the model this was measured against.
	Model string
	// MeasuredAt is when the probe ran.
	MeasuredAt time.Time
}

// probeCache holds the last result so repeated calls (a UI refresh, an
// ambiently-read widget) do not re-spend requests.
type probeCache struct {
	mu    sync.Mutex
	caps  *Capabilities
	model string
}

// Probe measures the account's capabilities against model.
//
// Every request here is deliberately tiny — max_tokens 1, one word of user
// text — because this runs automatically right after the user connects. The
// cost is a handful of output tokens; the value is turning a class of
// inexplicable failures into a table the user can read.
//
// A nil or empty model falls back to the caller's configured default.
func (m *Manager) Probe(ctx context.Context, model string) Capabilities {
	if strings.TrimSpace(model) == "" {
		model = claudeSubDefaultModelPublic
	}
	m.probe.mu.Lock()
	if c := m.probe.caps; c != nil && c.Model == model {
		m.probe.mu.Unlock()
		return *c
	}
	m.probe.mu.Unlock()

	caps := m.runProbe(ctx, model)

	m.probe.mu.Lock()
	m.probe.caps, m.probe.model = &caps, model
	m.probe.mu.Unlock()

	logx.Printf("claudesub: capabilities for %s — plain=%v tools=%v thinking=%v 1m=%v blocked=%v (%s)",
		model, caps.Plain, caps.Tools, caps.Thinking, caps.OneMContext, caps.EntitlementBlocked, caps.Detail)
	return caps
}

// LastCapabilities returns the cached result, or nil if no probe has run.
func (m *Manager) LastCapabilities() *Capabilities {
	m.probe.mu.Lock()
	defer m.probe.mu.Unlock()
	if m.probe.caps == nil {
		return nil
	}
	c := *m.probe.caps
	return &c
}

// claudeSubDefaultModelPublic mirrors claudeSubDefaultModel in internal/app,
// which cannot be imported from here (that package imports this one). Kept as a
// separate const with a test asserting the two stay equal, rather than
// exporting app's constant into a package that app already depends on.
const claudeSubDefaultModelPublic = "claude-haiku-4-5-20251001"

// probeTimeout bounds the whole measurement. Four sequential requests to
// Anthropic on a warm connection are fast; a hung one must not hold up the
// connect flow that triggers this.
const probeTimeout = 45 * time.Second

func (m *Manager) runProbe(ctx context.Context, model string) Capabilities {
	caps := Capabilities{Model: model, MeasuredAt: time.Now()}

	ts, err := m.TokenSource(ctx)
	if err != nil {
		caps.Detail = "not connected"
		return caps
	}
	tok, err := ts.Token()
	if err != nil {
		caps.Detail = "token refresh failed"
		return caps
	}

	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	caps.Plain = m.probeOnce(ctx, tok, probeReq{
		model: model, tool: false, thinking: false, oneM: false,
	})
	if !caps.Plain {
		caps.Detail = "even a minimal request was refused"
		return caps
	}

	caps.Tools = m.probeOnce(ctx, tok, probeReq{
		model: model, tool: true, thinking: false, oneM: false,
	})
	caps.Thinking = m.probeOnce(ctx, tok, probeReq{
		model: model, tool: false, thinking: true, oneM: false,
	})
	caps.OneMContext = m.probeOnce(ctx, tok, probeReq{
		model: model, tool: false, thinking: false, oneM: true,
	})

	// EntitlementBlocked is only meaningful when something actually failed.
	// Re-derive it from the worst class: if tools are refused but plain works,
	// the account is entitled to the model but not to agent-mode traffic on it.
	caps.EntitlementBlocked = !caps.Tools || !caps.Thinking
	switch {
	case caps.Plain && caps.Tools && caps.Thinking:
		caps.Detail = "all core features available"
	case caps.Plain && caps.Tools:
		caps.Detail = "thinking refused on this model; agent tools work"
	case caps.Plain:
		caps.Detail = "plain chat only — this account may not be entitled to agent traffic on " + model
	default:
		caps.Detail = "unexpected: plain chat was refused"
	}
	return caps
}

type probeReq struct {
	model    string
	tool     bool
	thinking bool
	oneM     bool
}

// probeOnce sends one minimal request and classifies the outcome.
func (m *Manager) probeOnce(ctx context.Context, tok *oauth2.Token, r probeReq) bool {
	body, betas, err := probeBody(r)
	if err != nil {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, probeEndpoint+"v1/messages", bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("anthropic-beta", betas)
	req.Header.Set("user-agent", userAgent())
	req.Header.Set("x-app", "cli")

	resp, err := probeClient.Do(req)
	if err != nil {
		logx.Printf("claudesub: probe request failed: %v", err)
		return false
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))

	if resp.StatusCode == http.StatusOK {
		return true
	}

	// The signature that matters: a refusal with NO anthropic-ratelimit-*
	// headers is an entitlement decision, not a quota wall. anthropic-ratelimit-
	// unified-* is what a real pushback carries, so its absence is the
	// discriminator.
	if isEntitlementRefusal(resp, raw) {
		logx.Printf("claudesub: probe refused (tools=%v thinking=%v 1m=%v) status=%d — entitlement gate, not quota",
			r.tool, r.thinking, r.oneM, resp.StatusCode)
	}
	return false
}

// isEntitlementRefusal reports whether this response is the shape gate rather
// than a rate limit: the documented refusals are 400 and 429, and the 429 case
// is distinguishable only by the ABSENCE of the headers a genuine pushback
// carries. Retry-After counts as one of those — a real quota wall always sets
// it, and the gate never does.
func isEntitlementRefusal(resp *http.Response, body []byte) bool {
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusTooManyRequests {
		return false
	}
	for name := range resp.Header {
		n := strings.ToLower(name)
		if strings.HasPrefix(n, "anthropic-ratelimit") || n == "retry-after" {
			return false // real quota headers present -> genuine pushback
		}
	}
	// A 400 naming a concrete cause is a normal rejection, not the opaque gate.
	// The gate answers with a generic message.
	if e := provider_msg(body); strings.Contains(e, "rate_limit") && !strings.Contains(e, "Error") {
		return false
	}
	return strings.Contains(provider_msg(body), "Error")
}

// probeBody builds the minimal request for one capability class.
//
// It deliberately mirrors buildClaudeRequest's SHAPE — identity block first,
// tools as Anthropic's own {name,description,input_schema} — rather than
// inventing a shape that would pass the gate and then fail in real use. A probe
// that does not look like Memo's requests measures the wrong thing.
func probeBody(r probeReq) ([]byte, string, error) {
	system := []map[string]any{{"type": "text", "text": identitySystem}}

	req := map[string]any{
		"model":      r.model,
		"max_tokens": 1,
		"system":     system,
		"messages": []map[string]any{
			{"role": "user", "content": []map[string]any{{"type": "text", "text": "hi"}}},
		},
	}
	if r.tool {
		req["tools"] = []map[string]any{{
			"name":         "probe",
			"description":  "Probe tool.",
			"input_schema": map[string]any{"type": "object", "properties": map[string]any{}},
		}}
	}
	if r.thinking {
		req["thinking"] = map[string]any{"type": "adaptive"}
		req["output_config"] = map[string]any{"effort": "low"}
	}

	betas := requiredBetas
	if r.oneM {
		betas += "," + oneMBeta
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, "", err
	}
	return body, betas, nil
}

// probeEndpoint is the API base the probe talks to. A var so a test can point
// it at an httptest server, and overridable in the environment so a test binary
// that forgets to is still safe: TestMain in internal/app and NewHarness in
// internal/e2e both point it at an unreachable port, because a probe that escapes
// to the real api.anthropic.com from a unit test is real network I/O against a
// real account's endpoint, made with whatever token happens to be in the temp
// data dir.
var probeEndpoint = apiURL()

const envAPIURL = "MEMO_CLAUDE_API_URL"

// apiURL returns the Messages API base, trailing slash included so callers can
// concatenate "v1/messages".
func apiURL() string {
	if v := strings.TrimSpace(os.Getenv(envAPIURL)); v != "" {
		return strings.TrimRight(v, "/") + "/"
	}
	return "https://api.anthropic.com/"
}

var probeClient = &http.Client{Timeout: 20 * time.Second}

// oneMContextSupported reports whether the measured account serves the 1M
// window, so the beta is only sent where it is known to be accepted.
//
// Sending it unconditionally is the other option, and hermes-agent chose that
// (PR #99842) with a reactive recovery. The difference: their recovery existed
// already. Here there is no recovery, because claude.go's 400-valve reads the
// error text to decide WHICH field to withdraw, and this rejection names the
// beta rather than a parameter, so the valve would misread it and latch
// temperature off for the process. Measuring first is both cheaper and
// quieter.
func (m *Manager) oneMContextSupported() bool {
	m.probe.mu.Lock()
	defer m.probe.mu.Unlock()
	return m.probe.caps != nil && m.probe.caps.OneMContext
}

// invalidateProbe drops the measurement, on connect and disconnect.
func (m *Manager) invalidateProbe() {
	m.probe.mu.Lock()
	m.probe.caps, m.probe.model = nil, ""
	m.probe.mu.Unlock()
}

var _ = fmt.Sprintf
