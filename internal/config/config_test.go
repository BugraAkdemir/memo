package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultValues(t *testing.T) {
	cfg := Default()
	if cfg.API.BaseURL != "http://127.0.0.1:8081/v1" {
		t.Errorf("BaseURL = %q, want %q", cfg.API.BaseURL, "http://127.0.0.1:8081/v1")
	}
	if cfg.API.TimeoutSeconds != 120 {
		t.Errorf("TimeoutSeconds = %d, want 120", cfg.API.TimeoutSeconds)
	}
	if cfg.Memory.TopK != 5 {
		t.Errorf("Memory.TopK = %d, want 5", cfg.Memory.TopK)
	}
	if cfg.Memory.MinSimilarity != 0.1 {
		t.Errorf("Memory.MinSimilarity = %f, want 0.1", cfg.Memory.MinSimilarity)
	}
	if cfg.Llama.CtxSize != 4096 {
		t.Errorf("Llama.CtxSize = %d, want 4096", cfg.Llama.CtxSize)
	}
	if cfg.RemoteAccess.Enabled != false {
		t.Errorf("RemoteAccess.Enabled = %v, want false", cfg.RemoteAccess.Enabled)
	}
}

func TestLoadNonExistentCreatesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.API.BaseURL != "http://127.0.0.1:8081/v1" {
		t.Errorf("BaseURL = %q, want default", cfg.API.BaseURL)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("config file was not created by Load()")
	}
}

func TestLoadValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yamlContent := `
api:
  base_url: "http://test:9090/v1"
  timeout_seconds: 30
llama:
  ctx_size: 8192
  engine_mode: "cpu"
`
	if err := os.WriteFile(path, []byte(yamlContent), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.API.BaseURL != "http://test:9090/v1" {
		t.Errorf("BaseURL = %q, want %q", cfg.API.BaseURL, "http://test:9090/v1")
	}
	if cfg.API.TimeoutSeconds != 30 {
		t.Errorf("TimeoutSeconds = %d, want 30", cfg.API.TimeoutSeconds)
	}
	if cfg.Llama.CtxSize != 8192 {
		t.Errorf("CtxSize = %d, want 8192", cfg.Llama.CtxSize)
	}
	if cfg.Llama.EngineMode != "cpu" {
		t.Errorf("EngineMode = %q, want cpu", cfg.Llama.EngineMode)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte("{invalid: "), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected error for invalid YAML")
	}
}

func TestLoadSetsInstance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	got := Get()
	if got.API.BaseURL != cfg.API.BaseURL {
		t.Errorf("Get().BaseURL = %q, want %q", got.API.BaseURL, cfg.API.BaseURL)
	}
}

func TestGetBeforeLoadReturnsDefault(t *testing.T) {
	instance = nil
	cfg := Get()
	if cfg.API.BaseURL != "http://127.0.0.1:8081/v1" {
		t.Errorf("Get() before Load() = %+v, want default", cfg)
	}
}

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	cfg.Llama.EngineMode = "nvidia"
	cfg.Llama.CtxSize = 16384
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload Load() error = %v", err)
	}
	if reloaded.Llama.EngineMode != "nvidia" {
		t.Errorf("EngineMode = %q, want nvidia", reloaded.Llama.EngineMode)
	}
	if reloaded.Llama.CtxSize != 16384 {
		t.Errorf("CtxSize = %d, want 16384", reloaded.Llama.CtxSize)
	}
}

func TestValidateFixesEmptyFields(t *testing.T) {
	cfg := &AppConfig{}
	cfg.validate()

	if cfg.API.BaseURL != "http://127.0.0.1:8081/v1" {
		t.Errorf("BaseURL = %q, want default after validate", cfg.API.BaseURL)
	}
	if cfg.API.TimeoutSeconds != 120 {
		t.Errorf("TimeoutSeconds = %d, want 120", cfg.API.TimeoutSeconds)
	}
	if cfg.Memory.TopK != 5 {
		t.Errorf("TopK = %d, want 5", cfg.Memory.TopK)
	}
	if cfg.Llama.CtxSize != 4096 {
		t.Errorf("CtxSize = %d, want 4096", cfg.Llama.CtxSize)
	}
	if cfg.Llama.Port != 8081 {
		t.Errorf("Port = %d, want 8081", cfg.Llama.Port)
	}
}

func TestFilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if _, err := Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0077 != 0 {
		t.Errorf("config file has group/other perms: %o", info.Mode())
	}
}

func TestRebaseDataPath(t *testing.T) {
	base := DataDir() // resolved once per process

	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"data", base},
		{"./data", base},
		{"data/models", DataPath("models")},
		{"./data/memory", DataPath("memory")},
		{"./data/sync_token.json", DataPath("sync_token.json")},
		{"custom/dir", "custom/dir"}, // not under data/ — unchanged
	}
	for _, c := range cases {
		if got := rebaseDataPath(c.in); got != c.want {
			t.Errorf("rebaseDataPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	// Absolute paths must be returned unchanged.
	abs := filepath.Join(t.TempDir(), "data", "models")
	if got := rebaseDataPath(abs); got != abs {
		t.Errorf("rebaseDataPath(%q) = %q, want unchanged", abs, got)
	}
}
