package provider

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/logx"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"memo/internal/config"
	"memo/internal/fileutil"
)

// ConfigManager manages provider configurations with encrypted API keys.
type ConfigManager struct {
	mu       sync.RWMutex
	filePath string
	configs  []ProviderConfig
	masterKey []byte
}

// NewConfigManager creates a new provider config manager.
// masterKey is used to encrypt/decrypt API keys (32 bytes for AES-256).
func NewConfigManager(filePath string, masterKey []byte) *ConfigManager {
	if len(masterKey) == 0 {
		// Fallback: derive from machine ID or use a hardware-bound key
		masterKey = defaultMachineKey()
	}
	// Ensure we have a 32-byte key for AES-256
	key := make([]byte, 32)
	copy(key, masterKey)

	cm := &ConfigManager{
		filePath:  filePath,
		configs:   []ProviderConfig{},
		masterKey: key,
	}
	cm.Load()
	return cm
}

// Load reads provider configs from the JSON file.
func (cm *ConfigManager) Load() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	data, err := os.ReadFile(cm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			cm.configs = defaultConfigs()
			cm.saveLocked()
			return
		}
		logx.Printf("PROVIDER: failed to load config: %v", err)
		cm.configs = defaultConfigs()
		return
	}

	var stored struct {
		Configs []providerConfigStored `json:"providers"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		logx.Printf("PROVIDER: failed to parse config: %v", err)
		cm.configs = defaultConfigs()
		return
	}

	cm.configs = make([]ProviderConfig, 0, len(stored.Configs))
	for _, s := range stored.Configs {
		apiKey, err := cm.decrypt(s.APIKeyEncrypted)
		if err != nil {
			logx.Printf("PROVIDER: failed to decrypt key for %s: %v", s.Type, err)
			apiKey = ""
		}
		cm.configs = append(cm.configs, ProviderConfig{
			Type:        s.Type,
			Name:        s.Name,
			APIKey:      apiKey,
			BaseURL:     s.BaseURL,
			Model:       s.Model,
			Enabled:     s.Enabled,
			Priority:    s.Priority,
			Temperature: s.Temperature,
			TopP:        s.TopP,
			MaxTokens:   s.MaxTokens,
		})
	}
}

// Save persists provider configs to the JSON file with encrypted API keys.
func (cm *ConfigManager) Save() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.saveLocked()
}

func (cm *ConfigManager) saveLocked() {
	stored := struct {
		Configs []providerConfigStored `json:"providers"`
	}{
		Configs: make([]providerConfigStored, 0, len(cm.configs)),
	}

	for _, cfg := range cm.configs {
		encrypted, err := cm.encrypt(cfg.APIKey)
		if err != nil {
			logx.Printf("PROVIDER: failed to encrypt key for %s: %v", cfg.Type, err)
			continue
		}
		stored.Configs = append(stored.Configs, providerConfigStored{
			Type:           cfg.Type,
			Name:           cfg.Name,
			APIKeyEncrypted: encrypted,
			BaseURL:        cfg.BaseURL,
			Model:          cfg.Model,
			Enabled:        cfg.Enabled,
			Priority:       cfg.Priority,
			Temperature:    cfg.Temperature,
			TopP:           cfg.TopP,
			MaxTokens:      cfg.MaxTokens,
		})
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		logx.Printf("PROVIDER: failed to marshal config: %v", err)
		return
	}

	dir := filepath.Dir(cm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logx.Printf("PROVIDER: failed to create config dir: %v", err)
		return
	}

	if err := fileutil.AtomicWrite(cm.filePath, data, 0600); err != nil {
		logx.Printf("PROVIDER: failed to write config: %v", err)
	}
}

// GetAll returns a copy of all provider configs (API keys in plaintext).
func (cm *ConfigManager) GetAll() []ProviderConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	configs := make([]ProviderConfig, len(cm.configs))
	copy(configs, cm.configs)
	return configs
}

// GetEnabled returns only enabled provider configs, sorted by Priority ascending
// (lower number = higher priority, matching the router's behaviour).
func (cm *ConfigManager) GetEnabled() []ProviderConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var enabled []ProviderConfig
	for _, cfg := range cm.configs {
		if cfg.Enabled {
			enabled = append(enabled, cfg)
		}
	}
	sort.Slice(enabled, func(i, j int) bool {
		return enabled[i].Priority < enabled[j].Priority
	})
	return enabled
}

// Set sets a provider config (add or update).
func (cm *ConfigManager) Set(cfg ProviderConfig) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Match on Name (user-facing unique label) first, fall back to Type for legacy.
	key := cfg.Name
	if key == "" {
		key = string(cfg.Type)
	}
	for i, c := range cm.configs {
		matchKey := c.Name
		if matchKey == "" {
			matchKey = string(c.Type)
		}
		if matchKey == key {
			cm.configs[i] = cfg
			cm.saveLocked()
			return
		}
	}
	cm.configs = append(cm.configs, cfg)
	cm.saveLocked()
}

// Delete removes a provider config by type or name.
// If name is non-empty, it matches on Name first. Falls back to Type match.
func (cm *ConfigManager) Delete(pt ProviderType, name ...string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(name) > 0 && name[0] != "" {
		for i, cfg := range cm.configs {
			if cfg.Name == name[0] {
				cm.configs = append(cm.configs[:i], cm.configs[i+1:]...)
				cm.saveLocked()
				return
			}
		}
		return
	}

	for i, cfg := range cm.configs {
		if cfg.Type == pt {
			cm.configs = append(cm.configs[:i], cm.configs[i+1:]...)
			cm.saveLocked()
			return
		}
	}
}

// encrypt encrypts plaintext with AES-256-GCM.
func (cm *ConfigManager) encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(cm.masterKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// decrypt decrypts hex-encoded AES-256-GCM ciphertext.
func (cm *ConfigManager) decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}

	ciphertext, err := hex.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(cm.masterKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// TestConnection tests a provider config and returns the result.
func (cm *ConfigManager) TestConnection(cfg *ProviderConfig) error {
	p, err := NewProvider(*cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Try a simple chat completion (more reliable than ListModels for testing)
	_, err = p.ChatCompletion(ctx, ChatRequest{
		Model: cfg.Model,
		Messages: []Message{
			TextMessage("user", "Say exactly 'ok' and nothing else"),
		},
		MaxTokens: 10,
		Temperature: 0,
	})
	if err != nil {
		// Fallback: try listing models
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel2()
		_, listErr := p.ListModels(ctx2)
		if listErr != nil {
			return fmt.Errorf("chat: %v; list: %v", err, listErr)
		}
	}
	return nil
}

type providerConfigStored struct {
	Type             ProviderType `json:"type"`
	Name             string       `json:"name"`
	APIKeyEncrypted  string       `json:"api_key_encrypted"`
	BaseURL          string       `json:"base_url,omitempty"`
	Model            string       `json:"model"`
	Enabled          bool         `json:"enabled"`
	Priority         int          `json:"priority"`
	Temperature      float64      `json:"temperature,omitempty"`
	TopP             float64      `json:"top_p,omitempty"`
	MaxTokens        int          `json:"max_tokens,omitempty"`
}

// defaultConfigs is what a fresh install's providers.json starts as. Used
// to seed every known provider type as a disabled placeholder entry — the
// Settings > API Providers list showed OpenAI/Gemini/Grok/Claude/
// OpenRouter/Groq/Ollama/OpenCode Zen/OpenCode Go as "Disabled" cards
// before the user had ever touched any of them, which only grew more
// cluttered as new provider types (Kilo, ...) were added. Direct user
// feedback: this list should show only what was actually added via "Add
// Provider," nothing pre-populated. Existing installs whose providers.json
// already has these seeded entries aren't touched here (Load() only calls
// this when the file doesn't exist yet or fails to parse) — see
// providers_tab.dart's own filter for how an already-cluttered existing
// list is handled without a destructive migration.
func defaultConfigs() []ProviderConfig {
	return []ProviderConfig{}
}

// defaultMachineKey derives a machine-specific key from hardware info.
// It first tries to load a previously generated random key from disk,
// then falls back to platform-specific machine IDs. If all fail, a new
// random key is generated and persisted for future use.
func defaultMachineKey() []byte {
	// 1. Try to load a previously generated random key. Route through the
	// canonical data dir (honours MEMO_DATA_DIR and %ProgramData% on Windows)
	// rather than a CWD-relative "data" folder, so a different launch directory
	// can't miss the key and silently re-key the encrypted provider secrets.
	keyDir := config.DataDir()
	keyPath := filepath.Join(keyDir, "machine.key")
	if data, err := os.ReadFile(keyPath); err == nil && len(data) >= 32 {
		return data[:32]
	}
	// Legacy fallback: the key used to be stored under a CWD-relative "data" dir.
	// If found there, migrate it to the canonical location so it survives.
	if legacyPath := filepath.Join("data", "machine.key"); legacyPath != keyPath {
		if data, err := os.ReadFile(legacyPath); err == nil && len(data) >= 32 {
			if err := os.MkdirAll(keyDir, 0700); err == nil {
				_ = os.WriteFile(keyPath, data[:32], 0600)
			}
			return data[:32]
		}
	}

	// 2. Generate a cryptographically random key and persist it with strict permissions.
	// Predictable machine IDs (/etc/machine-id, Registry MachineGuid, IOPlatformUUID)
	// are intentionally not used as key material — they are not secret.
	randomKey := make([]byte, 32)
	if _, err := rand.Read(randomKey); err != nil {
		logx.Printf("provider: crypto/rand failed: %v", err)
		return randomKey
	}
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		logx.Printf("provider: cannot create key dir %s: %v", keyDir, err)
		return randomKey
	}
	if err := os.WriteFile(keyPath, randomKey, 0600); err != nil {
		logx.Printf("provider: cannot persist machine key: %v", err)
		return randomKey
	}
	// On Windows, restrict the file to the current user via icacls.
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("icacls", keyPath, "/inheritance:r",
			"/grant:r", os.Getenv("USERNAME")+":F").CombinedOutput(); err != nil {
			logx.Printf("provider: icacls failed for machine key: %v — %s", err, out)
		}
	}
	return randomKey
}

// DefaultMachineKey exposes defaultMachineKey to other packages that need
// to encrypt secrets with the same machine-bound key providers.json already
// uses (e.g. internal/tts's own ConfigManager for TTS provider API keys) —
// without this, a second package deriving its own "default" key would
// either duplicate the machine.key read/generate/persist logic verbatim or,
// worse, silently mint a second key file and make two independent claims
// about what "the machine key" is.
func DefaultMachineKey() []byte {
	return defaultMachineKey()
}

// SetMasterKey allows setting a custom master key from outside.
func (cm *ConfigManager) SetMasterKey(key []byte) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	k := make([]byte, 32)
	copy(k, key)
	cm.masterKey = k
}
