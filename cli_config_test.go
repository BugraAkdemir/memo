package main

import (
	"testing"

	"memo/internal/config"
)

func TestConfigToMap_RoundTripsThroughDefault(t *testing.T) {
	cfg := config.Default()
	m, err := configToMap(cfg)
	if err != nil {
		t.Fatalf("configToMap: %v", err)
	}
	back, err := mapToConfig(m)
	if err != nil {
		t.Fatalf("mapToConfig: %v", err)
	}
	if back.Llama.Port != cfg.Llama.Port {
		t.Errorf("Llama.Port = %d, want %d", back.Llama.Port, cfg.Llama.Port)
	}
	if back.Identity.AssistantName != cfg.Identity.AssistantName {
		t.Errorf("Identity.AssistantName = %q, want %q", back.Identity.AssistantName, cfg.Identity.AssistantName)
	}
}

func TestGetConfigKey_NestedInt(t *testing.T) {
	m, err := configToMap(config.Default())
	if err != nil {
		t.Fatalf("configToMap: %v", err)
	}
	val, err := getConfigKey(m, "llama.port")
	if err != nil {
		t.Fatalf("getConfigKey: %v", err)
	}
	if val != config.Default().Llama.Port {
		t.Errorf("got %v, want %d", val, config.Default().Llama.Port)
	}
}

func TestGetConfigKey_UnknownKeyFails(t *testing.T) {
	m, _ := configToMap(config.Default())
	if _, err := getConfigKey(m, "llama.nonexistent_field"); err == nil {
		t.Fatal("expected an error for an unknown key")
	}
	if _, err := getConfigKey(m, "nonexistent_section.port"); err == nil {
		t.Fatal("expected an error for an unknown section")
	}
}

func TestGetConfigKey_SectionNotScalarFails(t *testing.T) {
	m, _ := configToMap(config.Default())
	if _, err := getConfigKey(m, "llama"); err == nil {
		t.Fatal("expected an error asking for a whole section as a single value")
	}
}

func TestSetConfigKey_UpdatesIntValue(t *testing.T) {
	m, err := configToMap(config.Default())
	if err != nil {
		t.Fatalf("configToMap: %v", err)
	}
	if err := setConfigKey(m, "llama.port", "9999"); err != nil {
		t.Fatalf("setConfigKey: %v", err)
	}
	cfg, err := mapToConfig(m)
	if err != nil {
		t.Fatalf("mapToConfig: %v", err)
	}
	if cfg.Llama.Port != 9999 {
		t.Errorf("Llama.Port = %d, want 9999", cfg.Llama.Port)
	}
}

func TestSetConfigKey_UpdatesBoolValue(t *testing.T) {
	m, err := configToMap(config.Default())
	if err != nil {
		t.Fatalf("configToMap: %v", err)
	}
	if err := setConfigKey(m, "memory.memory_enabled", "false"); err != nil {
		t.Fatalf("setConfigKey: %v", err)
	}
	cfg, err := mapToConfig(m)
	if err != nil {
		t.Fatalf("mapToConfig: %v", err)
	}
	if cfg.Memory.MemoryEnabled {
		t.Error("expected MemoryEnabled to be set to false")
	}
}

func TestSetConfigKey_UpdatesStringValue(t *testing.T) {
	m, err := configToMap(config.Default())
	if err != nil {
		t.Fatalf("configToMap: %v", err)
	}
	if err := setConfigKey(m, "identity.assistant_name", "Bot"); err != nil {
		t.Fatalf("setConfigKey: %v", err)
	}
	cfg, err := mapToConfig(m)
	if err != nil {
		t.Fatalf("mapToConfig: %v", err)
	}
	if cfg.Identity.AssistantName != "Bot" {
		t.Errorf("AssistantName = %q, want %q", cfg.Identity.AssistantName, "Bot")
	}
}

func TestSetConfigKey_RejectsUnknownKey(t *testing.T) {
	m, _ := configToMap(config.Default())
	if err := setConfigKey(m, "llama.not_a_real_field", "1"); err == nil {
		t.Fatal("expected an error for an unknown key")
	}
}

func TestSetConfigKey_RejectsSectionAsLeaf(t *testing.T) {
	m, _ := configToMap(config.Default())
	if err := setConfigKey(m, "llama", "1"); err == nil {
		t.Fatal("expected an error setting a whole section as a scalar")
	}
}

func TestSetConfigKey_RejectsTypeMismatch(t *testing.T) {
	m, _ := configToMap(config.Default())
	if err := setConfigKey(m, "llama.port", "not-a-number"); err == nil {
		t.Fatal("expected an error for a non-numeric value on an int field")
	}
}

func TestIsRemoteAccessKey(t *testing.T) {
	cases := map[string]bool{
		"remote_access":                true,
		"remote_access.auth_mode":      true,
		"remote_access.devices":        true,
		"llama.port":                   false,
		"remote_access_something_else": false, // must not match on a bare prefix without the dot
	}
	for key, want := range cases {
		if got := isRemoteAccessKey(key); got != want {
			t.Errorf("isRemoteAccessKey(%q) = %v, want %v", key, got, want)
		}
	}
}
