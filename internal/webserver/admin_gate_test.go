package webserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"memo/internal/config"
	"memo/internal/provider"
)

// providersBridge is the stub bridge with a real provider list.
type providersBridge struct {
	*swarmStubBridge
	list []provider.ProviderConfig
}

func (b *providersBridge) GetProviders() []provider.ProviderConfig { return b.list }

func restrictedBridge() *providersBridge {
	return &providersBridge{
		swarmStubBridge: &swarmStubBridge{
			sessionRole: func(token string) (string, bool) {
				if token == "admin-session" {
					return "admin", true
				}
				return "user", true
			},
			sessionPermissions: func(token string) (config.AccountPermissions, bool) {
				if token == "admin-session" {
					return config.AccountPermissions{Models: true}, true
				}
				return config.AccountPermissions{}, true // every box unchecked
			},
			devGatewayTokenValue: "memo-gateway-secret-123456",
		},
		list: []provider.ProviderConfig{{Name: "p", Type: provider.ProviderCustom, APIKey: "sk-live-SECRET-abcdef1234"}},
	}
}

func startRestricted(t *testing.T) string {
	t.Helper()
	port := freePort(t)
	s := New(restrictedBridge())
	if err := s.StartHTTPWithAddr(port, "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Stop() })
	waitForListening(t, port)
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func do(t *testing.T, method, url, token, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("X-Memo-Token", token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// TestDestructiveEndpoints_AdminOnly was found live: a "user" account with
// every permission box unchecked shut the backend down with one POST, and
// could equally export a full backup (every chat, the memory DB, and
// providers.json together with the machine key that decrypts it), import
// over all data, wipe it, or uninstall Memo — none of these handlers
// checked anything.
func TestDestructiveEndpoints_AdminOnly(t *testing.T) {
	base := startRestricted(t)
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/export"},
		{"POST", "/api/import"},
		{"POST", "/api/wipe"},
		{"POST", "/api/uninstall"},
		{"POST", "/api/shutdown"},
		{"POST", "/api/cli/remove"},
		{"POST", "/api/cli/reinstall"},
		{"POST", "/api/sync/pull"},
		{"POST", "/api/sync/now"},
		{"POST", "/api/sync/disconnect"},
		{"PUT", "/api/sync/settings"},
		{"POST", "/api/dev-gateway/token/rotate"},
		{"GET", "/api/dev-gateway/logs"},
		{"PUT", "/api/dev-gateway/config"},
		// Connect / disconnect / model-switch are state-changing admin writes
		// on both subscription-account routes. Their GET is deliberately open
		// (the settings screen reads it before the gate resolves), which is
		// only safe as long as the POSTs are listed here.
		{"POST", "/api/dev-gateway/google-account"},
		{"POST", "/api/dev-gateway/claude-account"},
		// Signing a vendor account in or out of the bundled CLIProxyAPI sidecar.
		{"POST", "/api/subscriptions"},
		{"POST", "/api/v1/wipe"},
	} {
		if st, _ := do(t, c.method, base+c.path, "user-session", "{}"); st != http.StatusForbidden {
			t.Errorf("%s %s as a restricted user: status %d, want 403", c.method, c.path, st)
		}
	}
	// An admin session still gets through (the stub's wipe is a no-op).
	if st, body := do(t, "POST", base+"/api/wipe", "admin-session", "{}"); st != http.StatusOK {
		t.Errorf("POST /api/wipe as admin: status %d (%s), want 200", st, body)
	}
}

// TestSecretsAreRedactedForAccountsThatMayNotManageThem: the provider list
// is read by every client (the chat header's model picker), but it used to
// carry every API key in plaintext; the dev-gateway token likewise.
func TestSecretsAreRedactedForAccountsThatMayNotManageThem(t *testing.T) {
	base := startRestricted(t)

	_, body := do(t, "GET", base+"/api/providers", "user-session", "")
	if strings.Contains(body, "SECRET") {
		t.Errorf("restricted user got a plaintext API key: %s", body)
	}
	var list []map[string]any
	json.Unmarshal([]byte(body), &list)
	if len(list) != 1 || list[0]["api_key"] != "••••1234" {
		t.Errorf("restricted provider list = %s, want the key masked but present", body)
	}
	if _, body := do(t, "GET", base+"/api/providers", "admin-session", ""); !strings.Contains(body, "sk-live-SECRET-abcdef1234") {
		t.Errorf("admin should still get the real key to edit it: %s", body)
	}

	if _, body := do(t, "GET", base+"/api/dev-gateway/config", "user-session", ""); strings.Contains(body, "gateway-secret") {
		t.Errorf("restricted user got the dev-gateway token: %s", body)
	}
	if _, body := do(t, "GET", base+"/api/dev-gateway/config", "admin-session", ""); !strings.Contains(body, "memo-gateway-secret-123456") {
		t.Errorf("admin should get the dev-gateway token: %s", body)
	}
}

func TestRedactSecrets_NestedAndShortValues(t *testing.T) {
	in := map[string]any{"a": []any{map[string]any{"api_key": "abc", "token": "0123456789xyz"}}, "token_path": "/keep/me"}
	out, _ := json.Marshal(redactSecrets(in))
	got := string(out)
	for _, want := range []string{`"api_key":"••••"`, `"token":"••••9xyz"`, `"token_path":"/keep/me"`} {
		if !strings.Contains(got, want) {
			t.Errorf("redactSecrets = %s, missing %s", got, want)
		}
	}
}

// The model selector's endpoint spends a STORED provider key (it lists models
// and switches the model server-side), so it must be gated like the provider
// list itself — unlike /api/providers/models, which only uses a key the caller
// supplies.
func TestProviderModelEndpoint_NeedsTheModelsPermission(t *testing.T) {
	base := startRestricted(t)
	for _, c := range []struct{ method, path, body string }{
		{"GET", "/api/providers/model?name=p", ""},
		{"PUT", "/api/providers/model", `{"name":"p","model":"x"}`},
	} {
		if st, _ := do(t, c.method, base+c.path, "user-session", c.body); st != http.StatusForbidden {
			t.Errorf("%s %s without the models permission: status %d, want 403", c.method, c.path, st)
		}
	}
	if st, body := do(t, "GET", base+"/api/providers/model?name=p", "admin-session", ""); st != http.StatusOK {
		t.Errorf("GET as an account that has the permission: status %d (%s), want 200", st, body)
	}
	if st, _ := do(t, "GET", base+"/api/providers/model", "admin-session", ""); st != http.StatusBadRequest {
		t.Errorf("GET without ?name: status %d, want 400", st)
	}
	if st, _ := do(t, "PUT", base+"/api/providers/model", "admin-session", `{"name":"p"}`); st != http.StatusBadRequest {
		t.Errorf("PUT without a model: status %d, want 400", st)
	}
}
