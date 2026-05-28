package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	API          APIConfig          `yaml:"api"`
	Identity     IdentityConfig     `yaml:"identity"`
	Memory       MemoryConfig       `yaml:"memory"`
	RemoteAccess RemoteAccessConfig `yaml:"remote_access"`
	Llama        LlamaConfig        `yaml:"llama"`
	Sync         SyncConfig         `yaml:"sync"`
}

// SyncConfig holds Google Drive backup settings.
// ClientID and ClientSecret come from the user's own Google Cloud project
// (OAuth 2.0 Desktop App credentials). Passphrase derives the AES-256 key;
// leave empty to use a hardware-derived key automatically.
type SyncConfig struct {
	Enabled          bool   `yaml:"enabled" json:"enabled"`
	ClientID         string `yaml:"client_id" json:"client_id"`
	ClientSecret     string `yaml:"client_secret" json:"client_secret"`
	Passphrase       string `yaml:"passphrase" json:"passphrase,omitempty"`
	TokenPath        string `yaml:"token_path" json:"token_path"`
	IntervalMessages int    `yaml:"interval_messages" json:"interval_messages"`
}

type RemoteAccessConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	Port    int  `yaml:"port" json:"port"`
}

type APIConfig struct {
	BaseURL        string `yaml:"base_url"`
	EmbeddingModel string `yaml:"embedding_model"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type LlamaConfig struct {
	BinaryPath    string `yaml:"binary_path" json:"binary_path"`       // path to llama-server, auto-detected if empty
	Port          int    `yaml:"port" json:"port"`                     // default 8081
	EmbeddingPort int    `yaml:"embedding_port" json:"embedding_port"` // default 8082
	CtxSize       int    `yaml:"ctx_size" json:"ctx_size"`             // default 4096
	MaxHistory    int    `yaml:"max_history" json:"max_history"`       // default 20
	ModelsDir     string `yaml:"models_dir" json:"models_dir"`         // default "./data/models"
}

type IdentityConfig struct {
	UserName        string `yaml:"user_name"`
	AssistantName   string `yaml:"assistant_name"`
	Style           string `yaml:"style"`
	SystemRole      string `yaml:"system_role"`
	IncognitoPrompt string `yaml:"incognito_prompt"`
}

type MemoryConfig struct {
	PersistDir    string  `yaml:"persist_dir" json:"persist_dir"`
	TopK          int     `yaml:"top_k" json:"top_k"`
	MinSimilarity float32 `yaml:"min_similarity" json:"min_similarity"`
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
			BaseURL:        "http://127.0.0.1:8081/v1",
			EmbeddingModel: "nomic-embed-text-v1.5",
			TimeoutSeconds: 120,
		},
		Identity: IdentityConfig{
			UserName:        "User",
			AssistantName:   "Memo",
			Style:           "casual",
			SystemRole:      "",
			IncognitoPrompt: "You are Memo, in Incognito Mode. This is a secure session. Never refer to past events, because you have no memory here. Do your best to assist the user right now.",
		},
		Memory: MemoryConfig{
			PersistDir:    "./data/memory",
			TopK:          5,
			MinSimilarity: 0.1,
		},
		RemoteAccess: RemoteAccessConfig{
			Enabled: false,
			Port:    8080,
		},
		Llama: LlamaConfig{
			BinaryPath:    "",
			Port:          8081,
			EmbeddingPort: 8082,
			CtxSize:       4096,
			MaxHistory:    20,
			ModelsDir:     "./data/models",
		},
		Sync: SyncConfig{
			Enabled:          false,
			TokenPath:        "./data/sync_token.json",
			IntervalMessages: 50,
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
		c.API.BaseURL = "http://127.0.0.1:8081/v1"
	}
	if c.API.TimeoutSeconds <= 0 {
		c.API.TimeoutSeconds = 120
	}
	if c.Identity.AssistantName == "" {
		c.Identity.AssistantName = "Memo"
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
	if c.RemoteAccess.Port <= 0 || c.RemoteAccess.Port > 65535 {
		c.RemoteAccess.Port = 8080
	}
	if c.Llama.Port <= 0 || c.Llama.Port > 65535 {
		c.Llama.Port = 8081
	}
	if c.Llama.EmbeddingPort <= 0 || c.Llama.EmbeddingPort > 65535 {
		c.Llama.EmbeddingPort = 8082
	}
	if (c.Llama.CtxSize <= 0) {
		c.Llama.CtxSize = 4096
	}
	if c.Llama.MaxHistory <= 0 {
		c.Llama.MaxHistory = 20
	}
	if c.Llama.ModelsDir == "" {
		c.Llama.ModelsDir = "./data/models"
	}
	if c.Sync.TokenPath == "" {
		c.Sync.TokenPath = "./data/sync_token.json"
	}
	if c.Sync.IntervalMessages <= 0 {
		c.Sync.IntervalMessages = 50
	}
}
