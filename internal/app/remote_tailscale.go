// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"fmt"
	"memo/internal/logx"

	"memo/internal/config"
	"memo/internal/tunnel"
)

// startTailscale brings up the embedded Tailscale tunnel that reverse-proxies to
// the local web server. The web server must already be running on the given port.
func (a *App) startTailscale(port int) error {
	rc := a.cfg.RemoteAccess
	if rc.TailscaleKey == "" {
		return fmt.Errorf("tailscale auth key not set")
	}
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
	if a.webServer != nil && a.webServer.IsRunning() {
		if p := a.webServer.GetPort(); p > 0 {
			target = p
		}
	}

	return a.tailscaleTunnel.Start(tunnel.TailscaleConfig{
		Hostname:  hostname,
		AuthKey:   rc.TailscaleKey,
		Funnel:    rc.TailscaleFunnel,
		LocalPort: target,
		StateDir:  config.DataPath("tailscale"),
	})
}

// stopTailscale tears the tunnel down if running.
func (a *App) stopTailscale() {
	if a.tailscaleTunnel != nil {
		a.tailscaleTunnel.Stop()
	}
}

// SetTailscaleMode configures and (re)starts the Tailscale tunnel. Passing an
// empty authKey keeps the previously stored key.
func (a *App) SetTailscaleMode(enabled bool, authKey, hostname string, funnel bool, port int) error {
	if a.cfg != nil && !a.cfg.Beta {
		return fmt.Errorf("Tailscale beta özelliğidir; Ayarlar'dan Beta'yı açın")
	}
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
	if a.webServer != nil && !a.webServer.IsRunning() {
		if err := a.webServer.StartHTTPWithAddr(port, "127.0.0.1"); err != nil {
			return fmt.Errorf("start web server: %w", err)
		}
	}

	if enabled {
		if err := a.startTailscale(port); err != nil {
			return fmt.Errorf("start tailscale: %w", err)
		}
		a.remoteAccessEnabled = true
		logx.Printf("[tailscale] tunnel started: %s", a.tailscaleTunnel.PublicURL())
	} else {
		a.stopTailscale()
	}
	return nil
}

// SetBeta toggles experimental features. Turning beta off immediately stops any
// running Tailscale tunnel so it cannot keep running or interfering.
func (a *App) SetBeta(enabled bool) error {
	if a.cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	a.cfg.Beta = enabled
	if !enabled {
		a.stopTailscale()
		if a.cfg.RemoteAccess.TunnelMode == "tailscale" {
			a.cfg.RemoteAccess.TunnelMode = "lan"
		}
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

// startupTailscale auto-starts the tunnel on launch when configured.
func (a *App) startupTailscale() {
	if a.cfg == nil || !a.cfg.Beta {
		return // beta features off: never run Tailscale
	}
	rc := a.cfg.RemoteAccess
	if rc.TunnelMode != "tailscale" || !rc.Enabled || rc.TailscaleKey == "" {
		return
	}
	port := rc.Port
	if a.webServer != nil && !a.webServer.IsRunning() {
		if err := a.webServer.StartHTTPWithAddr(port, "127.0.0.1"); err != nil {
			logx.Printf("[tailscale] web server start: %v", err)
			return
		}
	}
	if err := a.startTailscale(port); err != nil {
		logx.Printf("[tailscale] auto-start: %v", err)
	}
}
