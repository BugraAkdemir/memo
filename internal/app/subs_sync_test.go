// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/provider"
)

var (
	fakeCPAOnce sync.Once
	fakeCPAPath string
	fakeCPAErr  error
)

// bundleFakeSidecar installs the fake CLIProxyAPI (the one internal/cliproxy's
// tests use) as the "bundled" binary under the data dir.
func bundleFakeSidecar(t *testing.T) {
	t.Helper()
	fakeCPAOnce.Do(func() {
		dir, err := os.MkdirTemp("", "app-fakecpa")
		if err != nil {
			fakeCPAErr = err
			return
		}
		name := "cli-proxy-api"
		switch runtime.GOOS {
		case "windows":
			name += ".exe"
		case "darwin":
			name = "cli-proxy-api-x64"
			if runtime.GOARCH == "arm64" {
				name = "cli-proxy-api-arm64"
			}
		}
		fakeCPAPath = filepath.Join(dir, name)
		if out, err := exec.Command("go", "build", "-o", fakeCPAPath, "../cliproxy/testdata/fakecpa").CombinedOutput(); err != nil {
			fakeCPAErr = &fakeBuildError{err, string(out)}
		}
	})
	if fakeCPAErr != nil {
		t.Fatalf("building fakecpa: %v", fakeCPAErr)
	}
	b, err := os.ReadFile(fakeCPAPath)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(config.DataDir(), "binaries", runtime.GOOS, "cliproxy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(fakeCPAPath)
	os.WriteFile(filepath.Join(dir, name), b, 0o755)
	sum := sha256.Sum256(b)
	os.WriteFile(filepath.Join(dir, "SHA256"), []byte(hex.EncodeToString(sum[:])+"  "+name+"\n"), 0o644)
}

type fakeBuildError struct {
	err error
	out string
}

func (e *fakeBuildError) Error() string { return e.err.Error() + ": " + e.out }

func fastSubs(t *testing.T) {
	t.Helper()
	a, b, c, d := subsStartTimeout, subsModelsTimeout, subsRetryEvery, subsRetryAttempts
	subsStartTimeout, subsModelsTimeout, subsRetryEvery, subsRetryAttempts = 30*time.Second, 10*time.Second, 200*time.Millisecond, 3
	t.Cleanup(func() { subsStartTimeout, subsModelsTimeout, subsRetryEvery, subsRetryAttempts = a, b, c, d })
}

// syncApp is a minimal App whose only live parts are the provider list and the
// sidecar: enough to exercise syncSubscriptions without booting everything.
func syncApp(t *testing.T) *App {
	t.Helper()
	isolatedDataDir(t)
	bundleFakeSidecar(t)
	cfgMgr := provider.NewConfigManager(config.DataPath("providers.json"), nil)
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{cfg: &config.AppConfig{}, providerCfgMgr: cfgMgr, lifecycleCtx: ctx}
	t.Cleanup(func() { cancel(); a.stopSubscriptions() })
	return a
}

func seedCredential(t *testing.T) {
	t.Helper()
	dir := config.DataPath("cliproxy", "auth")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "antigravity-x.json"), []byte(`{"type":"antigravity","email":"x@example.com"}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

// After a restart the sidecar needs ~30s before it lists models. A model chosen
// in a previous session must work the moment the sidecar is healthy — the
// provider is registered with the sidecar's CURRENT port/key immediately, then
// the model is confirmed once the list is known.
func TestSyncSubscriptions_RegistersEarlyWithThePreviousModel(t *testing.T) {
	fastSubs(t)
	t.Setenv("FAKECPA_MODELS_DELAY_MS", "2500")
	a := syncApp(t)
	seedCredential(t)
	// The previous session's choice, with a stale port and key.
	a.providerCfgMgr.Set(provider.ProviderConfig{Type: provider.ProviderCustom, Name: subsProviderName, BaseURL: "http://127.0.0.1:1/v1", APIKey: "stale", Model: "antigravity-model-a", Enabled: true})

	done := make(chan error, 1)
	go func() { done <- a.syncSubscriptions(context.Background()) }()

	// While the sidecar is still hiding its models, the provider already points
	// at it and carries the earlier model.
	early := false
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		p, ok := a.subsMarkerConfig()
		m := a.subsManager()
		if ok && m.BaseURL() != "" && p.BaseURL == m.BaseURL() && p.APIKey == m.APIKey() && len(m.CachedModels()) == 0 {
			early = true
			if p.Model != "antigravity-model-a" {
				t.Errorf("early model = %q, want the previous session's choice", p.Model)
			}
			break
		}
	}
	if !early {
		t.Error("the provider was not registered with the sidecar's current address before its models were listed")
	}

	if err := <-done; err != nil {
		t.Fatalf("syncSubscriptions: %v", err)
	}
	if p, _ := a.subsMarkerConfig(); p.Model != "antigravity-model-a" {
		t.Errorf("after the list arrived the model is %q, want the still-offered choice kept", p.Model)
	}
}

// If the earlier choice is no longer offered once the list is known, it is
// replaced rather than left pointing at a model the account cannot use.
func TestSyncSubscriptions_ReplacesAModelThatIsNoLongerOffered(t *testing.T) {
	fastSubs(t)
	a := syncApp(t)
	seedCredential(t)
	a.providerCfgMgr.Set(provider.ProviderConfig{Type: provider.ProviderCustom, Name: subsProviderName, BaseURL: "http://127.0.0.1:1/v1", Model: "gemini-2.5-pro", Enabled: true})

	if err := a.syncSubscriptions(context.Background()); err != nil {
		t.Fatalf("syncSubscriptions: %v", err)
	}
	if p, _ := a.subsMarkerConfig(); p.Model != "antigravity-model-b" {
		t.Errorf("model = %q, want a model the account does offer", p.Model)
	}
}

// A sidecar that never lists a model is reported as "not yet" (retryable), not as
// a hard failure — and startSubscriptions keeps trying.
func TestStartSubscriptions_RetriesWhileTheSidecarStillListsNothing(t *testing.T) {
	fastSubs(t)
	subsModelsTimeout = 400 * time.Millisecond // each sync gives up quickly...
	t.Setenv("FAKECPA_MODELS_DELAY_MS", "1500")
	a := syncApp(t)
	seedCredential(t)

	a.startSubscriptions() // ...so it only succeeds by retrying

	if _, ok := a.subsMarkerConfig(); !ok {
		t.Error("startSubscriptions gave up before the sidecar listed its models")
	}
}
