package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// GetVersion returns the application version string.
func (a *App) GetVersion() string {
	return strings.TrimSpace(a.version)
}

// CheckLatestVersion checks the remote version at version-zeta.vercel.app/version.json
// and returns the latest version string if newer, or empty string if up-to-date.
func (a *App) CheckLatestVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://version-zeta.vercel.app/version.json", nil)
	if err != nil {
		return "", fmt.Errorf("version check request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("version check http: %w", err)
	}
	defer resp.Body.Close()

	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("version check decode: %w", err)
	}

	current := a.GetVersion()
	latest := strings.TrimSpace(body.Version)

	// Compare versions — if latest is different and not empty, notify
	if latest == "" || latest == current {
		return "", nil
	}
	return latest, nil
}
