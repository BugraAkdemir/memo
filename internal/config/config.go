package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	dataDirOnce sync.Once
	dataDirVal  string
)

// DataDir returns the writable base directory for Memo's persistent data
// (sessions, memory, models, providers, etc.).
//
// On Windows the application is normally installed under Program Files, which is
// read-only for standard (non-admin) users, so a process-relative "data" folder
// there cannot be written. Instead data lives under %ProgramData%\Memo\data —
// the same location installer.iss pre-creates with users-full permissions.
//
// On Linux/macOS the historical process-relative "data" directory is kept so
// portable (AppImage) and development runs are unaffected.
//
// The MEMO_DATA_DIR environment variable overrides the resolved location.
func DataDir() string {
	dataDirOnce.Do(func() {
		dataDirVal = resolveDataDir()
	})
	return dataDirVal
}

// DataPath joins one or more elements onto the resolved data directory.
func DataPath(elem ...string) string {
	return filepath.Join(append([]string{DataDir()}, elem...)...)
}

func resolveDataDir() string {
	if v := strings.TrimSpace(os.Getenv("MEMO_DATA_DIR")); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = os.Getenv("ALLUSERSPROFILE")
		}
		if base != "" {
			return filepath.Join(base, "Memo", "data")
		}
	}
	return "data"
}

type AppConfig struct {
	API            APIConfig          `yaml:"api"`
	Identity       IdentityConfig     `yaml:"identity"`
	Memory         MemoryConfig       `yaml:"memory"`
	RemoteAccess   RemoteAccessConfig `yaml:"remote_access"`
	Llama          LlamaConfig        `yaml:"llama"`
	Sync           SyncConfig         `yaml:"sync"`
	WhatsApp       WhatsAppConfig     `yaml:"whatsapp"`
	Proactive      ProactiveConfig    `yaml:"proactive" json:"proactive"`
	ActiveProvider string             `yaml:"active_provider" json:"active_provider"`
}

// ProactiveConfig controls the learning system's proactive engine. Disabled by
// default (privacy-first): the observer still records, but nothing is suggested.
type ProactiveConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Level   string `yaml:"level" json:"level"` // off | subtle | normal | assertive
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

// WhatsAppConfig holds WhatsApp integration settings.
type WhatsAppConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	DataDir        string `yaml:"data_dir" json:"data_dir"`
	AutoIndex      bool   `yaml:"auto_index" json:"auto_index"`
	MaxHistoryDays int    `yaml:"max_history_days" json:"max_history_days"`
}

type RemoteAccessConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	Port       int    `yaml:"port" json:"port"`
	Token      string `yaml:"token" json:"token"`
	NgrokMode  bool   `yaml:"ngrok_mode" json:"ngrok_mode"`
	NgrokToken string `yaml:"ngrok_token" json:"ngrok_token"`
}

type APIConfig struct {
	BaseURL        string `yaml:"base_url"`
	EmbeddingModel string `yaml:"embedding_model"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type LlamaConfig struct {
	EngineMode       string  `yaml:"engine_mode" json:"engine_mode"`               // "auto", "cpu", "nvidia", "amd"
	BinaryPath       string  `yaml:"binary_path" json:"binary_path"`               // path to llama-server, auto-detected if empty
	Port             int     `yaml:"port" json:"port"`                             // default 8081
	EmbeddingPort    int     `yaml:"embedding_port" json:"embedding_port"`         // default 8082
	CtxSize          int     `yaml:"ctx_size" json:"ctx_size"`                     // default 4096
	MaxHistory       int     `yaml:"max_history" json:"max_history"`               // default 20 (legacy, use MaxContextTokens instead)
	MaxContextTokens int     `yaml:"max_context_tokens" json:"max_context_tokens"` // default 0 = auto (128K for external, CtxSize for local)
	ModelsDir        string  `yaml:"models_dir" json:"models_dir"`                 // default "./data/models"
	Temperature      float64 `yaml:"temperature" json:"temperature"`               // default 0.7
	TopP             float64 `yaml:"top_p" json:"top_p"`                           // default 0.9
	MaxTokens        int     `yaml:"max_tokens" json:"max_tokens"`                 // default 0 (no limit)
}

type IdentityConfig struct {
	UserName        string `yaml:"user_name"`
	AssistantName   string `yaml:"assistant_name"`
	Style           string `yaml:"style"`
	SystemRole      string `yaml:"system_role"`
	IncognitoPrompt string `yaml:"incognito_prompt"`
}

type MemoryConfig struct {
	PersistDir         string  `yaml:"persist_dir" json:"persist_dir"`
	TopK               int     `yaml:"top_k" json:"top_k"`
	MinSimilarity      float32 `yaml:"min_similarity" json:"min_similarity"`
	MemoryEnabled      bool    `yaml:"memory_enabled" json:"memory_enabled"`
	EmbeddingDimension int     `yaml:"embedding_dimension" json:"embedding_dimension"`
	EmbeddingModelRepo string  `yaml:"embedding_model_repo" json:"embedding_model_repo"`
	EmbeddingModelFile string  `yaml:"embedding_model_file" json:"embedding_model_file"`
	EmbeddingAutoStart bool    `yaml:"embedding_auto_start" json:"embedding_auto_start"`
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
			PersistDir:         "./data/memory",
			TopK:               5,
			MinSimilarity:      0.1,
			MemoryEnabled:      true,
			EmbeddingDimension: 768,
			EmbeddingModelRepo: "nomic-ai/nomic-embed-text-v1.5-GGUF",
			EmbeddingModelFile: "nomic-embed-text-v1.5.Q4_K_M.gguf",
		},
		RemoteAccess: RemoteAccessConfig{
			Enabled: false,
			Port:    8090,
			Token:   "",
		},
		Llama: LlamaConfig{
			EngineMode:    "auto",
			BinaryPath:    "",
			Port:          8081,
			EmbeddingPort: 8082,
			CtxSize:       4096,
			MaxHistory:    20,
			ModelsDir:     "./data/models",
			Temperature:   0.7,
			TopP:          0.9,
			MaxTokens:     0,
		},
		Sync: SyncConfig{
			Enabled:          false,
			TokenPath:        "./data/sync_token.json",
			IntervalMessages: 50,
		},
		WhatsApp: WhatsAppConfig{
			Enabled:        true,
			DataDir:        "./data/whatsapp",
			AutoIndex:      true,
			MaxHistoryDays: 7,
		},
		Proactive: ProactiveConfig{
			Enabled: false,
			Level:   "off",
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
			cfg.validate() // normalize data paths (e.g. rebase onto DataDir) before first save
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

	if fixes := cfg.validate(); len(fixes) > 0 {
		log.Printf("config: applied defaults for: %v", fixes)
	}
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

	return os.WriteFile(path, data, 0600)
}

func (c *AppConfig) validate() []string {
	var fixes []string
	if c.API.BaseURL == "" {
		c.API.BaseURL = "http://127.0.0.1:8081/v1"
		fixes = append(fixes, "API.BaseURL")
	}
	if c.API.TimeoutSeconds <= 0 {
		c.API.TimeoutSeconds = 120
		fixes = append(fixes, "API.TimeoutSeconds")
	}
	if c.Identity.AssistantName == "" {
		c.Identity.AssistantName = "Memo"
		fixes = append(fixes, "Identity.AssistantName")
	}
	if c.Identity.Style == "" {
		c.Identity.Style = "casual"
		fixes = append(fixes, "Identity.Style")
	}
	if c.Memory.PersistDir == "" {
		c.Memory.PersistDir = "./data/memory"
		fixes = append(fixes, "Memory.PersistDir")
	}
	if c.Memory.TopK <= 0 {
		c.Memory.TopK = 5
		fixes = append(fixes, "Memory.TopK")
	}
	if c.Memory.MinSimilarity <= 0 {
		c.Memory.MinSimilarity = 0.3
		fixes = append(fixes, "Memory.MinSimilarity")
	}
	if c.Memory.EmbeddingDimension <= 0 {
		c.Memory.EmbeddingDimension = 768
		fixes = append(fixes, "Memory.EmbeddingDimension")
	}
	if c.RemoteAccess.Port <= 0 || c.RemoteAccess.Port > 65535 {
		c.RemoteAccess.Port = 8080
		fixes = append(fixes, "RemoteAccess.Port")
	}
	if c.Llama.Port <= 0 || c.Llama.Port > 65535 {
		c.Llama.Port = 8081
		fixes = append(fixes, "Llama.Port")
	}
	if c.Llama.EmbeddingPort <= 0 || c.Llama.EmbeddingPort > 65535 {
		c.Llama.EmbeddingPort = 8082
		fixes = append(fixes, "Llama.EmbeddingPort")
	}
	if c.Llama.CtxSize <= 0 {
		c.Llama.CtxSize = 4096
		fixes = append(fixes, "Llama.CtxSize")
	}
	if c.Llama.MaxHistory <= 0 {
		c.Llama.MaxHistory = 20
		fixes = append(fixes, "Llama.MaxHistory")
	}
	if c.Llama.ModelsDir == "" {
		c.Llama.ModelsDir = "./data/models"
		fixes = append(fixes, "Llama.ModelsDir")
	}
	if c.Llama.Temperature <= 0 {
		c.Llama.Temperature = 0.7
		fixes = append(fixes, "Llama.Temperature")
	}
	if c.Llama.TopP <= 0 {
		c.Llama.TopP = 0.9
		fixes = append(fixes, "Llama.TopP")
	}
	if c.Llama.MaxTokens < 0 {
		c.Llama.MaxTokens = 0
		fixes = append(fixes, "Llama.MaxTokens")
	}
	if c.Sync.TokenPath == "" {
		c.Sync.TokenPath = "./data/sync_token.json"
		fixes = append(fixes, "Sync.TokenPath")
	}
	if c.Sync.IntervalMessages <= 0 {
		c.Sync.IntervalMessages = 50
		fixes = append(fixes, "Sync.IntervalMessages")
	}

	// Rebase process-relative "data/..." paths onto the resolved DataDir so that
	// shipped/old configs keep working on Windows, where the install directory is
	// read-only. No-op on Linux/macOS (DataDir is the relative "data").
	c.Memory.PersistDir = rebaseDataPath(c.Memory.PersistDir)
	c.Llama.ModelsDir = rebaseDataPath(c.Llama.ModelsDir)
	c.Sync.TokenPath = rebaseDataPath(c.Sync.TokenPath)
	c.WhatsApp.DataDir = rebaseDataPath(c.WhatsApp.DataDir)

	return fixes
}

// rebaseDataPath rewrites a process-relative path under "data/" (optionally
// prefixed with "./") onto the resolved DataDir. Absolute paths and paths not
// under "data/" are returned unchanged.
func rebaseDataPath(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	norm := strings.TrimPrefix(filepath.ToSlash(p), "./")
	if norm == "data" {
		return DataDir()
	}
	if rest, ok := strings.CutPrefix(norm, "data/"); ok {
		return DataPath(filepath.FromSlash(rest))
	}
	return p
}
