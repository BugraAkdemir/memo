package replcli

import (
	"context"
	"net/http"
)

// RemoteStatus mirrors the fields of app.RemoteAccessStatus the REPL needs to
// start an ngrok tunnel and report its public URL.
type RemoteStatus struct {
	Running    bool   `json:"running"`
	NgrokMode  bool   `json:"ngrok_mode"`
	NgrokToken string `json:"ngrok_token"`
	NgrokURL   string `json:"ngrok_url"`
	NgrokError string `json:"ngrok_error"`
}

// RemoteAccessStatus returns the current remote-access/ngrok state.
func (c *Client) RemoteAccessStatus(ctx context.Context) (RemoteStatus, error) {
	var status RemoteStatus
	err := c.doJSON(ctx, http.MethodGet, "/api/remote-access", nil, &status)
	return status, err
}

// StartNgrok enables remote access in ngrok mode on port, using the given
// authtoken — the same request the desktop app's Remote Access settings send.
func (c *Client) StartNgrok(ctx context.Context, port int, token string) error {
	req := map[string]any{
		"enabled":     true,
		"port":        port,
		"ngrok_mode":  true,
		"ngrok_token": token,
	}
	return c.doJSON(ctx, http.MethodPut, "/api/remote-access", req, nil)
}
