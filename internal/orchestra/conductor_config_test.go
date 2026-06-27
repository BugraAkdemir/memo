package orchestra

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestMergeRolesPreservesBuiltins(t *testing.T) {
	roles := MergeRoles(nil)
	if len(roles) != 8 {
		t.Errorf("expected 8 built-in roles, got %d", len(roles))
	}
	names := defaultRoleNames()
	for _, name := range names {
		found := false
		for _, r := range roles {
			if r.Role == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing built-in role: %s", name)
		}
	}
}

func TestMergeRolesPreservesCustomRoles(t *testing.T) {
	existing := []RoleConfig{
		{Role: "custom_role", Enabled: true, ModelType: "openai", ModelName: "gpt-4"},
	}
	roles := MergeRoles(existing)
	found := false
	for _, r := range roles {
		if r.Role == "custom_role" {
			found = true
			if !r.Enabled {
				t.Error("custom role should preserve enabled=true")
			}
			break
		}
	}
	if !found {
		t.Error("custom role was dropped")
	}
}

func TestMergeRolesMissingBuiltinsDisabled(t *testing.T) {
	existing := []RoleConfig{
		{Role: RolePlanner, Enabled: true, ModelType: "openai", ModelName: "gpt-4"},
	}
	roles := MergeRoles(existing)
	for _, r := range roles {
		if r.Role != RolePlanner && r.Enabled {
			t.Errorf("missing role %s should be disabled", r.Role)
		}
	}
}

func TestMergeRolesPreservesExistingSettings(t *testing.T) {
	existing := []RoleConfig{
		{Role: RolePlanner, Enabled: true, ModelType: "custom", ModelName: "custom-model"},
	}
	roles := MergeRoles(existing)
	for _, r := range roles {
		if r.Role == RolePlanner {
			if r.ModelType != "custom" || r.ModelName != "custom-model" {
				t.Errorf("existing settings overwritten: %+v", r)
			}
			break
		}
	}
}

func TestIsBuiltinRole(t *testing.T) {
	if !IsBuiltinRole(RolePlanner) {
		t.Error("planner should be built-in")
	}
	if !IsBuiltinRole(RoleGeneral) {
		t.Error("general should be built-in")
	}
	if IsBuiltinRole("nonexistent") {
		t.Error("nonexistent should not be built-in")
	}
}

func TestSanitizeTrimsAllFields(t *testing.T) {
	cfg := OrchestraConfig{
		ChiefType:  "\t\n ",
		ChiefModel: "",
	}
	got := cfg.Sanitize()
	if got.ChiefType != "" {
		t.Errorf("expected empty, got %q", got.ChiefType)
	}
}

func TestDefaultConfigDisabled(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Enabled {
		t.Error("default config should have Enabled=false")
	}
	if len(cfg.Roles) != 8 {
		t.Errorf("expected 8 roles, got %d", len(cfg.Roles))
	}
}

func TestNewConductor(t *testing.T) {
	cfg := DefaultConfig()
	f := newMockFactory()
	c := NewConductor(cfg, f.factory, nil)
	if c == nil {
		t.Fatal("NewConductor returned nil")
	}
	got := c.Config()
	if got.Enabled != false {
		t.Error("config should be disabled by default")
	}
}

func TestConfigThreadSafety(t *testing.T) {
	cfg := DefaultConfig()
	f := newMockFactory()
	c := NewConductor(cfg, f.factory, nil)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Config()
		}()
	}
	wg.Wait()
}

func TestUpdateConfig(t *testing.T) {
	cfg := DefaultConfig()
	f := newMockFactory()
	c := NewConductor(cfg, f.factory, nil)

	updated := OrchestraConfig{
		Enabled:    true,
		ChiefType:  "openai",
		ChiefModel: "gpt-4o",
		Roles: []RoleConfig{
			{Role: RolePlanner, Enabled: true, ModelType: "openai", ModelName: "gpt-4o"},
		},
	}
	c.UpdateConfig(updated)

	got := c.Config()
	if !got.Enabled {
		t.Error("expected enabled")
	}
	if got.ChiefType != "openai" {
		t.Errorf("expected openai, got %s", got.ChiefType)
	}
	if len(got.Roles) < 8 {
		t.Errorf("expected at least 8 roles after merge, got %d", len(got.Roles))
	}
}

func TestConfigReturnsCopy(t *testing.T) {
	cfg := DefaultConfig()
	f := newMockFactory()
	c := NewConductor(cfg, f.factory, nil)

	got := c.Config()
	got.Roles[0].ModelType = "modified"
	got2 := c.Config()
	if got2.Roles[0].ModelType == "modified" {
		t.Error("Config should return a deep copy")
	}
}

func TestSaveLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "orchestra.json")

	cfg := OrchestraConfig{
		Enabled:    true,
		ChiefType:  "openai",
		ChiefModel: "gpt-4o",
		Roles: []RoleConfig{
			{Role: RolePlanner, Enabled: true, ModelType: "openai", ModelName: "gpt-4o"},
		},
	}

	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded := LoadConfig(path)
	if !loaded.Enabled {
		t.Error("expected enabled")
	}
	if loaded.ChiefType != "openai" {
		t.Errorf("expected openai, got %s", loaded.ChiefType)
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	cfg := LoadConfig("/nonexistent/path/orchestra.json")
	if cfg.Enabled {
		t.Error("expected default config (disabled) when file not found")
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "orchestra.json")
	os.WriteFile(path, []byte("{invalid}"), 0600)

	cfg := LoadConfig(path)
	if cfg.Enabled {
		t.Error("expected default config on invalid JSON")
	}
}

func TestSaveConfigCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "orchestra.json")

	cfg := DefaultConfig()
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig with nested dir: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("config file was not created")
	}
}
