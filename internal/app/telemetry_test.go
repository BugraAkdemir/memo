// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"errors"
	"fmt"
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
	"memo/internal/updater"
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

	// First start of an official build: sharing is on by default, but the day of
	// that default is never reported, so only the one-time install count leaves.
	m := managerAt(today)
	a.telemetry = m
	if st := a.TelemetryStatus(); !st.Enabled || st.Consent != telemetry.ConsentYes {
		t.Fatalf("a new official install shares by default: %+v", st)
	}
	_ = m.Tick(ctx)
	if got := sent(); len(got) != 1 || !strings.HasPrefix(got[0], "/v1/install ") {
		t.Fatalf("before any completed day only the install count may leave: %v", got)
	}

	// The answer ("yes") is given three days ago; today the cycle runs.
	if err := managerAt(today.AddDate(0, 0, -3)).SetConsent(true); err != nil {
		t.Fatal(err)
	}
	m = managerAt(today)
	a.telemetry = m
	pre, err := a.TelemetryPreview(ctx)
	if err != nil || pre.Empty {
		t.Fatalf("preview: %+v %v", pre, err)
	}
	if err := m.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, b := range sent() {
		if strings.HasPrefix(b, "/v1/report ") {
			got = append(got, b)
		}
	}
	if len(got) != 1 {
		t.Fatalf("want exactly one report, got %v", sent())
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

func TestProviderErrorCode_MapsTheErrorTextToTheSchemaCodes(t *testing.T) {
	cases := []struct{ text, want string }{
		{"API error: status 401 Unauthorized", "401"},
		{"HTTP 403 forbidden", "401"},
		{"status 429: rate limit", "429"},
		{"status 429: You exceeded your current quota", "quota"},
		{"insufficient_quota", "quota"},
		{"status 500 internal error", "5xx"},
		{"HTTP 503 overloaded", "5xx"},
		{"status 529", "5xx"},
		{"context deadline exceeded (Client.Timeout)", "timeout"},
		{"read tcp 1.2.3.4:5->6.7.8.9:443: i/o timeout", "timeout"},
		{"dial tcp: lookup api.example.com: no such host", "network"},
		{"connection refused", "network"},
		{"unexpected EOF", "network"},
		// Raw texts as Memo's own router produces them: no "status NNN" in them.
		{"LLM Error: all providers failed: [custom] provider rate limited: fakeprovider: forced error 429", "429"},
		{"too many requests", "429"},
		{"rate_limit_error: slow down", "429"},
		{"invalid api key provided", "401"},
		{"401 Unauthorized", "401"},
		{"authentication failed", "401"},
		{"service unavailable", "5xx"},
		{"overloaded_error", "5xx"},
		{"internal server error", "5xx"},
		{"bad gateway", "5xx"},
		{"status 400 bad request: maximum context length", ""},
		{"something nobody anticipated", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := providerErrorCode(c.text); got != c.want {
			t.Errorf("providerErrorCode(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestTelemetry_FailureAndEngineCountersReachTheNextReport(t *testing.T) {
	t.Setenv("MEMO_TELEMETRY", "")
	orig := telemetryHardware
	telemetryHardware = func() telemetry.HWInfo { return telemetry.HWInfo{GPUType: "cpu", RAMMB: 16000, Threads: 8} }
	t.Cleanup(func() { telemetryHardware = orig })
	t.Cleanup(func() { telemetry.SetDefault(nil) })

	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		if r.URL.Path == "/v1/report" { // the one-time install count is not a report
			bodies = append(bodies, string(b))
		}
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	a := newTelemetryApp(t)
	statePath := filepath.Join(t.TempDir(), "telemetry_state.json")
	today := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
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

	// Nothing is kept while the person has said no.
	m := managerAt(today)
	if err := m.SetConsent(false); err != nil {
		t.Fatal(err)
	}
	telemetry.SetDefault(m)
	a.noteProviderFailure("orchestra", "x", "status 429: slow down")
	noteLocalStartFail("llama", errors.New("fork/exec x: permission denied"))
	if m.Status().Enabled {
		t.Fatal("must not be enabled after a no")
	}

	if err := managerAt(today.AddDate(0, 0, -3)).SetConsent(true); err != nil {
		t.Fatal(err)
	}
	m = managerAt(today)
	telemetry.SetDefault(m)

	a.noteProviderFailure("orchestra", "x", "status 429: slow down")
	a.noteProviderFailure("orchestra", "x", "status 429: slow down")
	a.noteProviderFailure("orchestra", "x", "an error text no code fits") // not counted
	a.noteProviderFailure("local", "m.gguf", "status 500")                // local engines have their own counters
	noteLocalStartFail("llama", errors.New("fork/exec /home/alice/llama-server: permission denied"))
	noteLocalStartFail("embedding", context.Canceled) // a cancelled start is not a failure
	telemetry.NoteExit("llama", "oom_killed")

	if err := managerAt(today.AddDate(0, 0, 1)).Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 1 {
		t.Fatalf("want one report, got %d: %v", len(bodies), bodies)
	}
	body := bodies[0]
	for _, want := range []string{`"provider":"orchestra"`, `"code":"429"`, `"n":2`, `"component":"llama","code":"permission"`, `"code":"oom_killed"`} {
		if !strings.Contains(body, want) {
			t.Errorf("report lacks %s\n%s", want, body)
		}
	}
	for _, bad := range []string{"alice", "slow down", "embedding", `"500"`, "5xx"} {
		if strings.Contains(body, bad) {
			t.Errorf("report must not contain %q\n%s", bad, body)
		}
	}
}

func TestUpdateOutcome_MapsAnApplyResultToTheFixedEventAndCode(t *testing.T) {
	cases := []struct {
		err         error
		event, code string
		counted     bool
	}{
		{nil, "applied", "", true},
		{updater.ErrChecksum, "failed", "checksum", true},
		{fmt.Errorf("download: %w", updater.ErrChecksum), "failed", "checksum", true},
		{updater.ErrBadSignature, "failed", "signature", true},
		{errors.New("dial tcp: no such host"), "failed", "", true},
		{updater.ErrBusy, "", "", false},                             // not an outcome: another update was already running
		{context.Canceled, "", "", false},                            // the person quit
		{errors.New("there is no update to install"), "", "", false}, // nothing was attempted
	}
	for _, c := range cases {
		ev, code, ok := updateOutcome(c.err)
		if ev != c.event || code != c.code || ok != c.counted {
			t.Errorf("updateOutcome(%v) = %q,%q,%v; want %q,%q,%v", c.err, ev, code, ok, c.event, c.code, c.counted)
		}
	}
}

func TestRolledBackNote_CountsOnlyAFreshNoteAndOnlyOncePerProcess(t *testing.T) {
	t.Setenv("MEMO_TELEMETRY", "")
	dir := t.TempDir()
	note := filepath.Join(dir, updater.FailureFile)
	if err := updater.WriteFailure(note, "4.6.0", updater.ErrRolledBack); err != nil {
		t.Fatal(err)
	}
	var rt updateRuntime
	if ev, ok := rt.freshRollbackNote(dir, time.Now()); !ok || ev != "rolled_back" {
		t.Fatalf("fresh rolled-back note: %q %v", ev, ok)
	}
	if _, ok := rt.freshRollbackNote(dir, time.Now()); ok {
		t.Fatal("the same note was counted twice in one process")
	}
	// An old note (a restart days later) is not news.
	var rt2 updateRuntime
	if _, ok := rt2.freshRollbackNote(dir, time.Now().Add(72*time.Hour)); ok {
		t.Fatal("a stale note was counted")
	}
	// No note at all.
	var rt3 updateRuntime
	if _, ok := rt3.freshRollbackNote(t.TempDir(), time.Now()); ok {
		t.Fatal("counted a note that does not exist")
	}
}

// telemetryRig runs a manager that has agreed to share, over a fake server, and
// returns the next day's report body after the notes in fn were made.
func telemetryRig(t *testing.T, a *App, fn func()) string {
	t.Helper()
	t.Setenv("MEMO_TELEMETRY", "")
	orig := telemetryHardware
	telemetryHardware = func() telemetry.HWInfo { return telemetry.HWInfo{GPUType: "cpu", RAMMB: 16000, Threads: 8} }
	t.Cleanup(func() { telemetryHardware = orig; telemetry.SetDefault(nil) })

	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		if r.URL.Path == "/v1/report" { // the one-time install count is not a report
			bodies = append(bodies, string(b))
		}
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)

	statePath := filepath.Join(t.TempDir(), "telemetry_state.json")
	today := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
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
	if err := managerAt(today.AddDate(0, 0, -3)).SetConsent(true); err != nil {
		t.Fatal(err)
	}
	telemetry.SetDefault(managerAt(today))
	fn()
	if err := managerAt(today.AddDate(0, 0, 1)).Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 1 {
		t.Fatalf("want one report, got %d", len(bodies))
	}
	return bodies[0]
}

func TestNoteFeature_CountsUseButNeverIncognitoOrMinimalMode(t *testing.T) {
	a := newTelemetryApp(t)
	body := telemetryRig(t, a, func() {
		a.noteFeature("skills")
		a.noteFeature("skills") // twice is still once
		a.ToggleIncognito(true)
		a.noteFeature("voice")
		a.ToggleIncognito(false)
		a.identity.SetMinimalMode(true)
		a.noteFeature("image_gen")
		a.identity.SetMinimalMode(false)
		a.noteFeature("subagents")
	})
	for _, want := range []string{`"skills"`, `"subagents"`} {
		if !strings.Contains(body, want) {
			t.Errorf("report lacks %s\n%s", want, body)
		}
	}
	for _, bad := range []string{`"voice"`, `"image_gen"`} {
		if strings.Contains(body, bad) {
			t.Errorf("%s was used in incognito or Minimal Mode and must not be counted\n%s", bad, body)
		}
	}
}

func TestMilestones_FirstMessageAndProviderAddedAreDatesNotContent(t *testing.T) {
	a := newTelemetryApp(t)
	body := telemetryRig(t, a, func() {
		// A day with only a milestone is quiet and sends nothing; the date waits in the
		// state for the next day that has activity. Give this one a turn.
		_ = a.statsStore.RecordEvent(context.Background(), stats.Event{Timestamp: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC), Provider: "local", Model: "m.gguf", Category: "chat", PromptTokens: 5, CompletionTokens: 5})
		a.noteFirstMessage(usageMeta{Category: "title"}) // a background call is not a message
		a.noteFirstMessage(usageMeta{Category: "learning"})
		a.noteFirstMessage(usageMeta{Category: "chat"})
		telemetry.NoteMilestone("provider_added")
	})
	for _, want := range []string{`"first_message":"2026-10-27"`, `"provider_added":"2026-10-27"`} {
		if !strings.Contains(body, want) {
			t.Errorf("report lacks %s\n%s", want, body)
		}
	}
}

// With web tools on (the default) an ordinary chat turn is recorded as
// "web_search", and agent chats as "agent": both are real messages. Found by
// running the real binary, where the milestone never appeared.
func TestFirstMessage_CountsEveryCategoryAPersonalTurnIsRecordedUnder(t *testing.T) {
	for _, cat := range []string{"chat", "web_search", "agent"} {
		a := newTelemetryApp(t)
		body := telemetryRig(t, a, func() {
			_ = a.statsStore.RecordEvent(context.Background(), stats.Event{Timestamp: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC), Provider: "local", Model: "m.gguf", Category: cat, PromptTokens: 5, CompletionTokens: 5})
			a.noteFirstMessage(usageMeta{Category: cat})
		})
		if !strings.Contains(body, `"first_message":"2026-10-27"`) {
			t.Errorf("category %q is a real message but the milestone is missing\n%s", cat, body)
		}
	}
}
