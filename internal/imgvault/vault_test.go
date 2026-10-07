package imgvault

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/config"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, keyLen)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSealOpen_RoundTripAndNoPlaintextLeak(t *testing.T) {
	key := testKey(t)
	plain := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte("secret-pixels-"), 200)...)
	blob, err := Seal(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	if !IsSealed(blob) {
		t.Fatal("sealed blob lacks the header")
	}
	if bytes.Contains(blob, []byte("secret-pixels-")) || bytes.Contains(blob, []byte("PNG")) {
		t.Fatal("ciphertext leaks plaintext bytes")
	}
	got, err := Open(key, blob)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("round trip failed: err=%v equal=%v", err, bytes.Equal(got, plain))
	}
}

func TestSeal_EveryFileGetsItsOwnKeyAndNonce(t *testing.T) {
	key := testKey(t)
	a, _ := Seal(key, []byte("same picture"))
	b, _ := Seal(key, []byte("same picture"))
	if bytes.Equal(a, b) {
		t.Fatal("two seals of the same bytes are identical: salt/nonce are not random")
	}
}

func TestOpen_WrongKeyAndTamperingAreRejected(t *testing.T) {
	key := testKey(t)
	blob, _ := Seal(key, []byte("hello"))
	if _, err := Open(testKey(t), blob); err == nil {
		t.Fatal("a different key opened the file")
	}
	for _, i := range []int{len(magic) + 1, len(magic) + saltLen + 3, len(blob) - 1} {
		bad := append([]byte(nil), blob...)
		bad[i] ^= 0x01
		if _, err := Open(key, bad); err == nil {
			t.Fatalf("flipping byte %d went unnoticed", i)
		}
	}
	if _, err := Open(key, blob[:len(blob)-5]); err == nil {
		t.Fatal("a truncated file opened")
	}
	if _, err := Open(key, []byte("not sealed at all")); err != ErrNotSealed {
		t.Fatalf("plain bytes: got %v, want ErrNotSealed", err)
	}
}

func TestVault_FileRoundTripAndLegacyPlaintext(t *testing.T) {
	dir := t.TempDir()
	v, _ := New(testKey(t))

	sealed := filepath.Join(dir, "a.png")
	if err := v.WriteFile(sealed, []byte("picture A")); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(sealed)
	if bytes.Contains(raw, []byte("picture A")) {
		t.Fatal("file on disk holds the plaintext")
	}
	if got, err := v.ReadFile(sealed); err != nil || string(got) != "picture A" {
		t.Fatalf("read back: %q %v", got, err)
	}
	if fi, _ := os.Stat(sealed); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", fi.Mode().Perm())
	}

	legacy := filepath.Join(dir, "old.png")
	os.WriteFile(legacy, []byte("legacy plaintext"), 0o644)
	if got, err := v.ReadFile(legacy); err != nil || string(got) != "legacy plaintext" {
		t.Fatalf("legacy read: %q %v", got, err)
	}
}

func TestMigrateDir_SealsPlaintextOnceAndLeavesTheRest(t *testing.T) {
	dir := t.TempDir()
	v, _ := New(testKey(t))
	os.WriteFile(filepath.Join(dir, "p1.png"), []byte("one"), 0o644)
	os.WriteFile(filepath.Join(dir, "p2.jpg"), []byte("two"), 0o644)
	os.WriteFile(filepath.Join(dir, "empty.png"), nil, 0o644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	v.WriteFile(filepath.Join(dir, "done.png"), []byte("already"))

	n, err := v.MigrateDir(dir)
	if err != nil || n != 2 {
		t.Fatalf("MigrateDir = %d, %v; want 2", n, err)
	}
	for name, want := range map[string]string{"p1.png": "one", "p2.jpg": "two", "done.png": "already"} {
		raw, _ := os.ReadFile(filepath.Join(dir, name))
		if !IsSealed(raw) {
			t.Errorf("%s was not sealed", name)
		}
		if got, _ := v.ReadFile(filepath.Join(dir, name)); string(got) != want {
			t.Errorf("%s decrypts to %q, want %q", name, got, want)
		}
	}
	if n, _ := v.MigrateDir(dir); n != 0 {
		t.Errorf("second migrate sealed %d files, want 0", n)
	}
	if n, err := v.MigrateDir(filepath.Join(dir, "missing")); n != 0 || err != nil {
		t.Errorf("missing dir: %d, %v", n, err)
	}
}

func TestDefault_KeyLivesOutsideTheDataDirAndIsStable(t *testing.T) {
	cfgHome := t.TempDir()
	t.Setenv("MEMO_DATA_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	t.Setenv("HOME", cfgHome)
	t.Setenv("MEMO_IMAGE_KEY", "")
	t.Setenv("MEMO_IMAGE_KEY_FILE", "")
	ResetForTests()
	t.Cleanup(ResetForTests)

	v1, err := Default()
	if err != nil {
		t.Skipf("no user config dir on this platform: %v", err)
	}
	keyPath := keyFile()
	if !strings.HasPrefix(keyPath, cfgHome) {
		t.Fatalf("key at %s, want it under the user config dir %s", keyPath, cfgHome)
	}
	if strings.HasPrefix(keyPath, config.DataDir()+string(filepath.Separator)) {
		t.Fatalf("key at %s sits inside the data dir", keyPath)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("key file not created at %s: %v", keyPath, err)
	}
	if fi, _ := os.Stat(keyPath); fi.Mode().Perm() != 0o600 {
		t.Errorf("key mode = %v, want 0600", fi.Mode().Perm())
	}
	blob, _ := Seal(v1.key, []byte("x"))

	ResetForTests()
	v2, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(v2.key, blob); err != nil {
		t.Fatalf("a second load of the key cannot open the first's files: %v", err)
	}
}

func TestDefault_DataDirOverrideKeepsTheKeyWithTheData(t *testing.T) {
	data := t.TempDir()
	t.Setenv("MEMO_DATA_DIR", data)
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)
	t.Setenv("MEMO_IMAGE_KEY", "")
	t.Setenv("MEMO_IMAGE_KEY_FILE", "")
	if got := keyFile(); filepath.Dir(got) != data {
		t.Errorf("keyFile = %s, want it under %s", got, data)
	}
}

func TestDefault_RefusesToReplaceACorruptKeyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.key")
	os.WriteFile(path, []byte("garbage"), 0o600)
	t.Setenv("MEMO_IMAGE_KEY", "")
	t.Setenv("MEMO_IMAGE_KEY_FILE", path)
	ResetForTests()
	t.Cleanup(ResetForTests)
	if _, err := Default(); err == nil {
		t.Fatal("a corrupt key file was replaced silently — every stored picture would be lost")
	}
	if raw, _ := os.ReadFile(path); string(raw) != "garbage" {
		t.Fatal("the corrupt key file was overwritten")
	}
}

func TestKeyFromSecret(t *testing.T) {
	a, err := keyFromSecret("correct horse battery staple")
	if err != nil || len(a) != keyLen {
		t.Fatalf("passphrase: %v len=%d", err, len(a))
	}
	b, _ := keyFromSecret("correct horse battery staple")
	if !bytes.Equal(a, b) {
		t.Fatal("same passphrase gave different keys")
	}
	c, _ := keyFromSecret("another passphrase")
	if bytes.Equal(a, c) {
		t.Fatal("different passphrases gave the same key")
	}
	hexKey := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	k, _ := keyFromSecret(hexKey)
	if k[0] != 0x00 || k[1] != 0x11 || len(k) != keyLen {
		t.Fatalf("a hex key was not used as given: %x", k)
	}
}
