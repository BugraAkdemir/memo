package claudesub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// withProbeEndpoint points the probe at a stub and restores it afterwards.
func withProbeEndpoint(t *testing.T, url string) {
	t.Helper()
	old := probeEndpoint
	probeEndpoint = url
	t.Cleanup(func() { probeEndpoint = old })
}

// recorded is what one probe request looked like on the wire.
type recorded struct {
	beta   string
	body   map[string]any
	system []map[string]any
}

// probeStub answers /v1/messages with a per-request decision and records what it
// was asked.
type probeStub struct {
	mu   sync.Mutex
	seen []recorded
	// decide returns (status, headers, body) for a request.
	decide func(r recorded) (int, map[string]string, string)
	srv    *httptest.Server
}

func newProbeStub(t *testing.T, decide func(r recorded) (int, map[string]string, string)) *probeStub {
	t.Helper()
	p := &probeStub{decide: decide}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		rec := recorded{beta: r.Header.Get("anthropic-beta"), body: body}
		if sys, ok := body["system"].([]any); ok {
			for _, s := range sys {
				if m, ok := s.(map[string]any); ok {
					rec.system = append(rec.system, m)
				}
			}
		}
		p.mu.Lock()
		p.seen = append(p.seen, rec)
		p.mu.Unlock()

		status, headers, out := p.decide(rec)
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(out))
	}))
	t.Cleanup(p.srv.Close)
	withProbeEndpoint(t, p.srv.URL+"/")
	return p
}

func (p *probeStub) requests() []recorded {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]recorded(nil), p.seen...)
}

func okBody() string {
	return `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"output_tokens":1}}`
}

func connectedManager(t *testing.T) *Manager {
	t.Helper()
	m := newTestManager(t)
	m.token = &oauth2.Token{AccessToken: "sk-ant-oat01-probe", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}
	return m
}

func TestProbe_AllCapabilitiesWork(t *testing.T) {
	stub := newProbeStub(t, func(r recorded) (int, map[string]string, string) {
		return http.StatusOK, nil, okBody()
	})

	caps := connectedManager(t).Probe(context.Background(), "claude-sonnet-5")

	if !caps.Plain || !caps.Tools || !caps.Thinking || !caps.OneMContext {
		t.Fatalf("capabilities = %+v, want everything true", caps)
	}
	if caps.EntitlementBlocked {
		t.Error("EntitlementBlocked set although every class succeeded")
	}
	if caps.Model != "claude-sonnet-5" {
		t.Errorf("Model = %q", caps.Model)
	}
	if caps.MeasuredAt.IsZero() {
		t.Error("MeasuredAt was not stamped")
	}

	reqs := stub.requests()
	if len(reqs) != 4 {
		t.Fatalf("made %d requests, want one per capability class (4)", len(reqs))
	}
	// The order is fixed and each class must differ from the others in exactly
	// one way, or the measurement is measuring nothing.
	if _, ok := reqs[0].body["tools"]; ok {
		t.Error("the plain probe carried tools")
	}
	if _, ok := reqs[1].body["tools"]; !ok {
		t.Error("the tools probe carried no tools")
	}
	if _, ok := reqs[2].body["thinking"]; !ok {
		t.Error("the thinking probe carried no thinking block")
	}
	if !strings.Contains(reqs[3].beta, oneMBeta) {
		t.Errorf("the 1M probe did not ask for the 1M beta: %q", reqs[3].beta)
	}
	for i, r := range reqs {
		if strings.Contains(r.beta, oneMBeta) && i != 3 {
			t.Errorf("request %d carries the 1M beta but should not: %q", i, r.beta)
		}
		if !strings.Contains(r.beta, "oauth-2025-04-20") {
			t.Errorf("request %d is missing the OAuth beta: %q", i, r.beta)
		}
	}
}

// A probe that does not look like Memo's real requests measures the wrong
// thing: the identity block is the very gate this feature exists to satisfy, so
// the probe has to carry it or it would report success for requests that can
// never succeed.
func TestProbe_SendsTheIdentityBlockFirst(t *testing.T) {
	stub := newProbeStub(t, func(r recorded) (int, map[string]string, string) {
		return http.StatusOK, nil, okBody()
	})
	connectedManager(t).Probe(context.Background(), "claude-sonnet-5")

	for i, r := range stub.requests() {
		if len(r.system) == 0 {
			t.Fatalf("request %d carried no system block", i)
		}
		if r.system[0]["text"] != identitySystem {
			t.Errorf("request %d system[0] = %q, want the identity sentence first", i, r.system[0]["text"])
		}
		if r.body["max_tokens"] != float64(1) {
			t.Errorf("request %d max_tokens = %v; the probe must stay as cheap as possible", i, r.body["max_tokens"])
		}
	}
}

// The whole reason this probe exists: telling an entitlement refusal apart from
// a quota wall, which are otherwise indistinguishable on the wire.
func TestProbe_Headerless429IsEntitlementNotQuota(t *testing.T) {
	// A refusal with no anthropic-ratelimit-* headers and an opaque "Error".
	newProbeStub(t, func(r recorded) (int, map[string]string, string) {
		if _, ok := r.body["tools"]; ok {
			return http.StatusTooManyRequests, nil, `{"type":"error","error":{"type":"rate_limit_error","message":"Error"}}`
		}
		return http.StatusOK, nil, okBody()
	})
	caps := connectedManager(t).Probe(context.Background(), "claude-opus-5")
	if !caps.Plain {
		t.Error("plain should have succeeded")
	}
	if caps.Tools {
		t.Error("the tools class was reported working although it was refused")
	}
	if !caps.EntitlementBlocked {
		t.Error("EntitlementBlocked not set for a headerless 429 — this is the case the whole probe exists to catch")
	}

	// A real quota pushback DOES carry rate-limit headers, and must NOT be
	// reported as an entitlement problem — that mislabelling is what produces
	// false back-offs on accounts that were serving 200s a moment earlier.
	newProbeStub(t, func(r recorded) (int, map[string]string, string) {
		return http.StatusTooManyRequests, map[string]string{
			"anthropic-ratelimit-unified-5h-utilization": "0.97",
			"retry-after": "42",
		}, `{"type":"error","error":{"type":"rate_limit_error","message":"Number of requests has exceeded your rate limit"}}`
	})
	caps = connectedManager(t).Probe(context.Background(), "claude-opus-5")
	if caps.Plain {
		t.Error("plain should have failed against a 429")
	}
	if caps.EntitlementBlocked {
		t.Error("a genuine quota wall was misreported as an entitlement gate")
	}
}

func TestIsEntitlementRefusal(t *testing.T) {
	body := []byte(`{"type":"error","error":{"type":"rate_limit_error","message":"Error"}}`)
	concrete := []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"max_tokens: must be greater than 0"}}`)

	cases := []struct {
		name    string
		status  int
		headers map[string]string
		body    []byte
		want    bool
	}{
		{"bare 429 with opaque Error", 429, nil, body, true},
		{"bare 400 with opaque Error", 400, nil, body, true},
		{"429 carrying quota headers", 429, map[string]string{"anthropic-ratelimit-unified-5h-utilization": "0.9"}, body, false},
		{"429 carrying retry-after", 429, map[string]string{"retry-after": "30"}, body, false},
		{"400 naming a concrete cause", 400, nil, concrete, false},
		{"200", 200, nil, nil, false},
		{"500", 500, nil, body, false},
		{"404", 404, nil, body, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: tc.status, Header: http.Header{}}
			for k, v := range tc.headers {
				resp.Header.Set(k, v)
			}
			if got := isEntitlementRefusal(resp, tc.body); got != tc.want {
				t.Errorf("isEntitlementRefusal = %v, want %v", got, tc.want)
			}
		})
	}
}

// Once plain chat works but tools do not, the account is entitled to the model
// but not to agent traffic on it — which is precisely the case that decides
// whether "agent mode works" is honest to claim.
func TestProbe_ToolsRefusedLeavesAgentModeUnsupported(t *testing.T) {
	newProbeStub(t, func(r recorded) (int, map[string]string, string) {
		if _, ok := r.body["tools"]; ok {
			return http.StatusBadRequest, nil, `{"type":"error","error":{"type":"invalid_request_error","message":"Error"}}`
		}
		return http.StatusOK, nil, okBody()
	})
	caps := connectedManager(t).Probe(context.Background(), "claude-opus-5")

	// The stub refuses only the tools class, so 1M is still measured true —
	// only the tools verdict is what this case is about.
	if !caps.Plain || caps.Tools || !caps.Thinking {
		t.Errorf("unexpected capabilities: %+v", caps)
	}
	if !caps.EntitlementBlocked {
		t.Error("EntitlementBlocked not set though tools were refused")
	}
	if !strings.Contains(caps.Detail, "agent") {
		t.Errorf("detail %q does not tell the user agent tools are the problem", caps.Detail)
	}
}

// Results are cached per model: the probe runs automatically after connect, and
// a UI redraw must not re-spend four requests. A different model re-measures,
// since capability is a property of the model as much as the account.
func TestProbe_CachesPerModel(t *testing.T) {
	stub := newProbeStub(t, func(r recorded) (int, map[string]string, string) {
		return http.StatusOK, nil, okBody()
	})
	m := connectedManager(t)

	first := m.Probe(context.Background(), "claude-sonnet-5")
	if len(stub.requests()) != 4 {
		t.Fatalf("first probe made %d requests, want 4", len(stub.requests()))
	}
	second := m.Probe(context.Background(), "claude-sonnet-5")
	if len(stub.requests()) != 4 {
		t.Error("a repeat probe for the same model re-sent requests")
	}
	if second.Model != first.Model {
		t.Errorf("cached model = %q, want %q", second.Model, first.Model)
	}

	// A different model is a different question.
	m.Probe(context.Background(), "claude-opus-5")
	if len(stub.requests()) != 8 {
		t.Errorf("after measuring a second model there were %d requests, want 8", len(stub.requests()))
	}

	if got := m.LastCapabilities(); got == nil || got.Model != "claude-opus-5" {
		t.Errorf("LastCapabilities = %+v, want the opus measurement", got)
	}
}

// The 1M beta is only sent on real requests once it has been measured, because
// the alternative (send it and hope) has no compatible recovery here.
func TestBetasForRequest_OneMOnlyAfterMeasurement(t *testing.T) {
	m := connectedManager(t)

	if strings.Contains(m.betasForRequest(), oneMBeta) {
		t.Error("the 1M beta was sent before anything was measured")
	}

	newProbeStub(t, func(r recorded) (int, map[string]string, string) {
		return http.StatusOK, nil, okBody()
	})
	m.Probe(context.Background(), "claude-sonnet-5")

	if !strings.Contains(m.betasForRequest(), oneMBeta) {
		t.Errorf("the 1M beta is still withheld after it was measured as supported: %q", m.betasForRequest())
	}

	// An account that does NOT get the 1M window must never be asked.
	noOneM := connectedManager(t)
	newProbeStub(t, func(r recorded) (int, map[string]string, string) {
		if strings.Contains(r.beta, oneMBeta) {
			return http.StatusBadRequest, nil, `{"type":"error","error":{"type":"invalid_request_error","message":"Error"}}`
		}
		return http.StatusOK, nil, okBody()
	})
	noOneM.Probe(context.Background(), "claude-sonnet-5")
	if strings.Contains(noOneM.betasForRequest(), oneMBeta) {
		t.Error("the 1M beta is being sent to an account that refused it")
	}
}

func TestProbe_NotConnectedIsNotAnError(t *testing.T) {
	m := newTestManager(t)
	caps := m.Probe(context.Background(), "claude-sonnet-5")
	if caps.Plain {
		t.Error("a disconnected manager reported capabilities")
	}
	if caps.Detail == "" {
		t.Error("no detail explaining why nothing was measured")
	}
}

// The default model const is duplicated in internal/app (which imports this
// package and so cannot be imported back). If they drift, the probe measures a
// different model than the one the app just connected with.
func TestDefaultModelMatchesApp(t *testing.T) {
	const appDefault = "claude-haiku-4-5-20251001"
	if claudeSubDefaultModelPublic != appDefault {
		t.Fatalf("default model %q in claudesub != %q in internal/app",
			claudeSubDefaultModelPublic, appDefault)
	}
}
