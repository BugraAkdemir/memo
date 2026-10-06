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
		ID        string `json:"id"`
		IsDefault bool   `json:"isDefault"`
		// UserDefinedProject is set on tiers that need the caller's OWN Google
		// Cloud project (Code Assist Standard/Enterprise) — Google will not
		// create one for them.
		UserDefinedProject bool `json:"userDefinedCloudaicompanionProject"`
	} `json:"allowedTiers"`
	// IneligibleTiers explains why a tier the account might expect is closed.
	IneligibleTiers []struct {
		ReasonCode    string `json:"reasonCode"`
		ReasonMessage string `json:"reasonMessage"`
		TierID        string `json:"tierId"`
	} `json:"ineligibleTiers"`
}

// envProject names a Google Cloud project to use, exactly as gemini-cli reads
// it. Needed for paid Code Assist tiers, which do not get a project made for
// them.
func envProject() string {
	for _, k := range []string{"GOOGLE_CLOUD_PROJECT", "GOOGLE_CLOUD_PROJECT_ID"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

// ineligibleReason is Google's own explanation for a closed tier, or "".
func (l *loadResponse) ineligibleReason() string {
	var parts []string
	for _, t := range l.IneligibleTiers {
		if t.ReasonMessage != "" {
			parts = append(parts, t.ReasonMessage)
		}
	}
	return strings.Join(parts, " ")
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
	needsOwnProject := false
	if lr.CurrentTier != nil {
		tier = lr.CurrentTier.ID
	}
	for _, t := range lr.AllowedTiers {
		if (tier == "" && t.IsDefault) || (tier != "" && t.ID == tier) {
			tier = t.ID
			needsOwnProject = t.UserDefinedProject
			break
		}
	}

	project := lr.CloudaicompanionProject
	if project == "" {
		// A tier that wants the caller's own Cloud project cannot be
		// onboarded without one — calling onboardUser anyway only ever ended
		// in a project-less "done" that told the user nothing. Say what
		// Google said instead.
		own := envProject()
		if needsOwnProject && own == "" {
			return bootstrap{}, fmt.Errorf("geminisub: Google will not serve this account through the Gemini sign-in: %s", lr.noProjectExplanation(tier))
		}
		project, err = m.onboardUser(ctx, hc, tier, own)
		if err != nil {
			if reason := lr.ineligibleReason(); reason != "" {
				return bootstrap{}, fmt.Errorf("%w (Google says: %s)", err, reason)
			}
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

// noProjectExplanation builds the message for an account whose only open
// tier needs a Cloud project the user has not supplied.
func (l *loadResponse) noProjectExplanation(tier string) string {
	msg := "the only tier open to it (" + tier + ") needs your own Google Cloud project — set GOOGLE_CLOUD_PROJECT to one with the Gemini for Google Cloud API enabled"
	if reason := l.ineligibleReason(); reason != "" {
		msg = reason + " Also: " + msg
	}
	return msg
}

func (m *Manager) loadCodeAssist(ctx context.Context, hc *http.Client) (*loadResponse, error) {
	body := map[string]any{
		"metadata": clientMetadata,
	}
	if p := envProject(); p != "" {
		body["cloudaicompanionProject"] = p
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
func (m *Manager) onboardUser(ctx context.Context, hc *http.Client, tierID, project string) (string, error) {
	if tierID == "" {
		tierID = "free-tier"
	}
	body := map[string]any{
		"tierId":   tierID,
		"metadata": clientMetadata,
	}
	// Free tier gets a project made for it; every other tier names its own
	// (gemini-cli sends it twice, as cloudaicompanionProject and duetProject).
	if project != "" && tierID != "free-tier" {
		body["cloudaicompanionProject"] = project
		body["metadata"] = map[string]string{
			"ideType":     clientMetadata["ideType"],
			"platform":    clientMetadata["platform"],
			"pluginType":  clientMetadata["pluginType"],
			"duetProject": project,
		}
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
	if project != "" && tierID != "free-tier" {
		// gemini-cli does the same: a paid tier answers without echoing the
		// project it was given.
		return project, nil
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
