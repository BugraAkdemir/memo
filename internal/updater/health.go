// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// HTTPHealth returns a health check that asks a backend's /api/version URL
// which version it is.
func HTTPHealth(versionURL string) func(ctx context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		if err := checkURL(versionURL); err != nil {
			return "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionURL, nil)
		if err != nil {
			return "", err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		var body struct {
			Version string `json:"version"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&body); err != nil {
			return "", err
		}
		return body.Version, nil
	}
}
