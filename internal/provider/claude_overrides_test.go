package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// decodeSystem flattens a built request's "system" field into the ordered list
// of text blocks it carries, so a test can assert POSITION — which is the
// whole point of the prepend: Anthropic validates the identity sentence by
// where it sits, not by whether it appears at all.
func decodeSystem(t *testing.T, clReq claudeRequest) []string {
	t.Helper()
	switch s := clReq.System.(type) {
	case nil:
		return nil
	case string:
		return []string{s}
	case []claudeSystemBlock:
		out := make([]string, 0, len(s))
		for _, b := range s {
			out = append(out, b.Text)
		}
		return out
	default:
		t.Fatalf("unexpected system type %T", s)
		return nil
	}
}

func reqWithSystemAndTools(system string, tools int) ChatRequest {
	req := ChatRequest{
		Model:       "claude-test",
		MaxTokens:   1024,
		Temperature: 0.2,
		TopP:        0.9,
		Messages:    []Message{{Role: "system", Content: system}, {Role: "user", Content: "hi"}},
	}
	for i := 0; i < tools; i++ {
		req.Tools = append(req.Tools, ToolDefinition{
			Function: ToolFunction{Name: "t", Description: "d", Parameters: json.RawMessage(`{}`)},
		})
	}
	return req
}

// The zero value must reproduce the shipped claudeProvider exactly — this is
// the whole justification for touching a hot file: a subscription provider
// gets a different identity without any behaviour change leaking back into
// ProviderClaude or ProviderCustomAnthropic.
func TestClaudeOverrides_ZeroValueIsByteIdenticalToShippedProvider(t *testing.T) {
	base, err := newClaudeProvider(ProviderConfig{BaseURL: "https://api.anthropic.com", Model: "m", APIKey: "k"})
	if err != nil {
		t.Fatalf("newClaudeProvider: %v", err)
	}
	via, err := NewClaudeProviderWith(ProviderConfig{BaseURL: "https://api.anthropic.com", Model: "m", APIKey: "k"}, ClaudeOverrides{})
	if err != nil {
		t.Fatalf("NewClaudeProviderWith: %v", err)
	}
	ov := via.(*claudeProvider)

	if ov.Name() != ProviderClaude {
		t.Errorf("Name() = %q, want %q", ov.Name(), ProviderClaude)
	}
	if ov.DisplayName() != "Anthropic Claude" {
		t.Errorf("DisplayName() = %q", ov.DisplayName())
	}

	for _, tc := range []struct {
		name  string
		sys   string
		tools int
	}{
		{"plain, string system", "persona", 0},
		{"agent, cacheable system", "persona", 2},
		{"no system at all", "", 0},
		{"no system but tools", "", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := base.buildClaudeRequest(reqWithSystemAndTools(tc.sys, tc.tools), "m", false)
			got := ov.buildClaudeRequest(reqWithSystemAndTools(tc.sys, tc.tools), "m", false)
			wantJSON, _ := json.Marshal(want)
			gotJSON, _ := json.Marshal(got)
			if string(wantJSON) != string(gotJSON) {
				t.Errorf("zero-value overrides changed the request\n base: %s\n over: %s", wantJSON, gotJSON)
			}
		})
	}
}

func TestClaudeOverrides_SystemPrependIsFirstBlock(t *testing.T) {
	const identity = "You are Claude Code, Anthropic's official CLI for Claude."
	p, err := NewClaudeProviderWith(
		ProviderConfig{BaseURL: "https://api.anthropic.com", Model: "m"},
		ClaudeOverrides{Type: ProviderType("override-test"), SystemPrepend: identity},
	)
	if err != nil {
		t.Fatalf("NewClaudeProviderWith: %v", err)
	}

	t.Run("plain chat", func(t *testing.T) {
		blocks := decodeSystem(t, p.(*claudeProvider).buildClaudeRequest(reqWithSystemAndTools("persona", 0), "m", false))
		if len(blocks) != 2 {
			t.Fatalf("blocks = %d (%q), want 2", len(blocks), blocks)
		}
		if blocks[0] != identity {
			t.Errorf("blocks[0] = %q, want the identity sentence", blocks[0])
		}
		if blocks[1] != "persona" {
			t.Errorf("blocks[1] = %q, want Memo's prompt", blocks[1])
		}
	})

	t.Run("agent turn keeps the cache breakpoint on the last block", func(t *testing.T) {
		clReq := p.(*claudeProvider).buildClaudeRequest(reqWithSystemAndTools("persona", 2), "m", false)
		sys, ok := clReq.System.([]claudeSystemBlock)
		if !ok {
			t.Fatalf("System is %T, want []claudeSystemBlock", clReq.System)
		}
		if len(sys) != 2 {
			t.Fatalf("blocks = %d, want 2", len(sys))
		}
		// The breakpoint marks the end of the cacheable prefix, so it belongs on
		// the LAST block. Putting it on the identity block would exclude the
		// tool list that follows it from the cached span.
		if sys[0].CacheControl != nil {
			t.Error("identity block carries cache_control; breakpoint must be last")
		}
		if sys[1].CacheControl == nil {
			t.Error("last system block is missing cache_control")
		}
	})

	t.Run("identity is emitted even with no Memo prompt", func(t *testing.T) {
		blocks := decodeSystem(t, p.(*claudeProvider).buildClaudeRequest(reqWithSystemAndTools("", 0), "m", false))
		if len(blocks) != 1 || blocks[0] != identity {
			t.Errorf("blocks = %q, want exactly the identity", blocks)
		}
	})
}

// If the endpoint refuses cache_control, postMessages withdraws it and
// stripCacheControl collapses the blocks into one plain string. The identity
// must survive that as the PREFIX — a plain system string is accepted, but
// only when it starts with the sentence.
func TestClaudeOverrides_IdentitySurvivesCacheWithdrawal(t *testing.T) {
	const identity = "You are Claude Code, Anthropic's official CLI for Claude."
	p := &claudeProvider{baseURL: "https://api.anthropic.com", systemPrepend: identity}
	clReq := p.buildClaudeRequest(reqWithSystemAndTools("persona", 2), "m", false)
	if !carriesCacheControl(clReq) {
		t.Fatal("agent turn did not carry cache_control to begin with")
	}

	stripped := stripCacheControl(clReq)
	s, ok := stripped.System.(string)
	if !ok {
		t.Fatalf("System is %T after withdrawal, want a plain string", stripped.System)
	}
	if !strings.HasPrefix(s, identity) {
		t.Errorf("collapsed system does not start with the identity: %q", s[:min(80, len(s))])
	}
}

func TestClaudeOverrides_SuppressSamplingDropsTemperatureUpFront(t *testing.T) {
	req := reqWithSystemAndTools("persona", 0)

	p, err := NewClaudeProviderWith(
		ProviderConfig{BaseURL: "https://api.anthropic.com", Model: "m"},
		ClaudeOverrides{Type: ProviderType("override-test"), SuppressSampling: true},
	)
	if err != nil {
		t.Fatalf("NewClaudeProviderWith: %v", err)
	}

	clReq := p.(*claudeProvider).buildClaudeRequest(req, "m", false)
	if clReq.Temperature != 0 || clReq.TopP != 0 {
		t.Errorf("sampling survived suppression: temperature=%v top_p=%v", clReq.Temperature, clReq.TopP)
	}

	// ...and the shipped provider is untouched by it.
	base, err := newClaudeProvider(ProviderConfig{BaseURL: "https://api.anthropic.com", Model: "m"})
	if err != nil {
		t.Fatalf("newClaudeProvider: %v", err)
	}
	if got := base.buildClaudeRequest(req, "m", false); got.Temperature != 0.2 || got.TopP != 0.9 {
		t.Errorf("suppression leaked into the shipped provider: temperature=%v top_p=%v", got.Temperature, got.TopP)
	}
}

func TestClaudeOverrides_AuthHookReplacesAPIKey(t *testing.T) {
	p, err := NewClaudeProviderWith(
		ProviderConfig{BaseURL: "https://api.anthropic.com", Model: "m", APIKey: "sk-ant-should-never-be-sent"},
		ClaudeOverrides{
			Type: ProviderType("override-test"),
			Auth: func(ctx context.Context, req *http.Request) error {
				req.Header.Set("Authorization", "Bearer sk-ant-oat01-test")
				req.Header.Set("anthropic-beta", "oauth-2025-04-20,claude-code-20250219")
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("NewClaudeProviderWith: %v", err)
	}

	req, _ := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	if err := p.(*claudeProvider).setAuth(context.Background(), req); err != nil {
		t.Fatalf("setAuth: %v", err)
	}
	if h := req.Header.Get("x-api-key"); h != "" {
		t.Errorf("x-api-key = %q; the Auth hook must fully replace it", h)
	}
	if h := req.Header.Get("Authorization"); h != "Bearer sk-ant-oat01-test" {
		t.Errorf("Authorization = %q", h)
	}
	if h := req.Header.Get("anthropic-beta"); h != "oauth-2025-04-20,claude-code-20250219" {
		t.Errorf("anthropic-beta = %q", h)
	}
}

func TestClaudeOverrides_AuthHookErrorAbortsRequest(t *testing.T) {
	p, err := NewClaudeProviderWith(
		ProviderConfig{BaseURL: "https://api.anthropic.com", Model: "m"},
		ClaudeOverrides{
			Type: ProviderType("override-test"),
			Auth: func(context.Context, *http.Request) error { return context.DeadlineExceeded },
		},
	)
	if err != nil {
		t.Fatalf("NewClaudeProviderWith: %v", err)
	}
	_, err = p.ChatCompletion(context.Background(), ChatRequest{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err == nil {
		t.Fatal("ChatCompletion succeeded although the Auth hook failed")
	}
}

func TestClaudeOverrides_IdentityAndDisplayNameAreReported(t *testing.T) {
	p, err := NewClaudeProviderWith(
		ProviderConfig{BaseURL: "https://api.anthropic.com", Model: "m"},
		ClaudeOverrides{Type: ProviderType("override-test"), DisplayName: "Anthropic Claude (subscription)"},
	)
	if err != nil {
		t.Fatalf("NewClaudeProviderWith: %v", err)
	}
	if p.Name() != ProviderType("override-test") {
		t.Errorf("Name() = %q, want %q", p.Name(), ProviderType("override-test"))
	}
	if p.DisplayName() != "Anthropic Claude (subscription)" {
		t.Errorf("DisplayName() = %q", p.DisplayName())
	}
}

func TestClaudeOverrides_BaseURLOverride(t *testing.T) {
	p, err := NewClaudeProviderWith(
		ProviderConfig{Model: "m"},
		ClaudeOverrides{BaseURL: "https://example.test/anthropic"},
	)
	if err != nil {
		t.Fatalf("NewClaudeProviderWith: %v", err)
	}
	if got := p.(*claudeProvider).baseURL; got != "https://example.test/anthropic" {
		t.Errorf("baseURL = %q", got)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
