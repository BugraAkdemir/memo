// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"fmt"
	"memo/internal/logx"
	"time"

	"memo/internal/config"
	"memo/internal/tunnel"
)

// startTailscale brings up the embedded Tailscale tunnel that reverse-proxies to
// the local web server. The web server must already be running on the given
// port. rc.TailscaleKey may be empty — Start() then triggers interactive
// browser login instead of failing. interactive must be true only when this
// call is a direct response to the user clicking "Tailscale ile Bağlan" —
// see TailscaleConfig.Interactive's doc comment for why the boot-time
// auto-start path (startupTailscale) must always pass false.
func (a *App) startTailscale(port int, interactive bool) error {
	rc := a.cfg.RemoteAccess
	if a.tailscaleTunnel == nil {
		a.tailscaleTunnel = tunnel.NewTailscale()
	}
	if a.tailscaleTunnel.IsRunning() {
		a.tailscaleTunnel.Stop()
	}

	hostname := rc.TailscaleHostname
	if hostname == "" {
		hostname = "memo"
	}

	// Proxy to the port the web server is actually listening on, not the
	// (possibly stale) value passed from the UI.
	target := port
	if ws := a.getWebServer(); ws != nil && ws.IsRunning() {
		if p := ws.GetPort(); p > 0 {
			target = p
		}
	}

	return a.tailscaleTunnel.Start(tunnel.TailscaleConfig{
		Hostname:    hostname,
		AuthKey:     rc.TailscaleKey,
		Funnel:      rc.TailscaleFunnel,
		LocalPort:   target,
		StateDir:    config.DataPath("tailscale"),
		Interactive: interactive,
	})
}

// stopTailscale tears the tunnel down if running.
func (a *App) stopTailscale() {
	if a.tailscaleTunnel != nil {
		a.tailscaleTunnel.Stop()
	}
}

// SetTailscaleMode configures and (re)starts the Tailscale tunnel. Passing an
// empty authKey keeps the previously stored key. No longer Beta-gated — see
// SetBeta's doc comment for why Tailscale and Beta were decoupled.
func (a *App) SetTailscaleMode(enabled bool, authKey, hostname string, funnel bool, port int) error {
	if authKey != "" {
		a.cfg.RemoteAccess.TailscaleKey = authKey
	}
	if hostname != "" {
		a.cfg.RemoteAccess.TailscaleHostname = hostname
	}
	a.cfg.RemoteAccess.TailscaleFunnel = funnel

	if enabled {
		a.cfg.RemoteAccess.TunnelMode = "tailscale"
		a.cfg.RemoteAccess.Enabled = true
	} else if a.cfg.RemoteAccess.TunnelMode == "tailscale" {
		a.cfg.RemoteAccess.TunnelMode = "lan"
	}

	if err := config.Save(a.cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	// Ensure the local web server is up (LAN bind) so the tunnel has a target.
	if ws := a.getWebServer(); ws != nil && !ws.IsRunning() {
		if err := ws.StartHTTPWithAddr(port, "127.0.0.1"); err != nil {
			return fmt.Errorf("start web server: %w", err)
		}
	}

	if enabled {
		// Set synchronously, matching cfg.RemoteAccess.Enabled above — this
		// reflects "remote access is turned on" (the user's intent), not
		// "the tunnel is currently connected" (that's Tailscale.IsRunning(),
		// the actual status GetRemoteAccessStatus already reports). Setting
		// it eagerly means the interactive-login goroutine below never
		// touches it, avoiding an unguarded write racing against
		// remote.go's SetRemoteAccess (which reads/writes the same field on
		// whatever goroutine an unrelated, concurrently-handled HTTP request
		// lands on) up to interactiveLoginTimeout later.
		a.remoteAccessEnabled = true
		if a.cfg.RemoteAccess.TailscaleKey == "" {
			// Interactive login: startTailscale blocks (up to
			// interactiveLoginTimeout) waiting for the user to approve the
			// browser prompt tsnet just opened. Running it inline would hang
			// this request for minutes, so it goes to the background —
			// progress is polled via GetRemoteAccessStatus's
			// tailscale_running/tailscale_auth_url/tailscale_error fields.
			go func() {
				if err := a.startTailscale(port, true); err != nil {
					logx.Printf("[tailscale] interactive login failed: %v", err)
					return
				}
				a.cfg.RemoteAccess.TailscaleConnectedOnce = true
				if err := config.Save(a.cfg); err != nil {
					logx.Printf("WARN: save config: %v", err)
				}
				logx.Printf("[tailscale] tunnel started: %s", a.tailscaleTunnel.PublicURL())
			}()
		} else {
			if err := a.startTailscale(port, true); err != nil {
				return fmt.Errorf("start tailscale: %w", err)
			}
			a.cfg.RemoteAccess.TailscaleConnectedOnce = true
			if err := config.Save(a.cfg); err != nil {
				logx.Printf("WARN: save config: %v", err)
			}
			logx.Printf("[tailscale] tunnel started: %s", a.tailscaleTunnel.PublicURL())
		}
	} else {
		a.stopTailscale()
	}
	return nil
}

// SetBeta toggles experimental features. Tailscale graduated out of Beta and
// is no longer affected by this — it has its own on/off toggle
// (SetTailscaleMode) and keeps running across a Beta flip. Turning beta off
// still immediately stops Swarm, which remains genuinely experimental.
func (a *App) SetBeta(enabled bool) error {
	if a.cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	a.cfg.Beta = enabled
	if !enabled {
		// Beta-off must not leave experimental processes (swarm coordinator
		// / rpc-server) running in the background.
		a.stopSwarmForBetaOff()
	}
	if err := config.Save(a.cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

// GetBeta reports whether beta features are enabled.
func (a *App) GetBeta() bool {
	return a.cfg != nil && a.cfg.Beta
}

// startupTailscaleRetries/-Delay bound the boot-time retry below. Once
// startTailscale succeeds, the tunnel package's own reconnectLoop takes over
// for any *later* drop — this is only for the network not being up yet at
// the exact moment Memo launches (e.g. right after a machine reboot).
const (
	startupTailscaleRetries = 4
	startupTailscaleDelay   = 15 * time.Second
)

// startupTailscale auto-starts the tunnel on launch when configured.
func (a *App) startupTailscale() {
	if a.cfg == nil {
		return
	}
	rc := a.cfg.RemoteAccess
	if rc.TunnelMode != "tailscale" || !rc.Enabled {
		return
	}
	if rc.TailscaleKey == "" && !rc.TailscaleConnectedOnce {
		// Never actually connected (interactive login never completed, or a
		// keyed setup that's never once succeeded) — auto-starting now would
		// either silently no-op or, for the key-free case, pop an
		// interactive browser login at boot with no human necessarily
		// present to approve it. Once TailscaleConnectedOnce is true, tsnet
		// usually reconnects from its persisted node identity in StateDir
		// without needing a fresh login, so this only gates the
		// never-connected case. If the persisted session has since gone
		// stale (revoked node, expired key) and a fresh login turns out to
		// be needed anyway, startTailscale's interactive=false below is the
		// actual safety net — it can't fully rule that out in advance.
		return
	}
	port := rc.Port
	if ws := a.getWebServer(); ws != nil && !ws.IsRunning() {
		if err := ws.StartHTTPWithAddr(port, "127.0.0.1"); err != nil {
			logx.Printf("[tailscale] web server start: %v", err)
			return
		}
	}

	var err error
	for attempt := 1; attempt <= startupTailscaleRetries; attempt++ {
		if err = a.startTailscale(port, false); err == nil {
			return
		}
		logx.Printf("[tailscale] auto-start attempt %d/%d failed: %v", attempt, startupTailscaleRetries, err)
		if attempt < startupTailscaleRetries {
			time.Sleep(startupTailscaleDelay)
		}
	}
	logx.Printf("[tailscale] auto-start gave up after %d attempts: %v", startupTailscaleRetries, err)
}
