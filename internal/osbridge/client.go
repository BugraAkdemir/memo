package osbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

const DefaultSocketPath = "/run/korteksos/sysd.sock"

// ActionRequest mirrors memo-sysd's guarded execution payload.
type ActionRequest struct {
	Tool         string            `json:"tool"`
	Action       string            `json:"action"`
	TargetSubvol string            `json:"target_subvol,omitempty"`
	Args         map[string]string `json:"args,omitempty"`
	Approved     bool              `json:"approved,omitempty"`
}

// ActionResult captures the pre-snapshot-guarded result returned by memo-sysd.
type ActionResult struct {
	Status        string `json:"status"`
	Ring          int    `json:"ring"`
	SnapshotID    string `json:"snapshot_id,omitempty"`
	Output        string `json:"output,omitempty"`
	Reason        string `json:"reason,omitempty"`
	UndoAvailable bool   `json:"undo_available"`
}

// Client communicates with the Ring-1 memo-sysd root daemon over Unix Domain Socket.
type Client struct {
	SocketPath string
	HTTP       *http.Client
}

// NewClient creates an osbridge client pointing to socketPath (or DefaultSocketPath).
func NewClient(socketPath string) *Client {
	if socketPath == "" {
		if env := os.Getenv("KORTEKS_SYSD_SOCKET"); env != "" {
			socketPath = env
		} else {
			socketPath = DefaultSocketPath
		}
	}
	return &Client{
		SocketPath: socketPath,
		HTTP: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
				},
			},
		},
	}
}

// Execute sends an OS-level action to memo-sysd.
func (c *Client) Execute(ctx context.Context, req ActionRequest) (*ActionResult, error) {
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix/v1/execute", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("memo-sysd unreachable at %s: %w", c.SocketPath, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("memo-sysd error (%d): %s", resp.StatusCode, string(body))
	}
	var out ActionResult
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode memo-sysd response: %w", err)
	}
	return &out, nil
}

// Health returns memo-sysd health and detected APU/GPU boost telemetry.
func (c *Client) Health(ctx context.Context) (string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/v1/health", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("memo-sysd health failed (%d): %s", resp.StatusCode, string(body))
	}
	return string(body), nil
}

// Rollback requests an immediate Btrfs rollback via memo-sysd.
func (c *Client) Rollback(ctx context.Context, snapshotID string) (string, error) {
	raw, _ := json.Marshal(map[string]string{"snapshot_id": snapshotID})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix/v1/rollback", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("memo-sysd rollback failed (%d): %s", resp.StatusCode, string(body))
	}
	return string(body), nil
}

// SetMode switches the active KorteksOS authority mode ("locked", "hybrid", "autonomous") via memo-sysd.
func (c *Client) SetMode(ctx context.Context, mode string) (string, error) {
	raw, _ := json.Marshal(map[string]string{"mode": mode})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix/v1/mode", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("memo-sysd set mode failed (%d): %s", resp.StatusCode, string(body))
	}
	return string(body), nil
}
