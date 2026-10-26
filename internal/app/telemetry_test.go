// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/provider"
	"memo/internal/stats"
	"memo/internal/telemetry"
)

func newTelemetryApp(t *testing.T) *App {
	t.Helper()
	st, err := stats.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	cfgMgr := provider.NewConfigManager(filepath.Join(t.TempDir(), "providers.json"), nil)
	_ = cfgMgr.Set(provider.ProviderConfig{Type: provider.ProviderOpenAI, Name: "My Secret OpenAI Account", Model: "gpt-5.5", Enabled: true})
	_ = cfgMgr.Set(provider.ProviderConfig{Type: provider.ProviderCustom, Name: subsProviderName, Model: "claude-sonnet-5-5", Enabled: true})
	_ = cfgMgr.Set(provider.ProviderConfig{Type: provider.ProviderCustom, Name: "work gateway", BaseURL: "http://10.1.2.3:8000/v1", Model: "internal-model", Enabled: true})
	cfg := config.Default()
	cfg.Memory.MemoryEnabled = true
	cfg.Sync.Enabled = true
	cfg.WhatsApp.Enabled = false
	cfg.LiveMode.Enabled = false
	return &App{statsStore: st, providerCfgMgr: cfgMgr, cfg: cfg, identity: identity.New("Test", "Memo", "casual", "", false), version: "5.0.0"}
}

func TestTelemetrySources_ProviderOfSendsTheTypeNeverThePersonsName(t *testing.T) {
	a := newTelemetryApp(t)
	s := &telemetrySources{a: a}
	cases := []struct{ name, model, kind, prov string }{
		{"local", "x.gguf", "local", "local"},
		{"", "x.gguf", "local", "local"},
		{"orchestra", "m", "api", "orchestra"},
		{"My Secret OpenAI Account", "gpt-5.5", "api", "openai"},
		{subsProviderName, "claude-sonnet-5-5", "sub", "claude"},
		{subsProviderName, "gpt-5.5-codex", "sub", "codex"},
		{subsProviderName, "gemini-3.8-flash", "sub", "antigravity"},
		{"work gateway", "internal-model", "api", "custom"},
		{"a provider that was deleted", "m", "api", "other"},
	}
	for _, c := range cases {
		kind, prov := s.ProviderOf(c.name, c.model)
		if kind != c.kind || prov != c.prov {
			t.Errorf("ProviderOf(%q, %q) = %s/%s, want %s/%s", c.name, c.model, kind, prov, c.kind, c.prov)
		}
	}
}

func TestTelemetrySources_ActiveFeaturesFollowTheConfig(t *testing.T) {
	a := newTelemetryApp(t)
	s := &telemetrySources{a: a}
	if got := strings.Join(s.ActiveFeatures(), ","); got != "memory,cloud_sync" {
		t.Fatalf("features = %q", got)
	}
	a.cfg.Memory.MemoryEnabled = false
	a.cfg.WhatsApp.Enabled = true
	if got := strings.Join(s.ActiveFeatures(), ","); got != "cloud_sync,whatsapp" {
		t.Fatalf("features after a config change = %q", got)
	}
}

// The whole path on one machine: turns are recorded, the person agrees, and the
// report that reaches a (fake) server describes them - without a word of content.
func TestTelemetry_EndToEndAgainstAFakeServer(t *testing.T) {
	t.Setenv("MEMO_TELEMETRY", "")
	orig := telemetryHardware
	telemetryHardware = func() telemetry.HWInfo {
		return telemetry.HWInfo{GPUType: "nvidia", GPUName: "NVIDIA GeForce RTX 4070", VRAMMB: 12282, RAMMB: 32000, Threads: 20}
	}
	t.Cleanup(func() { telemetryHardware = orig })

	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, r.URL.Path+" "+string(b))
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	sent := func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), bodies...) }

	a := newTelemetryApp(t)
	statePath := filepath.Join(t.TempDir(), "telemetry_state.json")
	today := time.Date(2026, 10, 27, 3, 0, 0, 0, time.UTC)
	managerAt := func(clock time.Time) *telemetry.Manager {
		m, err := telemetry.New(telemetry.Config{
			StatePath: statePath, Endpoint: srv.URL, Sources: &telemetrySources{a: a},
			Info: telemetry.Info{Version: a.GetVersion(), Build: telemetry.BuildOfficial, OS: "linux", Arch: "amd64"},
			Now:  func() time.Time { return clock },
		})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}

	ctx := context.Background()
	yesterday := today.AddDate(0, 0, -1).Add(10 * time.Hour)
	_ = a.statsStore.RecordEvent(ctx, stats.Event{Timestamp: yesterday, Provider: "My Secret OpenAI Account", Model: "gpt-5.5", Category: "chat", PromptTokens: 1200, CompletionTokens: 300})
	_ = a.statsStore.RecordEvent(ctx, stats.Event{Timestamp: yesterday, Provider: "local", Model: "/home/alice/models/Qwen3-8B-Q4_K_M.gguf", Category: "chat", PromptTokens: 800, CompletionTokens: 200, TokensPerSecond: 61, TTFTMs: 450})
	_ = a.statsStore.RecordEvent(ctx, stats.Event{Timestamp: yesterday, Provider: "local", Model: "minimal-mode-secret.gguf", Category: "chat", PromptTokens: 9, CompletionTokens: 9, Private: true})

	// Before the answer: the choice is due, nothing leaves, and the person can look at what would.
	m := managerAt(today)
	a.telemetry = m
	if st := a.TelemetryStatus(); !st.NeedsAsk || st.Enabled {
		t.Fatalf("a new official install must ask: %+v", st)
	}
	_ = m.Tick(ctx)
	if len(sent()) != 0 {
		t.Fatalf("sent before the answer: %v", sent())
	}
	pre, err := a.TelemetryPreview(ctx)
	if err != nil || pre.Empty {
		t.Fatalf("preview: %+v %v", pre, err)
	}

	// The answer ("yes") is given three days ago; today the cycle runs.
	if err := managerAt(today.AddDate(0, 0, -3)).SetConsent(true); err != nil {
		t.Fatal(err)
	}
	m = managerAt(today)
	a.telemetry = m
	if err := m.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	got := sent()
	if len(got) != 1 || !strings.HasPrefix(got[0], "/v1/report ") {
		t.Fatalf("want exactly one report, got %v", got)
	}
	body := strings.TrimPrefix(got[0], "/v1/report ")
	if body != string(pre.JSON) && !strings.Contains(body, `"day":"2026-10-26"`) {
		t.Errorf("unexpected report day: %s", body)
	}
	for _, want := range []string{`"build":"official"`, `"gpu_model":"RTX 4070"`, `"model":"Qwen3 8B"`, `"provider":"openai"`, `"model":"gpt-5.5"`, `"ram_gb":32`, `"vram_gb":12`, `"engine":"nvidia"`, `"tps_bucket":"40-80"`, `"in_tokens":800`} {
		if !strings.Contains(body, want) {
			t.Errorf("report lacks %s\n%s", want, body)
		}
	}
	for _, leak := range []string{"Secret", "secret", "alice", "/home", "minimal-mode", `"turns":3`} { // the private (Minimal Mode) turn is not counted: 2 turns, never 3
		if strings.Contains(body, leak) {
			t.Errorf("%q must not be in a report\n%s", leak, body)
		}
	}
}
