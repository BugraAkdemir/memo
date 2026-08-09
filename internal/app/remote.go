package app

import (
	"fmt"
	"memo/internal/logx"

	"memo/internal/config"
	"memo/internal/ngrok"
	"memo/internal/webserver"
)

// RemoteAccessStatus holds the current remote access configuration and state.
type RemoteAccessStatus struct {
	Enabled   bool     `json:"enabled"`
	Port      int      `json:"port"`
	Running   bool     `json:"running"`
	Addresses []string `json:"addresses"`
	// Token is populated only immediately after a brand new device token
	// was minted by this exact call (see SetRemoteAccess's auto-provision
	// of a first device, and pendingDeviceToken) — empty on every other
	// GET. This is the caller's one and only chance to learn a freshly
	// generated plaintext token; only a hash is ever persisted afterward.
	Token    string `json:"token"`
	AuthMode string `json:"auth_mode"`
	Username string `json:"username"`
	// AuthWarning is non-empty exactly when AuthMode is "none" and remote
	// access is enabled — see the assignment at the bottom of
	// GetRemoteAccessStatus for why this is computed once here rather than
	// left to every caller to re-derive.
	AuthWarning string `json:"auth_warning,omitempty"`
	NgrokMode   bool   `json:"ngrok_mode"`
	NgrokToken  string `json:"ngrok_token"`
	NgrokURL    string `json:"ngrok_url"`
	NgrokError  string `json:"ngrok_error"`

	TunnelMode        string `json:"tunnel_mode"`
	TailscaleHostname string `json:"tailscale_hostname"`
	TailscaleFunnel   bool   `json:"tailscale_funnel"`
	TailscaleURL      string `json:"tailscale_url"`
	TailscaleIP       string `json:"tailscale_ip"`
	TailscaleError    string `json:"tailscale_error"`
	TailscaleRunning  bool   `json:"tailscale_running"`
	TailscaleAuthURL  string `json:"tailscale_auth_url"` // set while an interactive login awaits browser approval

	Beta bool `json:"beta"`
}

// GetRemoteAccessStatus returns the current state of the remote access server.
func (a *App) GetRemoteAccessStatus() interface{} {
	a.remoteDevicesMu.Lock()
	pendingToken := a.pendingDeviceToken
	a.pendingDeviceToken = ""
	a.remoteDevicesMu.Unlock()

	status := RemoteAccessStatus{
		Enabled:           a.cfg.RemoteAccess.Enabled,
		Port:              a.cfg.RemoteAccess.Port,
		Token:             pendingToken,
		AuthMode:          a.cfg.RemoteAccess.AuthMode,
		Username:          a.cfg.RemoteAccess.Username,
		NgrokMode:         a.cfg.RemoteAccess.NgrokMode,
		NgrokToken:        a.cfg.RemoteAccess.NgrokToken,
		TunnelMode:        a.cfg.RemoteAccess.TunnelMode,
		TailscaleHostname: a.cfg.RemoteAccess.TailscaleHostname,
		TailscaleFunnel:   a.cfg.RemoteAccess.TailscaleFunnel,
		Beta:              a.cfg.Beta,
	}
	if ws := a.getWebServer(); ws != nil {
		status.Running = ws.IsRunning()
		status.Addresses = ws.GetAddresses()
	}
	if a.ngrokServer != nil {
		if a.ngrokServer.IsRunning() {
			if url := a.ngrokServer.PublicURL(); url != "" {
				status.NgrokURL = url
				status.Addresses = append(status.Addresses, url)
			}
		}
		if err := a.ngrokServer.LastError(); err != "" {
			status.NgrokError = err
		}
	}
	if a.tailscaleTunnel != nil {
		status.TailscaleRunning = a.tailscaleTunnel.IsRunning()
		if url := a.tailscaleTunnel.PublicURL(); url != "" {
			status.TailscaleURL = url
			status.Addresses = append(status.Addresses, url)
		}
		if ip := a.tailscaleTunnel.IPURL(); ip != "" {
			status.TailscaleIP = ip
			status.Addresses = append(status.Addresses, ip)
		}
		if err := a.tailscaleTunnel.LastError(); err != "" {
			status.TailscaleError = err
		}
		status.TailscaleAuthURL = a.tailscaleTunnel.AuthURL()
	}
	if status.Enabled && status.AuthMode == "none" {
		// Surfaced here (not just the --lan CLI startup log) so any
		// client polling this status — Settings' Remote Access tab in
		// particular — can render an impossible-to-miss warning without
		// each one having to duplicate "AuthMode == none" as its own
		// trigger condition.
		status.AuthWarning = "AUTH DISABLED — this server accepts requests from this network/tunnel with no credential at all."
	}
	return status
}

// SetRemoteAccess enables or disables remote access and restarts the web server.
func (a *App) SetRemoteAccess(enabled bool, port int) error {
	ws := a.getWebServer()
	if ws == nil {
		return fmt.Errorf("server not initialized")
	}

	// Skip no-op only when listen state already matches the request. If remote
	// is "enabled" but still bound to 127.0.0.1 (failed rebind, race with
	// Shutdown), fall through and try again — Swarm LAN join depends on this.
	if enabled == a.remoteAccessEnabled && ws.GetPort() == port && a.cfg.RemoteAccess.NgrokMode == (a.ngrokServer != nil) {
		if !enabled || ws.GetListenAddr() == "0.0.0.0" {
			return nil
		}
	}

	a.remoteAccessEnabled = enabled
	a.cfg.RemoteAccess.Enabled = enabled
	a.cfg.RemoteAccess.Port = port

	// Auto-provision a first device the same way the old single shared
	// token used to be generated on first enable — but only when the
	// configured mode actually checks device tokens; auto-minting one
	// under "password" mode (where it's never even checked) would be
	// misleading. CreateRemoteDevice persists its own save; the plaintext
	// is stashed for GetRemoteAccessStatus's next call to surface exactly
	// once (see RemoteAccessStatus.Token's doc comment), matching the
	// pre-Faz-2 "PUT is the caller's one chance to learn the token" flow.
	mode := a.cfg.RemoteAccess.AuthMode
	if enabled && len(a.cfg.RemoteAccess.Devices) == 0 && (mode == "token" || mode == "token_password") {
		if tok, err := a.CreateRemoteDevice("Varsayılan"); err != nil {
			logx.Printf("remote_auth: failed to auto-provision default device: %v", err)
		} else {
			a.remoteDevicesMu.Lock()
			a.pendingDeviceToken = tok
			a.remoteDevicesMu.Unlock()
		}
	}

	if enabled && a.cfg.RemoteAccess.NgrokMode && a.cfg.RemoteAccess.NgrokToken != "" {
		if a.ngrokServer != nil {
			a.ngrokServer.Stop()
		}
		binPath, err := ngrok.Install(config.DataDir())
		if err != nil {
			logx.Printf("[ngrok] Install error: %v", err)
		} else {
			mgr := ngrok.NewManager(binPath)
			if err := mgr.Start(port, a.cfg.RemoteAccess.NgrokToken); err != nil {
				logx.Printf("[ngrok] Start error: %v", err)
			} else {
				a.ngrokServer = mgr
			}
		}
	} else if !enabled && a.ngrokServer != nil {
		a.ngrokServer.Stop()
		a.ngrokServer = nil
	}

	if err := config.Save(a.cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	if err := ws.Stop(); err != nil {
		logx.Printf("Error stopping server: %v", err)
	}
	addr := "127.0.0.1"
	if enabled {
		addr = "0.0.0.0"
	}
	return ws.StartHTTPWithAddr(port, addr)
}

// SetNgrokMode configures ngrok tunnelling and optionally restarts remote access.
func (a *App) SetNgrokMode(enabled bool, port int, ngrokToken string) error {
	a.cfg.RemoteAccess.NgrokMode = enabled
	if ngrokToken != "" {
		a.cfg.RemoteAccess.NgrokToken = ngrokToken
	}

	if !enabled && a.ngrokServer != nil {
		a.ngrokServer.Stop()
		a.ngrokServer = nil
	}

	return a.SetRemoteAccess(enabled, port)
}

// GetListenAddr returns the current listen address of the web server.
func (a *App) GetListenAddr() string {
	if ws := a.getWebServer(); ws != nil {
		return ws.GetListenAddr()
	}
	return "127.0.0.1"
}

// SetListenAddr changes the listen address of the web server.
func (a *App) SetListenAddr(addr string) {
	if ws := a.getWebServer(); ws != nil {
		ws.SetListenAddr(addr)
	}
}

// SetNgrokAutoStart sets whether ngrok should start automatically on app launch.
func (a *App) SetNgrokAutoStart(autoStart bool) {
	a.cfg.RemoteAccess.NgrokAutoStart = autoStart
	if err := config.Save(a.cfg); err != nil {
		logx.Printf("WARN: save config: %v", err)
	}
}

// startWebServerForRemote is the internal helper used during startup for TLS remote access.
func (a *App) startWebServerForRemote(port int) {
	ws := a.getWebServer()
	if ws == nil {
		ws = webserver.New(a)
		a.setWebServer(ws)
	}
	if err := ws.Start(port); err != nil {
		logx.Printf("Remote access server: %v", err)
	}
}
