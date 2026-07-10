package replcli

import (
	"context"
	"net/http"
	"time"
)

// clientHeartbeatInterval matches the cadence internal/app/clients.go's
// clientStaleAfter (90s) tolerates about three missed beats of.
const clientHeartbeatInterval = 25 * time.Second

// heartbeatLoop keeps this session's client registration alive on the
// backend until ctx is cancelled (Run returning). Errors are ignored — a
// missed heartbeat or two is exactly what the backend's staleness window
// tolerates, and a failure here must never interrupt the chat itself.
func heartbeatLoop(ctx context.Context, client *Client, clientID string) {
	ticker := time.NewTicker(clientHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = client.HeartbeatClient(ctx, clientID)
		}
	}
}

// RegisterClient attaches this CLI session to the backend's client registry
// (internal/app/clients.go) and returns a client ID to pass to
// HeartbeatClient/UnregisterClient. Only meaningful for a backend that has
// auto-shutdown armed (one main.go spawned for this session) — a
// standalone `--headless` backend still accepts and tracks the
// registration, it just never acts on it.
func (c *Client) RegisterClient(ctx context.Context) (string, error) {
	var resp struct {
		ClientID string `json:"client_id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/clients/register", nil, &resp); err != nil {
		return "", err
	}
	return resp.ClientID, nil
}

// HeartbeatClient refreshes clientID's last-seen time on the backend.
func (c *Client) HeartbeatClient(ctx context.Context, clientID string) error {
	return c.doJSON(ctx, http.MethodPost, "/api/clients/heartbeat", map[string]string{"client_id": clientID}, nil)
}

// UnregisterClient tells the backend this CLI session is going away — its
// graceful goodbye, so a backend with nothing else attached can shut down
// promptly instead of waiting out the heartbeat staleness timeout.
func (c *Client) UnregisterClient(ctx context.Context, clientID string) error {
	return c.doJSON(ctx, http.MethodPost, "/api/clients/unregister", map[string]string{"client_id": clientID}, nil)
}
