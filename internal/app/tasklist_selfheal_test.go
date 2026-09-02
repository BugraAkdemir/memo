package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"memo/internal/agent"
	"memo/internal/config"
	"memo/internal/provider"
)

func newSelfHealApp(t *testing.T, names ...string) *App {
	t.Helper()
	cfgMgr := provider.NewConfigManager(filepath.Join(t.TempDir(), "providers.json"), make([]byte, 32))
	for i, n := range names {
		cfgMgr.Set(provider.ProviderConfig{
			Type:     provider.ProviderCustom,
			Name:     n,
			BaseURL:  "http://127.0.0.1:1",
			Model:    n + "-model",
			Enabled:  true,
			Priority: len(names) - i,
		})
	}
	return &App{
		cfg:                &config.AppConfig{},
		providerCfgMgr:     cfgMgr,
		activeProviderName: firstOr(names, ""),
		agentExecutor:      agent.NewExecutor(t.TempDir(), nil, nil, nil),
		taskRunCfgs:        make(map[string]*taskRunConfig),
	}
}

func firstOr(s []string, def string) string {
	if len(s) > 0 {
		return s[0]
	}
	return def
}

func (a *App) seedTaskRunConfig(listID, providerName string) *taskRunConfig {
	return a.seedTaskRunConfigRoaming(listID, providerName, true)
}

func (a *App) seedTaskRunConfigRoaming(listID, providerName string, roaming bool) *taskRunConfig {
	trc := &taskRunConfig{
		exec:            agent.NewTaskExecutor(a.agentExecutor, provider.NewRouter(a.enabledProviderConfigs())),
		providerName:    providerName,
		model:           providerName + "-model",
		triedProviders:  map[string]bool{providerName: true},
		providerRoaming: roaming,
	}
	a.taskRunMu.Lock()
	a.taskRunCfgs[listID] = trc
	a.taskRunMu.Unlock()
	return trc
}

// With the provider lock on (default), self-heal never switches — any
// non-rate-limit error returns false so the engine parks/retries instead.
func TestHealTaskProvider_LockedByDefault_NeverSwitches(t *testing.T) {
	a := newSelfHealApp(t, "primary", "backup")
	trc := a.seedTaskRunConfigRoaming("L1", "primary", false)

	for _, e := range []string{"status 401: invalid api key", "status 503: service unavailable", "connection refused"} {
		if a.healTaskProvider(context.Background(), "L1", errors.New(e)) {
			t.Fatalf("healTaskProvider switched provider on %q with the lock on", e)
		}
	}
	if trc.providerName != "primary" {
		t.Fatalf("task provider = %q, must stay pinned to primary with the lock on", trc.providerName)
	}
}

func TestHealTaskProvider_SwitchesOnAuthError(t *testing.T) {
	a := newSelfHealApp(t, "primary", "backup")
	trc := a.seedTaskRunConfig("L1", "primary")

	healed := a.healTaskProvider(context.Background(), "L1", errors.New("status 401: invalid api key"))
	if !healed {
		t.Fatal("healTaskProvider returned false on an auth error with a spare provider")
	}
	if trc.providerName != "backup" {
		t.Fatalf("task provider = %q, want backup", trc.providerName)
	}
	if a.activeProviderName != "primary" {
		t.Fatalf("global activeProviderName = %q, must stay untouched", a.activeProviderName)
	}
}

func TestHealTaskProvider_RateLimitDoesNotSwitch(t *testing.T) {
	a := newSelfHealApp(t, "primary", "backup")
	trc := a.seedTaskRunConfig("L1", "primary")

	if a.healTaskProvider(context.Background(), "L1", errors.New("HTTP 429 quota exceeded")) {
		t.Fatal("healTaskProvider switched provider on a rate-limit error")
	}
	if trc.providerName != "primary" {
		t.Fatalf("task provider changed to %q on a rate-limit error", trc.providerName)
	}
}

func TestHealTaskProvider_ExhaustedReturnsFalse(t *testing.T) {
	a := newSelfHealApp(t, "only")
	a.seedTaskRunConfig("L1", "only")

	if a.healTaskProvider(context.Background(), "L1", errors.New("401 unauthorized")) {
		t.Fatal("healTaskProvider returned true with no spare provider")
	}
}

func TestHealTaskProvider_TransientRetriesThenSwitches(t *testing.T) {
	a := newSelfHealApp(t, "primary", "backup")
	trc := a.seedTaskRunConfig("L1", "primary")

	transient := errors.New("status 503: service unavailable")
	// First two transient errors: retry same provider.
	for i := 0; i < maxTaskTransientRetries; i++ {
		if !a.healTaskProvider(context.Background(), "L1", transient) {
			t.Fatalf("transient retry %d returned false", i)
		}
		if trc.providerName != "primary" {
			t.Fatalf("switched provider on transient retry %d", i)
		}
	}
	// Third: switch.
	if !a.healTaskProvider(context.Background(), "L1", transient) {
		t.Fatal("third transient error did not heal")
	}
	if trc.providerName != "backup" {
		t.Fatalf("provider = %q, want backup after exhausting transient retries", trc.providerName)
	}
}
