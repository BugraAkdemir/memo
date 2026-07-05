package orchestra

import (
	"context"
	"errors"
	"strings"
	"testing"

	"memo/internal/provider"
)

func TestRunWithProgressCancelledContext(t *testing.T) {
	f := newMockFactory()
	cfg := defaultEnabledConfig()

	c := NewConductor(cfg, func(cfg provider.ProviderConfig) (provider.Provider, error) {
		return f.get(string(cfg.Type)), nil
	}, testGetConfigs)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := c.RunWithProgress(ctx, "test", nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestCreateProviderForTypeLocal(t *testing.T) {
	c := NewConductor(DefaultConfig(), nil, func() []provider.ProviderConfig {
		return []provider.ProviderConfig{
			{Type: "openai", Enabled: true},
		}
	})

	_, err := c.CreateProviderForType("local", "model")
	if err == nil {
		t.Fatal("expected error for local provider")
	}
}

func TestCreateProviderForTypeEmpty(t *testing.T) {
	c := NewConductor(DefaultConfig(), nil, func() []provider.ProviderConfig {
		return []provider.ProviderConfig{
			{Type: "openai", Enabled: true},
		}
	})

	_, err := c.CreateProviderForType("", "model")
	if err == nil {
		t.Fatal("expected error for empty type")
	}
}

func TestCreateProviderForTypeNotFound(t *testing.T) {
	f := newMockFactory()
	c := NewConductor(DefaultConfig(), f.factory, func() []provider.ProviderConfig {
		return []provider.ProviderConfig{
			{Type: "openai", Enabled: true},
		}
	})

	_, err := c.CreateProviderForType("nonexistent", "model")
	if err == nil {
		t.Fatal("expected error for nonexistent provider")
	}
}

func TestCreateProviderForTypeExactMatch(t *testing.T) {
	callCount := 0
	c := NewConductor(DefaultConfig(), func(cfg provider.ProviderConfig) (provider.Provider, error) {
		callCount++
		if cfg.Type != "openai" {
			t.Errorf("expected type openai, got %s", cfg.Type)
		}
		if cfg.Model != "gpt-4o" {
			t.Errorf("expected model gpt-4o, got %s", cfg.Model)
		}
		return &mockProvider{name: "openai"}, nil
	}, func() []provider.ProviderConfig {
		return []provider.ProviderConfig{
			{Name: "my-openai", Type: "openai", Enabled: true, Model: "gpt-4"},
			{Name: "my-claude", Type: "claude", Enabled: true, Model: "claude-4"},
		}
	})

	p, err := c.CreateProviderForType("openai", "gpt-4o")
	if err != nil {
		t.Fatalf("CreateProviderForType: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if callCount != 1 {
		t.Errorf("expected 1 factory call, got %d", callCount)
	}
}

func TestBuildRoleInfo(t *testing.T) {
	cfg := DefaultConfig()
	c := NewConductor(cfg, nil, nil)
	info := c.buildRoleInfo(cfg)
	if info == "" {
		t.Error("expected non-empty role info")
	}
	if !strings.Contains(info, "planner") {
		t.Error("expected planner in role info")
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input string
		min   int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world foo bar", 4},
		{"short", 1},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := estimateTokens(tt.input)
			if got < tt.min {
				t.Errorf("estimateTokens(%q) = %d, want >= %d", tt.input, got, tt.min)
			}
		})
	}
}

func TestTruncateUTF8(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"日本語", 2, "日本..."},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncateUTF8(tt.input, tt.n)
			if got != tt.want {
				t.Errorf("truncateUTF8(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
			}
		})
	}
}
