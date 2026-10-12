package app

import (
	"bytes"
	"encoding/json"
	"memo/internal/cloudsync"
	"memo/internal/config"
	"memo/internal/imgvault"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptedBackupImagesAcrossIndependentDeviceKeys(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	t.Setenv("MEMO_DATA_DIR", source)
	t.Setenv("MEMO_IMAGE_KEY_FILE", filepath.Join(source, "image.key"))
	config.ResetForTests()
	imgvault.ResetForTests()
	t.Cleanup(func() { config.ResetForTests(); imgvault.ResetForTests() })
	picture := []byte("portable image fixture bytes")
	image := config.DataPath("images", "one.png")
	if err := os.MkdirAll(filepath.Dir(image), 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeImageFile(image, picture); err != nil {
		t.Fatal(err)
	}
	v, err := imgvault.Default()
	if err != nil {
		t.Fatal(err)
	}
	originalCipher, _ := os.ReadFile(image)
	session := []byte(`{"id":"one","messages":[{"image_path":"` + filepath.ToSlash(image) + `"}]}`)
	if err := os.MkdirAll(config.DataPath("sessions"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.DataPath("sessions", "one.json"), session, 0600); err != nil {
		t.Fatal(err)
	}
	a := &App{cfg:config.Default()}
	archive, err := a.ExportEncryptedData(false, "long-backup-password")
	if err != nil {
		t.Fatal(err)
	}
	if !cloudsync.IsEncryptedBackup(archive) || bytes.Contains(archive, picture) {
		t.Fatal("picture exposed in encrypted file")
	}
	t.Setenv("MEMO_DATA_DIR", target)
	t.Setenv("MEMO_IMAGE_KEY_FILE", filepath.Join(target, "image.key"))
	config.ResetForTests()
	imgvault.ResetForTests()
	sentinel := config.DataPath("sessions", "existing.json")
	os.MkdirAll(filepath.Dir(sentinel), 0700)
	os.WriteFile(sentinel, []byte("unchanged"), 0600)
	if err := a.ImportEncryptedData(archive, "wrong-password"); err == nil {
		t.Fatal("wrong password accepted")
	}
	if b, _ := os.ReadFile(sentinel); string(b) != "unchanged" {
		t.Fatal("failed decrypt touched existing data")
	}
	if err := a.ImportEncryptedData(archive, "long-backup-password"); err != nil {
		t.Fatal(err)
	}
	restored := config.DataPath("images", "one.png")
	raw, err := os.ReadFile(restored)
	if err != nil || !imgvault.IsSealed(raw) {
		t.Fatalf("not sealed: %v", err)
	}
	if bytes.Equal(raw, originalCipher) {
		t.Fatal("source encryption copied")
	}
	if _, err := v.OpenData(raw); err == nil {
		t.Fatal("source vault can decrypt target image")
	}
	got, err := readImageFile(restored)
	if err != nil || !bytes.Equal(got, picture) {
		t.Fatalf("picture lost: %v", err)
	}
	b, _ := os.ReadFile(config.DataPath("sessions", "one.json"))
	var doc map[string]any
	if json.Unmarshal(b, &doc) != nil {
		t.Fatal("bad session")
	}
	if !bytes.Contains(b, []byte(filepath.ToSlash(restored))) {
		t.Fatalf("path not remapped: %s", b)
	}
	if _, err := os.Stat(filepath.Join(target, "image.key")); err != nil {
		t.Fatal(err)
	}
}
