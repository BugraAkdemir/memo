package cloudsync

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	passphrase := "test-passphrase-123"
	plaintext := []byte("hello world, this is a secret message")

	ciphertext, err := encrypt(passphrase, plaintext)
	if err != nil {
		t.Fatalf("encrypt() error = %v", err)
	}

	if bytes.Equal(ciphertext, plaintext) {
		t.Error("ciphertext equals plaintext — no encryption")
	}

	decrypted, err := decrypt(passphrase, ciphertext)
	if err != nil {
		t.Fatalf("decrypt() error = %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted = %q, want %q", string(decrypted), string(plaintext))
	}
}

func TestEncryptDecryptEmptyPassphrase(t *testing.T) {
	// Empty passphrase must now be rejected — callers (sync_manager.New) must
	// always resolve a passphrase before calling encrypt/decrypt.
	_, err := encrypt("", []byte("test"))
	if err == nil {
		t.Fatal("encrypt() with empty passphrase should return error")
	}
	_, err = decrypt("", []byte("test"))
	if err == nil {
		t.Fatal("decrypt() with empty passphrase should return error")
	}
}

func TestDecryptWrongPassphrase(t *testing.T) {
	plaintext := []byte("secret data")
	ciphertext, err := encrypt("correct-passphrase", plaintext)
	if err != nil {
		t.Fatalf("encrypt() error = %v", err)
	}

	_, err = decrypt("wrong-passphrase", ciphertext)
	if err == nil {
		t.Error("decrypt() with wrong passphrase should fail")
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	plaintext := []byte("secret data")
	ciphertext, err := encrypt("passphrase", plaintext)
	if err != nil {
		t.Fatalf("encrypt() error = %v", err)
	}

	ciphertext[len(ciphertext)-1] ^= 0xFF
	_, err = decrypt("passphrase", ciphertext)
	if err == nil {
		t.Error("decrypt() with tampered ciphertext should fail")
	}
}

func TestDecryptTooShortData(t *testing.T) {
	_, err := decrypt("passphrase", []byte{1, 2, 3})
	if err == nil {
		t.Error("decrypt() with too-short data should fail")
	}
}

func TestEncryptProducesDifferentOutputEachTime(t *testing.T) {
	passphrase := "test"
	plaintext := []byte("same message")

	c1, _ := encrypt(passphrase, plaintext)
	c2, _ := encrypt(passphrase, plaintext)

	if bytes.Equal(c1, c2) {
		t.Error("encrypt() should produce different output each time (random salt+nonce)")
	}

	d1, _ := decrypt(passphrase, c1)
	d2, _ := decrypt(passphrase, c2)
	if !bytes.Equal(d1, d2) {
		t.Error("decrypt of different ciphertexts should yield same plaintext")
	}
}

func TestDecryptLegacySHA256Format(t *testing.T) {
	passphrase := "legacy-passphrase"

	plaintext := []byte("data encrypted with old SHA-256 method")
	key := deriveKey(passphrase)

	// Simulate legacy encryption: AES-256-GCM with SHA-256 derived key (no salt)
	ciphertext, err := encryptWithKey(key[:], plaintext)
	if err != nil {
		t.Fatalf("encryptWithKey error = %v", err)
	}

	decrypted, err := decrypt(passphrase, ciphertext)
	if err != nil {
		t.Fatalf("decrypt() legacy format error = %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted = %q, want %q", string(decrypted), string(plaintext))
	}
}

func encryptWithKey(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func TestDeriveKeyDeterministic(t *testing.T) {
	k1 := deriveKey("test")
	k2 := deriveKey("test")
	if k1 != k2 {
		t.Error("deriveKey() should be deterministic for same passphrase")
	}
}

func TestDeriveKeyDifferentPassphrases(t *testing.T) {
	k1 := deriveKey("pass1")
	k2 := deriveKey("pass2")
	if k1 == k2 {
		t.Error("deriveKey() should differ for different passphrases")
	}
}

func TestDeriveKeyEmptyPassphrase(t *testing.T) {
	k := deriveKey("")
	if k == [32]byte{} {
		t.Error("deriveKey('') should not return zero key")
	}
}
