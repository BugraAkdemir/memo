// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"memo/internal/config"
)

func TestClaudeCodeSettingsPath_UnderHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path, err := claudeCodeSettingsPath()
	if err != nil {
		t.Fatalf("claudeCodeSettingsPath: %v", err)
	}
	want := filepath.Join(home, ".claude", "settings.json")
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
}

func TestReadJSONObject_MissingFileReturnsEmpty(t *testing.T) {
	doc, err := readJSONObject(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("readJSONObject: %v", err)
	}
	if len(doc) != 0 {
		t.Errorf("doc = %+v, want empty", doc)
	}
}

func TestReadJSONObject_ParsesExistingRealisticFile(t *testing.T) {
	// A trimmed-down but structurally real Claude Code settings.json — the
	// point is that readJSONObject must decode arbitrary nested structure
	// (hooks arrays of objects) without Memo's Go structs knowing anything
	// about that schema, since it's just parsed as map[string]any.
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	content := `{
		"hooks": {
			"PreToolUse": [
				{"matcher": "Grep|Glob", "hooks": [{"type": "command", "command": "echo hi", "timeout": 5}]}
			]
		},
		"model": "opus",
		"env": {"SOME_OTHER_VAR": "keep-me"}
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	doc, err := readJSONObject(path)
	if err != nil {
		t.Fatalf("readJSONObject: %v", err)
	}
	if doc["model"] != "opus" {
		t.Errorf("model = %v", doc["model"])
	}
	hooks, ok := doc["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks = %+v, want a map", doc["hooks"])
	}
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Errorf("hooks missing PreToolUse: %+v", hooks)
	}
}

func TestWriteJSONObject_RoundTripsAndPreservesPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	// Pre-create with a distinctive, non-default permission so we can prove
	// writeJSONObject preserved it rather than defaulting to 0600.
	if err := os.WriteFile(path, []byte(`{}`), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	doc := map[string]any{"hello": "world", "n": float64(42)}
	if err := writeJSONObject(path, doc); err != nil {
		t.Fatalf("writeJSONObject: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("mode = %v, want 0644 preserved from the pre-existing file", info.Mode().Perm())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["hello"] != "world" || got["n"] != float64(42) {
		t.Errorf("got = %+v", got)
	}

	// No leftover temp file from the atomic write.
	if _, err := os.Stat(path + ".memo-tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file was not cleaned up: err = %v", err)
	}
}

func TestWriteJSONObject_NewFileDefaultsTo0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := writeJSONObject(path, map[string]any{"a": "b"}); err != nil {
		t.Fatalf("writeJSONObject: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode = %v, want 0600 for a brand-new file that may carry a token", info.Mode().Perm())
	}
}

func TestApplyConnectEnv_FreshConnectNoPriorEnv(t *testing.T) {
	env := map[string]any{}
	got := applyConnectEnv(env, config.ClaudeCodeCLIState{}, "http://127.0.0.1:8090", "memo-token-123")

	if env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:8090" || env["ANTHROPIC_API_KEY"] != "memo-token-123" {
		t.Errorf("env = %+v", env)
	}
	if !got.Connected {
		t.Error("Connected = false, want true")
	}
	if got.PrevBaseURLSet || got.PrevAPIKeySet {
		t.Errorf("got = %+v, want no prior values backed up (there were none)", got)
	}
}

func TestApplyConnectEnv_BacksUpExistingUserValues(t *testing.T) {
	env := map[string]any{
		"ANTHROPIC_BASE_URL": "https://my-custom-proxy.example.com",
		"ANTHROPIC_API_KEY":  "sk-user-own-key",
		"SOME_OTHER_VAR":     "untouched",
	}
	got := applyConnectEnv(env, config.ClaudeCodeCLIState{}, "http://127.0.0.1:8090", "memo-token")

	if !got.PrevBaseURLSet || got.PrevBaseURL != "https://my-custom-proxy.example.com" {
		t.Errorf("PrevBaseURL not backed up correctly: %+v", got)
	}
	if !got.PrevAPIKeySet || got.PrevAPIKey != "sk-user-own-key" {
		t.Errorf("PrevAPIKey not backed up correctly: %+v", got)
	}
	if env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:8090" {
		t.Errorf("ANTHROPIC_BASE_URL not overwritten: %v", env["ANTHROPIC_BASE_URL"])
	}
	if env["SOME_OTHER_VAR"] != "untouched" {
		t.Errorf("unrelated env key was touched: %+v", env)
	}
}

// TestApplyConnectEnv_ReconnectDoesNotClobberBackup is the regression test
// for the exact bug this two-call structure exists to prevent: calling
// Connect a second time (e.g. the base URL changed, or the user double-
// clicked) must not treat Memo's own already-written values as "the user's
// original configuration" and back those up instead.
func TestApplyConnectEnv_ReconnectDoesNotClobberBackup(t *testing.T) {
	env := map[string]any{
		"ANTHROPIC_BASE_URL": "https://original-custom.example.com",
		"ANTHROPIC_API_KEY":  "sk-original",
	}
	first := applyConnectEnv(env, config.ClaudeCodeCLIState{}, "http://127.0.0.1:8090", "memo-token")
	// Second call simulates a reconnect with a different port — env now
	// holds Memo's own values from the first call, and first.Connected is
	// true, so the backup must be left exactly as it was.
	second := applyConnectEnv(env, first, "http://127.0.0.1:9999", "memo-token")

	if second.PrevBaseURL != "https://original-custom.example.com" {
		t.Errorf("PrevBaseURL = %q, want the ORIGINAL user value preserved across reconnect, not Memo's own", second.PrevBaseURL)
	}
	if second.PrevAPIKey != "sk-original" {
		t.Errorf("PrevAPIKey = %q, want the original preserved", second.PrevAPIKey)
	}
	if env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:9999" {
		t.Errorf("env not updated to the new base URL on reconnect: %v", env["ANTHROPIC_BASE_URL"])
	}
}

func TestApplyDisconnectEnv_RestoresPriorValues(t *testing.T) {
	env := map[string]any{
		"ANTHROPIC_BASE_URL": "http://127.0.0.1:8090",
		"ANTHROPIC_API_KEY":  "memo-token",
	}
	st := config.ClaudeCodeCLIState{
		Connected:      true,
		PrevBaseURLSet: true, PrevBaseURL: "https://original-custom.example.com",
		PrevAPIKeySet: true, PrevAPIKey: "sk-original",
	}
	applyDisconnectEnv(env, st)

	if env["ANTHROPIC_BASE_URL"] != "https://original-custom.example.com" {
		t.Errorf("ANTHROPIC_BASE_URL = %v, want restored", env["ANTHROPIC_BASE_URL"])
	}
	if env["ANTHROPIC_API_KEY"] != "sk-original" {
		t.Errorf("ANTHROPIC_API_KEY = %v, want restored", env["ANTHROPIC_API_KEY"])
	}
}

func TestApplyDisconnectEnv_RemovesKeysWhenNoPriorValue(t *testing.T) {
	env := map[string]any{
		"ANTHROPIC_BASE_URL": "http://127.0.0.1:8090",
		"ANTHROPIC_API_KEY":  "memo-token",
		"SOME_OTHER_VAR":     "untouched",
	}
	st := config.ClaudeCodeCLIState{Connected: true} // nothing was there before
	applyDisconnectEnv(env, st)

	if _, ok := env["ANTHROPIC_BASE_URL"]; ok {
		t.Errorf("ANTHROPIC_BASE_URL should have been removed entirely, got %v", env["ANTHROPIC_BASE_URL"])
	}
	if _, ok := env["ANTHROPIC_API_KEY"]; ok {
		t.Errorf("ANTHROPIC_API_KEY should have been removed entirely, got %v", env["ANTHROPIC_API_KEY"])
	}
	if env["SOME_OTHER_VAR"] != "untouched" {
		t.Errorf("unrelated env key was touched: %+v", env)
	}
}

// TestConnectDisconnectRoundTrip_PreservesUnrelatedFileContent exercises
// the full read -> mutate -> write cycle (without touching App/config.Save
// — see applyConnectEnv's doc comment for why those stay separate) against
// a realistic settings.json, proving hooks and other keys survive both a
// connect and a subsequent disconnect untouched.
func TestConnectDisconnectRoundTrip_PreservesUnrelatedFileContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	original := `{
		"hooks": {"PreToolUse": [{"matcher": "Grep", "hooks": [{"type": "command", "command": "echo hi"}]}]},
		"model": "opus"
	}`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	// Connect.
	doc, err := readJSONObject(path)
	if err != nil {
		t.Fatalf("readJSONObject: %v", err)
	}
	env, _ := doc["env"].(map[string]any)
	if env == nil {
		env = map[string]any{}
	}
	st := applyConnectEnv(env, config.ClaudeCodeCLIState{}, "http://127.0.0.1:8090", "memo-token")
	doc["env"] = env
	if err := writeJSONObject(path, doc); err != nil {
		t.Fatalf("writeJSONObject: %v", err)
	}

	afterConnect, err := readJSONObject(path)
	if err != nil {
		t.Fatalf("readJSONObject after connect: %v", err)
	}
	if afterConnect["model"] != "opus" {
		t.Errorf("model lost after connect: %+v", afterConnect)
	}
	if _, ok := afterConnect["hooks"]; !ok {
		t.Errorf("hooks lost after connect: %+v", afterConnect)
	}
	connectedEnv := afterConnect["env"].(map[string]any)
	if connectedEnv["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:8090" {
		t.Errorf("env not set after connect: %+v", connectedEnv)
	}

	// Disconnect.
	doc2, err := readJSONObject(path)
	if err != nil {
		t.Fatalf("readJSONObject before disconnect: %v", err)
	}
	env2, _ := doc2["env"].(map[string]any)
	applyDisconnectEnv(env2, st)
	if len(env2) == 0 {
		delete(doc2, "env")
	} else {
		doc2["env"] = env2
	}
	if err := writeJSONObject(path, doc2); err != nil {
		t.Fatalf("writeJSONObject: %v", err)
	}

	final, err := readJSONObject(path)
	if err != nil {
		t.Fatalf("readJSONObject after disconnect: %v", err)
	}
	if final["model"] != "opus" {
		t.Errorf("model lost after disconnect: %+v", final)
	}
	if _, ok := final["hooks"]; !ok {
		t.Errorf("hooks lost after disconnect: %+v", final)
	}
	if _, ok := final["env"]; ok {
		t.Errorf("env should have been removed entirely (nothing was there originally): %+v", final)
	}
}
