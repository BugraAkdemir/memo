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
// long-running operation (gemini-cli waits 5s). A package var so tests can
// shorten it.
var onboardPollInterval = 5 * time.Second

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
		ID          string `json:"id"`
		IsDefault   bool   `json:"isDefault"`
		UserDefined bool   `json:"userDefined"`
	} `json:"allowedTiers"`
}

type onboardResponse struct {
	// Name is the long-running operation's resource name ("operations/..."),
	// polled with a GET until Done. Absent when the call finished inline.
	Name     string `json:"name"`
	Done     bool   `json:"done"`
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
// long-running operation to report a project id.
//
// onboardUser is sent exactly ONCE. If it answers with an unfinished
// operation, that operation is polled with GET <base>/<name> — the way
// gemini-cli does. Re-POSTing onboardUser to "poll" (what this used to do,
// every 2s) re-submits the enrolment each time and ends in a 429
// RESOURCE_EXHAUSTED before the first one has had time to finish.
func (m *Manager) onboardUser(ctx context.Context, hc *http.Client, tierID string) (string, error) {
	if tierID == "" {
		tierID = "free-tier"
	}
	body := map[string]any{
		"tierId":   tierID,
		"metadata": clientMetadata,
	}

	var out onboardResponse
	if err := postJSON(ctx, hc, codeAssistBase()+":onboardUser", body, &out); err != nil {
		return "", fmt.Errorf("geminisub: onboardUser: %w", err)
	}

	opName := out.Name
	deadline := time.Now().Add(90 * time.Second)
	for !out.Done {
		if opName == "" {
			// Neither done nor pollable: nothing to wait for.
			return "", fmt.Errorf("geminisub: onboardUser returned an unfinished operation with no name")
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("geminisub: onboardUser did not complete within 90s")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(onboardPollInterval):
		}
		out = onboardResponse{}
		if err := getJSON(ctx, hc, codeAssistBase()+"/"+strings.TrimLeft(opName, "/"), &out); err != nil {
			return "", fmt.Errorf("geminisub: onboardUser: poll operation: %w", err)
		}
	}

	if out.Response != nil && out.Response.CloudaicompanionProject != nil {
		if id := out.Response.CloudaicompanionProject.ID; id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("geminisub: onboardUser finished without a project id")
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
	return doJSON(hc, req, out)
}

func getJSON(ctx context.Context, hc *http.Client, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	return doJSON(hc, req, out)
}

func doJSON(hc *http.Client, req *http.Request, out any) error {
	req.Header.Set("User-Agent", userAgent)

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
