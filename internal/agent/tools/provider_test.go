package tools

import (
	"context"
	"encoding/json"
	"testing"

	"memo/internal/provider"
)

type mockConfigurator struct {
	got provider.ProviderConfig
	err error
}

func (m *mockConfigurator) UpdateProvider(cfg provider.ProviderConfig) error {
	m.got = cfg
	return m.err
}

func TestConfigureProviderNoConfigurator(t *testing.T) {
	Configurator = nil
	args, _ := json.Marshal(map[string]string{"type": "custom", "model": "m"})
	if _, err := ConfigureProvider(context.Background(), args, "", nil); err == nil {
		t.Error("beklenen hata: provider system not available")
	}
}

func TestConfigureProviderRequiresBaseURLForCustom(t *testing.T) {
	m := &mockConfigurator{}
	Configurator = m
	defer func() { Configurator = nil }()

	args, _ := json.Marshal(map[string]string{"type": "custom", "model": "m"})
	if _, err := ConfigureProvider(context.Background(), args, "", nil); err == nil {
		t.Error("custom provider için base_url zorunlu olmalı")
	}
}

func TestConfigureProviderSuccess(t *testing.T) {
	m := &mockConfigurator{}
	Configurator = m
	defer func() { Configurator = nil }()

	args, _ := json.Marshal(map[string]any{
		"type":     "custom",
		"name":     "my-endpoint",
		"base_url": "https://example.com/v1",
		"api_key":  "sk-test",
		"model":    "gpt-4o-mini",
	})
	out, err := ConfigureProvider(context.Background(), args, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.got.Name != "my-endpoint" || m.got.BaseURL != "https://example.com/v1" || m.got.Model != "gpt-4o-mini" {
		t.Errorf("configurator'a yanlış config geçti: %+v", m.got)
	}
	if !m.got.Enabled {
		t.Error("enabled belirtilmediğinde varsayılan true olmalı")
	}
	if out == "" {
		t.Error("başarı mesajı boş dönmemeli")
	}
}
