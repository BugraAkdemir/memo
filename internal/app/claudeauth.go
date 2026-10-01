// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"strings"
	"time"

	"memo/internal/claudesub"
	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/provider"
)

// claudeSubProviderName is the Name of the marker ProviderConfig that
// ConnectClaudeAccount writes. It carries no secret — its only job is to make
// the claude-sub type show up in the router and the dev gateway's model list
// (resolveGatewayProvider / ListGatewayModels match on Type). The OAuth token
// lives encrypted under DataDir()/claudesub/, reached at call time through
// claudesub.Default().
const claudeSubProviderName = "Anthropic — Claude (subscription)"

// claudeSubDefaultModel is the marker's initial model until the user picks one
// from the account's live list in the Claude Subscription settings tab.
//
// Deliberately the most conservative member of the family rather than the most
// capable one: Anthropic's subscription-OAuth endpoint applies a stricter
// entitlement check to premium models than to Haiku, and a user who lands on a
// premium model the plan does not cover gets a bare 429 with no rate-limit
// headers that is impossible to tell apart from a real quota wall. Starting
// conservative means the first thing anyone tries works.
const claudeSubDefaultModel = "claude-haiku-4-5-20251001"

// claudeSubMarkerConfig is the enabled ProviderConfig written on connect and
// removed on disconnect. model may be "" to use the default.
func claudeSubMarkerConfig(model string) provider.ProviderConfig {
	if strings.TrimSpace(model) == "" {
		model = claudeSubDefaultModel
	}
	return provider.ProviderConfig{
		Type:    provider.ProviderClaudeSub,
		Name:    claudeSubProviderName,
		Model:   model,
		Enabled: true,
	}
}

// ClaudeAccountState reports whether a Claude subscription is connected, the
// cached display label, how it was connected, and the selected model id.
func (a *App) ClaudeAccountState() (connected bool, account, source, model string) {
	// A token on disk is the ground truth, not the config flag: a user who
	// deletes the data dir's token.enc behind our back would otherwise be told
	// "connected" forever while every turn failed with ErrNotConnected. The
	// config's Connected field still gates writes (see below), so this is a
	// read-side correction rather than a second source of truth.
	if !claudesub.Default().Connected() {
		return false, "", "", ""
	}
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	st := a.cfg.DevGateway.ClaudeSub
	return true, st.Account, st.Source, st.Model
}

// ConnectClaudeAccount signs in without a browser wherever possible.
//
// This is the whole point of adopting a local login: a user who is already
// signed into Claude Code clicks "Connect" and is simply connected. Only when
// nothing is found locally does it fall back to the hosted OAuth flow, and in
// that case it returns an authorize URL plus the state the pasted code will be
// checked against (see claudesub.StartAuth).
//
// There is no background "wait for the callback" goroutine the way
// StartGoogleAuth has one, because Anthropic does not redirect to us: the
// browser lands on an Anthropic page that displays the code, and the user
// brings it back to CompleteClaudeAuth themselves.
func (a *App) ConnectClaudeAccount() (connected bool, source, authURL, state string, err error) {
	m := claudesub.Default()

	if res := claudesub.AdoptLocal(); res.Token != nil {
		if err := m.Adopt(res); err != nil {
			return false, "", "", "", err
		}
		if err := a.finalizeClaudeConnect(res.Source); err != nil {
			return false, "", "", "", err
		}
		return true, res.Source, "", "", nil
	}

	// Already connected — most often because Default() adopted a local login at
	// startup and adoptClaudeLoginIfPresent already wrote the marker. Report the
	// source the Manager actually recorded rather than assuming "browser":
	// telling a user who was adopted from ~/.claude/.credentials.json that they
	// signed in through the browser is a story that never happened, and it is
	// the only clue they have about why no browser opened.
	if m.Connected() {
		src := m.Source()
		if src == "" {
			src = "browser"
		}
		if err := a.finalizeClaudeConnect(src); err != nil {
			return false, "", "", "", err
		}
		return true, src, "", "", nil
	}

	authURL, state, _, err = m.StartAuth(0)
	if err != nil {
		return false, "", "", "", err
	}
	return false, "browser", authURL, state, nil
}

// CompleteClaudeAuth finishes the browser flow with the code the user pasted
// from Anthropic's callback page. code may be the bare code or the whole URL
// the browser ended on.
func (a *App) CompleteClaudeAuth(code, state string) error {
	parent := a.lifecycleCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	if err := claudesub.Default().CompleteAuth(ctx, code, state); err != nil {
		return err
	}
	return a.finalizeClaudeConnect("browser")
}

// finalizeClaudeConnect writes the enabled marker provider so the type is
// reachable, then records the connected state. Keeps any model the user had
// already selected on a reconnect.
func (a *App) finalizeClaudeConnect(source string) error {
	a.cfgMu.RLock()
	model := a.cfg.DevGateway.ClaudeSub.Model
	a.cfgMu.RUnlock()

	marker := claudeSubMarkerConfig(model)
	if err := a.UpdateProvider(marker); err != nil {
		return err
	}

	a.cfgMu.Lock()
	a.cfg.DevGateway.ClaudeSub = config.ClaudeSubState{
		Connected: true,
		Account:   claudeSubAccountLabel(),
		Source:    source,
		Model:     marker.Model,
	}
	cfg := a.cfg
	a.cfgMu.Unlock()
	if err := config.Save(cfg); err != nil {
		return err
	}

	a.measureClaudeCapabilities(marker.Model)
	return nil
}

// measureClaudeCapabilities probes what the account can actually do and, when
// it turns out to serve the 1M window, raises the marker provider's context
// budget to match.
//
// This runs in the background on purpose: it is four tiny requests, and the
// user has just finished a connect flow they are waiting to see the result of.
// Blocking that on 45s of network is not acceptable, and nothing downstream
// depends on the answer — the first real turn uses the conservative 200K budget
// either way. The probe's real job is to make the NEXT turn correct and to give
// the user a table explaining a class of otherwise inexplicable failures.
func (a *App) measureClaudeCapabilities(model string) {
	parent := a.lifecycleCtx
	if parent == nil {
		parent = context.Background()
	}
	goRecover("claudeauth.probe", func() {
		ctx, cancel := context.WithTimeout(parent, 60*time.Second)
		defer cancel()
		caps := claudesub.Default().Probe(ctx, model)
		if !caps.OneMContext || caps.Model != model {
			return
		}
		// Re-write the marker with a 1M budget. modelContextWindow consults
		// ContextTokens before its per-type fallbacks, so this is the only
		// thing that has to change for the agent's truncation to stop
		// compacting a conversation that had room to spare.
		a.cfgMu.RLock()
		cfgMgr := a.providerCfgMgr
		a.cfgMu.RUnlock()
		if cfgMgr == nil {
			return
		}
		marker := claudeSubMarkerConfig(model)
		marker.ContextTokens = 1024 * 1024
		if err := a.UpdateProvider(marker); err != nil {
			logx.Printf("claudeauth: raise context budget after 1M probe: %v", err)
		}
	})
}

// claudeSubAccountLabel is what the settings tab shows as the connected
// account. The OAuth scope set this uses has no userinfo endpoint, so there is
// no email to display the way Gemini-sub shows one; the honest label is the
// product name and how it was connected.
func claudeSubAccountLabel() string {
	return claudeSubProviderName
}

// SetClaudeAccountModel changes which model the claude-sub marker provider
// (and Memo's own chat, when this provider is active) uses. No-op if not
// connected.
func (a *App) SetClaudeAccountModel(model string) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil
	}
	a.cfgMu.Lock()
	if !a.cfg.DevGateway.ClaudeSub.Connected {
		a.cfgMu.Unlock()
		return nil
	}
	a.cfg.DevGateway.ClaudeSub.Model = model
	cfg := a.cfg
	a.cfgMu.Unlock()

	if err := a.UpdateProvider(claudeSubMarkerConfig(model)); err != nil {
		return err
	}
	return config.Save(cfg)
}

// adoptClaudeLoginIfPresent is called once at startup. If the user has not
// connected in-app but Claude Code (or a CLAUDE_CODE_OAUTH_TOKEN in the
// environment) already holds a usable login, write the marker provider and the
// connected state so claude-sub works with no click at all.
//
// Mirrors adoptGeminiCLILoginIfPresent. Best-effort: nothing here can fail
// startup — a dead or absent local login is silently ignored, and the user
// still has the Settings tab.
func (a *App) adoptClaudeLoginIfPresent() {
	a.cfgMu.RLock()
	already := a.cfg.DevGateway.ClaudeSub.Connected
	a.cfgMu.RUnlock()
	if already {
		return
	}
	if !claudesub.Default().Connected() {
		return // nothing adopted at construction time
	}
	if err := a.finalizeClaudeConnect("adopted"); err != nil {
		logx.Printf("claudeauth: adopt local Claude login: %v", err)
	}
}

// DisconnectClaudeAccount clears the stored token, removes the marker provider,
// and clears the connected state. A no-op if not connected.
//
// It does not revoke with Anthropic, and that is deliberate: the token may be
// shared with the user's own Claude Code install, and revoking it there would
// sign the CLI out of their own account because they clicked something in
// Memo. The user signs out from Claude Code if that is what they want.
func (a *App) DisconnectClaudeAccount() error {
	a.cfgMu.Lock()
	connected := a.cfg.DevGateway.ClaudeSub.Connected
	a.cfgMu.Unlock()
	if !connected && !claudesub.Default().Connected() {
		return nil
	}

	if err := claudesub.Default().Disconnect(); err != nil {
		logx.Printf("claudeauth: token clear (non-fatal): %v", err)
	}
	if err := a.DeleteProvider(provider.ProviderClaudeSub, claudeSubProviderName); err != nil {
		logx.Printf("claudeauth: remove marker provider (non-fatal): %v", err)
	}

	a.cfgMu.Lock()
	a.cfg.DevGateway.ClaudeSub = config.ClaudeSubState{}
	cfg := a.cfg
	a.cfgMu.Unlock()
	return config.Save(cfg)
}

// ClaudeCapabilities exposes the measured capability table to the REST surface.
// Returns nil before the first probe finishes, which is the honest answer: the
// probe fires in the background after connecting, so "not measured yet" and
// "measured, nothing works" must not look the same to a caller.
func (a *App) ClaudeCapabilities() map[string]any {
	caps := claudesub.Default().LastCapabilities()
	if caps == nil {
		return nil
	}
	return map[string]any{
		"model":               caps.Model,
		"plain":               caps.Plain,
		"tools":               caps.Tools,
		"thinking":            caps.Thinking,
		"one_m_context":       caps.OneMContext,
		"entitlement_blocked": caps.EntitlementBlocked,
		"detail":              caps.Detail,
		"measured_at":         caps.MeasuredAt.UTC().Format(time.RFC3339),
	}
}
