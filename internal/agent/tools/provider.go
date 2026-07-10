package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"memo/internal/provider"
)

// ProviderConfigurator is the interface tools use to add/update an AI
// provider (base URL, API key, model) from an agent tool call.
type ProviderConfigurator interface {
	UpdateProvider(cfg provider.ProviderConfig) error
}

// Configurator is wired up at startup (see internal/app) so this tool can
// reach the running App without importing it directly (would cycle).
var Configurator ProviderConfigurator

type ConfigureProviderArgs struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	Enabled *bool  `json:"enabled"`
}

// ConfigureProvider adds or updates an AI provider configuration (base URL,
// API key, model) — the same thing Settings → API Providers does, but
// callable from chat when the user asks to add/configure a provider.
func ConfigureProvider(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	if Configurator == nil {
		return "", fmt.Errorf("provider system not available")
	}
	var args ConfigureProviderArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	pt := provider.ProviderType(strings.ToLower(strings.TrimSpace(args.Type)))
	if pt == "" {
		return "", fmt.Errorf("type is required (e.g. openai, claude, custom, ollama, llama.cpp)")
	}
	enabled := true
	if args.Enabled != nil {
		enabled = *args.Enabled
	}
	cfg := provider.ProviderConfig{
		Type:    pt,
		Name:    strings.TrimSpace(args.Name),
		APIKey:  args.APIKey,
		BaseURL: strings.TrimSpace(args.BaseURL),
		Model:   strings.TrimSpace(args.Model),
		Enabled: enabled,
	}
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	if err := Configurator.UpdateProvider(cfg); err != nil {
		return "", fmt.Errorf("provider could not be saved: %w", err)
	}

	label := cfg.Name
	if label == "" {
		label = string(cfg.Type)
	}
	keyNote := "no API key"
	if cfg.APIKey != "" {
		keyNote = "API key set"
	}
	return fmt.Sprintf("Provider '%s' saved (type=%s, model=%s, %s, enabled=%v).", label, cfg.Type, cfg.Model, keyNote, cfg.Enabled), nil
}
