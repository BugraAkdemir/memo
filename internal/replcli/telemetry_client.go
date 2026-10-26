// SPDX-License-Identifier: AGPL-3.0-or-later

package replcli

import "context"

// TelemetryStatus mirrors GET /api/telemetry/status — the person's answer about
// the optional anonymous daily summary (docs/TELEMETRY.md).
type TelemetryStatus struct {
	Enabled     bool   `json:"enabled"`
	NeedsAsk    bool   `json:"needs_ask"`
	Consent     string `json:"consent"` // "", "yes", "no"
	EnvOff      bool   `json:"env_off"` // MEMO_TELEMETRY=off
	Build       string `json:"build"`
	Endpoint    string `json:"endpoint"`
	DeviceID    string `json:"device_id"`
	ConsentDay  string `json:"consent_day"`
	LastSentDay string `json:"last_sent_day"`
}

// TelemetryPreview is the report that would be sent next, as the exact bytes.
type TelemetryPreview struct {
	Day   string `json:"day"`
	Empty bool   `json:"empty"`
	JSON  string `json:"json"`
}

func (c *Client) GetTelemetryStatus(ctx context.Context) (TelemetryStatus, error) {
	var st TelemetryStatus
	err := c.doJSON(ctx, "GET", "/api/telemetry/status", nil, &st)
	return st, err
}

func (c *Client) GetTelemetryPreview(ctx context.Context) (TelemetryPreview, error) {
	var p TelemetryPreview
	err := c.doJSON(ctx, "GET", "/api/telemetry/preview", nil, &p)
	return p, err
}

// SetTelemetryConsent records an explicit yes (share) or no.
func (c *Client) SetTelemetryConsent(ctx context.Context, share bool) (TelemetryStatus, error) {
	var st TelemetryStatus
	err := c.doJSON(ctx, "POST", "/api/telemetry/consent", map[string]bool{"share": share}, &st)
	return st, err
}

// ResetTelemetryID gives the install a new random id (the old one is forgotten server-side).
func (c *Client) ResetTelemetryID(ctx context.Context) error {
	return c.doJSON(ctx, "POST", "/api/telemetry/reset-id", struct{}{}, nil)
}

// DeleteTelemetryData deletes this install's data on the server and stops sharing.
func (c *Client) DeleteTelemetryData(ctx context.Context) error {
	return c.doJSON(ctx, "POST", "/api/telemetry/delete", struct{}{}, nil)
}
