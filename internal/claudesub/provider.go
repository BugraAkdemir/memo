// SPDX-License-Identifier: AGPL-3.0-or-later

package claudesub

import (
	"context"
	"net/http"
	"strings"

	"memo/internal/logx"
	"memo/internal/provider"
)

func init() {
	provider.RegisterConstructor(provider.ProviderClaudeSub, NewProvider)
}

// identitySystem is the sentence Anthropic requires as the FIRST system block
// when a request authenticates with a subscription OAuth token.
//
// The check is undocumented and it fails in two different shapes: a bare 400
// {"message":"Error"}, or — on current accounts and premium models — a 429
// rate_limit_error carrying no anthropic-ratelimit-* headers at all, which is
// indistinguishable from a genuine quota wall. anthropics/claude-code#87420
// documents exactly that confusion producing a cascade of false back-offs on
// accounts that were serving 200s a moment earlier. Haiku is exempt.
//
// Position matters as much as presence: the same sentence second in the system
// array, or at the end of a system string, is rejected.
const identitySystem = "You are Claude Code, Anthropic's official CLI for Claude."

// requiredBetas is the beta flag set this endpoint expects.
//
// oauth-2025-04-20 is what marks the request as subscription traffic;
// claude-code-20250219 is the CLI fingerprint; the rest mirror what the real
// client sends on a tool-carrying turn, and each one is a field or behaviour
// claaudesub relies on. context-1m-2025-08-07 is deliberately NOT in this list
// yet: a subscription entitled to the 1M window silently serves 200K without
// it, but sending it to a plan that has no 1M access gets a 400 that claude.go's
// existing 400-valve would misread (it withdraws temperature, then latches that
// off for the process). It lands together with the connect-time capability
// probe, which is what decides whether to send it at all.
const requiredBetas = "oauth-2025-04-20," +
	"claude-code-20250219," +
	"interleaved-thinking-2025-05-14," +
	"fine-grained-tool-streaming-2025-05-14," +
	"context-management-2025-06-27," +
	"advanced-tool-use-2025-11-20," +
	"effort-2025-11-24"

// oneMBeta is the 1M-context flag. It is sent only on requests whose capability
// the probe measured as supported, so an account without 1M access is never
// asked for it — see oneMContextSupported in probe.go for why the reactive
// alternative is worse here.
const oneMBeta = "context-1m-2025-08-07"

// claudeSubProvider is the provider.Provider for ProviderClaudeSub.
//
// It owns NO wire translation. Every call is carried by a claude.go-derived
// provider built through provider.NewClaudeProviderWith; this type's entire job
// is to answer the one question that provider cannot — "what credentials go on
// the request?" — by installing a decoration hook that resolves a live,
// refreshable token on every call. Resolving per call rather than caching one
// on the instance is what lets the user connect or disconnect and have the very
// next turn reflect it without the router being rebuilt.
type claudeSubProvider struct {
	mgr   *Manager
	inner provider.Provider
}

// NewProvider is the RegisterConstructor entry point. It never fails on a
// missing or expired token: a disconnected account must not break
// provider.NewRouter construction or the whole provider subsystem. Every call
// surfaces ErrNotConnected instead, until the user connects — the same
// contract internal/geminisub settled on.
func NewProvider(cfg provider.ProviderConfig) (provider.Provider, error) {
	mgr := Default()
	inner, err := provider.NewClaudeProviderWith(
		// APIKey deliberately empty: the Auth hook fully replaces x-api-key, and
		// a subscription account has no key to send anyway.
		provider.ProviderConfig{Model: cfg.Model, BaseURL: cfg.BaseURL},
		provider.ClaudeOverrides{
			Type:             provider.ProviderClaudeSub,
			DisplayName:      "Anthropic Claude (subscription)",
			SystemPrepend:    identitySystem,
			SuppressSampling: true,
			Auth:             mgr.decorateRequest,
		},
	)
	if err != nil {
		return nil, err
	}
	return &claudeSubProvider{mgr: mgr, inner: inner}, nil
}

func (p *claudeSubProvider) Name() provider.ProviderType { return provider.ProviderClaudeSub }
func (p *claudeSubProvider) DisplayName() string         { return "Anthropic Claude (subscription)" }

// ListModels returns the account's live model list. The inner claude provider
// already issues GET /v1/models with our Bearer token — the subscription
// exposes the same endpoint as the API-key path (anthropics/claude-code#35269:
// "the GET /v1/models endpoint works fine with the same token") — so this adds
// only the caching and a never-empty guarantee. Nothing here restricts which
// models appear: whatever the account is entitled to is what shows up.
func (p *claudeSubProvider) ListModels(ctx context.Context) ([]string, error) {
	if !p.mgr.Connected() {
		return nil, ErrNotConnected
	}
	if cached, ok := p.mgr.cachedModels(); ok {
		return cached, nil
	}

	list, err := p.inner.ListModels(ctx)
	if err != nil || len(list) == 0 {
		if cached, ok := p.mgr.cachedModels(); ok {
			return cached, nil
		}
		// An account that cannot be listed right now is still an account.
		// Returning the last-resort list keeps the model dropdown usable; the
		// first successful fetch replaces it wholesale.
		logx.Printf("claudesub: live model list unavailable, using fallback: %v", err)
		return append([]string(nil), fallbackModels...), nil
	}

	p.mgr.storeModels(list)
	return append([]string(nil), list...), nil
}

func (p *claudeSubProvider) ChatCompletion(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	return p.inner.ChatCompletion(ctx, withBareModel(req))
}

func (p *claudeSubProvider) ChatCompletionStream(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	return p.inner.ChatCompletionStream(ctx, withBareModel(req))
}

// withBareModel removes the "claude-sub/" qualifier the dev-gateway model list
// uses to namespace these entries, so the id that reaches Anthropic is the bare
// one.
func withBareModel(req provider.ChatRequest) provider.ChatRequest {
	req.Model = strings.TrimPrefix(req.Model, "claude-sub/")
	return req
}

// decorateRequest is the provider.ClaudeOverrides.Auth hook. It runs per
// request, with that request's own ctx, and is the only place a subscription
// credential is read.
//
// Errors are wrapped as *provider.ProviderError so the router classifies them
// the same way it classifies every other provider failure — in particular
// ErrNotConnected must not look like a 401 or a rate limit, or the router would
// back off and fall through to a different provider the user did not ask for.
func (m *Manager) decorateRequest(ctx context.Context, req *http.Request) error {
	ts, err := m.TokenSource(ctx)
	if err != nil {
		return &provider.ProviderError{Provider: provider.ProviderClaudeSub, Err: err}
	}
	tok, err := ts.Token()
	if err != nil {
		return &provider.ProviderError{Provider: provider.ProviderClaudeSub, Err: err}
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("anthropic-beta", m.betasForRequest())
	req.Header.Set("user-agent", userAgent())
	req.Header.Set("x-app", "cli")
	return nil
}

// betasForRequest is the beta set for a real request: the required flags, plus
// the 1M-context flag only when a probe measured this account as entitled to
// it. Sending the 1M beta speculatively is what hermes-agent does with a
// reactive recovery attached; there is no such recovery to attach it to here.
func (m *Manager) betasForRequest() string {
	if m.oneMContextSupported() {
		return requiredBetas + "," + oneMBeta
	}
	return requiredBetas
}

// userAgent identifies the request as the Claude Code client, which is part of
// what the endpoint's fingerprint checks look at. The version is the one this
// build of Memo was developed against; it is a claim about identity, not a
// promise of parity with that release.
var claudeCLIVersion = "2.1.280"

func userAgent() string {
	return "claude-cli/" + claudeCLIVersion + " (external, cli)"
}
