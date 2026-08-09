package replcli

import (
	"context"
	"net/http"
	"time"
)

// RemoteFullStatus mirrors the fields of app.RemoteAccessStatus the `memo
// remote` CLI command (cli_remote.go) needs — a separate, fuller type from
// RemoteStatus above, which only covers what starting an ngrok tunnel needs.
type RemoteFullStatus struct {
	Enabled     bool     `json:"enabled"`
	Running     bool     `json:"running"`
	Addresses   []string `json:"addresses"`
	AuthMode    string   `json:"auth_mode"`
	Username    string   `json:"username"`
	AuthWarning string   `json:"auth_warning"`
	// Token is only ever non-empty immediately after a call that just
	// minted a fresh device token (see app.RemoteAccessStatus's identical
	// field) — empty on an ordinary status check.
	Token string `json:"token"`
}

// RemoteFullAccessStatus returns the full remote-access/auth state.
func (c *Client) RemoteFullAccessStatus(ctx context.Context) (RemoteFullStatus, error) {
	var status RemoteFullStatus
	err := c.doJSON(ctx, http.MethodGet, "/api/remote-access", nil, &status)
	return status, err
}

// RemoteDevice mirrors app.RemoteDeviceInfo.
type RemoteDevice struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
}

// ListRemoteDevices returns every paired device (never the token itself).
func (c *Client) ListRemoteDevices(ctx context.Context) ([]RemoteDevice, error) {
	var devices []RemoteDevice
	err := c.doJSON(ctx, http.MethodGet, "/api/remote-access/devices", nil, &devices)
	return devices, err
}

// AddRemoteDevice creates a new paired device and returns its plaintext
// token — the only time it's ever available.
func (c *Client) AddRemoteDevice(ctx context.Context, name string) (string, error) {
	var resp struct {
		Token string `json:"token"`
	}
	err := c.doJSON(ctx, http.MethodPost, "/api/remote-access/devices", map[string]string{"name": name}, &resp)
	return resp.Token, err
}

// RevokeRemoteDevice permanently revokes a paired device's access.
func (c *Client) RevokeRemoteDevice(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/remote-access/devices/"+id, nil, nil)
}

// SetRemoteAuthMode updates the remote-access auth mode
// (none/token/password/token_password) and, for password-backed modes, the
// username/password. An empty password leaves any existing hash untouched
// (see app.SetRemoteAuthConfig).
func (c *Client) SetRemoteAuthMode(ctx context.Context, mode, username, password string) error {
	req := map[string]string{"auth_mode": mode, "username": username, "password": password}
	return c.doJSON(ctx, http.MethodPut, "/api/remote-access", req, nil)
}

// LoginRemote logs in with username/password (only valid in "password"/
// "token_password" modes) and returns a session token on success.
func (c *Client) LoginRemote(ctx context.Context, username, password string) (string, error) {
	var resp struct {
		SessionToken string `json:"session_token"`
	}
	err := c.doJSON(ctx, http.MethodPost, "/api/auth/login", map[string]string{"username": username, "password": password}, &resp)
	return resp.SessionToken, err
}
