// Package imgvault keeps the pictures Memo stores on disk unreadable to anyone
// who only has the files.
//
// What is protected: every image a person sends Memo (a phone photo, an upload)
// and every image Memo draws for them, from the moment it is written. The file
// on disk is ciphertext; the picture only exists in memory while Memo serves it.
// If the data folder is copied, backed up, synced or leaked, the images in it are
// noise.
//
// How:
//
//   - XChaCha20-Poly1305 (256-bit key, 192-bit random nonce, authenticated), so a
//     flipped bit is detected rather than rendered as a corrupt picture.
//   - A fresh key per file: HKDF-SHA256 over the master secret and a random 16-byte
//     salt. Two files never share a key or a nonce, and one file's key reveals
//     nothing about another's.
//   - The master secret is NOT in the data folder (see keyFile): by default it
//     lives in the user's own config directory, mode 0600, so a copy of the data
//     folder alone does not carry the key. MEMO_IMAGE_KEY replaces the file with a
//     passphrase supplied from outside (a container secret, a password manager),
//     stretched with scrypt, so nothing about the key touches the disk at all.
//
// What is not protected, and cannot be: the picture while it is open in memory,
// and the copy a model provider, Telegram or WhatsApp receives when it is sent
// there. This package is about the files at rest.
package imgvault

import (
	"bytes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/scrypt"

	"memo/internal/config"
	"memo/internal/logx"
)

// magic opens every sealed file. It also doubles as the AEAD's associated data,
// so a sealed blob cannot be passed off as some other kind of ciphertext.
var magic = []byte("MEMOIMG1")

const (
	saltLen  = 16
	keyLen   = 32
	hkdfInfo = "memo/imgvault/v1/file-key"
)

// ErrNotSealed is returned by Open for a blob that does not start with the
// vault header (a picture written before sealing existed).
var ErrNotSealed = errors.New("imgvault: not a sealed file")

// IsSealed reports whether blob starts with the vault header.
func IsSealed(blob []byte) bool {
	return len(blob) > len(magic) && bytes.Equal(blob[:len(magic)], magic)
}

// Seal encrypts plain under a fresh per-file key derived from master.
func Seal(master, plain []byte) ([]byte, error) {
	if len(master) != keyLen {
		return nil, fmt.Errorf("imgvault: master key must be %d bytes", keyLen)
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	aead, err := fileAEAD(master, salt)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(magic)+saltLen+len(nonce)+len(plain)+aead.Overhead())
	out = append(out, magic...)
	out = append(out, salt...)
	out = append(out, nonce...)
	return aead.Seal(out, nonce, plain, magic), nil
}

// Open decrypts a blob made by Seal. A blob without the header is ErrNotSealed;
// a wrong key or a damaged file is an authentication error.
func Open(master, blob []byte) ([]byte, error) {
	if !IsSealed(blob) {
		return nil, ErrNotSealed
	}
	if len(master) != keyLen {
		return nil, fmt.Errorf("imgvault: master key must be %d bytes", keyLen)
	}
	body := blob[len(magic):]
	if len(body) < saltLen+chacha20poly1305.NonceSizeX+chacha20poly1305.Overhead {
		return nil, errors.New("imgvault: sealed file is truncated")
	}
	salt := body[:saltLen]
	aead, err := fileAEAD(master, salt)
	if err != nil {
		return nil, err
	}
	nonce := body[saltLen : saltLen+aead.NonceSize()]
	plain, err := aead.Open(nil, nonce, body[saltLen+aead.NonceSize():], magic)
	if err != nil {
		return nil, errors.New("imgvault: cannot decrypt (wrong key or damaged file)")
	}
	return plain, nil
}

func fileAEAD(master, salt []byte) (cipher.AEAD, error) {
	k := make([]byte, keyLen)
	if _, err := io.ReadFull(hkdf.New(sha256.New, master, salt, []byte(hkdfInfo)), k); err != nil {
		return nil, err
	}
	return chacha20poly1305.NewX(k)
}

// Vault reads and writes sealed files with one master key.
type Vault struct{ key []byte }

// New builds a Vault around an explicit key (tests, or a caller that manages the
// key itself).
func New(key []byte) (*Vault, error) {
	if len(key) != keyLen {
		return nil, fmt.Errorf("imgvault: key must be %d bytes", keyLen)
	}
	return &Vault{key: append([]byte(nil), key...)}, nil
}

// WriteFile seals plain and writes it to path atomically (temp file in the same
// directory, then rename), so a crash never leaves half a file where a picture
// was expected. The file is private to the user (0600).
func (v *Vault) WriteFile(path string, plain []byte) error {
	blob, err := Seal(v.key, plain)
	if err != nil {
		return err
	}
	return writeAtomic(path, blob)
}

// ReadFile returns the picture stored at path. A file written before sealing
// existed is returned as it is, so older chats keep their pictures until
// MigrateDir has sealed them.
func (v *Vault) ReadFile(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !IsSealed(raw) {
		return raw, nil
	}
	return Open(v.key, raw)
}

// SealInPlace encrypts a plaintext file where it lies. A file that is already
// sealed is left alone (changed is false).
func (v *Vault) SealInPlace(path string) (changed bool, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if IsSealed(raw) {
		return false, nil
	}
	if len(raw) == 0 {
		return false, nil
	}
	return true, v.WriteFile(path, raw)
}

// MigrateDir seals every plaintext regular file directly inside dir. It returns
// how many it sealed; a file it cannot seal is logged and skipped, never fatal.
func (v *Vault) MigrateDir(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".tmp") {
			continue
		}
		changed, err := v.SealInPlace(filepath.Join(dir, e.Name()))
		if err != nil {
			logx.Printf("imgvault: could not seal %s: %v", e.Name(), err)
			continue
		}
		if changed {
			n++
		}
	}
	return n, nil
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".seal-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	cleanup := func() { _ = os.Remove(tmp) }
	if _, err := f.Write(data); err != nil {
		f.Close()
		cleanup()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		cleanup()
		return err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil && runtime.GOOS != "windows" {
		cleanup()
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

// ─── The process-wide vault ──────────────────────────────────────

var (
	defaultMu sync.Mutex
	defaultV  *Vault
	defaultOK bool
)

// Default is the vault the app uses, built on first call from the master secret
// (see keyFile / MEMO_IMAGE_KEY). It is created once per process.
func Default() (*Vault, error) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultOK {
		return defaultV, nil
	}
	key, err := loadMasterKey()
	if err != nil {
		return nil, err
	}
	v, err := New(key)
	if err != nil {
		return nil, err
	}
	defaultV, defaultOK = v, true
	return v, nil
}

// ResetForTests drops the cached vault so the next Default() re-reads the key
// from the environment. Test-only.
func ResetForTests() {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultV, defaultOK = nil, false
}

// keyFile says where the master key lives when no MEMO_IMAGE_KEY is given.
//
//   - MEMO_IMAGE_KEY_FILE names it explicitly.
//   - With MEMO_DATA_DIR set (a container volume, a portable or test run) the key
//     travels with that data directory: a config directory that is thrown away
//     with the container would otherwise take every picture with it.
//   - Otherwise it is in the user's own config directory (~/.config/Memo,
//     %AppData%\Memo, ~/Library/Application Support/Memo) — NOT under the data
//     directory, so copying, backing up or syncing the data folder does not carry
//     the key with it.
func keyFile() string {
	if p := strings.TrimSpace(os.Getenv("MEMO_IMAGE_KEY_FILE")); p != "" {
		return p
	}
	if strings.TrimSpace(os.Getenv("MEMO_DATA_DIR")) == "" {
		if dir, err := os.UserConfigDir(); err == nil && dir != "" {
			return filepath.Join(dir, "Memo", "image.key")
		}
	}
	return filepath.Join(config.DataDir(), "image.key")
}

func loadMasterKey() ([]byte, error) {
	if pass := os.Getenv("MEMO_IMAGE_KEY"); pass != "" {
		return keyFromSecret(pass)
	}
	path := keyFile()
	if raw, err := os.ReadFile(path); err == nil {
		if k, ok := decodeKey(raw); ok {
			return k, nil
		}
		return nil, fmt.Errorf("imgvault: %s is not a valid key file (refusing to replace it: that would lock every stored picture)", path)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("imgvault: read key: %w", err)
	}

	key := make([]byte, keyLen)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("imgvault: create key dir: %w", err)
	}
	encoded := []byte(hex.EncodeToString(key) + "\n")
	// O_EXCL: if two processes race to create the key, the loser reads the
	// winner's instead of silently replacing it.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			raw, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil, rerr
			}
			if k, ok := decodeKey(raw); ok {
				return k, nil
			}
		}
		return nil, fmt.Errorf("imgvault: write key: %w", err)
	}
	if _, err := f.Write(encoded); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	logx.Printf("imgvault: created a new image key at %s", path)
	return key, nil
}

func decodeKey(raw []byte) ([]byte, bool) {
	s := strings.TrimSpace(string(raw))
	if k, err := hex.DecodeString(s); err == nil && len(k) == keyLen {
		return k, true
	}
	if k, err := base64.StdEncoding.DecodeString(s); err == nil && len(k) == keyLen {
		return k, true
	}
	return nil, false
}

// keyFromSecret turns MEMO_IMAGE_KEY into a key: a 32-byte hex/base64 value is
// used as it is, anything else is a passphrase stretched with scrypt.
func keyFromSecret(secret string) ([]byte, error) {
	if k, ok := decodeKey([]byte(secret)); ok {
		return k, nil
	}
	return scrypt.Key([]byte(secret), []byte("memo/imgvault/v1/passphrase"), 1<<15, 8, 1, keyLen)
}
