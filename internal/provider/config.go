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
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
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
		log.Printf("PROVIDER: failed to load config: %v", err)
		cm.configs = defaultConfigs()
		return
	}

	var stored struct {
		Configs []providerConfigStored `json:"providers"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		log.Printf("PROVIDER: failed to parse config: %v", err)
		cm.configs = defaultConfigs()
		return
	}

	cm.configs = make([]ProviderConfig, 0, len(stored.Configs))
	for _, s := range stored.Configs {
		apiKey, err := cm.decrypt(s.APIKeyEncrypted)
		if err != nil {
			log.Printf("PROVIDER: failed to decrypt key for %s: %v", s.Type, err)
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
			log.Printf("PROVIDER: failed to encrypt key for %s: %v", cfg.Type, err)
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
		log.Printf("PROVIDER: failed to marshal config: %v", err)
		return
	}

	dir := filepath.Dir(cm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("PROVIDER: failed to create config dir: %v", err)
		return
	}

	if err := os.WriteFile(cm.filePath, data, 0600); err != nil {
		log.Printf("PROVIDER: failed to write config: %v", err)
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

// GetEnabled returns only enabled provider configs.
func (cm *ConfigManager) GetEnabled() []ProviderConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var enabled []ProviderConfig
	for _, cfg := range cm.configs {
		if cfg.Enabled {
			enabled = append(enabled, cfg)
		}
	}
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

func defaultConfigs() []ProviderConfig {
	return []ProviderConfig{
		{
			Type:    ProviderOpenAI,
			Name:    "OpenAI",
			Model:   DefaultModels[ProviderOpenAI],
			Enabled: false,
		},
		{
			Type:    ProviderGemini,
			Name:    "Google Gemini",
			Model:   DefaultModels[ProviderGemini],
			Enabled: false,
		},
		{
			Type:    ProviderGrok,
			Name:    "xAI Grok",
			Model:   DefaultModels[ProviderGrok],
			Enabled: false,
		},
		{
			Type:    ProviderClaude,
			Name:    "Anthropic Claude",
			Model:   DefaultModels[ProviderClaude],
			Enabled: false,
		},
		{
			Type:    ProviderOpenRouter,
			Name:    "OpenRouter",
			Model:   DefaultModels[ProviderOpenRouter],
			Enabled: false,
		},
		{
			Type:    ProviderGroq,
			Name:    "Groq",
			Model:   DefaultModels[ProviderGroq],
			Enabled: false,
		},
		{
			Type:    ProviderOllama,
			Name:    "Ollama",
			Model:   DefaultModels[ProviderOllama],
			Enabled: false,
		},
	}
}

// defaultMachineKey derives a machine-specific key from hardware info.
// It first tries to load a previously generated random key from disk,
// then falls back to platform-specific machine IDs. If all fail, a new
// random key is generated and persisted for future use.
func defaultMachineKey() []byte {
	// 1. Try to load a previously generated random key
	keyDir := filepath.Join("data")
	keyPath := filepath.Join(keyDir, "machine.key")
	if data, err := os.ReadFile(keyPath); err == nil && len(data) >= 32 {
		return data[:32]
	}

	// 2. Try platform-specific machine IDs
	if runtime.GOOS == "windows" {
		out, err := exec.Command("powershell", "-NoProfile", "-Command",
			`(Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Cryptography' MachineGuid).MachineGuid`).Output()
		if err == nil {
			if id := strings.TrimSpace(string(out)); len(id) >= 32 {
				return []byte(id[:32])
			}
		}
	} else {
		if data, err := os.ReadFile("/etc/machine-id"); err == nil && len(data) >= 32 {
			return []byte(data[:32])
		}
		out, err := exec.Command("sh", "-c",
			`ioreg -d2 -c IOPlatformExpertDevice | awk -F\" '/IOPlatformUUID/{print $4}'`).Output()
		if err == nil {
			if id := strings.TrimSpace(string(out)); len(id) >= 32 {
				return []byte(id[:32])
			}
		}
	}

	// 3. No machine-specific ID available — generate a random key and persist it
	randomKey := make([]byte, 32)
	if _, err := rand.Read(randomKey); err != nil {
		log.Printf("provider: crypto/rand failed, generating fallback key: %v", err)
		for i := range randomKey {
			randomKey[i] = byte(i ^ 0xAA)
		}
	}
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		log.Printf("provider: cannot create key dir %s: %v", keyDir, err)
	} else if err := os.WriteFile(keyPath, randomKey, 0600); err != nil {
		log.Printf("provider: cannot persist machine key: %v", err)
	}
	return randomKey
}

// SetMasterKey allows setting a custom master key from outside.
func (cm *ConfigManager) SetMasterKey(key []byte) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	k := make([]byte, 32)
	copy(k, key)
	cm.masterKey = k
}
