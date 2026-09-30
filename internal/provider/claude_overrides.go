package provider

import (
	"context"
	"net/http"
)

// This file is the seam that lets a provider type reuse claude.go's Anthropic
// wire format verbatim instead of copying it.
//
// Why it exists: internal/claudesub speaks to api.anthropic.com's /v1/messages
// with the EXACT payload claude.go builds — same Messages API, same streaming
// events, same tool_use/tool_result translation, same adaptive thinking, same
// cache accounting. The only differences are authentication (a refreshable
// OAuth Bearer token instead of x-api-key), three extra request headers, and a
// mandated first system block. internal/geminisub deliberately copied
// gemini.go, because Google's Code Assist endpoint wraps GenerateContent in a
// different envelope that no amount of configuration reaches; there is no such
// envelope here, so a copy would be 950 duplicated lines that silently fall
// behind every bugfix claude.go receives — and claude.go is one of the
// most-frequently-patched files in the repo (sampling latch, thinking-block
// replay, cache accounting).
//
// So: internal/claudesub owns only the account lifecycle (OAuth, encrypted
// token at rest, auto-adopting a local Claude Code login) and describes the
// differences as data.

// ClaudeOverrides parameterizes a claudeProvider built by
// NewClaudeProviderWith. The zero value reproduces today's claudeProvider
// exactly, which is what makes this seam safe to add alongside a shipped hot
// path rather than beside it.
type ClaudeOverrides struct {
	// Type is what Name() reports. It feeds Router's active-provider matching,
	// every ProviderError's Provider field, and the marker-provider lookup in
	// internal/app. Empty means ProviderClaude.
	Type ProviderType

	// DisplayName is the provider-list label. Empty means "Anthropic Claude".
	DisplayName string

	// BaseURL overrides the endpoint. Empty means api.anthropic.com.
	BaseURL string

	// SystemPrepend, when non-empty, becomes the FIRST entry of the request's
	// system array — before Memo's own prompt, with no separator and nothing
	// ahead of it. Anthropic's subscription-OAuth endpoint validates a literal
	// Claude Code identity sentence at exactly that position and answers
	// anything else with an opaque 400 ("Error") or a headerless 429 that is
	// indistinguishable from a genuine rate limit. Position matters: the same
	// sentence at the end of the system prompt is rejected.
	SystemPrepend string

	// SuppressSampling drops temperature/top_p from every request up front
	// rather than waiting for the 400-and-retry valve. The OAuth endpoint
	// refuses temperature != 1.0, but answers with a generic "Error" body that
	// names no parameter, so mentionsSampling can never fire and noSampling
	// would never latch — every turn would 400 forever.
	SuppressSampling bool

	// Auth replaces the x-api-key header wholesale: a Bearer token plus
	// whatever beta/user-agent set the endpoint needs. Returning an error
	// aborts the request before it is sent (the subscription path resolves a
	// refreshable token here and can fail doing so). When Auth is non-nil the
	// configured APIKey is never transmitted.
	Auth func(ctx context.Context, req *http.Request) error
}

// NewClaudeProviderWith builds a Provider that speaks claude.go's wire format
// with the given overrides applied. It is the only supported way to obtain
// such a provider — newClaudeProvider stays unexported because its output is
// exactly claude, and callers outside this package should not be able to
// accidentally produce an x-api-key-authenticated provider under a different
// Name().
//
// Never fails on a missing or expired credential, by design: a disconnected
// account must not break router construction or the provider subsystem as a
// whole. internal/claudesub's Auth hook reports the problem per call instead.
func NewClaudeProviderWith(cfg ProviderConfig, ov ClaudeOverrides) (Provider, error) {
	p, err := newClaudeProvider(cfg)
	if err != nil {
		return nil, err
	}
	if ov.BaseURL != "" {
		p.baseURL = ov.BaseURL
	}
	p.name = ov.Type
	p.displayName = ov.DisplayName
	p.systemPrepend = ov.SystemPrepend
	p.suppressSampling = ov.SuppressSampling
	p.decorateAuth = ov.Auth
	return p, nil
}
