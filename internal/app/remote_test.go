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
