// SPDX-License-Identifier: AGPL-3.0-or-later

package geminisub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Code Assist is Google's endpoint that Gemini CLI uses to reach Gemini with
// a personal Google account instead of an API key. Before the first
// generateContent call it needs a bootstrap handshake — loadCodeAssist to
// discover the caller's GCP project and subscription tier, and onboardUser
// once if the account has never used Code Assist. The shapes below match
// what Gemini CLI sends; they are a real unknown until exercised live (see
// the plan's fallback notes), so every URL is overridable and the parsing is
// defensive.

const defaultCodeAssistBase = "https://cloudcode-pa.googleapis.com/v1internal"

// envEndpoint overrides the Code Assist base URL (no trailing slash). Used
// for the fallback paths in the plan and by tests.
const envEndpoint = "MEMO_GEMINI_SUB_ENDPOINT"

// clientMetadata is echoed into both bootstrap calls, identifying the caller
// the way Gemini CLI's own metadata block does.
var clientMetadata = map[string]string{
	"ideType":    "IDE_UNSPECIFIED",
	"platform":   "PLATFORM_UNSPECIFIED",
	"pluginType": "GEMINI",
}

func codeAssistBase() string {
	if v := strings.TrimRight(strings.TrimSpace(os.Getenv(envEndpoint)), "/"); v != "" {
		return v
	}
	return defaultCodeAssistBase
}

// onboardPollInterval is how long onboardUser waits between polls of the
// long-running operation. A package var so tests can shorten it.
var onboardPollInterval = 2 * time.Second

// bootstrap is the resolved result of the Code Assist handshake.
type bootstrap struct {
	ProjectID string
	TierID    string
}

type loadResponse struct {
	CloudaicompanionProject string `json:"cloudaicompanionProject"`
	CurrentTier             *struct {
		ID string `json:"id"`
	} `json:"currentTier"`
	AllowedTiers []struct {
		ID           string `json:"id"`
		IsDefault    bool   `json:"isDefault"`
		UserDefined  bool   `json:"userDefined"`
	} `json:"allowedTiers"`
}

type onboardResponse struct {
	Done     bool `json:"done"`
	Response *struct {
		CloudaicompanionProject *struct {
			ID string `json:"id"`
		} `json:"cloudaicompanionProject"`
	} `json:"response"`
}

// ensureBootstrap runs (and caches) the Code Assist handshake. Concurrent
// callers share one run via bootMu.
func (m *Manager) ensureBootstrap(ctx context.Context) (bootstrap, error) {
	m.bootMu.Lock()
	defer m.bootMu.Unlock()

	if m.boot != nil {
		return *m.boot, nil
	}

	ts, err := m.TokenSource(ctx)
	if err != nil {
		return bootstrap{}, err
	}
	hc := oauth2.NewClient(ctx, ts)

	lr, err := m.loadCodeAssist(ctx, hc)
	if err != nil {
		return bootstrap{}, err
	}

	tier := ""
	if lr.CurrentTier != nil {
		tier = lr.CurrentTier.ID
	}
	if tier == "" {
		for _, t := range lr.AllowedTiers {
			if t.IsDefault {
				tier = t.ID
				break
			}
		}
	}

	project := lr.CloudaicompanionProject
	if project == "" {
		project, err = m.onboardUser(ctx, hc, tier)
		if err != nil {
			return bootstrap{}, err
		}
	}

	b := bootstrap{ProjectID: project, TierID: tier}
	m.boot = &b
	return b, nil
}

// invalidateBootstrap drops the cached handshake (called on disconnect).
func (m *Manager) invalidateBootstrap() {
	m.bootMu.Lock()
	m.boot = nil
	m.bootMu.Unlock()
}

func (m *Manager) loadCodeAssist(ctx context.Context, hc *http.Client) (*loadResponse, error) {
	body := map[string]any{
		"metadata": clientMetadata,
	}
	var out loadResponse
	if err := postJSON(ctx, hc, codeAssistBase()+":loadCodeAssist", body, &out); err != nil {
		return nil, fmt.Errorf("geminisub: loadCodeAssist: %w", err)
	}
	return &out, nil
}

// onboardUser enrols the account in Code Assist and waits for the
// long-running operation to report a project id. Bounded retry.
func (m *Manager) onboardUser(ctx context.Context, hc *http.Client, tierID string) (string, error) {
	if tierID == "" {
		tierID = "free-tier"
	}
	body := map[string]any{
		"tierId":   tierID,
		"metadata": clientMetadata,
	}

	deadline := time.Now().Add(60 * time.Second)
	for attempt := 0; ; attempt++ {
		var out onboardResponse
		if err := postJSON(ctx, hc, codeAssistBase()+":onboardUser", body, &out); err != nil {
			return "", fmt.Errorf("geminisub: onboardUser: %w", err)
		}
		if out.Done && out.Response != nil && out.Response.CloudaicompanionProject != nil {
			id := out.Response.CloudaicompanionProject.ID
			if id != "" {
				return id, nil
			}
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("geminisub: onboardUser did not complete within 60s")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(onboardPollInterval):
		}
	}
}

func postJSON(ctx context.Context, hc *http.Client, url string, in, out any) error {
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(respBody, out)
}
