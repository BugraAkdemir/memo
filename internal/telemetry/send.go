// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// DefaultEndpoint is the public, write-only address reports go to. MEMO_TELEMETRY_URL
// replaces it for development and tests.
const DefaultEndpoint = "https://tel.memocpp.com"

// maxBody is a little under the server's 16 KiB limit.
const maxBody = 16000

// maxUsageRows is well under the schema's 200: a normal install has a handful.
const maxUsageRows = 150

// encode is the ONE place a report becomes bytes: the preview shows these exact
// bytes and the sender posts them. An oversize report (many distinct models) is
// trimmed from its least significant rows instead of being refused.
func encode(r Report) ([]byte, error) {
	if len(r.Usage) > maxUsageRows {
		r.Usage = trimUsage(r.Usage, maxUsageRows)
	}
	raw, err := json.Marshal(r)
	for err == nil && len(raw) > maxBody && len(r.Usage) > 1 {
		r.Usage = trimUsage(r.Usage, len(r.Usage)-max(1, len(r.Usage)/10))
		raw, err = json.Marshal(r)
	}
	return raw, err
}

// trimUsage keeps the n rows with the most tokens (in a stable order).
func trimUsage(rows []UsageRow, n int) []UsageRow {
	out := append([]UsageRow(nil), rows...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].InTokens+out[i].OutTokens > out[j].InTokens+out[j].OutTokens })
	if len(out) > n {
		out = out[:n]
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		return strings.Join([]string{a.Kind, a.Provider, a.Model, a.Category}, "\x00") < strings.Join([]string{b.Kind, b.Provider, b.Model, b.Category}, "\x00")
	})
	return out
}

// outcome of one POST.
type outcome int

const (
	outAccepted outcome = iota // stored (202) or already stored (200)
	outRefused                 // 4xx the client cannot fix by retrying: drop the day
	outRetry                   // server or network trouble: keep the day, try later
)

// post sends body to path. It sets exactly two headers of its own (a fixed
// user agent and the content type); nothing identifies the install except what
// is in the body.
func (m *Manager) post(ctx context.Context, path string, body []byte) (outcome, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint()+path, bytes.NewReader(body))
	if err != nil {
		return outRetry, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "memo")
	resp, err := m.client.Do(req)
	if err != nil {
		return outRetry, fmt.Errorf("telemetry: %s: %w", path, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusNoContent:
		return outAccepted, nil
	case resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusRequestEntityTooLarge || resp.StatusCode == http.StatusUnsupportedMediaType:
		return outRefused, fmt.Errorf("telemetry: %s refused (HTTP %d)", path, resp.StatusCode)
	}
	return outRetry, fmt.Errorf("telemetry: %s: HTTP %d", path, resp.StatusCode)
}

// forget asks the server to delete everything it holds for tid.
func (m *Manager) forget(ctx context.Context, tid string) error {
	raw, _ := json.Marshal(map[string]string{"tid": tid})
	out, err := m.post(ctx, "/v1/forget", raw)
	if out == outRefused { // a malformed id has nothing to forget
		return nil
	}
	return err
}
