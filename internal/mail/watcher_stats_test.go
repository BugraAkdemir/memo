package mail

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestWatcherCountersConcurrentPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mail", "accounts.json")
	mgr, err := NewManager(path, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	w := NewWatcher(mgr, nil, WatcherConfig{})
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() { defer wg.Done(); w.RecordCalendarAdded(); w.RecordSecurityBlocked("prompt_injection") }()
	}
	wg.Wait()
	got := NewWatcher(mgr, nil, WatcherConfig{}).Stats()
	if got.CalendarExtracted != 16 || got.SecurityBlocked != 16 {
		t.Fatalf("lost writes: %+v", got)
	}
	info, err := os.Stat(filepath.Join(filepath.Dir(path), "watcher-stats.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("unsafe permissions: %v", info.Mode())
	}
}
