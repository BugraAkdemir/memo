// SPDX-License-Identifier: AGPL-3.0-or-later

package geminisub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"memo/internal/logx"

	"golang.org/x/oauth2"
)

// modelsEndpoint is Google's Generative Language "list models" endpoint. A
// var so tests can point it at an httptest server. The OAuth token minted for
// Code Assist (cloud-platform scope) also authorises this call, so the model
// list is the account's real, current set rather than a hard-coded guess.
var modelsEndpoint = "https://generativelanguage.googleapis.com/v1beta/models"

// modelsCacheTTL bounds how often Models() hits Google — a model picker being
// opened repeatedly should not spam the API.
var modelsCacheTTL = time.Hour

// fallbackModels is used only when the live list can't be fetched (offline,
// API disabled, transient error) so the picker is never empty. Kept short and
// current on purpose; the live list is the real source.
var fallbackModels = []string{"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.5-flash-lite"}

type modelsCache struct {
	mu   sync.Mutex
	list []string
	at   time.Time
}

// Models returns the Gemini models this account can call generateContent on,
// fetched from Google and cached for modelsCacheTTL. On any failure it
// returns the last good list if one is cached, otherwise fallbackModels —
// it never returns an error to the caller once connected, so a flaky network
// can't empty the picker.
func (m *Manager) Models(ctx context.Context) ([]string, error) {
	if !m.Connected() {
		return nil, ErrNotConnected
	}

	m.models.mu.Lock()
	if len(m.models.list) > 0 && time.Since(m.models.at) < modelsCacheTTL {
		cached := append([]string(nil), m.models.list...)
		m.models.mu.Unlock()
		return cached, nil
	}
	m.models.mu.Unlock()

	live, err := m.fetchModels(ctx)
	if err != nil || len(live) == 0 {
		if err != nil {
			logx.Printf("geminisub: live model list unavailable (%v) — using fallback", err)
		}
		m.models.mu.Lock()
		last := append([]string(nil), m.models.list...)
		m.models.mu.Unlock()
		if len(last) > 0 {
			return last, nil
		}
		return append([]string(nil), fallbackModels...), nil
	}

	m.models.mu.Lock()
	m.models.list = live
	m.models.at = time.Now()
	m.models.mu.Unlock()
	return append([]string(nil), live...), nil
}

// CachedModels returns the last fetched model list without hitting the
// network (nil if none fetched yet). Used by the dev-gateway model listing,
// which has no request context to do a live fetch.
func (m *Manager) CachedModels() []string {
	m.models.mu.Lock()
	defer m.models.mu.Unlock()
	if len(m.models.list) == 0 {
		return nil
	}
	return append([]string(nil), m.models.list...)
}

// invalidateModels drops the cached model list (called on disconnect).
func (m *Manager) invalidateModels() {
	m.models.mu.Lock()
	m.models.list = nil
	m.models.at = time.Time{}
	m.models.mu.Unlock()
}

func (m *Manager) fetchModels(ctx context.Context) ([]string, error) {
	ts, err := m.TokenSource(ctx)
	if err != nil {
		return nil, err
	}
	hc := oauth2.NewClient(ctx, ts)

	seen := map[string]bool{}
	var out []string
	pageToken := ""
	for {
		url := modelsEndpoint + "?pageSize=1000"
		if pageToken != "" {
			url += "&pageToken=" + pageToken
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := hc.Do(req)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var parsed struct {
			Models []struct {
				Name                       string   `json:"name"`
				SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
			} `json:"models"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, err
		}

		for _, md := range parsed.Models {
			id := strings.TrimPrefix(md.Name, "models/")
			if id == "" || seen[id] {
				continue
			}
			if !contains(md.SupportedGenerationMethods, "generateContent") {
				continue
			}
			// Chat-capable Gemini/Gemma text models only — skip embeddings,
			// AQA, and anything else that slips through.
			low := strings.ToLower(id)
			if !strings.HasPrefix(low, "gemini") && !strings.HasPrefix(low, "gemma") {
				continue
			}
			if strings.Contains(low, "embedding") || strings.Contains(low, "aqa") {
				continue
			}
			seen[id] = true
			out = append(out, id)
		}

		pageToken = parsed.NextPageToken
		if pageToken == "" {
			break
		}
	}

	sort.Strings(out)
	return out, nil
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
