// SPDX-License-Identifier: AGPL-3.0-or-later

package geminisub

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
	"sync"

	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/provider"

	"golang.org/x/oauth2"
)

// tokenStore persists a single OAuth2 token, encrypted at rest with
// AES-256-GCM under the shared machine key (provider.DefaultMachineKey — the
// same key that protects providers.json API keys). This is deliberately
// stricter than internal/cloudsync, which stores its Drive token as
// plaintext JSON.
//
// The AES-GCM helpers are copied (not imported) from internal/provider so
// this package stays self-contained; they are ~30 lines and allowed to
// diverge.
type tokenStore struct {
	mu   sync.Mutex
	path string
	key  []byte
}

func newTokenStore() *tokenStore {
	return newTokenStoreAt(filepath.Join(config.DataDir(), "geminisub", "token.enc"), provider.DefaultMachineKey())
}

// newTokenStoreAt is the injectable form used by tests.
func newTokenStoreAt(path string, key []byte) *tokenStore {
	return &tokenStore{path: path, key: key}
}

// load returns the stored token, or (nil, false) when there is no readable
// token — a missing file, an unreadable file, a decrypt failure, or a
// malformed payload all read as "not connected" rather than an error, so a
// corrupted token file never wedges startup.
func (s *tokenStore) load() (*oauth2.Token, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, false
	}
	plain, err := decrypt(s.key, string(data))
	if err != nil {
		logx.Printf("geminisub: stored token failed to decrypt, ignoring it: %v", err)
		return nil, false
	}
	var t oauth2.Token
	if err := json.Unmarshal([]byte(plain), &t); err != nil {
		logx.Printf("geminisub: stored token failed to parse, ignoring it: %v", err)
		return nil, false
	}
	return &t, true
}

func (s *tokenStore) save(t *oauth2.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := json.Marshal(t)
	if err != nil {
		return err
	}
	enc, err := encrypt(s.key, string(raw))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	return os.WriteFile(s.path, []byte(enc), 0600)
}

// clear removes the token file. A missing file is not an error.
func (s *tokenStore) clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// encrypt encrypts plaintext with AES-256-GCM, hex-encoded, nonce prepended.
func encrypt(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
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
	return hex.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil)), nil
}

// decrypt reverses encrypt.
func decrypt(key []byte, encoded string) (string, error) {
	raw, err := hex.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
