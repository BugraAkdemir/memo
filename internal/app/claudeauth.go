// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"errors"
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

// requireClaudeSubBeta is the backend half of claude-sub being a Beta feature.
// The settings UI only shows the panel while Beta is on, but the REST surface
// is reachable without it, so every state-changing entry point checks here too.
func (a *App) requireClaudeSubBeta() error {
	if a.cfg == nil || !a.cfg.Beta {
		return errors.New(a.t(
			"Claude aboneliği bir beta özelliğidir; Ayarlar › Beta'dan Beta'yı açın",
			"Claude Subscription is a beta feature; enable Beta in Settings › Beta",
		))
	}
	return nil
}

// ClaudeAccountState reports whether a Claude subscription is connected, the
// cached display label, how it was connected, and the selected model id.
func (a *App) ClaudeAccountState() (connected bool, account, source, model string) {
	// Connected needs BOTH the token and the recorded connection. The token is
	// the ground truth for "can a turn work": a user who deletes the data
	// dir's token.enc behind our back would otherwise be told "connected"
	// forever while every turn failed with ErrNotConnected. The config flag is
	// the ground truth for "did the user connect": a token alone, with no
	// recorded connection, has no marker provider behind it and no model.
	if !claudesub.Default().Connected() {
		return false, "", "", ""
	}
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	st := a.cfg.DevGateway.ClaudeSub
	if !st.Connected {
		return false, "", "", ""
	}
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
	if err := a.requireClaudeSubBeta(); err != nil {
		return false, "", "", "", err
	}
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

	// Already connected — a token from an earlier browser sign-in, or a local
	// login adopted earlier whose source has since gone away. Report the
	// source the Manager actually recorded rather than assuming "browser":
	// telling a user who was adopted from ~/.claude/.credentials.json that they
	// signed in through the browser is a story that never happened, and it is
	// the only clue they have about why no browser opened.
	if m.Connected() {
		src := m.Source()
		if src == "" {
			src = claudesub.SourceBrowser
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
	return false, claudesub.SourceBrowser, authURL, state, nil
}

// CompleteClaudeAuth finishes the browser flow with the code the user pasted
// from Anthropic's callback page. code may be the bare code or the whole URL
// the browser ended on.
func (a *App) CompleteClaudeAuth(code, state string) error {
	if err := a.requireClaudeSubBeta(); err != nil {
		return err
	}
	parent := a.lifecycleCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	if err := claudesub.Default().CompleteAuth(ctx, code, state); err != nil {
		return err
	}
	return a.finalizeClaudeConnect(claudesub.SourceBrowser)
}

// finalizeClaudeConnect writes the enabled marker provider so the type is
// reachable, then records the connected state. Keeps any model the user had
// already selected on a reconnect. Recording a fresh state also clears
// UserDisconnected: connecting is the user taking that decision back.
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
//
// Capability is a property of the model as much as of the account, so the
// switch re-runs the probe. Without that the table kept describing the
// previous model, and the 1M budget it raised stayed on the marker for a model
// nobody had measured.
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
	if !a.cfg.Beta {
		a.cfgMu.Unlock()
		return a.requireClaudeSubBeta()
	}
	a.cfg.DevGateway.ClaudeSub.Model = model
	cfg := a.cfg
	a.cfgMu.Unlock()

	if err := a.UpdateProvider(claudeSubMarkerConfig(model)); err != nil {
		return err
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	a.measureClaudeCapabilities(model)
	return nil
}

// syncClaudeSubWithBeta makes the claude-sub provider exist exactly when Beta
// is on. Called at startup and whenever Beta is toggled.
//
//   - Beta off: the marker provider is removed so the type never reaches the
//     router (DeleteProvider also drops it as the active provider). The token
//     and the recorded connection are kept, so switching Beta back on restores
//     the account without a second sign-in.
//   - Beta on, connected: the marker is (re)written at the conservative context
//     budget and the probe re-measures in the background. After a restart the
//     probe cache is empty, so a 1M budget persisted in providers.json would
//     otherwise outlive the measurement that justified it — the beta header is
//     only sent while a measurement says so.
//   - Beta on, not connected: a Claude Code login already on this machine is
//     adopted so claude-sub works with no click — unless the user pressed
//     Disconnect, which this respects across restarts.
//
// Best-effort: nothing here can fail startup or the Beta toggle.
func (a *App) syncClaudeSubWithBeta() {
	if a.cfg == nil {
		return
	}
	a.cfgMu.RLock()
	beta := a.cfg.Beta
	st := a.cfg.DevGateway.ClaudeSub
	a.cfgMu.RUnlock()
	m := claudesub.Default()

	if !beta {
		if a.hasClaudeSubMarker() {
			if err := a.DeleteProvider(provider.ProviderClaudeSub, claudeSubProviderName); err != nil {
				logx.Printf("claudeauth: remove marker while Beta is off: %v", err)
			}
		}
		return
	}
	if st.Connected && m.Connected() {
		marker := claudeSubMarkerConfig(st.Model)
		if err := a.UpdateProvider(marker); err != nil {
			logx.Printf("claudeauth: restore marker: %v", err)
			return
		}
		a.measureClaudeCapabilities(marker.Model)
		return
	}
	if st.UserDisconnected {
		return
	}
	res := claudesub.AdoptLocal()
	if res.Token == nil {
		return
	}
	if err := m.Adopt(res); err != nil {
		logx.Printf("claudeauth: adopt local Claude login: %v", err)
		return
	}
	if err := a.finalizeClaudeConnect(res.Source); err != nil {
		logx.Printf("claudeauth: adopt local Claude login: %v", err)
		return
	}
	logx.Printf("claudeauth: adopted an existing Claude Code login from %s", res.Source)
}

// hasClaudeSubMarker reports whether the marker provider config exists.
func (a *App) hasClaudeSubMarker() bool {
	for _, p := range a.GetProviders() {
		if p.Type == provider.ProviderClaudeSub {
			return true
		}
	}
	return false
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
	// Remembered, so the next startup does not quietly adopt the same
	// Claude Code login the user just disconnected from.
	a.cfg.DevGateway.ClaudeSub = config.ClaudeSubState{UserDisconnected: true}
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
