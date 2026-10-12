package cloudsync

import (
	"context"
	"memo/internal/provider"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveRestoresCompleteInventoryAndProviderKey(t *testing.T) {
	source := t.TempDir()
	memory := filepath.Join(source, "custom-memory")
	configPath := filepath.Join(source, "configuration", "config.yaml")
	dataDir := filepath.Join(source, "data")
	key := []byte("fixture-provider-key-32-bytes-long")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatal(err)
	}
	cm := provider.NewConfigManager(filepath.Join(dataDir, "providers.json"), key)
	if err := cm.Set(provider.ProviderConfig{Type: provider.ProviderCustom, Name: "fixture", Model: "chat", APIKey: "fixture-secret", BaseURL: "http://localhost:1", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"machine.key": string(key), "telegram.json": "telegram settings", "calendar/events.db": "calendar", "routines/list.json": "routines", "tasklists/list.json": "tasks", "stats/usage.db": "usage", "profile/patterns.json": "patterns", "skills/test/SKILL.md": "skill"}
	for name, body := range files {
		writeBackupFixture(t, filepath.Join(dataDir, name), []byte(body))
	}
	writeBackupFixture(t, configPath, []byte("resolved config"))
	m := &Manager{ctx: context.Background(), dataDir: dataDir, persistDir: memory, configFile: configPath}
	archive, err := m.archive()
	if err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	restoredDir := filepath.Join(target, "data")
	restoredConfig := filepath.Join(target, "config", "config.yaml")
	destination := &Manager{dataDir: restoredDir, persistDir: filepath.Join(restoredDir, "memory"), configFile: restoredConfig}
	if err := destination.restoreZip(archive); err != nil {
		t.Fatal(err)
	}
	for name, want := range files {
		b, err := os.ReadFile(filepath.Join(restoredDir, name))
		if err != nil || string(b) != want {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	if b, err := os.ReadFile(restoredConfig); err != nil || string(b) != "resolved config" {
		t.Fatal("config not restored")
	}
	restoredKey, err := os.ReadFile(filepath.Join(restoredDir, "machine.key"))
	if err != nil {
		t.Fatal(err)
	}
	got := provider.NewConfigManager(filepath.Join(restoredDir, "providers.json"), restoredKey).GetAll()
	if len(got) != 1 || got[0].APIKey != "fixture-secret" {
		t.Fatal("cross-device provider key not usable")
	}
}
func TestCloudArchiveDoesNotHideUnreadableSource(t *testing.T) {
	dir := t.TempDir()
	writeBackupFixture(t, filepath.Join(dir, "sessions", "one.json"), []byte("{}"))
	if err := os.Symlink(filepath.Join(dir, "missing"), filepath.Join(dir, "sessions", "broken.json")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	m := &Manager{ctx: context.Background(), dataDir: dir, persistDir: filepath.Join(dir, "memory")}
	if b, err := m.archive(); err == nil || len(b) != 0 {
		t.Fatal("incomplete cloud archive reported success")
	}
}

func writeBackupFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, data)
}
