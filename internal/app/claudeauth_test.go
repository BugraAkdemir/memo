package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"memo/internal/claudesub"
	"memo/internal/config"
	"memo/internal/provider"
)

func TestClaudeSubMarkerConfig(t *testing.T) {
	cfg := claudeSubMarkerConfig("")
	if err := cfg.Validate(); err != nil {
		t.Fatalf("marker config does not validate: %v", err)
	}
	if cfg.Type != provider.ProviderClaudeSub {
		t.Errorf("Type = %q", cfg.Type)
	}
	if cfg.Name != claudeSubProviderName {
		t.Errorf("Name = %q", cfg.Name)
	}
	if !cfg.Enabled {
		t.Error("marker must be enabled or the type never reaches the router")
	}
	// The marker is what makes claude-sub visible; it must never carry a
	// credential, because the OAuth token lives encrypted under DataDir().
	if cfg.APIKey != "" {
		t.Errorf("marker carries an API key: %q", cfg.APIKey)
	}
	if cfg.BaseURL != "" {
		t.Errorf("marker carries a base URL: %q", cfg.BaseURL)
	}
	if cfg.Model != claudeSubDefaultModel {
		t.Errorf("default model = %q, want %q", cfg.Model, claudeSubDefaultModel)
	}
	if got := claudeSubMarkerConfig("claude-sonnet-5"); got.Model != "claude-sonnet-5" {
		t.Errorf("explicit model not honoured: %q", got.Model)
	}
}

// A subscription provider must never become the sticky global active provider
// across restarts (BUG_REPORT's sticky-active-provider bug).
func TestIsSessionProviderName_ClaudeSub(t *testing.T) {
	configs := []provider.ProviderConfig{
		{Type: provider.ProviderClaude, Name: "api-claude"},
		{Type: provider.ProviderClaudeSub, Name: claudeSubProviderName},
	}
	if !isSessionProviderName(claudeSubProviderName, configs) {
		t.Error("claude-sub was treated as a sticky active provider")
	}
	if isSessionProviderName("api-claude", configs) {
		t.Error("an ordinary API-key claude provider must stay sticky")
	}
}

// State is reported from the token, not the config flag: a token removed
// behind our back must not read as connected forever.
func TestClaudeAccountState_FollowsTheTokenNotTheFlag(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}

	connected, account, source, model := a.ClaudeAccountState()
	if connected {
		t.Error("a fresh app reported connected")
	}
	if account != "" || source != "" || model != "" {
		t.Errorf("disconnected app returned %q/%q/%q", account, source, model)
	}

	a.cfg.DevGateway.ClaudeSub = config.ClaudeSubState{
		Connected: true, Account: "x", Source: "browser", Model: "claude-x",
	}
	// The flag says connected but there is no token, so this must still read
	// as disconnected.
	if connected, _, _, _ := a.ClaudeAccountState(); connected {
		t.Error("config flag alone reported connected; a stale flag would show a broken provider forever")
	}
}

func TestSetClaudeAccountModel_NotConnectedIsNoop(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	if err := a.SetClaudeAccountModel("claude-sonnet-5"); err != nil {
		t.Fatalf("SetClaudeAccountModel: %v", err)
	}
	if a.cfg.DevGateway.ClaudeSub.Model != "" {
		t.Errorf("model was written while disconnected: %q", a.cfg.DevGateway.ClaudeSub.Model)
	}
	if err := a.SetClaudeAccountModel("  "); err != nil {
		t.Errorf("blank model errored: %v", err)
	}
}

func TestDisconnectClaudeAccount_NotConnectedIsNoop(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	if err := a.DisconnectClaudeAccount(); err != nil {
		t.Fatalf("DisconnectClaudeAccount: %v", err)
	}
}

// Connect must not send an already-signed-in user to a browser. With a Claude
// Code credential present, one call is the whole connection.
func TestConnectClaudeAccount_AdoptsLocalLoginWithoutABrowser(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	doc := `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-local","refreshToken":"r","expiresAt":99999999999999}}`
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(doc), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	os.Unsetenv("MEMO_CLAUDE_TOKEN")
	os.Unsetenv("CLAUDE_CODE_OAUTH_TOKEN")

	a := newTestAppForAuth(t)

	connected, source, authURL, state, err := a.ConnectClaudeAccount()
	if err != nil {
		t.Fatalf("ConnectClaudeAccount: %v", err)
	}
	if !connected {
		t.Fatal("did not connect despite a local Claude Code login being present")
	}
	if authURL != "" || state != "" {
		t.Errorf("a browser flow was handed out anyway (url=%q state=%q)", authURL, state)
	}
	if source != "claude-code-file" {
		t.Errorf("source = %q", source)
	}
	if !a.cfg.DevGateway.ClaudeSub.Connected {
		t.Error("connected state was not recorded")
	}

	// And the marker provider must exist, or the type never reaches the router.
	found := false
	for _, p := range a.GetProviders() {
		if p.Type == provider.ProviderClaudeSub && p.Enabled {
			found = true
		}
	}
	if !found {
		t.Error("no enabled claude-sub marker provider was written")
	}
}

// Clicking Connect a second time must not relabel an adopted login as a
// browser sign-in. This is not cosmetic: it is the only clue the user has about
// why no browser opened, and "signed in through the browser" describes an
// event that never happened.
func TestConnectClaudeAccount_SecondClickKeepsTheRealSource(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	doc := `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-local","refreshToken":"r","expiresAt":99999999999999}}`
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(doc), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	os.Unsetenv("MEMO_CLAUDE_TOKEN")
	os.Unsetenv("CLAUDE_CODE_OAUTH_TOKEN")

	a := newTestAppForAuth(t)
	if _, _, _, _, err := a.ConnectClaudeAccount(); err != nil {
		t.Fatalf("first ConnectClaudeAccount: %v", err)
	}

	// Now make the local source disappear while the token stays: this is a real
	// sequence (the user logs out of Claude Code, or the file is cleaned up,
	// and then clicks Connect again). It is also the only way into the
	// already-connected branch, since while the file exists AdoptLocal wins.
	if err := os.Remove(filepath.Join(dir, ".credentials.json")); err != nil {
		t.Fatal(err)
	}

	connected, source, authURL, _, err := a.ConnectClaudeAccount()
	if err != nil {
		t.Fatalf("second ConnectClaudeAccount: %v", err)
	}
	if !connected {
		t.Fatal("second connect lost the connection")
	}
	if authURL != "" {
		t.Errorf("second click handed out another browser flow: %q", authURL)
	}
	if source != "claude-code-file" {
		t.Errorf("source = %q after a second click; an adopted login was relabelled", source)
	}
}

// With nothing local, Connect must fall back to the hosted flow and hand back
// everything the UI needs to show a paste box.
func TestConnectClaudeAccount_FallsBackToHostedFlow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	os.Unsetenv("MEMO_CLAUDE_TOKEN")
	os.Unsetenv("CLAUDE_CODE_OAUTH_TOKEN")

	a := newTestAppForAuth(t)

	connected, source, authURL, state, err := a.ConnectClaudeAccount()
	if err != nil {
		t.Fatalf("ConnectClaudeAccount: %v", err)
	}
	if connected {
		t.Fatal("reported connected with nothing to adopt")
	}
	if authURL == "" || state == "" {
		t.Fatalf("no flow handed out (url=%q state=%q)", authURL, state)
	}
	if source != "browser" {
		t.Errorf("source = %q, want browser", source)
	}
	if a.cfg.DevGateway.ClaudeSub.Connected {
		t.Error("state was recorded as connected before the code was pasted")
	}
}

// newTestAppForAuth builds the smallest App the connect surface can run
// against. providerCfgMgr must be present because connect writes the marker
// provider through UpdateProvider, and lifecycleCtx because the OAuth exchange
// is bounded by it.
func newTestAppForAuth(t *testing.T) *App {
	t.Helper()
	// Both halves of the leak have to be closed. claudesub.Default() is a
	// process-wide singleton, AND it re-reads DataDir()/claudesub/token.enc —
	// so resetting only the singleton still lets the previous test's adopted
	// token come back off disk. Hence the temp MEMO_DATA_DIR plus
	// config.ResetForTests() (DataDir is cached per process) and the
	// claudesub reset, exactly the combination TestMain documents. Without it
	// this test fails while passing in isolation, which is the worst possible
	// failure mode.
	t.Setenv("MEMO_DATA_DIR", t.TempDir())
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)
	claudesub.ResetForTests()
	t.Cleanup(claudesub.ResetForTests)
	dir := t.TempDir()
	a := &App{
		// claude-sub is a Beta feature; the connect surface refuses without it
		// (TestClaudeSub_RefusedWhileBetaIsOff covers that side).
		cfg:             &config.AppConfig{Beta: true},
		providerCfgMgr:  provider.NewConfigManager(filepath.Join(dir, "providers.json"), make([]byte, 32)),
		lifecycleCtx:    context.Background(),
		lifecycleCancel: func() {},
	}
	return a
}

// writeClaudeCodeLogin puts a Claude Code credential file in a fresh HOME.
func writeClaudeCodeLogin(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	doc := `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-local","refreshToken":"r","expiresAt":99999999999999}}`
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(doc), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	os.Unsetenv("MEMO_CLAUDE_TOKEN")
	os.Unsetenv("CLAUDE_CODE_OAUTH_TOKEN")
	return home
}

func hasEnabledClaudeSub(a *App) bool {
	for _, p := range a.GetProviders() {
		if p.Type == provider.ProviderClaudeSub && p.Enabled {
			return true
		}
	}
	return false
}

// claude-sub is a Beta feature in the backend too, not only in the settings
// UI: with Beta off nothing connects and nothing is adopted.
func TestClaudeSub_RefusedWhileBetaIsOff(t *testing.T) {
	writeClaudeCodeLogin(t)
	a := newTestAppForAuth(t)
	a.cfg.Beta = false

	if _, _, _, _, err := a.ConnectClaudeAccount(); err == nil {
		t.Error("ConnectClaudeAccount succeeded with Beta off")
	}
	if err := a.CompleteClaudeAuth("abc#def", ""); err == nil {
		t.Error("CompleteClaudeAuth succeeded with Beta off")
	}
	a.syncClaudeSubWithBeta()
	if hasEnabledClaudeSub(a) || a.cfg.DevGateway.ClaudeSub.Connected {
		t.Error("a local Claude Code login was adopted while Beta is off")
	}
}

// Switching Beta off removes the provider but keeps the account; switching it
// back on restores it without another sign-in.
func TestClaudeSub_BetaToggleRemovesAndRestoresTheProvider(t *testing.T) {
	writeClaudeCodeLogin(t)
	a := newTestAppForAuth(t)
	if _, _, _, _, err := a.ConnectClaudeAccount(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if !hasEnabledClaudeSub(a) {
		t.Fatal("setup: no claude-sub provider after connecting")
	}

	a.cfg.Beta = false
	a.syncClaudeSubWithBeta()
	if hasEnabledClaudeSub(a) {
		t.Error("the claude-sub provider is still routable with Beta off")
	}
	if !claudesub.Default().Connected() {
		t.Error("turning Beta off threw the token away")
	}

	a.cfg.Beta = true
	a.syncClaudeSubWithBeta()
	if !hasEnabledClaudeSub(a) {
		t.Error("turning Beta back on did not restore the provider")
	}
	if c, _, src, _ := a.ClaudeAccountState(); !c || src != claudesub.SourceClaudeCodeFile {
		t.Errorf("state after Beta on = connected:%v source:%q", c, src)
	}
}

// Disconnect must survive a restart. Before, Default() adopted the very same
// Claude Code login again on the next start and the account came back
// (reproduced live against an isolated backend).
func TestClaudeSub_DisconnectSurvivesARestart(t *testing.T) {
	writeClaudeCodeLogin(t)
	a := newTestAppForAuth(t)
	if _, _, _, _, err := a.ConnectClaudeAccount(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := a.DisconnectClaudeAccount(); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	// A restart: fresh Manager over the same data dir, same config.
	claudesub.ResetForTests()
	a.syncClaudeSubWithBeta()

	if c, _, _, _ := a.ClaudeAccountState(); c {
		t.Error("the account came back after a restart although the user disconnected")
	}
	if hasEnabledClaudeSub(a) {
		t.Error("the claude-sub provider came back after a restart")
	}

	// An explicit Connect is the user changing their mind.
	if c, _, _, _, err := a.ConnectClaudeAccount(); err != nil || !c {
		t.Fatalf("explicit reconnect: connected=%v err=%v", c, err)
	}
	if a.cfg.DevGateway.ClaudeSub.UserDisconnected {
		t.Error("an explicit Connect left UserDisconnected set")
	}
}

// Adoption at startup records where the login came from, not a generic label.
func TestClaudeSub_StartupAdoptionRecordsTheRealSource(t *testing.T) {
	writeClaudeCodeLogin(t)
	a := newTestAppForAuth(t)
	a.syncClaudeSubWithBeta()
	if c, _, src, _ := a.ClaudeAccountState(); !c || src != claudesub.SourceClaudeCodeFile {
		t.Errorf("after startup adoption: connected=%v source=%q, want %q", c, src, claudesub.SourceClaudeCodeFile)
	}
}
