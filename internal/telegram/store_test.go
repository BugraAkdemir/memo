package telegram

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStore_RoundTripsEncryptedToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telegram.json")
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	s := NewStore(path, key)
	s.Set(State{Enabled: true, BotToken: "123456:ABC-DEF", BotUsername: "memo_bot"})
	s.SetOwner(999, "Bugra")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("state file not written: %v", err)
	}
	if strings.Contains(string(raw), "123456:ABC-DEF") {
		t.Fatalf("bot token stored in plaintext on disk: %s", raw)
	}

	s2 := NewStore(path, key)
	got := s2.Get()
	if got.BotToken != "123456:ABC-DEF" {
		t.Errorf("BotToken after reload = %q, want the original token", got.BotToken)
	}
	if got.OwnerChatID != 999 || got.OwnerName != "Bugra" || !got.Linked() {
		t.Errorf("owner not persisted: %+v", got)
	}
	if !got.Enabled || got.BotUsername != "memo_bot" {
		t.Errorf("Enabled/BotUsername not persisted: %+v", got)
	}
}

func TestStore_ClearWipesTokenAndOwner(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telegram.json")

	key := make([]byte, 32) // fixed key: nil falls back to provider.DefaultMachineKey(), touching config.DataDir()
	s := NewStore(path, key)
	s.Set(State{Enabled: true, BotToken: "123456:ABC-DEF"})
	s.SetOwner(999, "Bugra")

	s.Clear()

	s2 := NewStore(path, key)
	if got := s2.Get(); got.Linked() || got.BotToken != "" || got.Enabled {
		t.Errorf("Clear() did not wipe state: %+v", got)
	}
}

func TestStore_WrongKeyFailsToDecrypt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telegram.json")
	key1 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i)
	}
	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = byte(i + 1)
	}

	s := NewStore(path, key1)
	s.Set(State{BotToken: "secret-token"})

	// A different key can't decrypt it — must degrade to an empty token
	// rather than panicking or returning garbage.
	s2 := NewStore(path, key2)
	if got := s2.Get().BotToken; got != "" {
		t.Errorf("BotToken decrypted with wrong key = %q, want empty", got)
	}
}
