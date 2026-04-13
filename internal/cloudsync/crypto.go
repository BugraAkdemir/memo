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
)

// deriveKey produces a 32-byte AES-256 key from the given passphrase.
// If passphrase is empty, a hardware-derived identifier is used as fallback.
func deriveKey(passphrase string) [32]byte {
	if passphrase == "" {
		passphrase = hardwareID()
	}
	return sha256.Sum256([]byte("memo-sync-v1:" + passphrase))
}

// hardwareID returns a stable machine identifier for key derivation.
// It prefers machine-id files when available, then falls back to hostname.
func hardwareID() string {
	switch runtime.GOOS {
	case "windows":
		// Read MachineGuid from registry — stable across reboots
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

// encrypt encrypts plaintext with AES-256-GCM.
// Output format: [12-byte nonce][ciphertext + 16-byte GCM auth tag]
func encrypt(key [32]byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
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
	// Seal appends ciphertext+tag to nonce — result is self-contained.
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decrypt reverses encrypt.
func decrypt(key [32]byte, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("cloudsync: decrypt: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cloudsync: decrypt: new gcm: %w", err)
	}
	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, fmt.Errorf("cloudsync: decrypt: ciphertext too short")
	}
	plain, err := gcm.Open(nil, ciphertext[:ns], ciphertext[ns:], nil)
	if err != nil {
		return nil, fmt.Errorf("cloudsync: decrypt: %w", err)
	}
	return plain, nil
}
