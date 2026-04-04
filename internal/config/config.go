package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	API      APIConfig      `yaml:"api"`
	Identity IdentityConfig `yaml:"identity"`
	Memory   MemoryConfig   `yaml:"memory"`
}

type APIConfig struct {
	BaseURL        string `yaml:"base_url"`
	EmbeddingModel string `yaml:"embedding_model"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type IdentityConfig struct {
	UserName      string `yaml:"user_name"`
	AssistantName string `yaml:"assistant_name"`
	Style         string `yaml:"style"`
	SystemRole    string `yaml:"system_role"`
}

type MemoryConfig struct {
	PersistDir    string  `yaml:"persist_dir"`
	TopK          int     `yaml:"top_k"`
	MinSimilarity float32 `yaml:"min_similarity"`
}

var (
	instance *AppConfig
	once     sync.Once
	mu       sync.RWMutex
	cfgPath  string
)

func Default() *AppConfig {
	return &AppConfig{
		API: APIConfig{
			BaseURL:        "http://localhost:1234/v1",
			EmbeddingModel: "nomic-embed-text-v1.5",
			TimeoutSeconds: 120,
		},
		Identity: IdentityConfig{
			UserName:      "User",
			AssistantName: "Cortex",
			Style:         "casual",
			SystemRole:    "",
		},
		Memory: MemoryConfig{
			PersistDir:    "./data/memory",
			TopK:          5,
			MinSimilarity: 0.3,
		},
	}
}

func Load(path string) (*AppConfig, error) {
	mu.Lock()
	defer mu.Unlock()

	cfgPath = path
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if saveErr := saveToFile(cfg, path); saveErr != nil {
				return nil, fmt.Errorf("config.Load: failed to create default config: %w", saveErr)
			}
			instance = cfg
			return cfg, nil
		}
		return nil, fmt.Errorf("config.Load: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config.Load: failed to parse YAML: %w", err)
	}

	cfg.validate()
	instance = cfg
	return cfg, nil
}

func Get() *AppConfig {
	mu.RLock()
	defer mu.RUnlock()

	if instance == nil {
		return Default()
	}
	return instance
}

func Save(cfg *AppConfig) error {
	mu.Lock()
	defer mu.Unlock()

	if cfgPath == "" {
		cfgPath = "config/config.yaml"
	}

	cfg.validate()
	instance = cfg
	return saveToFile(cfg, cfgPath)
}

func saveToFile(cfg *AppConfig, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("config.saveToFile: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config.saveToFile: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

func (c *AppConfig) validate() {
	if c.API.BaseURL == "" {
		c.API.BaseURL = "http://localhost:1234/v1"
	}
	if c.API.TimeoutSeconds <= 0 {
		c.API.TimeoutSeconds = 120
	}
	if c.Identity.AssistantName == "" {
		c.Identity.AssistantName = "Cortex"
	}
	if c.Identity.Style == "" {
		c.Identity.Style = "casual"
	}
	if c.Memory.PersistDir == "" {
		c.Memory.PersistDir = "./data/memory"
	}
	if c.Memory.TopK <= 0 {
		c.Memory.TopK = 5
	}
	if c.Memory.MinSimilarity <= 0 {
		c.Memory.MinSimilarity = 0.3
	}
}
