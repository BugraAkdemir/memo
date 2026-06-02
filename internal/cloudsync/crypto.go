package cloudsync

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	saltLen      = 16
	pbkdf2Iter   = 600000
)

// deriveKey produces a 32-byte AES-256 key from the given passphrase.
// If passphrase is empty, a hardware-derived identifier is used as fallback.
// Kept for backward compatibility with data encrypted before v3.0.0.
func deriveKey(passphrase string) [32]byte {
	if passphrase == "" {
		passphrase = hardwareID()
	}
	return sha256.Sum256([]byte("memo-sync-v1:" + passphrase))
}

// encrypt encrypts plaintext with AES-256-GCM using PBKDF2 key derivation.
// Output format: [16-byte salt][12-byte nonce][ciphertext + 16-byte GCM auth tag]
func encrypt(passphrase string, plaintext []byte) ([]byte, error) {
	if passphrase == "" {
		passphrase = hardwareID()
	}

	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("cloudsync: encrypt: salt: %w", err)
	}

	key := pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iter, 32, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cloudsync: encrypt: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cloudsync: encrypt: new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("cloudsync: encrypt: nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	// Prepend salt — decrypt extracts it first
	return append(salt, ciphertext...), nil
}

// decrypt reverses encrypt. Tries PBKDF2 first (new format with salt prefix),
// falls back to SHA-256 (legacy format for data encrypted before v3.0.0).
func decrypt(passphrase string, data []byte) ([]byte, error) {
	if passphrase == "" {
		passphrase = hardwareID()
	}

	// Try PBKDF2 format: [16-byte salt][12-byte nonce][ciphertext+tag]
	if len(data) > saltLen+12 {
		salt := data[:saltLen]
		body := data[saltLen:]
		key := pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iter, 32, sha256.New)

		plain, err := decryptWithKey(key, body)
		if err == nil {
			return plain, nil
		}
	}

	// Fallback: legacy SHA-256 format (pre-v3.0.0)
	key := deriveKey(passphrase)
	return decryptWithKey(key[:], data)
}

// decryptWithKey is the raw AES-256-GCM decrypt given a 32-byte key.
// Input format: [12-byte nonce][ciphertext + 16-byte GCM auth tag]
func decryptWithKey(key []byte, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cloudsync: decrypt: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cloudsync: decrypt: new gcm: %w", err)
	}
	ns := gcm.NonceSize()
	if len(data) < ns {
		return nil, fmt.Errorf("cloudsync: decrypt: data too short")
	}
	plain, err := gcm.Open(nil, data[:ns], data[ns:], nil)
	if err != nil {
		return nil, fmt.Errorf("cloudsync: decrypt: %w", err)
	}
	return plain, nil
}

// hardwareID returns a stable machine identifier for key derivation.
// It prefers machine-id files when available, then falls back to hostname.
func hardwareID() string {
	switch runtime.GOOS {
	case "windows":
		out, err := exec.Command("powershell", "-NoProfile", "-Command",
			`(Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Cryptography' MachineGuid).MachineGuid`).Output()
		if err == nil {
			if id := strings.TrimSpace(string(out)); id != "" {
				return id
			}
		}
	case "linux":
		if data, err := os.ReadFile("/etc/machine-id"); err == nil {
			return strings.TrimSpace(string(data))
		}
		if data, err := os.ReadFile("/var/lib/dbus/machine-id"); err == nil {
			return strings.TrimSpace(string(data))
		}
	case "darwin":
		out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, "IOPlatformUUID") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						return strings.Trim(strings.TrimSpace(parts[1]), `"`)
					}
				}
			}
		}
	}
	host, err := os.Hostname()
	if err != nil {
		return "memo-fallback-key"
	}
	return host
}
