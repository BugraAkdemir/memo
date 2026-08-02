// SPDX-License-Identifier: AGPL-3.0-or-later

package agentcli

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"memo/internal/provider"
)

func TestAvailableModels_ClaudeReturnsDocumentedAliases(t *testing.T) {
	got := AvailableModels(provider.ProviderClaudeCodeCLI)
	want := []string{"opus", "sonnet", "fable"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAvailableModels_ClaudeReturnsACopyNotTheSharedSlice(t *testing.T) {
	got := AvailableModels(provider.ProviderClaudeCodeCLI)
	got[0] = "MUTATED"
	if again := AvailableModels(provider.ProviderClaudeCodeCLI); again[0] != "opus" {
		t.Errorf("caller mutated the shared alias list: %v", again)
	}
}

func TestAvailableModels_UnknownProviderReturnsNil(t *testing.T) {
	if got := AvailableModels(provider.ProviderOpenAI); got != nil {
		t.Errorf("got %v, want nil for a non-CLI provider", got)
	}
}

func TestAvailableModels_CodexReadsModelsCacheFile(t *testing.T) {
	home := isolatedHome(t)
	cache := map[string]any{
		"models": []map[string]string{
			{"id": "gpt-5.6-terra"},
			{"id": "codex-auto-review"},
			{"id": ""}, // must be skipped
		},
	}
	data, err := json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(home, ".codex", "models_cache.json"), string(data))

	got := AvailableModels(provider.ProviderCodexCLI)
	want := []string{"gpt-5.6-terra", "codex-auto-review"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAvailableModels_CodexMissingCacheFileReturnsNilNotAGuess(t *testing.T) {
	isolatedHome(t)
	// No models_cache.json written — a fresh install / codex never run yet.
	if got := AvailableModels(provider.ProviderCodexCLI); got != nil {
		t.Errorf("got %v, want nil rather than a hardcoded guess", got)
	}
}

func TestAvailableModels_CodexMalformedCacheFileReturnsNil(t *testing.T) {
	home := isolatedHome(t)
	writeFile(t, filepath.Join(home, ".codex", "models_cache.json"), "not json")
	if got := AvailableModels(provider.ProviderCodexCLI); got != nil {
		t.Errorf("got %v, want nil for malformed cache", got)
	}
}

func TestAvailableModels_CodexUnreadableHomeReturnsNil(t *testing.T) {
	// A HOME that doesn't exist at all — os.ReadFile fails, must not panic.
	t.Setenv("HOME", filepath.Join(t.TempDir(), "does-not-exist"))
	if got := AvailableModels(provider.ProviderCodexCLI); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestCodexCachedModels_UserHomeDirErrorReturnsNil(t *testing.T) {
	// Neither $HOME nor $USERPROFILE set — os.UserHomeDir itself errors.
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	if got := codexCachedModels(); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}
