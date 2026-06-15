package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"

	"memo/internal/config"
	"memo/internal/ngrok"
	"memo/internal/webserver"
)

// RemoteAccessStatus holds the current remote access configuration and state.
type RemoteAccessStatus struct {
	Enabled    bool     `json:"enabled"`
	Port       int      `json:"port"`
	Running    bool     `json:"running"`
	Addresses  []string `json:"addresses"`
	Token      string   `json:"token"`
	NgrokMode  bool     `json:"ngrok_mode"`
	NgrokToken string   `json:"ngrok_token"`
	NgrokURL   string   `json:"ngrok_url"`
	NgrokError string   `json:"ngrok_error"`
}

// GetRemoteAccessStatus returns the current state of the remote access server.
func (a *App) GetRemoteAccessStatus() interface{} {
	status := RemoteAccessStatus{
		Enabled:    a.cfg.RemoteAccess.Enabled,
		Port:       a.cfg.RemoteAccess.Port,
		Token:      a.cfg.RemoteAccess.Token,
		NgrokMode:  a.cfg.RemoteAccess.NgrokMode,
		NgrokToken: a.cfg.RemoteAccess.NgrokToken,
	}
	if a.webServer != nil {
		status.Running = a.webServer.IsRunning()
		status.Addresses = a.webServer.GetAddresses()
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
	return status
}

// SetRemoteAccess enables or disables remote access and restarts the web server.
func (a *App) SetRemoteAccess(enabled bool, port int) error {
	if a.webServer == nil {
		return fmt.Errorf("server not initialized")
	}

	if enabled == a.remoteAccessEnabled && a.webServer.GetPort() == port && a.cfg.RemoteAccess.NgrokMode == (a.ngrokServer != nil) {
		return nil
	}

	a.remoteAccessEnabled = enabled
	a.cfg.RemoteAccess.Enabled = enabled
	a.cfg.RemoteAccess.Port = port

	if a.cfg.RemoteAccess.Token == "" {
		a.cfg.RemoteAccess.Token = generateToken()
	}

	if enabled && a.cfg.RemoteAccess.NgrokMode && a.cfg.RemoteAccess.NgrokToken != "" {
		if a.ngrokServer != nil {
			a.ngrokServer.Stop()
		}
		binPath, err := ngrok.Install(config.DataDir())
		if err != nil {
			log.Printf("[ngrok] Install error: %v", err)
		} else {
			mgr := ngrok.NewManager(binPath)
			if err := mgr.Start(port, a.cfg.RemoteAccess.NgrokToken); err != nil {
				log.Printf("[ngrok] Start error: %v", err)
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

	if err := a.webServer.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}
	addr := "127.0.0.1"
	if enabled {
		addr = "0.0.0.0"
	}
	return a.webServer.StartHTTPWithAddr(port, addr)
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

func generateToken() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "memo-" + hex.EncodeToString(b)
}

// GetListenAddr returns the current listen address of the web server.
func (a *App) GetListenAddr() string {
	if a.webServer == nil {
		return "127.0.0.1"
	}
	return a.webServer.GetListenAddr()
}

// SetListenAddr changes the listen address of the web server.
func (a *App) SetListenAddr(addr string) {
	if a.webServer != nil {
		a.webServer.SetListenAddr(addr)
	}
}

// SetNgrokAutoStart sets whether ngrok should start automatically on app launch.
func (a *App) SetNgrokAutoStart(autoStart bool) {
	a.cfg.RemoteAccess.NgrokAutoStart = autoStart
	if err := config.Save(a.cfg); err != nil {
		log.Printf("WARN: save config: %v", err)
	}
}

// startWebServerForRemote is the internal helper used during startup for TLS remote access.
func (a *App) startWebServerForRemote(port int) {
	if a.webServer == nil {
		a.webServer = webserver.New(a)
	}
	if err := a.webServer.Start(port); err != nil {
		log.Printf("Remote access server: %v", err)
	}
}
