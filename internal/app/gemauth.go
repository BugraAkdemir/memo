// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"strings"
	"time"

	"memo/internal/config"
	"memo/internal/geminisub"
	"memo/internal/logx"
	"memo/internal/provider"
)

// geminiSubProviderName is the Name of the marker ProviderConfig that
// ConnectGoogleAccount writes. It carries no secret — its only job is to
// make the gemini-sub type show up in the router and the dev gateway's model
// list (resolveGatewayProvider / ListGatewayModels match on Type). The OAuth
// token lives encrypted under DataDir()/geminisub/, reached at call time
// through geminisub.Default().
const geminiSubProviderName = "Google — Gemini (subscription)"

// geminiSubDefaultModel is the marker's initial model until the user picks
// one from the account's live list in the Gemini Subscription settings tab.
const geminiSubDefaultModel = "gemini-2.5-pro"

// geminiSubMarkerConfig is the enabled ProviderConfig written on connect and
// removed on disconnect. model may be "" to use the default.
func geminiSubMarkerConfig(model string) provider.ProviderConfig {
	if strings.TrimSpace(model) == "" {
		model = geminiSubDefaultModel
	}
	return provider.ProviderConfig{
		Type:    provider.ProviderGeminiSub,
		Name:    geminiSubProviderName,
		Model:   model,
		Enabled: true,
	}
}

// GoogleAccountState reports whether a Google account is connected for the
// gemini-sub provider, the cached display email, and the selected model id.
func (a *App) GoogleAccountState() (connected bool, email, model string) {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	st := a.cfg.DevGateway.GeminiSub
	return st.Connected, st.Email, st.Model
}

// StartGoogleAuth begins the OAuth loopback flow and returns the URL the
// user must open in a browser. A background goroutine waits for the flow to
// finish and then finalizes the connection (records the account, writes the
// enabled gemini-sub marker provider). The caller polls GoogleAccountState
// for completion — mirrors the cloud-sync connect UX.
func (a *App) StartGoogleAuth() (string, error) {
	m := geminisub.Default()
	url, err := m.StartAuth()
	if err != nil {
		return "", err
	}
	goRecover("gemauth.finalize", func() {
		parent := a.lifecycleCtx
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
		defer cancel()
		if err := m.AwaitAuth(ctx); err != nil {
			logx.Printf("gemauth: OAuth flow did not complete: %v", err)
			return
		}
		if err := a.finalizeGoogleConnect(ctx); err != nil {
			logx.Printf("gemauth: finalize after connect failed: %v", err)
		}
	})
	return url, nil
}

// finalizeGoogleConnect runs once the OAuth flow has produced a token: fetch
// the account email (best-effort), write the enabled marker provider so the
// model is reachable, and persist the connected state. Keeps any model the
// user had already selected on a reconnect.
func (a *App) finalizeGoogleConnect(ctx context.Context) error {
	m := geminisub.Default()

	email := ""
	if _, e, err := m.AccountInfo(ctx); err != nil {
		logx.Printf("gemauth: account info (non-fatal): %v", err)
	} else {
		email = e
	}

	a.cfgMu.RLock()
	model := a.cfg.DevGateway.GeminiSub.Model
	a.cfgMu.RUnlock()

	marker := geminiSubMarkerConfig(model)
	if err := a.UpdateProvider(marker); err != nil {
		return err
	}

	a.cfgMu.Lock()
	a.cfg.DevGateway.GeminiSub = config.GeminiSubState{Connected: true, Email: email, Model: marker.Model}
	cfg := a.cfg
	a.cfgMu.Unlock()
	return config.Save(cfg)
}

// SetGoogleAccountModel changes which model the gemini-sub marker provider
// (and Memo's own chat, when this provider is active) uses. No-op if not
// connected.
func (a *App) SetGoogleAccountModel(model string) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil
	}
	a.cfgMu.Lock()
	if !a.cfg.DevGateway.GeminiSub.Connected {
		a.cfgMu.Unlock()
		return nil
	}
	a.cfg.DevGateway.GeminiSub.Model = model
	cfg := a.cfg
	a.cfgMu.Unlock()

	if err := a.UpdateProvider(geminiSubMarkerConfig(model)); err != nil {
		return err
	}
	return config.Save(cfg)
}

// adoptGeminiCLILoginIfPresent is called once at startup. If the user has
// not connected in-app but the official gemini-cli is already logged in
// (~/.gemini/oauth_creds.json), verify that token works and adopt it —
// writing the marker provider and connected state so gemini-sub is usable
// with no extra click. Mirrors claude-code-proxy's ~/.claude/.credentials.json
// fallback. Best-effort: a dead/absent gemini-cli token is silently ignored.
func (a *App) adoptGeminiCLILoginIfPresent() {
	a.cfgMu.RLock()
	already := a.cfg.DevGateway.GeminiSub.Connected
	a.cfgMu.RUnlock()
	if already {
		return
	}

	m := geminisub.Default()
	if !m.Connected() {
		return // no gemini-cli token seeded
	}

	parent := a.lifecycleCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()

	_, email, err := m.AccountInfo(ctx)
	if err != nil {
		logx.Printf("gemauth: gemini-cli token present but not usable, ignoring: %v", err)
		return
	}

	marker := geminiSubMarkerConfig("")
	if err := a.UpdateProvider(marker); err != nil {
		logx.Printf("gemauth: adopt gemini-cli login — write marker: %v", err)
		return
	}
	a.cfgMu.Lock()
	a.cfg.DevGateway.GeminiSub = config.GeminiSubState{Connected: true, Email: email, Model: marker.Model}
	cfg := a.cfg
	a.cfgMu.Unlock()
	if err := config.Save(cfg); err != nil {
		logx.Printf("gemauth: adopt gemini-cli login — save config: %v", err)
		return
	}
	logx.Printf("gemauth: adopted existing gemini-cli login (%s) for gemini-sub", email)
}

// DisconnectGoogleAccount revokes and deletes the stored token, removes the
// marker provider, and clears the connected state. A no-op if not connected.
func (a *App) DisconnectGoogleAccount() error {
	a.cfgMu.Lock()
	connected := a.cfg.DevGateway.GeminiSub.Connected
	a.cfgMu.Unlock()
	if !connected {
		return nil
	}

	parent := a.lifecycleCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	if err := geminisub.Default().Disconnect(ctx); err != nil {
		logx.Printf("gemauth: token revoke/clear (non-fatal): %v", err)
	}

	if err := a.DeleteProvider(provider.ProviderGeminiSub, geminiSubProviderName); err != nil {
		logx.Printf("gemauth: remove marker provider (non-fatal): %v", err)
	}

	a.cfgMu.Lock()
	a.cfg.DevGateway.GeminiSub = config.GeminiSubState{}
	cfg := a.cfg
	a.cfgMu.Unlock()
	return config.Save(cfg)
}
