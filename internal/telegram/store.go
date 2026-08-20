package telegram

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

	"memo/internal/fileutil"
	"memo/internal/logx"
	"memo/internal/provider"
)

// State is the persisted Telegram integration state: the bot token (secret)
// plus which chat is locked in as "the owner" — the only account the bot
// will ever reply to (see internal/app/telegram.go's message loop). Unlike
// internal/whatsapp (a personal account paired via QR, so ownership is
// implicit), a Telegram bot token can be messaged by anyone who finds the
// bot, so the owner lock is the actual security boundary here, not just a
// convenience.
type State struct {
	Enabled     bool   `json:"enabled"`
	BotToken    string `json:"-"` // plaintext in memory only, never marshaled directly
	BotUsername string `json:"bot_username"`
	OwnerChatID int64  `json:"owner_chat_id"` // 0 = not yet linked
	OwnerName   string `json:"owner_name"`
	// AutoApprovePermissions, when true, skips the agent's tool-call
	// permission prompt entirely for the Telegram bot (AllowOnce on every
	// request) instead of asking a y/n question in the chat itself — see
	// the matching field on config.WhatsAppConfig for the WhatsApp side.
	// Opt-in (default false).
	AutoApprovePermissions bool `json:"auto_approve_permissions"`
}

// Linked reports whether a chat has been locked in as the owner.
func (s State) Linked() bool { return s.OwnerChatID != 0 }

// Store persists State to a single JSON file with the bot token encrypted
// at rest — same AES-256-GCM + machine-key pattern as
// internal/provider.ConfigManager and internal/tts.ConfigManager, scaled
// down to one record instead of a list (Telegram only ever has one bot
// token, not a set of provider configs).
type Store struct {
	mu        sync.RWMutex
	filePath  string
	state     State
	masterKey []byte
}

// NewStore loads (or initializes) the Telegram state from filePath.
// masterKey encrypts/decrypts the token; pass nil to use the shared machine
// key (provider.DefaultMachineKey()) — the same key providers.json and
// TTS provider secrets already use.
func NewStore(filePath string, masterKey []byte) *Store {
	if len(masterKey) == 0 {
		masterKey = provider.DefaultMachineKey()
	}
	key := make([]byte, 32)
	copy(key, masterKey)

	s := &Store{filePath: filePath, masterKey: key}
	s.load()
	return s
}

func (s *Store) load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logx.Printf("TELEGRAM: failed to load state: %v", err)
		}
		s.state = State{}
		return
	}

	var stored storedState
	if err := json.Unmarshal(data, &stored); err != nil {
		logx.Printf("TELEGRAM: failed to parse state: %v", err)
		s.state = State{}
		return
	}

	token, err := s.decrypt(stored.BotTokenEncrypted)
	if err != nil {
		logx.Printf("TELEGRAM: failed to decrypt bot token: %v", err)
		token = ""
	}
	s.state = State{
		Enabled:                stored.Enabled,
		BotToken:               token,
		BotUsername:            stored.BotUsername,
		OwnerChatID:            stored.OwnerChatID,
		OwnerName:              stored.OwnerName,
		AutoApprovePermissions: stored.AutoApprovePermissions,
	}
}

// Get returns a copy of the current state.
func (s *Store) Get() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Set replaces the current state and persists it.
func (s *Store) Set(state State) {
	s.mu.Lock()
	s.state = state
	s.mu.Unlock()
	s.save()
}

// SetOwner locks in chatID/name as the owner and persists it — called once,
// the first time an incoming message arrives with no owner linked yet.
func (s *Store) SetOwner(chatID int64, name string) {
	s.mu.Lock()
	s.state.OwnerChatID = chatID
	s.state.OwnerName = name
	s.mu.Unlock()
	s.save()
}

// Clear wipes the stored bot token and owner link entirely (the Telegram
// equivalent of WhatsApp's Logout — as opposed to Set(Enabled:false), which
// only pauses polling and keeps everything else intact).
func (s *Store) Clear() {
	s.mu.Lock()
	s.state = State{}
	s.mu.Unlock()
	s.save()
}

func (s *Store) save() {
	s.mu.RLock()
	st := s.state
	s.mu.RUnlock()

	encrypted, err := s.encrypt(st.BotToken)
	if err != nil {
		logx.Printf("TELEGRAM: failed to encrypt bot token: %v", err)
		return
	}
	stored := storedState{
		Enabled:                st.Enabled,
		BotTokenEncrypted:      encrypted,
		BotUsername:            st.BotUsername,
		OwnerChatID:            st.OwnerChatID,
		OwnerName:              st.OwnerName,
		AutoApprovePermissions: st.AutoApprovePermissions,
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		logx.Printf("TELEGRAM: failed to marshal state: %v", err)
		return
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logx.Printf("TELEGRAM: failed to create state dir: %v", err)
		return
	}
	if err := fileutil.AtomicWrite(s.filePath, data, 0600); err != nil {
		logx.Printf("TELEGRAM: failed to write state: %v", err)
	}
}

type storedState struct {
	Enabled                bool   `json:"enabled"`
	BotTokenEncrypted      string `json:"bot_token_encrypted"`
	BotUsername            string `json:"bot_username"`
	OwnerChatID            int64  `json:"owner_chat_id"`
	OwnerName              string `json:"owner_name"`
	AutoApprovePermissions bool   `json:"auto_approve_permissions"`
}

// encrypt/decrypt: identical AES-256-GCM scheme to
// internal/provider.ConfigManager and internal/tts.ConfigManager
// (duplicated rather than shared for the same reason tts/config.go gives —
// small enough boilerplate that a cross-package type isn't worth it).

func (s *Store) encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(s.masterKey)
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

func (s *Store) decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	ciphertext, err := hex.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(s.masterKey)
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
