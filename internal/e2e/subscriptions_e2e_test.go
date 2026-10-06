// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/models"
	"memo/internal/provider"
)

// The fake CLIProxyAPI is the one internal/cliproxy's own tests use; building
// it here (once) keeps a single source of truth for "what the sidecar looks
// like from outside".
var (
	fakeCPAOnce sync.Once
	fakeCPAPath string
	fakeCPAErr  error
)

func buildFakeCPA(t *testing.T) string {
	t.Helper()
	fakeCPAOnce.Do(func() {
		dir, err := os.MkdirTemp("", "e2e-fakecpa")
		if err != nil {
			fakeCPAErr = err
			return
		}
		name := "cli-proxy-api"
		switch runtime.GOOS {
		case "windows":
			name += ".exe"
		case "darwin":
			name = map[string]string{"arm64": "cli-proxy-api-arm64"}[runtime.GOARCH]
			if name == "" {
				name = "cli-proxy-api-x64"
			}
		}
		fakeCPAPath = filepath.Join(dir, name)
		out, err := exec.Command("go", "build", "-o", fakeCPAPath, "../cliproxy/testdata/fakecpa").CombinedOutput()
		if err != nil {
			fakeCPAErr = &buildErr{err, string(out)}
		}
	})
	if fakeCPAErr != nil {
		t.Fatalf("building fakecpa: %v", fakeCPAErr)
	}
	return fakeCPAPath
}

type buildErr struct {
	err error
	out string
}

func (e *buildErr) Error() string { return e.err.Error() + ": " + e.out }

// bundleSidecar installs the fake as the "bundled" binary under the app's data
// dir — one of the places the app looks (the launch scripts copy binaries/
// there on first run).
func bundleSidecar(t *testing.T) {
	t.Helper()
	src := buildFakeCPA(t)
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(config.DataDir(), "binaries", runtime.GOOS, "cliproxy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(src)
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	os.WriteFile(filepath.Join(dir, "SHA256"), []byte(hex.EncodeToString(sum[:])+"  "+name+"\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "VERSION"), []byte("v0.0.0-e2e\n"), 0o644)
}

func (h *Harness) subscriptions(t *testing.T) models.SubscriptionsState {
	t.Helper()
	var st models.SubscriptionsState
	decodeInto(t, h.getJSON("/api/subscriptions"), &st)
	return st
}

func waitFor(t *testing.T, what string, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(60 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func (h *Harness) providerNamed(t *testing.T, name string) (provider.ProviderConfig, bool) {
	t.Helper()
	for _, p := range h.App.GetProviders() {
		if p.Name == name {
			return p, true
		}
	}
	return provider.ProviderConfig{}, false
}

// TestSubscriptions_LoginToChat drives the whole feature through the real REST
// API and a real App: nothing bundled -> bundled -> sign in -> sidecar running
// -> provider registered -> models listed (settings, dev gateway) -> a chat turn
// reaches the sidecar with the selected model -> sign out tears it all down.
func TestSubscriptions_LoginToChat(t *testing.T) {
	h := NewHarness(t)

	// 1. A build without the sidecar says so, and starts nothing.
	st := h.subscriptions(t)
	if st.Bundled || st.Problem == "" || st.Running || len(st.Accounts) != 0 {
		t.Fatalf("empty state = %+v", st)
	}

	// 2. Bundled: usable, still not started (GET is side-effect free).
	bundleSidecar(t)
	st = h.subscriptions(t)
	if !st.Bundled || st.Version != "v0.0.0-e2e" || st.Problem != "" || st.Running {
		t.Fatalf("bundled state = %+v", st)
	}
	if len(st.Providers) != 3 {
		t.Errorf("providers = %v, want antigravity, claude, codex", st.Providers)
	}

	// 3. Sign in.
	resp := h.postJSON("/api/subscriptions", map[string]string{"action": "login", "provider": "antigravity"})
	var login struct {
		AuthURL string `json:"auth_url"`
	}
	decodeInto(t, resp, &login)
	if !strings.HasPrefix(login.AuthURL, "https://example.test/oauth/authorize") {
		t.Fatalf("auth_url = %q", login.AuthURL)
	}
	waitFor(t, "sign-in to register the provider", 40*time.Second, func() bool {
		st = h.subscriptions(t)
		_, ok := h.providerNamed(t, "Subscriptions")
		return ok && st.Running && len(st.Accounts) == 1 && len(st.Models) > 0
	})
	if st.Accounts[0].Provider != "antigravity" || st.Accounts[0].Email != "fake@example.com" {
		t.Errorf("accounts = %+v", st.Accounts)
	}
	if st.Model != "antigravity-model-b" {
		t.Errorf("default model = %q (no preference matched, so the first listed)", st.Model)
	}

	p, _ := h.providerNamed(t, "Subscriptions")
	if p.Type != provider.ProviderCustom || !strings.HasPrefix(p.BaseURL, "http://127.0.0.1:") || p.APIKey == "" || !p.Enabled {
		t.Errorf("provider = %+v", p)
	}

	// 4. The dev gateway lists the sidecar's models as "subs/<id>" and never
	// as "custom/…", which is the spelling for the user's own custom providers.
	var gw []string
	for _, m := range h.App.ListGatewayModels() {
		gw = append(gw, m.ID)
	}
	joined := strings.Join(gw, ",")
	if !strings.Contains(joined, "subs/antigravity-model-a") || !strings.Contains(joined, "subs/antigravity-model-b") {
		t.Errorf("gateway models = %v, want the sidecar's models as subs/<id>", gw)
	}
	if strings.Contains(joined, "custom/antigravity") {
		t.Errorf("sidecar model leaked into the custom/ namespace: %v", gw)
	}

	// 4b. The selectors read the live list over REST, grouped by vendor, and
	// never see the stored key.
	var listed struct {
		Models  []models.ProviderModel `json:"models"`
		Current string                 `json:"current"`
	}
	lresp := h.getJSON("/api/providers/model?name=Subscriptions")
	raw, _ := io.ReadAll(lresp.Body)
	lresp.Body.Close()
	if strings.Contains(string(raw), "api_key") || strings.Contains(string(raw), p.APIKey) {
		t.Errorf("model list response leaks the stored key: %s", raw)
	}
	if err := json.Unmarshal(raw, &listed); err != nil {
		t.Fatalf("model list JSON: %v (%s)", err, raw)
	}
	if listed.Current != "antigravity-model-b" || len(listed.Models) != 2 || listed.Models[0].OwnedBy != "antigravity" {
		t.Errorf("model list = %+v", listed)
	}

	// 5. A real chat turn through the Subscriptions provider.
	h.SetAgentEnabled(false)
	h.SetWebSearchEnabled(false)
	h.App.SetActiveProvider("Subscriptions")
	chatID := h.NewChat()
	events := h.SendMessageStream(chatID, "selam")
	if got := FinalContent(events); got != "pong from antigravity-model-b" {
		t.Fatalf("reply = %q, want the sidecar to have been asked for antigravity-model-b", got)
	}

	// 6. Switching the model rewrites the provider's model and the NEXT turn
	// uses it — without touching the rest of the config.
	h.App.SetActiveProvider("") // so the PUT below has to activate it itself
	sresp := h.putJSON("/api/providers/model", map[string]any{"name": "Subscriptions", "model": "antigravity-model-a", "activate": true})
	if sresp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /api/providers/model = %d", sresp.StatusCode)
	}
	sresp.Body.Close()
	if got := h.App.GetActiveProvider(); got != "Subscriptions" {
		t.Errorf("active provider after activate = %q", got)
	}
	bad := h.putJSON("/api/providers/model", map[string]any{"name": "no-such-provider", "model": "x"})
	if bad.StatusCode != http.StatusBadRequest {
		t.Errorf("switching an unknown provider = %d, want 400", bad.StatusCode)
	}
	bad.Body.Close()
	p2, _ := h.providerNamed(t, "Subscriptions")
	if p2.Model != "antigravity-model-a" || p2.BaseURL != p.BaseURL || p2.APIKey != p.APIKey {
		t.Errorf("after switch: %+v (was %+v)", p2, p)
	}
	events = h.SendMessageStream(chatID, "bir daha")
	if got := FinalContent(events); got != "pong from antigravity-model-a" {
		t.Errorf("reply after switching = %q", got)
	}

	// 7. The gateway can drive a sidecar model directly, and "custom/…" still
	// reaches the user's own custom provider even though the Subscriptions
	// provider is ALSO type custom and was registered first.
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		return FakeChatResponse{Text: "from the user's custom provider"}
	}
	if err := h.App.DeleteProvider(provider.ProviderCustom, "e2e-fake"); err != nil {
		t.Fatal(err)
	}
	if err := h.App.UpdateProvider(provider.ProviderConfig{Type: provider.ProviderCustom, Name: "e2e-fake", BaseURL: h.Fake.Srv.URL, Model: "fake-model", APIKey: "e2e-test-key", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	resp2, label, err := h.App.DevGatewayChat(context.Background(), "subs/antigravity-model-a", provider.ChatRequest{Messages: []provider.Message{{Role: "user", Content: "hi"}}})
	if err != nil || label != "subs/antigravity-model-a" || resp2.Content != "pong from antigravity-model-a" {
		t.Errorf("subs gateway chat: label=%q err=%v resp=%+v", label, err, resp2)
	}
	resp3, _, err := h.App.DevGatewayChat(context.Background(), "custom/fake-model", provider.ChatRequest{Messages: []provider.Message{{Role: "user", Content: "hi"}}})
	if err != nil || resp3.Content == "pong from fake-model" {
		t.Errorf("custom/ was routed to the Subscriptions provider: err=%v resp=%+v", err, resp3)
	}

	// 8. Signing out removes the account, the provider and the sidecar.
	h.postJSON("/api/subscriptions", map[string]string{"action": "logout", "provider": "antigravity"})
	waitFor(t, "logout to tear everything down", 15*time.Second, func() bool {
		st = h.subscriptions(t)
		_, still := h.providerNamed(t, "Subscriptions")
		return len(st.Accounts) == 0 && !st.Running && !still
	})
	for _, m := range h.App.ListGatewayModels() {
		if strings.HasPrefix(m.ID, "subs/") {
			t.Errorf("gateway still lists %s after logout", m.ID)
		}
	}
}

func TestSubscriptions_RestRejectsBadInput(t *testing.T) {
	h := NewHarness(t)
	for _, body := range []map[string]string{
		{"action": "login", "provider": "nope"},
		{"action": "login"},
		{"action": "logout", "provider": "nope"},
		{"action": "explode"},
	} {
		resp := h.postJSON("/api/subscriptions", body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%v -> %d, want 400", body, resp.StatusCode)
		}
		resp.Body.Close()
	}
}
