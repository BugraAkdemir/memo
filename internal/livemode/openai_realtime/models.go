// Package openai_realtime implements the OpenAI Realtime API engine for
// Live Mode — model discovery in this phase, the realtime WebSocket
// session client in a later phase. See docs/plans/PLAN_live_mode_v2.md.
package openai_realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/provider"
	"net/http"
	"strings"
	"time"
)

// DiscoveryBaseURL is a var (not const) so tests can point it at an
// httptest server instead of the real API.
var DiscoveryBaseURL = "https://api.openai.com/v1"

var discoveryClient = &http.Client{Timeout: 20 * time.Second}

// RealtimeModel is the subset of OpenAI's GET /v1/models response this
// package cares about.
type RealtimeModel struct {
	ID string `json:"id"`
}

// ListRealtimeModels fetches every model from OpenAI's models endpoint and
// returns only the ones whose ID identifies them as part of the Realtime
// family (current model IDs as of this writing: gpt-realtime,
// gpt-realtime-2, gpt-realtime-2.1 — all contain "realtime"). OpenAI's
// models endpoint has no dedicated capability flag for this the way
// Google's supportedGenerationMethods does, so an ID substring match is the
// best available filter without hardcoding a specific model name — this
// keeps working as OpenAI ships new realtime-family model versions,
// unlike a fixed allowlist would.
func ListRealtimeModels(ctx context.Context, apiKey string) ([]RealtimeModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DiscoveryBaseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("livemode openai: models request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := discoveryClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("livemode openai: models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("livemode openai: models status %d: %s", resp.StatusCode, provider.ExtractErrorMessage(errBody))
	}

	var result struct {
		Data []RealtimeModel `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("livemode openai: decode models: %w", err)
	}

	realtime := make([]RealtimeModel, 0, len(result.Data))
	for _, m := range result.Data {
		if strings.Contains(m.ID, "realtime") {
			realtime = append(realtime, m)
		}
	}
	return realtime, nil
}
