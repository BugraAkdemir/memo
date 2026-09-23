package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelfCloneBlocksSelf(t *testing.T) {
	src := t.TempDir()
	args, _ := json.Marshal(map[string]string{"dest": src})
	_, err := SelfClone(context.Background(), args, src, nil)
	if err == nil {
		t.Error("kaynağın kendisine klonlama engellenmeliydi")
	}
}

func TestSelfCloneBlocksSubdir(t *testing.T) {
	src := t.TempDir()
	dest := filepath.Join(src, "subdir")
	args, _ := json.Marshal(map[string]string{"dest": dest})
	_, err := SelfClone(context.Background(), args, src, nil)
	if err == nil {
		t.Error("kaynak alt dizinine klonlama engellenmeliydi")
	}
}

// Eski HasPrefix bug: src="/tmp/memo", dest="/tmp/memo-backup" yanlışlıkla bloklanıyordu.
func TestSelfCloneAllowsSiblingWithSimilarName(t *testing.T) {
	parent := t.TempDir()
	src := filepath.Join(parent, "memo")
	dest := filepath.Join(parent, "memo-backup")

	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	// Kaynak dizine bir dosya ekle
	if err := os.WriteFile(filepath.Join(src, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	args, _ := json.Marshal(map[string]string{"dest": dest})
	out, err := SelfClone(context.Background(), args, src, nil)
	if err != nil {
		t.Fatalf("sibling dizine klonlama izin verilmeliydi, hata: %v", err)
	}
	if !strings.Contains(out, "Cloned") {
		t.Errorf("başarı mesajı bekleniyor, got: %q", out)
	}

	// Dosyanın kopyalandığını doğrula
	cloned := filepath.Join(dest, "main.go")
	if _, err := os.Stat(cloned); os.IsNotExist(err) {
		t.Error("klonlanan dosya hedefte bulunamadı")
	}
}

func TestSelfCloneCopiesFiles(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()

	// Birkaç dosya ve alt dizin oluştur
	files := map[string]string{
		"main.go":         "package main",
		"internal/app.go": "package app",
		"config/cfg.yaml": "key: value",
	}
	for rel, content := range files {
		path := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// .git dizini ekleniyor — atlanmalı
	gitDir := filepath.Join(src, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main"), 0644); err != nil {
		t.Fatal(err)
	}

	args, _ := json.Marshal(map[string]string{"dest": dest})
	out, err := SelfClone(context.Background(), args, src, nil)
	if err != nil {
		t.Fatalf("klonlama başarısız: %v", err)
	}
	if !strings.Contains(out, "Cloned") {
		t.Errorf("başarı mesajı bekleniyor: %q", out)
	}

	// Dosyalar kopyalandı mı?
	for rel := range files {
		cloned := filepath.Join(dest, rel)
		if _, err := os.Stat(cloned); os.IsNotExist(err) {
			t.Errorf("dosya klonlanmadı: %s", rel)
		}
	}

	// .git atlandı mı?
	if _, err := os.Stat(filepath.Join(dest, ".git")); !os.IsNotExist(err) {
		t.Error(".git dizini kopyalanmamalıydı")
	}
}

func TestSelfCloneMissingDest(t *testing.T) {
	src := t.TempDir()
	args, _ := json.Marshal(map[string]string{"dest": ""})
	_, err := SelfClone(context.Background(), args, src, nil)
	if err == nil {
		t.Error("boş dest ile hata bekleniyor")
	}
}

// TestSelfCloneBlocksDotfileDestination is the regression test for a real
// P0 found in a 2026-09-23 security audit: SelfClone had NO protected-path
// check at all before this — only "destination isn't inside the source
// directory." An unattended (bypass-permissions) Self-Driving task could
// write_file a malicious file inside the sandbox, then call self_clone with
// dest=~/.ssh to silently overwrite the real ~/.ssh/authorized_keys with
// it. This test simulates that shape without touching a real home
// directory: a source tree containing a file that would land at
// <dest>/authorized_keys, and a dest resolving into a dotfile directory.
func TestSelfCloneBlocksDotfileDestination(t *testing.T) {
	parent := t.TempDir()
	src := filepath.Join(parent, "project")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "authorized_keys"), []byte("ssh-ed25519 AAAA...attacker"), 0644); err != nil {
		t.Fatal(err)
	}

	for _, dest := range []string{
		filepath.Join(parent, ".ssh"),
		filepath.Join(parent, ".ssh", "nested"),
		filepath.Join(parent, ".config", "systemd", "user"),
		filepath.Join(parent, ".bashrc-lookalike-dir"), // still dot-prefixed
	} {
		args, _ := json.Marshal(map[string]string{"dest": dest})
		out, err := SelfClone(context.Background(), args, src, nil)
		if err == nil || !strings.Contains(err.Error(), "hidden/dotfile") {
			t.Errorf("dest=%q: expected a hidden/dotfile rejection, got out=%q err=%v", dest, out, err)
		}
		if _, statErr := os.Stat(filepath.Join(dest, "authorized_keys")); !os.IsNotExist(statErr) {
			t.Errorf("dest=%q: authorized_keys must not have been written", dest)
		}
	}
}

// TestSelfCloneBlocksProtectedSystemPath verifies the narrower, self_clone-
// specific protected list (selfCloneProtectedPaths) still refuses true
// system directories — the ones excluded from that list are /home/ and
// /tmp/ specifically (see its doc comment), not everything.
func TestSelfCloneBlocksProtectedSystemPath(t *testing.T) {
	if testing.Short() {
		t.Skip("touches real absolute system paths, skip in -short")
	}
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "x.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	for _, dest := range []string{"/etc/memo-selfclone-test", "/var/memo-selfclone-test"} {
		args, _ := json.Marshal(map[string]string{"dest": dest})
		_, err := SelfClone(context.Background(), args, src, nil)
		// Assert on the exact rejection reason, not just "any error" — a
		// non-root test process would ALSO fail to os.MkdirAll under /etc
		// or /var for plain OS-permission reasons, which would pass this
		// test for the wrong reason (not proving the new check fired at
		// all, since it runs before MkdirAll would even be attempted).
		if err == nil || !strings.Contains(err.Error(), "protected system directory") {
			t.Errorf("dest=%q: expected a protected-system-directory rejection, got: %v", dest, err)
			_ = os.RemoveAll(dest)
		}
	}
}

// TestSelfCloneAllowsPlainHomeSubdirectory is the flip side of the two
// tests above: selfCloneProtectedPaths deliberately excludes /home/ and
// /tmp/ (the only realistic writable locations for a non-root desktop
// user) — an ordinary, non-dotfile destination under one must still work.
func TestSelfCloneAllowsPlainHomeSubdirectory(t *testing.T) {
	parent := t.TempDir() // stands in for a home-directory-like writable tree
	src := filepath.Join(parent, "project")
	dest := filepath.Join(parent, "backups", "project-copy")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	args, _ := json.Marshal(map[string]string{"dest": dest})
	out, err := SelfClone(context.Background(), args, src, nil)
	if err != nil {
		t.Fatalf("plain non-dotfile destination should be allowed, got error: %v", err)
	}
	if !strings.Contains(out, "Cloned") {
		t.Errorf("expected a success message, got: %q", out)
	}
}

func TestSelfCloneContextCancel(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()

	// Bir sürü dosya oluştur
	for i := range 20 {
		path := filepath.Join(src, strings.Repeat("x", i+1)+".txt")
		_ = os.WriteFile(path, []byte("data"), 0644)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // hemen iptal et

	args, _ := json.Marshal(map[string]string{"dest": dest})
	_, err := SelfClone(ctx, args, src, nil)
	// context iptal edildi — hata bekleniyor ya da bazı dosyalar kopyalanmamış olabilir
	// ikisi de kabul edilebilir; paniklememesi yeterli
	_ = err
}
