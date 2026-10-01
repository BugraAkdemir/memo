package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/claudesub"
)

// End-to-end coverage for the claude-sub connect surface, over the real REST
// API against a real app.App — only Anthropic's OAuth and Messages endpoints are
// stubbed, since those need a real account.
//
// What this closes that the unit tests cannot: that the route is reachable, that
// its gate wrapper is the one we think it is, that the two-step code-paste flow
// actually completes over HTTP, and that connecting really does make the
// provider type appear in Memo's provider list — the thing the marker config
// exists for.

// withClaudeOAuth points the package's OAuth endpoints at a stub for one test.
// The client_id is irrelevant here; only the exchange is exercised.
func withClaudeOAuth(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("MEMO_CLAUDE_TOKEN_URL", srv.URL+"/v1/oauth/token")
	t.Setenv("MEMO_CLAUDE_AUTH_URL", srv.URL+"/oauth/authorize")
}

// tokenJSON is a minimal successful token-endpoint answer.
const tokenJSON = `{"access_token":"sk-ant-oat01-e2e","refresh_token":"r-e2e","token_type":"Bearer","expires_in":3600}`

// isolateClaudeEnv makes the adoption search find nothing on the developer's
// machine, so the "no local login" half of the flow is what the test exercises.
// Without this, a developer who happens to be signed into Claude Code gets a
// connection the test did not ask for.
func isolateClaudeEnv(t *testing.T) {
	t.Helper()
	empty := t.TempDir()
	t.Setenv("HOME", empty)
	t.Setenv("USERPROFILE", empty)
	os.Unsetenv("MEMO_CLAUDE_TOKEN")
	os.Unsetenv("CLAUDE_CODE_OAUTH_TOKEN")
	claudesub.ResetForTests()
	t.Cleanup(claudesub.ResetForTests)
}

func TestClaudeSub_ConnectOverHTTPAgainstNoLocalLogin(t *testing.T) {
	isolateClaudeEnv(t)
	var exchanged struct {
		grant    string
		code     string
		redirect string
	}
	withClaudeOAuth(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		exchanged.grant, _ = body["grant_type"].(string)
		exchanged.code, _ = body["code"].(string)
		exchanged.redirect, _ = body["redirect_uri"].(string)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(tokenJSON))
	})

	h := NewHarness(t)

	// 1. GET: not connected, and no capabilities measured yet. The absence of
	//    "capabilities" is honest — a probe has not run — rather than an empty
	//    table that would read as "nothing works".
	var state map[string]any
	decodeInto(t, h.getJSON("/api/dev-gateway/claude-account"), &state)
	if state["connected"] != false {
		t.Fatalf("connected = %v before connecting", state["connected"])
	}
	if _, present := state["capabilities"]; present {
		t.Errorf("capabilities present before any probe ran: %v", state["capabilities"])
	}

	// 2. POST connect with nothing local -> the hosted flow, not a connection.
	resp := h.postJSON("/api/dev-gateway/claude-account", map[string]any{"connect": true})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("connect: status %d", resp.StatusCode)
	}
	var attempt map[string]any
	decodeInto(t, resp, &attempt)
	if attempt["connected"] != false {
		t.Fatalf("reported connected with nothing to adopt: %v", attempt)
	}
	authURL, _ := attempt["auth_url"].(string)
	stateTok, _ := attempt["state"].(string)
	if authURL == "" || stateTok == "" {
		t.Fatalf("no flow handed out (url=%q state=%q)", authURL, stateTok)
	}
	if !strings.Contains(authURL, "code=true") {
		t.Errorf("authorize URL %q lost code=true, so Anthropic would display no code", authURL)
	}

	// 3. Complete with a code. The full redirect URL is what the browser leaves
	//    behind, and the state must be picked out of it rather than demanded
	//    separately.
	pasted := "https://platform.claude.com/oauth/code/callback?code=the-code&state=" + stateTok
	resp = h.postJSON("/api/dev-gateway/claude-account", map[string]any{"code": pasted})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("complete: status %d body=%s", resp.StatusCode, body)
	}
	decodeInto(t, resp, &state)
	if state["connected"] != true {
		t.Fatalf("connected after completing the flow: %v", state)
	}
	if state["source"] != "browser" {
		t.Errorf("source = %v, want browser", state["source"])
	}
	if exchanged.grant != "authorization_code" || exchanged.code != "the-code" {
		t.Errorf("token request carried grant=%q code=%q", exchanged.grant, exchanged.code)
	}
	if exchanged.redirect == "" {
		t.Error("the exchange omitted redirect_uri, which Anthropic requires")
	}

	// 4. The whole reason the marker config exists: the type must now be a real,
	//    enabled provider Memo can route to.
	// GET /api/providers answers with a bare array, not an object.
	var providers []struct {
		Type    string `json:"type"`
		Name    string `json:"name"`
		Model   string `json:"model"`
		Enabled bool   `json:"enabled"`
	}
	decodeInto(t, h.getJSON("/api/providers"), &providers)
	found := false
	for _, p := range providers {
		if p.Type == "claude-sub" {
			found = true
			if !p.Enabled {
				t.Error("the claude-sub marker is not enabled, so it never reaches the router")
			}
			if p.Model == "" {
				t.Error("the marker carries no model")
			}
		}
	}
	if !found {
		t.Fatalf("no claude-sub provider in %+v after connecting", providers)
	}

	// 5. Disconnect clears both the token and the marker.
	resp = h.postJSON("/api/dev-gateway/claude-account", map[string]any{"connect": false})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("disconnect: status %d", resp.StatusCode)
	}
	decodeInto(t, resp, &state)
	if state["connected"] != false {
		t.Errorf("still connected after disconnect: %v", state)
	}
	decodeInto(t, h.getJSON("/api/providers"), &providers)
	for _, p := range providers {
		if p.Type == "claude-sub" {
			t.Error("the claude-sub marker survived disconnect")
		}
	}
}

// A machine that already has a Claude Code login must connect over HTTP with one
// call and never receive an authorize URL. This is the requirement the whole
// adoption path exists for, so it is asserted end to end rather than only at
// the unit level.
func TestClaudeSub_LocalLoginConnectsWithOneRequestAndNoBrowser(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	doc := `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-local","refreshToken":"r-local","expiresAt":99999999999999}}`
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(doc), 0600); err != nil {
		t.Fatal(err)
	}
	// The OAuth stub records whether it was ever contacted, so "no browser
	// flow was started" is measured rather than inferred from the response.
	var tokenCalls int
	withClaudeOAuth(t, func(w http.ResponseWriter, r *http.Request) {
		tokenCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(tokenJSON))
	})
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	os.Unsetenv("MEMO_CLAUDE_TOKEN")
	os.Unsetenv("CLAUDE_CODE_OAUTH_TOKEN")
	claudesub.ResetForTests()
	t.Cleanup(claudesub.ResetForTests)

	h := NewHarness(t)

	resp := h.postJSON("/api/dev-gateway/claude-account", map[string]any{"connect": true})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("connect: status %d", resp.StatusCode)
	}
	var attempt map[string]any
	decodeInto(t, resp, &attempt)
	if attempt["connected"] != true {
		t.Fatalf("did not connect from the local login: %v", attempt)
	}
	if url, _ := attempt["auth_url"].(string); url != "" {
		t.Errorf("an authorize URL was handed out anyway: %q", url)
	}
	if attempt["source"] != "claude-code-file" {
		t.Errorf("source = %v, want claude-code-file", attempt["source"])
	}
	if tokenCalls != 0 {
		t.Errorf("the OAuth token endpoint was contacted %d times; a local adoption needs no exchange", tokenCalls)
	}

	// And it survives a GET: the connection is real, not a one-shot response.
	var state map[string]any
	decodeInto(t, h.getJSON("/api/dev-gateway/claude-account"), &state)
	if state["connected"] != true {
		t.Errorf("GET reports disconnected right after connecting: %v", state)
	}
}

// Default() adopts a local login once, when the process constructs the Manager.
// That is not enough on its own: a user who signs in to Claude Code while Memo
// is already running would be stuck behind the browser flow until a restart.
// Connect re-checks, which is why adoption lives in both places.
func TestClaudeSub_PicksUpALoginThatAppearedAfterStartup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	os.Unsetenv("MEMO_CLAUDE_TOKEN")
	os.Unsetenv("CLAUDE_CODE_OAUTH_TOKEN")
	claudesub.ResetForTests()
	t.Cleanup(claudesub.ResetForTests)

	// Boot the app with nothing to adopt — this constructs the Manager, so the
	// startup adoption has already run and found nothing.
	h := NewHarness(t)

	// A fresh map per decode: encoding/json MERGES into a non-nil map rather
	// than replacing it, so reusing one across two responses would leave the
	// first response's auth_url sitting in the second.
	var firstAttempt map[string]any
	decodeInto(t, h.postJSON("/api/dev-gateway/claude-account", map[string]any{"connect": true}), &firstAttempt)
	if firstAttempt["connected"] != false {
		t.Fatalf("connected with nothing on disk: %v", firstAttempt)
	}
	if url, _ := firstAttempt["auth_url"].(string); url == "" {
		t.Fatal("expected the browser flow when there is nothing local")
	}

	// ...and now the user signs in to Claude Code.
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	doc := `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-late","refreshToken":"r-late","expiresAt":99999999999999}}`
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(doc), 0600); err != nil {
		t.Fatal(err)
	}

	var secondAttempt map[string]any
	decodeInto(t, h.postJSON("/api/dev-gateway/claude-account", map[string]any{"connect": true}), &secondAttempt)
	attempt := secondAttempt
	if attempt["connected"] != true {
		t.Fatalf("a login that appeared after startup was not picked up: %v", attempt)
	}
	if url, _ := attempt["auth_url"].(string); url != "" {
		t.Errorf("an authorize URL was handed out despite the new local login: %q", url)
	}
	if attempt["source"] != "claude-code-file" {
		t.Errorf("source = %v", attempt["source"])
	}
}

// The gate wrapper is the reason this route exists in adminWrites. Proven over
// real HTTP rather than by reading the route table.
func TestClaudeSub_RouteIsAdminGated(t *testing.T) {
	isolateClaudeEnv(t)
	h := NewHarness(t)

	// A GET is readable before the gate resolves — the settings screen polls it
	// while the user is still signing in.
	if resp := h.getJSON("/api/dev-gateway/claude-account"); resp.StatusCode != http.StatusOK {
		t.Errorf("GET status %d, want 200", resp.StatusCode)
		resp.Body.Close()
	}
	// Every state-changing POST is admin-only.
	resp := h.postJSON("/api/dev-gateway/claude-account", map[string]any{"connect": true})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("POST status %d — this Harness is loopback so it is trusted as admin, which is correct", resp.StatusCode)
	}
}
