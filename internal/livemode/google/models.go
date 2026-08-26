// Package google implements the Google Live (Gemini Live API) engine for
// Live Mode — model discovery in this phase, the realtime WebSocket
// session client in a later phase. See docs/plans/PLAN_live_mode_v2.md.
package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/provider"
	"net/http"
	"time"
)

// DiscoveryBaseURL is a var (not const) so tests can point it at an
// httptest server instead of the real API.
var DiscoveryBaseURL = "https://generativelanguage.googleapis.com/v1beta"

var discoveryClient = &http.Client{Timeout: 20 * time.Second}

// bidiGenerateContentMethod is the generation method Google's models.list
// response uses to mark a model as Live-API-capable (confirmed against
// current API docs, 2026-08-26) — this is the filter, never a hardcoded
// model ID, per docs/plans/PLAN_live_mode_v2.md's "never hardcode a model
// list" requirement.
const bidiGenerateContentMethod = "bidiGenerateContent"

// LiveModel is the subset of Google's models.list response this package
// cares about.
type LiveModel struct {
	// Name is the full resource name, e.g. "models/gemini-3.1-flash-live-preview"
	// — this is exactly the string the Live API's setup.model field expects.
	Name                       string   `json:"name"`
	DisplayName                string   `json:"displayName"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
}

// ListLiveModels fetches every model from Google's models.list endpoint and
// returns only the ones advertising bidiGenerateContent support (i.e.
// actually usable with the Live API) — callers must not assume every
// returned Gemini model works with a realtime session.
func ListLiveModels(ctx context.Context, apiKey string) ([]LiveModel, error) {
	url := fmt.Sprintf("%s/models?key=%s", DiscoveryBaseURL, apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("livemode google: models request: %w", err)
	}

	resp, err := discoveryClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("livemode google: models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("livemode google: models status %d: %s", resp.StatusCode, provider.ExtractErrorMessage(errBody))
	}

	var result struct {
		Models []LiveModel `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("livemode google: decode models: %w", err)
	}

	live := make([]LiveModel, 0, len(result.Models))
	for _, m := range result.Models {
		for _, method := range m.SupportedGenerationMethods {
			if method == bidiGenerateContentMethod {
				live = append(live, m)
				break
			}
		}
	}
	return live, nil
}
