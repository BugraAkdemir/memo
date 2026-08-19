// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"memo/internal/config"
)

// claudeCodeSettingsPath returns the Claude Code CLI's own settings file —
// deliberately the CLI/SDK's config (~/.claude/settings.json, documented to
// support an "env" object of string environment variables applied to every
// `claude` invocation), NOT any separate Claude desktop app's config, which
// lives elsewhere and is untouched by this. Same path on every OS: the CLI
// is a cross-platform Node binary that resolves its config dir the same way
// regardless of platform.
func claudeCodeSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// readJSONObject reads path as a generic JSON object, preserving every key
// Memo doesn't know about (hooks, permissions, all the rest of Claude
// Code's real settings schema) so a later writeJSONObject only ever touches
// the two keys this feature owns. A missing file reads as an empty object,
// not an error — Claude Code creates settings.json lazily.
func readJSONObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

// writeJSONObject writes doc back to path atomically (write to a temp file
// in the same directory, then rename) so a crash mid-write can never leave
// a real config file — one with hooks and other settings a user actually
// depends on — truncated or corrupted. Preserves the existing file's
// permission bits if it already existed; a brand new file (this feature's
// first run) defaults to 0600 since it may now carry a bearer token.
func writeJSONObject(path string, doc map[string]any) error {
	mode := os.FileMode(0600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	tmp := path + ".memo-tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename %s: %w", tmp, err)
	}
	return nil
}

// applyConnectEnv mutates env in place to point ANTHROPIC_BASE_URL/
// ANTHROPIC_API_KEY at this gateway, and returns the updated
// ClaudeCodeCLIState to persist. Pure — no file or config I/O — so it's
// directly unit-testable without config.Save's process-wide DataDir
// caching getting in the way (see TestGetSetDevGatewayConfig's doc comment
// in devgateway_test.go for the same concern with a different setter,
// which this mirrors).
//
// The backup only happens on the FIRST connect (prev.Connected == false) —
// a second Connect call (e.g. re-clicking, or reconnecting after the base
// URL changed) must not overwrite the backup with Memo's own
// already-written values, or the eventual disconnect would "restore"
// Memo's own settings instead of the user's original ones.
func applyConnectEnv(env map[string]any, prev config.ClaudeCodeCLIState, baseURL, token string) config.ClaudeCodeCLIState {
	next := prev
	if !prev.Connected {
		if v, ok := env["ANTHROPIC_BASE_URL"].(string); ok {
			next.PrevBaseURLSet, next.PrevBaseURL = true, v
		} else {
			next.PrevBaseURLSet, next.PrevBaseURL = false, ""
		}
		if v, ok := env["ANTHROPIC_API_KEY"].(string); ok {
			next.PrevAPIKeySet, next.PrevAPIKey = true, v
		} else {
			next.PrevAPIKeySet, next.PrevAPIKey = false, ""
		}
	}
	env["ANTHROPIC_BASE_URL"] = baseURL
	// Always written, even while "Require API Key" is currently off — the
	// gateway simply ignores it in that case (devGatewayAuthOK short-
	// circuits to true), and writing it now means turning "Require API Key"
	// on later doesn't silently break an already-connected CLI.
	env["ANTHROPIC_API_KEY"] = token
	next.Connected = true
	return next
}

// applyDisconnectEnv mutates env in place to restore whatever
// applyConnectEnv backed up (or removes the two keys entirely if there was
// nothing there before). Pure, for the same testability reason as
// applyConnectEnv.
func applyDisconnectEnv(env map[string]any, st config.ClaudeCodeCLIState) {
	if st.PrevBaseURLSet {
		env["ANTHROPIC_BASE_URL"] = st.PrevBaseURL
	} else {
		delete(env, "ANTHROPIC_BASE_URL")
	}
	if st.PrevAPIKeySet {
		env["ANTHROPIC_API_KEY"] = st.PrevAPIKey
	} else {
		delete(env, "ANTHROPIC_API_KEY")
	}
}

// applyConnectModel mutates doc's top-level "model" field (documented by
// Claude Code CLI's own settings schema as "Override the default model used
// by Claude Code" — distinct from the env block) and returns prev with its
// model-tracking fields updated. Mirrors applyConnectEnv's backup-only-on-
// first-connect discipline via the same prev.Connected check, so repeatedly
// changing the model dropdown while already connected never clobbers the
// backup of whatever model the user's settings.json had before Memo ever
// touched it.
//
// model == "" means "no override chosen" — doc is left untouched (an
// existing custom model the user configured outside Memo survives), but the
// state still records the (now-empty) Model so the dropdown reflects it.
func applyConnectModel(doc map[string]any, prev config.ClaudeCodeCLIState, model string) config.ClaudeCodeCLIState {
	next := prev
	if !prev.Connected {
		if v, ok := doc["model"].(string); ok {
			next.PrevModelSet, next.PrevModel = true, v
		} else {
			next.PrevModelSet, next.PrevModel = false, ""
		}
	}
	if model != "" {
		doc["model"] = model
	}
	next.Model = model
	return next
}

// applyDisconnectModel restores whatever applyConnectModel backed up (or
// removes the "model" key entirely if there was nothing there before).
// Pure, for the same testability reason as applyDisconnectEnv.
func applyDisconnectModel(doc map[string]any, st config.ClaudeCodeCLIState) {
	if st.PrevModelSet {
		doc["model"] = st.PrevModel
	} else {
		delete(doc, "model")
	}
}

// GetClaudeCodeCLIConnected reports whether Memo currently has the Claude
// Code CLI pointed at this gateway.
func (a *App) GetClaudeCodeCLIConnected() bool {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.DevGateway.ClaudeCodeCLI.Connected
}

// GetClaudeCodeCLIModel reports the "type/model-id" string currently
// written to the Claude Code CLI's settings.json "model" field, or "" if no
// override is configured.
func (a *App) GetClaudeCodeCLIModel() string {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.DevGateway.ClaudeCodeCLI.Model
}

// ConnectClaudeCodeCLI is the Developer screen's "one-click connect": sets
// env.ANTHROPIC_BASE_URL and env.ANTHROPIC_API_KEY in the Claude Code CLI's
// own settings.json so every future `claude` invocation on this machine
// talks to this gateway instead of the real Anthropic API, with no manual
// environment-variable exporting required. If model is non-empty, also
// writes settings.json's top-level "model" field — Claude Code otherwise
// sends its own built-in default model name, which this gateway rejects
// (it only accepts "type/model-id" strings matching a configured Memo
// model/provider). Backs up whatever was already in those keys (if
// anything) before overwriting, so DisconnectClaudeCodeCLI can restore the
// user's own prior configuration exactly rather than just deleting them —
// important for the (rare but real) case where the user had already
// pointed Claude Code at some other custom endpoint/model before ever
// touching Memo. See applyConnectEnv/applyConnectModel for the actual
// (unit-tested) merge logic — this is just the I/O shell around it.
func (a *App) ConnectClaudeCodeCLI(baseURL, model string) error {
	path, err := claudeCodeSettingsPath()
	if err != nil {
		return err
	}
	doc, err := readJSONObject(path)
	if err != nil {
		return err
	}
	env, _ := doc["env"].(map[string]any)
	if env == nil {
		env = map[string]any{}
	}

	token := a.GetDevGatewayToken()

	a.cfgMu.Lock()
	prev := a.cfg.DevGateway.ClaudeCodeCLI
	next := applyConnectEnv(env, prev, baseURL, token)
	modelState := applyConnectModel(doc, prev, model)
	next.Model, next.PrevModelSet, next.PrevModel = modelState.Model, modelState.PrevModelSet, modelState.PrevModel
	a.cfg.DevGateway.ClaudeCodeCLI = next
	cfg := a.cfg
	a.cfgMu.Unlock()

	doc["env"] = env
	if err := writeJSONObject(path, doc); err != nil {
		return err
	}
	return config.Save(cfg)
}

// DisconnectClaudeCodeCLI restores whatever ConnectClaudeCodeCLI backed up
// (or removes the keys/field entirely if there was nothing there before)
// and clears the connected flag. A no-op if not currently connected. See
// applyDisconnectEnv/applyDisconnectModel for the actual (unit-tested)
// merge logic.
func (a *App) DisconnectClaudeCodeCLI() error {
	a.cfgMu.Lock()
	st := a.cfg.DevGateway.ClaudeCodeCLI
	if !st.Connected {
		a.cfgMu.Unlock()
		return nil
	}
	a.cfg.DevGateway.ClaudeCodeCLI = config.ClaudeCodeCLIState{}
	cfg := a.cfg
	a.cfgMu.Unlock()

	path, err := claudeCodeSettingsPath()
	if err != nil {
		return err
	}
	doc, err := readJSONObject(path)
	if err != nil {
		return err
	}
	if env, ok := doc["env"].(map[string]any); ok {
		applyDisconnectEnv(env, st)
		if len(env) == 0 {
			delete(doc, "env")
		} else {
			doc["env"] = env
		}
	}
	applyDisconnectModel(doc, st)
	if err := writeJSONObject(path, doc); err != nil {
		return err
	}
	return config.Save(cfg)
}
