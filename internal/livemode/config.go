package livemode

import (
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
)

// ConfigManager manages the four non-local engines' configurations with
// encrypted API keys, persisted to data/livemode_engines.json. Same
// file-persist-with-encrypted-keys pattern and shared machine key
// (provider.DefaultMachineKey()) internal/tts.ConfigManager already uses —
// but keyed one-entry-per-EngineType rather than a priority-ordered slice,
// since Live Mode has exactly one active engine at a time, not a fallback
// chain (see engine.go's package doc comment).
type ConfigManager struct {
	mu        sync.RWMutex
	filePath  string
	configs   map[EngineType]EngineConfig
	masterKey []byte
}

// NewConfigManager creates a new Live Mode engine config manager. masterKey
// is used to encrypt/decrypt API keys (32 bytes for AES-256); pass nil to
// use the shared machine key (provider.DefaultMachineKey()).
func NewConfigManager(filePath string, masterKey []byte) *ConfigManager {
	if len(masterKey) == 0 {
		masterKey = provider.DefaultMachineKey()
	}
	key := make([]byte, 32)
	copy(key, masterKey)

	cm := &ConfigManager{
		filePath:  filePath,
		configs:   map[EngineType]EngineConfig{},
		masterKey: key,
	}
	cm.Load()
	return cm
}

// Load reads engine configs from the JSON file. A missing file seeds an
// empty map — no sensible default API key/model exists to pre-fill for any
// of the four engine types.
func (cm *ConfigManager) Load() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	data, err := os.ReadFile(cm.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logx.Printf("LiveMode: failed to load engine config: %v", err)
		}
		cm.configs = map[EngineType]EngineConfig{}
		return
	}

	var stored struct {
		Engines []engineConfigStored `json:"engines"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		logx.Printf("LiveMode: failed to parse engine config: %v", err)
		cm.configs = map[EngineType]EngineConfig{}
		return
	}

	cm.configs = make(map[EngineType]EngineConfig, len(stored.Engines))
	for _, s := range stored.Engines {
		apiKey, err := cm.decrypt(s.APIKeyEncrypted)
		if err != nil {
			logx.Printf("LiveMode: failed to decrypt key for %s: %v", s.Type, err)
			apiKey = ""
		}
		cm.configs[s.Type] = EngineConfig{
			Type:    s.Type,
			APIKey:  apiKey,
			Model:   s.Model,
			Voice:   s.Voice,
			BaseURL: s.BaseURL,
			Enabled: s.Enabled,
		}
	}
}

// Save persists engine configs to the JSON file with encrypted API keys.
func (cm *ConfigManager) Save() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.saveLocked()
}

func (cm *ConfigManager) saveLocked() {
	stored := struct {
		Engines []engineConfigStored `json:"engines"`
	}{
		Engines: make([]engineConfigStored, 0, len(cm.configs)),
	}

	for _, cfg := range cm.configs {
		encrypted, err := cm.encrypt(cfg.APIKey)
		if err != nil {
			logx.Printf("LiveMode: failed to encrypt key for %s: %v", cfg.Type, err)
			continue
		}
		stored.Engines = append(stored.Engines, engineConfigStored{
			Type:            cfg.Type,
			APIKeyEncrypted: encrypted,
			Model:           cfg.Model,
			Voice:           cfg.Voice,
			BaseURL:         cfg.BaseURL,
			Enabled:         cfg.Enabled,
		})
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		logx.Printf("LiveMode: failed to marshal engine config: %v", err)
		return
	}

	dir := filepath.Dir(cm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logx.Printf("LiveMode: failed to create engine config dir: %v", err)
		return
	}

	if err := fileutil.AtomicWrite(cm.filePath, data, 0600); err != nil {
		logx.Printf("LiveMode: failed to write engine config: %v", err)
	}
}

// Get returns the config for one engine type, if any is saved.
func (cm *ConfigManager) Get(t EngineType) (EngineConfig, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	cfg, ok := cm.configs[t]
	return cfg, ok
}

// GetAll returns every saved engine config (API keys in plaintext), in no
// particular order.
func (cm *ConfigManager) GetAll() []EngineConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	configs := make([]EngineConfig, 0, len(cm.configs))
	for _, cfg := range cm.configs {
		configs = append(configs, cfg)
	}
	return configs
}

// Set saves or replaces one engine's config, keyed by Type — unlike
// internal/tts.ConfigManager.Set there is no separate Name field to match
// on, since only one config per EngineType can ever exist.
func (cm *ConfigManager) Set(cfg EngineConfig) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.configs[cfg.Type] = cfg
	cm.saveLocked()
}

// Delete removes one engine's saved config.
func (cm *ConfigManager) Delete(t EngineType) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.configs, t)
	cm.saveLocked()
}

// encrypt/decrypt: AES-256-GCM, duplicated from internal/tts.ConfigManager
// (same algorithm, same key source) — same reasoning as that copy's own
// doc comment: small enough that a cross-package dependency isn't worth
// it, and the source's fields are unexported so there's no existing seam
// to share instead.
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

type engineConfigStored struct {
	Type            EngineType `json:"type"`
	APIKeyEncrypted string     `json:"api_key_encrypted"`
	Model           string     `json:"model,omitempty"`
	Voice           string     `json:"voice,omitempty"`
	BaseURL         string     `json:"base_url,omitempty"`
	Enabled         bool       `json:"enabled"`
}
