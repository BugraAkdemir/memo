package app

import (
	"testing"

	"memo/internal/config"
)

func TestGetRemoteAccessStatus_AuthWarning(t *testing.T) {
	cases := []struct {
		name        string
		enabled     bool
		mode        string
		wantWarning bool
	}{
		{"none mode while enabled warns", true, "none", true},
		{"none mode while disabled does not warn", false, "none", false},
		{"token mode never warns", true, "token", false},
		{"password mode never warns", true, "password", false},
		{"token_password mode never warns", true, "token_password", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := &App{cfg: &config.AppConfig{RemoteAccess: config.RemoteAccessConfig{
				Enabled:  c.enabled,
				AuthMode: c.mode,
			}}}
			status := a.GetRemoteAccessStatus().(RemoteAccessStatus)
			if (status.AuthWarning != "") != c.wantWarning {
				t.Errorf("AuthWarning = %q, wantWarning=%v", status.AuthWarning, c.wantWarning)
			}
		})
	}
}

// TestGetRemoteAccessStatus_TransportWarning guards Y1 (stability audit,
// deliberately deferred until now — a product decision, discussed with the
// user, to warn rather than implement TLS+client cert trust): SetRemoteAccess
// always binds the web server to plain HTTP on 0.0.0.0 whenever enabled,
// regardless of AuthMode or NgrokMode/TunnelMode (a tunnel only encrypts the
// leg it carries, never the LAN segment itself) — so TransportWarning must
// fire on Enabled alone, independent of every other field, unlike
// AuthWarning above which depends on AuthMode too.
func TestGetRemoteAccessStatus_TransportWarning(t *testing.T) {
	cases := []struct {
		name        string
		enabled     bool
		mode        string
		ngrokMode   bool
		wantWarning bool
	}{
		{"enabled, password mode, no tunnel — still warns", true, "password", false, true},
		{"enabled, token mode, no tunnel — still warns", true, "token", false, true},
		{"enabled with ngrok configured — still warns (tunnel doesn't cover the LAN segment)", true, "password", true, true},
		{"disabled — no warning", false, "password", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := &App{cfg: &config.AppConfig{RemoteAccess: config.RemoteAccessConfig{
				Enabled:   c.enabled,
				AuthMode:  c.mode,
				NgrokMode: c.ngrokMode,
			}}}
			status := a.GetRemoteAccessStatus().(RemoteAccessStatus)
			if (status.TransportWarning != "") != c.wantWarning {
				t.Errorf("TransportWarning = %q, wantWarning=%v", status.TransportWarning, c.wantWarning)
			}
		})
	}
}
