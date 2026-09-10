// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
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

// geminiSubMarkerConfig is the enabled ProviderConfig written on connect and
// removed on disconnect.
func geminiSubMarkerConfig() provider.ProviderConfig {
	return provider.ProviderConfig{
		Type:    provider.ProviderGeminiSub,
		Name:    geminiSubProviderName,
		Model:   "gemini-2.5-pro",
		Enabled: true,
	}
}

// GoogleAccountState reports whether a Google account is connected for the
// gemini-sub provider, and the cached display email.
func (a *App) GoogleAccountState() (connected bool, email string) {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.DevGateway.GeminiSub.Connected, a.cfg.DevGateway.GeminiSub.Email
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
// model is reachable, and persist the connected state.
func (a *App) finalizeGoogleConnect(ctx context.Context) error {
	m := geminisub.Default()

	email := ""
	if _, e, err := m.AccountInfo(ctx); err != nil {
		logx.Printf("gemauth: account info (non-fatal): %v", err)
	} else {
		email = e
	}

	if err := a.UpdateProvider(geminiSubMarkerConfig()); err != nil {
		return err
	}

	a.cfgMu.Lock()
	a.cfg.DevGateway.GeminiSub = config.GeminiSubState{Connected: true, Email: email}
	cfg := a.cfg
	a.cfgMu.Unlock()
	return config.Save(cfg)
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
