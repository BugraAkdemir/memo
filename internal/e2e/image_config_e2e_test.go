// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"net/http"
	"testing"
)

type imageConfigJSON struct {
	AutoRoute       bool   `json:"auto_route"`
	DefaultProvider string `json:"default_provider"`
	DefaultModel    string `json:"default_model"`
}

func TestImageConfig_ReadChangeAndRefuseNonsense(t *testing.T) {
	h := NewHarness(t)

	var got imageConfigJSON
	decodeInto(t, h.getJSON("/api/image/config"), &got)
	if !got.AutoRoute || got.DefaultProvider != "" || got.DefaultModel != "" {
		t.Fatalf("default config = %+v, want auto-routing on and no default image model", got)
	}

	// Set the default image model (the harness's provider is named e2e-fake).
	resp := h.putJSON("/api/image/config", map[string]any{"default_provider": "e2e-fake", "default_model": "img-1"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT = %d", resp.StatusCode)
	}
	decodeInto(t, resp, &got)
	if got.DefaultProvider != "e2e-fake" || got.DefaultModel != "img-1" || !got.AutoRoute {
		t.Fatalf("after PUT = %+v", got)
	}

	// A partial update keeps the rest.
	resp = h.putJSON("/api/image/config", map[string]any{"auto_route": false})
	decodeInto(t, resp, &got)
	if got.AutoRoute || got.DefaultProvider != "e2e-fake" || got.DefaultModel != "img-1" {
		t.Fatalf("after toggling auto_route = %+v, want the default model kept", got)
	}
	if cfg := h.App.GetImageConfig(); cfg.AutoRoute || cfg.DefaultModel != "img-1" {
		t.Errorf("app config = %+v", cfg)
	}

	// Nonsense is refused, and nothing changes.
	for _, body := range []map[string]any{
		{"default_provider": "no-such-provider", "default_model": "x"},
		{"default_provider": "e2e-fake", "default_model": ""},
		{"default_provider": "", "default_model": "img-1"},
	} {
		r := h.putJSON("/api/image/config", body)
		if r.StatusCode != http.StatusBadRequest {
			t.Errorf("PUT %v = %d, want 400", body, r.StatusCode)
		}
		r.Body.Close()
	}
	if cfg := h.App.GetImageConfig(); cfg.DefaultProvider != "e2e-fake" || cfg.DefaultModel != "img-1" {
		t.Errorf("a refused update changed the config: %+v", cfg)
	}

	// Clearing both is allowed.
	resp = h.putJSON("/api/image/config", map[string]any{"default_provider": "", "default_model": ""})
	decodeInto(t, resp, &got)
	if got.DefaultProvider != "" || got.DefaultModel != "" {
		t.Errorf("cleared config = %+v", got)
	}
}
