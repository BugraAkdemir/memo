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
