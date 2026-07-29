package tts

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/fileutil"
	"memo/internal/logx"
	"memo/internal/provider"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ConfigManager manages external TTS provider configurations with encrypted
// API keys. Mirrors internal/provider.ConfigManager's shape and behavior
// (same file-persist-with-encrypted-keys pattern), but keyed by provider.
// DefaultMachineKey() rather than deriving/persisting its own machine key —
// TTS provider secrets are encrypted with the exact same machine-bound key
// providers.json already uses, not a second independent one.
type ConfigManager struct {
	mu        sync.RWMutex
	filePath  string
	configs   []ProviderConfig
	masterKey []byte
}

// NewConfigManager creates a new TTS provider config manager. masterKey is
// used to encrypt/decrypt API keys (32 bytes for AES-256); pass nil to use
// the shared machine key (provider.DefaultMachineKey()).
func NewConfigManager(filePath string, masterKey []byte) *ConfigManager {
	if len(masterKey) == 0 {
		masterKey = provider.DefaultMachineKey()
	}
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

// Load reads TTS provider configs from the JSON file. Unlike
// provider.ConfigManager.Load, a missing file seeds an empty list, not a
// curated set of disabled defaults — Faz 2 only ships one provider type
// (OpenAI) and there's no sensible default API key/voice to pre-fill.
func (cm *ConfigManager) Load() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	data, err := os.ReadFile(cm.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logx.Printf("TTS: failed to load provider config: %v", err)
		}
		cm.configs = []ProviderConfig{}
		return
	}

	var stored struct {
		Configs []providerConfigStored `json:"providers"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		logx.Printf("TTS: failed to parse provider config: %v", err)
		cm.configs = []ProviderConfig{}
		return
	}

	cm.configs = make([]ProviderConfig, 0, len(stored.Configs))
	for _, s := range stored.Configs {
		apiKey, err := cm.decrypt(s.APIKeyEncrypted)
		if err != nil {
			logx.Printf("TTS: failed to decrypt key for %s: %v", s.Type, err)
			apiKey = ""
		}
		cm.configs = append(cm.configs, ProviderConfig{
			Type:     s.Type,
			Name:     s.Name,
			APIKey:   apiKey,
			Voice:    s.Voice,
			Enabled:  s.Enabled,
			Priority: s.Priority,
		})
	}
}

// Save persists TTS provider configs to the JSON file with encrypted API keys.
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
			logx.Printf("TTS: failed to encrypt key for %s: %v", cfg.Type, err)
			continue
		}
		stored.Configs = append(stored.Configs, providerConfigStored{
			Type:            cfg.Type,
			Name:            cfg.Name,
			APIKeyEncrypted: encrypted,
			Voice:           cfg.Voice,
			Enabled:         cfg.Enabled,
			Priority:        cfg.Priority,
		})
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		logx.Printf("TTS: failed to marshal provider config: %v", err)
		return
	}

	dir := filepath.Dir(cm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logx.Printf("TTS: failed to create provider config dir: %v", err)
		return
	}

	if err := fileutil.AtomicWrite(cm.filePath, data, 0600); err != nil {
		logx.Printf("TTS: failed to write provider config: %v", err)
	}
}

// GetAll returns a copy of all TTS provider configs (API keys in plaintext).
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

// Set sets a TTS provider config (add or update), matched by Name (falling
// back to Type if Name is empty) — same convention as
// internal/provider.ConfigManager.Set.
func (cm *ConfigManager) Set(cfg ProviderConfig) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

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

// Delete removes a TTS provider config by type or name.
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

// TestConnection constructs a provider from cfg and tries a short real
// synthesis call — the most reliable way to confirm an API key/voice combo
// actually works, mirroring provider.ConfigManager.TestConnection.
func (cm *ConfigManager) TestConnection(cfg *ProviderConfig) error {
	p, err := NewProvider(*cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = p.Synthesize(ctx, "test", cfg.Voice)
	return err
}

// encrypt encrypts plaintext with AES-256-GCM. Duplicated from
// internal/provider.ConfigManager.encrypt (same algorithm, same key
// source via provider.DefaultMachineKey) rather than sharing a type across
// packages — small enough (AES-GCM boilerplate) that a cross-package
// dependency here isn't worth it, and provider.ConfigManager's fields are
// unexported so there's no existing seam to reuse instead.
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

type providerConfigStored struct {
	Type            ProviderType `json:"type"`
	Name            string       `json:"name"`
	APIKeyEncrypted string       `json:"api_key_encrypted"`
	Voice           string       `json:"voice"`
	Enabled         bool         `json:"enabled"`
	Priority        int          `json:"priority"`
}
