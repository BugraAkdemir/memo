// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"memo/internal/components"
	"memo/internal/config"
	"memo/internal/llama"
	"memo/internal/logx"
	"memo/internal/stats"
	"memo/internal/telemetry"
)

// The optional anonymous daily summary (docs/TELEMETRY.md). Everything that
// decides whether something may be sent lives in internal/telemetry; this file
// only lets that package read the parts of Memo it describes. Until the person
// has said yes, none of this counts or sends anything.

// telemetryHardware is the machine probe; replaced in tests so they do not
// shell out to nvidia-smi.
var telemetryHardware = func() telemetry.HWInfo {
	g := llama.DetectGPU()
	return telemetry.HWInfo{GPUType: string(g.Type), GPUName: g.Name, VRAMMB: g.VRAM, RAMMB: g.RAMTotalMb, Threads: runtime.NumCPU()}
}

type telemetrySources struct {
	a      *App
	hwOnce sync.Once
	hw     telemetry.HWInfo
}

func (s *telemetrySources) UsageEvents(ctx context.Context, from, to time.Time) ([]stats.Event, error) {
	if s.a.statsStore == nil {
		return nil, nil
	}
	return s.a.statsStore.EventsBetween(ctx, from, to)
}

// Hardware is probed once per process (it can run vendor tools) and the engine
// folder in use is read fresh, because it can change with an engine update.
func (s *telemetrySources) Hardware() telemetry.HWInfo {
	s.hwOnce.Do(func() { s.hw = telemetryHardware() })
	h := s.hw
	h.EngineFlavor = s.a.engineFolder(components.Llama)
	return h
}

func (s *telemetrySources) MemoryRows() int {
	if s.a.store == nil {
		return -1
	}
	return s.a.store.Count()
}

func (s *telemetrySources) UILang() string { return s.a.GetUILanguage() }

// ActiveFeatures are the capabilities switched on in the config. They count
// only on a day the device was used (see telemetry.Collector).
func (s *telemetrySources) ActiveFeatures() []string {
	s.a.cfgMu.RLock()
	defer s.a.cfgMu.RUnlock()
	var out []string
	if s.a.cfg.Memory.MemoryEnabled {
		out = append(out, "memory")
	}
	if s.a.cfg.Sync.Enabled {
		out = append(out, "cloud_sync")
	}
	if s.a.cfg.WhatsApp.Enabled {
		out = append(out, "whatsapp")
	}
	if s.a.cfg.LiveMode.Enabled {
		out = append(out, "live_mode")
	}
	return out
}

// ProviderOf turns the provider NAME a usage row carries into ("local" | "api" |
// "sub", provider type). Names are the person's own ("My OpenAI"), so they are
// never sent; only the type is.
func (s *telemetrySources) ProviderOf(name, model string) (string, string) {
	switch name {
	case "local", "":
		return "local", "local"
	case "orchestra":
		return "api", "orchestra"
	}
	cfg, ok := s.a.providerConfigByName(name)
	if !ok {
		return "api", "other"
	}
	if isSubsMarker(cfg) {
		return "sub", subscriptionVendor(model)
	}
	return "api", strings.ToLower(string(cfg.Type))
}

// subscriptionVendor guesses which vendor account served a Subscriptions turn
// from the model family (the usage row has no vendor field).
func subscriptionVendor(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, "claude"):
		return "claude"
	case strings.HasPrefix(m, "gpt"), strings.HasPrefix(m, "codex"), strings.HasPrefix(m, "o1"), strings.HasPrefix(m, "o3"), strings.HasPrefix(m, "o4"):
		return "codex"
	}
	return "antigravity"
}

// initTelemetry installs the telemetry manager and starts its background cycle.
// It runs after initStats (the usage store) and never blocks startup.
func (a *App) initTelemetry() {
	statePath := config.DataPath("telemetry_state.json")
	_, statErr := os.Stat(statePath)
	brandNew := errors.Is(statErr, os.ErrNotExist) && a.GetUsageStats(0).TotalRequests == 0

	m, err := telemetry.New(telemetry.Config{
		StatePath: statePath,
		Sources:   &telemetrySources{a: a},
		Info:      telemetry.Info{Version: a.GetVersion(), Build: telemetry.BuildChannel, OS: runtime.GOOS, Arch: runtime.GOARCH},
	})
	if err != nil {
		logx.Printf("WARN: telemetry: %v", err)
		return
	}
	if brandNew {
		_ = m.SetInstallDay() // an upgraded install leaves it unset: it did not "install" today
	}
	a.telemetry = m
	telemetry.SetDefault(m)
	logx.SetPanicHook(telemetry.NotePanic) // a recovered panic is counted as a sanitized signature (type name + Memo frames), if sharing is on
	goRecover("telemetry.Start", func() { m.Start(a.lifecycleCtx) })
}

// TelemetryStatus is what Settings shows; a zero Status when telemetry could not start.
func (a *App) TelemetryStatus() telemetry.Status {
	if a.telemetry == nil {
		return telemetry.Status{EnvOff: true}
	}
	return a.telemetry.Status()
}

// SetTelemetryConsent records the person's answer (true = share the daily summary).
func (a *App) SetTelemetryConsent(yes bool) error {
	if a.telemetry == nil {
		return errors.New("telemetry is not available")
	}
	return a.telemetry.SetConsent(yes)
}

// TelemetryPreview returns the exact report that would be sent next.
func (a *App) TelemetryPreview(ctx context.Context) (*telemetry.PreviewResult, error) {
	if a.telemetry == nil {
		return &telemetry.PreviewResult{Empty: true}, nil
	}
	return a.telemetry.Preview(ctx)
}

// TelemetryResetID gives this install a new random id (the old one is forgotten server-side).
func (a *App) TelemetryResetID(ctx context.Context) error {
	if a.telemetry == nil {
		return errors.New("telemetry is not available")
	}
	return a.telemetry.ResetID(ctx)
}

// TelemetryDelete deletes this install's data on the server and stops sharing.
func (a *App) TelemetryDelete(ctx context.Context) error {
	if a.telemetry == nil {
		return errors.New("telemetry is not available")
	}
	return a.telemetry.DeleteData(ctx)
}

// providerErrorCode maps a raw provider error text onto the small fixed set of
// codes the daily summary may carry ("401", "429", "5xx", "quota", "timeout",
// "network"). Anything it cannot place yields "" and is not counted: the
// summary has no free-text bucket, so an error text can never leak through it.
func providerErrorCode(text string) string {
	lower := strings.ToLower(text)
	status := errStatus(text)
	switch {
	case quotaWording(lower):
		return "quota"
	case status == 401 || status == 403:
		return "401"
	case status == 429:
		return "429"
	case status >= 500:
		return "5xx"
	case containsAny(lower, "timeout", "deadline exceeded", "timed out"):
		return "timeout"
	case containsAny(lower, "no such host", "connection refused", "connection reset", "unexpected eof", "network is unreachable", "broken pipe", "tls:"):
		return "network"
	}
	return ""
}

// noteProviderFailure counts one failed provider turn by provider TYPE (never
// the person's own provider name). Local engines are reported through their own
// start/exit counters instead.
func (a *App) noteProviderFailure(providerName, model, errText string) {
	code := providerErrorCode(errText)
	if code == "" {
		return
	}
	kind, typ := (&telemetrySources{a: a}).ProviderOf(providerName, model)
	if kind == "local" {
		return
	}
	telemetry.NoteProviderErr(typ, code)
}

// noteLocalStartFail counts one failed start of a local engine ("llama" or
// "embedding") under its fixed failure code. A start the person cancelled is
// not a failure.
func noteLocalStartFail(component string, err error) {
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	telemetry.NoteStartFail(component, llama.ClassifyStartError(err))
}
