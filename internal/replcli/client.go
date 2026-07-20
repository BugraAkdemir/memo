package replcli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"memo/internal/api"
)

// Client is a minimal HTTP client for the subset of the Memo REST API the
// terminal REPL needs. It talks to the exact same endpoints the Flutter
// desktop app uses — no new backend surface.
type Client struct {
	baseURL    string
	httpClient *http.Client // short timeout, for plain JSON calls
	streamHTTP *http.Client // no timeout — SSE responses can run for minutes
	longOpHTTP *http.Client // no timeout — long-running ops (model load) bounded by the caller's context instead
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		streamHTTP: &http.Client{}, // caller controls duration via ctx
		longOpHTTP: &http.Client{}, // caller controls duration via ctx
	}
}

// doJSON makes a plain request/response JSON call with the default 10s
// timeout — the right choice for anything the backend answers quickly.
func (c *Client) doJSON(ctx context.Context, method, path string, reqBody, respBody any) error {
	return c.doJSONWith(ctx, c.httpClient, method, path, reqBody, respBody)
}

// doJSONWith is doJSON with the HTTP client made explicit, so a call whose
// backend handler can legitimately run far longer than 10s (model loading)
// can use a client with no fixed timeout and rely purely on ctx instead.
func (c *Client) doJSONWith(ctx context.Context, httpClient *http.Client, method, path string, reqBody, respBody any) error {
	var buf bytes.Buffer
	if reqBody != nil {
		if err := json.NewEncoder(&buf).Encode(reqBody); err != nil {
			return err
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s: %s (%s)", method, path, resp.Status, strings.TrimSpace(string(body)))
	}

	if respBody != nil {
		return json.NewDecoder(resp.Body).Decode(respBody)
	}
	return nil
}

// Status checks whether a Memo backend is reachable at baseURL.
func (c *Client) Status(ctx context.Context) error {
	return c.doJSON(ctx, http.MethodGet, "/api/status", nil, nil)
}

// CheckVersionUpdate reports the latest available version, or "" if this
// build is already current. Mirrors the Flutter GUI's own update check
// (GET /api/version/check, already wired to app.CheckLatestVersion — an
// external network call server-side, capped at 10s there). The caller
// should pass a short-deadline ctx: this is a best-effort startup nicety,
// not something worth stalling the welcome panel over on a slow or absent
// connection.
func (c *Client) CheckVersionUpdate(ctx context.Context) (latest string, err error) {
	var resp struct {
		Latest string `json:"latest"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/version/check", nil, &resp); err != nil {
		return "", err
	}
	return resp.Latest, nil
}

// GetUILanguage returns the GUI's last-known display language ("tr"/"en",
// or "" if never set) so the CLI's own text can follow it — see
// internal/config's Identity.UILanguage and replcli's SetLanguage.
func (c *Client) GetUILanguage(ctx context.Context) (string, error) {
	var resp struct {
		Language string `json:"language"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/system-prompt/ui-language", nil, &resp); err != nil {
		return "", err
	}
	return resp.Language, nil
}

// Version returns the connected backend's version string (e.g. "3.1.2"),
// shown in the welcome panel's title line.
func (c *Client) Version(ctx context.Context) (string, error) {
	var resp struct {
		Version string `json:"version"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/version", nil, &resp); err != nil {
		return "", err
	}
	return resp.Version, nil
}

// NewAgentChat creates a fresh agent-mode chat rooted at projectPath and
// returns its chat ID.
func (c *Client) NewAgentChat(ctx context.Context, projectPath string) (string, error) {
	var resp struct {
		ID string `json:"id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/agent/chat",
		map[string]string{"project_path": projectPath}, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SwitchChat makes id the backend's active chat.
func (c *Client) SwitchChat(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodPost, "/api/chats/switch", map[string]string{"id": id}, nil)
}

// SetAgentEnabled turns agent (tool-use) mode on or off for the active chat.
func (c *Client) SetAgentEnabled(ctx context.Context, enabled bool) error {
	return c.doJSON(ctx, http.MethodPut, "/api/agent/enabled", map[string]bool{"enabled": enabled}, nil)
}

// SendPermission answers a pending tool permission request with policy
// "allow_once" or "deny_once".
func (c *Client) SendPermission(ctx context.Context, requestID, policy string) error {
	return c.doJSON(ctx, http.MethodPost, "/api/agent/permission",
		map[string]string{"request_id": requestID, "policy": policy}, nil)
}

// SendStream POSTs message to /api/send/stream and invokes onChunk once per
// parsed SSE chunk, in order. It stops when the backend sends done:true, when
// onChunk returns an error, or when ctx is cancelled.
func (c *Client) SendStream(ctx context.Context, message string, onChunk func(api.StreamChunk) error) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(map[string]string{"message": message}); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/send/stream", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.streamHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST /api/send/stream: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		chunk, ok := ParseSSELine(scanner.Text())
		if !ok {
			continue
		}
		if err := onChunk(chunk); err != nil {
			return err
		}
		if chunk.Done {
			return nil
		}
	}
	return scanner.Err()
}
