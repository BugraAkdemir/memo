// SPDX-License-Identifier: AGPL-3.0-or-later

package cliproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Model is one entry of the sidecar's /v1/models.
type Model struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by"`
}

type modelsMemo struct {
	at   time.Time
	list []Model
}

// modelsTTL keeps a model dropdown being opened repeatedly from hammering the
// sidecar; the list changes on the vendors' release cadence, not per second.
var modelsTTL = 30 * time.Second

// Models returns the models the signed-in accounts can use, newest-looking
// first within each owner. It needs the sidecar to be running.
func (m *Manager) Models(ctx context.Context) ([]Model, error) {
	m.modelsMu.Lock()
	if c := m.modelsMem; c != nil && time.Since(c.at) < modelsTTL {
		out := append([]Model(nil), c.list...)
		m.modelsMu.Unlock()
		return out, nil
	}
	m.modelsMu.Unlock()

	base, key := m.BaseURL(), m.APIKey()
	if base == "" {
		return nil, fmt.Errorf("cliproxy: sidecar has not been started")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cliproxy: /v1/models status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed struct {
		Data []Model `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("cliproxy: /v1/models: %w", err)
	}

	seen := map[string]bool{}
	var list []Model
	for _, md := range parsed.Data {
		if md.ID == "" || seen[md.ID] {
			continue
		}
		seen[md.ID] = true
		list = append(list, md)
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].OwnedBy != list[j].OwnedBy {
			return list[i].OwnedBy < list[j].OwnedBy
		}
		return list[i].ID > list[j].ID // gemini-3 before gemini-2.5, opus-4-6 before opus-4-5
	})

	// An empty list is NOT cached: a freshly started sidecar answers /v1/models
	// with nothing for the first ~30s while it loads the account, and caching
	// that would hold the empty answer for a full TTL after the real one exists.
	if len(list) > 0 {
		m.modelsMu.Lock()
		m.modelsMem = &modelsMemo{at: time.Now(), list: list}
		m.modelsMu.Unlock()
	}
	return append([]Model(nil), list...), nil
}

// CachedModels returns the last fetched list without touching the network
// (nil if none). The dev-gateway model listing uses it: it has no request
// context to do a live fetch.
func (m *Manager) CachedModels() []Model {
	m.modelsMu.Lock()
	defer m.modelsMu.Unlock()
	if m.modelsMem == nil {
		return nil
	}
	return append([]Model(nil), m.modelsMem.list...)
}

func (m *Manager) invalidateModels() {
	m.modelsMu.Lock()
	m.modelsMem = nil
	m.modelsMu.Unlock()
}
