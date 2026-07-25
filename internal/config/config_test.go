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
	if cfg.Memory.TopK != 8 {
		t.Errorf("Memory.TopK = %d, want 8", cfg.Memory.TopK)
	}
	if cfg.Memory.MinSimilarity != 0.1 {
		t.Errorf("Memory.MinSimilarity = %f, want 0.1", cfg.Memory.MinSimilarity)
	}
	if !cfg.Memory.AutoFactExtraction {
		t.Error("Memory.AutoFactExtraction = false, want true for fresh installs")
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
	if cfg.Memory.TopK != 8 {
		t.Errorf("TopK = %d, want 8", cfg.Memory.TopK)
	}
	if cfg.Llama.CtxSize != 4096 {
		t.Errorf("CtxSize = %d, want 4096", cfg.Llama.CtxSize)
	}
	if cfg.Llama.Port != 8081 {
		t.Errorf("Port = %d, want 8081", cfg.Llama.Port)
	}
	if cfg.Swarm.RPCPort != 50052 {
		t.Errorf("Swarm.RPCPort = %d, want 50052", cfg.Swarm.RPCPort)
	}
	if cfg.Swarm.Role != "none" {
		t.Errorf("Swarm.Role = %q, want %q", cfg.Swarm.Role, "none")
	}
}

func TestTTSDefaults(t *testing.T) {
	cfg := Default()
	if cfg.TTS.Enabled {
		t.Error("Default().TTS.Enabled = true, want false — Piper isn't bundled by default yet")
	}
	if cfg.TTS.ModelPath != "" {
		t.Errorf("Default().TTS.ModelPath = %q, want empty — no voice auto-selection in Faz 1", cfg.TTS.ModelPath)
	}
}

func TestSwarmDefaults(t *testing.T) {
	cfg := Default()
	if cfg.Swarm.RPCPort != 50052 {
		t.Errorf("Default().Swarm.RPCPort = %d, want 50052", cfg.Swarm.RPCPort)
	}
	if cfg.Swarm.Role != "none" {
		t.Errorf("Default().Swarm.Role = %q, want %q", cfg.Swarm.Role, "none")
	}
	if len(cfg.Swarm.Workers) != 0 {
		t.Errorf("Default().Swarm.Workers = %v, want empty", cfg.Swarm.Workers)
	}
}

func TestValidateClampsSwarmRPCPort(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, 50052},
		{-1, 50052},
		{70000, 50052},
		{50052, 50052},
		{12345, 12345},
	}
	for _, c := range cases {
		cfg := &AppConfig{Swarm: SwarmConfig{RPCPort: c.in}}
		cfg.validate()
		if cfg.Swarm.RPCPort != c.want {
			t.Errorf("validate() with RPCPort=%d = %d, want %d", c.in, cfg.Swarm.RPCPort, c.want)
		}
	}
}

// TestValidateClampsSwarmWorkerSharePercent mirrors the Llama.Temperature/
// TopP negative-only-clamp reasoning above, but for a bounded [0,100] range
// instead of a floor-only one: an out-of-range share (typo, a bad client
// request) gets pulled back into range rather than silently accepted and
// fed straight into --tensor-split.
func TestValidateClampsSwarmWorkerSharePercent(t *testing.T) {
	cfg := &AppConfig{Swarm: SwarmConfig{Workers: []SwarmWorkerConfig{
		{ID: "a", SharePercent: -5},
		{ID: "b", SharePercent: 150},
		{ID: "c", SharePercent: 30},
	}}}
	cfg.validate()

	want := []float64{0, 100, 30}
	for i, w := range cfg.Swarm.Workers {
		if w.SharePercent != want[i] {
			t.Errorf("Workers[%d].SharePercent = %v, want %v", i, w.SharePercent, want[i])
		}
	}
}

// TestValidateDedupesSwarmWorkerIDs is the regression test for a duplicate
// worker ID silently aliasing two WorkerSlot entries at the same
// --tensor-split position when internal/swarm looks one up by ID — only the
// first occurrence of a given ID should survive validate().
func TestValidateDedupesSwarmWorkerIDs(t *testing.T) {
	cfg := &AppConfig{Swarm: SwarmConfig{Workers: []SwarmWorkerConfig{
		{ID: "dup", Label: "first"},
		{ID: "unique", Label: "kept"},
		{ID: "dup", Label: "second — should be dropped"},
	}}}
	cfg.validate()

	if len(cfg.Swarm.Workers) != 2 {
		t.Fatalf("len(Workers) = %d, want 2 after dedup, got %+v", len(cfg.Swarm.Workers), cfg.Swarm.Workers)
	}
	if cfg.Swarm.Workers[0].Label != "first" {
		t.Errorf("Workers[0].Label = %q, want %q (first occurrence kept)", cfg.Swarm.Workers[0].Label, "first")
	}
	if cfg.Swarm.Workers[1].ID != "unique" {
		t.Errorf("Workers[1].ID = %q, want %q", cfg.Swarm.Workers[1].ID, "unique")
	}
}

// TestValidatePreservesExplicitZeroTemperatureAndTopP is a regression test
// for BUG-QM2: validate() used a `<= 0` check for Temperature/TopP, so an
// intentional 0 (greedy decoding) was indistinguishable from "never set" and
// got silently reset to the 0.7/0.9 default on every config.Save() call —
// not just on initial load. Only a genuinely invalid negative value should
// be defaulted now (matching the `< 0` check MaxTokens already used).
func TestValidatePreservesExplicitZeroTemperatureAndTopP(t *testing.T) {
	cfg := &AppConfig{Llama: LlamaConfig{Temperature: 0, TopP: 0, MaxTokens: 0}}
	cfg.validate()

	if cfg.Llama.Temperature != 0 {
		t.Errorf("Temperature = %v, want 0 to survive validate()", cfg.Llama.Temperature)
	}
	if cfg.Llama.TopP != 0 {
		t.Errorf("TopP = %v, want 0 to survive validate()", cfg.Llama.TopP)
	}
	if cfg.Llama.MaxTokens != 0 {
		t.Errorf("MaxTokens = %v, want 0 to survive validate()", cfg.Llama.MaxTokens)
	}

	// A genuinely invalid negative value must still be defaulted.
	cfg2 := &AppConfig{Llama: LlamaConfig{Temperature: -1, TopP: -1, MaxTokens: -1}}
	cfg2.validate()
	if cfg2.Llama.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want defaulted to 0.7 for a negative input", cfg2.Llama.Temperature)
	}
	if cfg2.Llama.TopP != 0.9 {
		t.Errorf("TopP = %v, want defaulted to 0.9 for a negative input", cfg2.Llama.TopP)
	}
	if cfg2.Llama.MaxTokens != 0 {
		t.Errorf("MaxTokens = %v, want defaulted to 0 for a negative input", cfg2.Llama.MaxTokens)
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
