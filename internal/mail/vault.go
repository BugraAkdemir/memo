// SPDX-License-Identifier: AGPL-3.0-or-later

package mail

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"memo/internal/fileutil"
	"memo/internal/provider"
)

type storedAccountRecord struct {
	ID              string       `json:"id"`
	Email           string       `json:"email"`
	DisplayName     string       `json:"display_name,omitempty"`
	Provider        ProviderType `json:"provider"`
	AuthType        AuthType     `json:"auth_type"`
	Server          ServerConfig `json:"server"`
	SecretEncrypted string       `json:"secret_encrypted,omitempty"` // AES-256-GCM encrypted OAuthToken JSON or Password
	LastSeenUID     uint32       `json:"last_seen_uid,omitempty"`
	ConnectedAt     time.Time    `json:"connected_at"`
}

type storedAccountSecret struct {
	Token    *OAuthToken `json:"token,omitempty"`
	Password string      `json:"password,omitempty"`
}

// Vault encrypts and persists mail account credentials at rest using AES-256-GCM.
type Vault struct {
	filePath  string
	masterKey []byte
}

// NewVault creates a Vault backed by filePath. When masterKey is empty,
// provider.DefaultMachineKey() is used.
func NewVault(filePath string, masterKey []byte) *Vault {
	if len(masterKey) == 0 {
		masterKey = provider.DefaultMachineKey()
	}
	key := make([]byte, 32)
	copy(key, masterKey)
	return &Vault{
		filePath:  filePath,
		masterKey: key,
	}
}

// Load reads and decrypts all saved accounts from disk.
func (v *Vault) Load() ([]*Account, error) {
	data, err := os.ReadFile(v.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Account{}, nil
		}
		return nil, err
	}

	var stored struct {
		Accounts []storedAccountRecord `json:"accounts"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("mail vault parse: %w", err)
	}

	out := make([]*Account, 0, len(stored.Accounts))
	for _, rec := range stored.Accounts {
		acct := &Account{
			ID:          rec.ID,
			Email:       rec.Email,
			DisplayName: rec.DisplayName,
			Provider:    rec.Provider,
			AuthType:    rec.AuthType,
			Server:      rec.Server,
			LastSeenUID: rec.LastSeenUID,
			ConnectedAt: rec.ConnectedAt,
		}
		if rec.SecretEncrypted != "" {
			plain, err := v.decrypt(rec.SecretEncrypted)
			if err != nil {
				return nil, fmt.Errorf("mail vault decrypt %s: %w", rec.Email, err)
			}
			var sec storedAccountSecret
			if err := json.Unmarshal(plain, &sec); err != nil {
				return nil, fmt.Errorf("mail vault secret unmarshal %s: %w", rec.Email, err)
			}
			acct.Token = sec.Token
			acct.Password = sec.Password
		}
		out = append(out, acct)
	}
	return out, nil
}

// Save encrypts and atomically writes all accounts to disk with 0600 permissions.
func (v *Vault) Save(accounts []*Account) error {
	if err := os.MkdirAll(filepath.Dir(v.filePath), 0o700); err != nil {
		return err
	}

	records := make([]storedAccountRecord, 0, len(accounts))
	for _, a := range accounts {
		if a == nil {
			continue
		}
		secBytes, err := json.Marshal(storedAccountSecret{
			Token:    a.Token,
			Password: a.Password,
		})
		if err != nil {
			return err
		}
		enc, err := v.encrypt(secBytes)
		if err != nil {
			return err
		}
		records = append(records, storedAccountRecord{
			ID:              a.ID,
			Email:           a.Email,
			DisplayName:     a.DisplayName,
			Provider:        a.Provider,
			AuthType:        a.AuthType,
			Server:          a.Server,
			SecretEncrypted: enc,
			LastSeenUID:     a.LastSeenUID,
			ConnectedAt:     a.ConnectedAt,
		})
	}

	payload, err := json.MarshalIndent(struct {
		Accounts []storedAccountRecord `json:"accounts"`
	}{Accounts: records}, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.AtomicWrite(v.filePath, payload, 0o600)
}

func (v *Vault) encrypt(plain []byte) (string, error) {
	block, err := aes.NewCipher(v.masterKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, plain, nil)
	return hex.EncodeToString(ciphertext), nil
}

func (v *Vault) decrypt(hexCipher string) ([]byte, error) {
	raw, err := hex.DecodeString(hexCipher)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(v.masterKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return gcm.Open(nil, raw[:ns], raw[ns:], nil)
}
