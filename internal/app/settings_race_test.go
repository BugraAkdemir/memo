package app

import (
	"sync"
	"testing"

	"memo/internal/config"
)

// TestMemoryAndWebSearchEnabledConcurrentAccess guards BUG-M3:
// a.cfg.Memory.MemoryEnabled and a.cfg.WebSearch.Enabled used to be read
// directly (buildMessages, chat.go, models.go, llama.go) with no lock, while
// SetMemoryEnabled/UpdateWebSearchConfig wrote them under cfgMu — an
// unsynchronized read raced with those writes under -race whenever a
// setting was toggled mid-stream. All access now goes through
// GetMemoryEnabled/GetWebSearchEnabled (RLock) on the read side.
func TestMemoryAndWebSearchEnabledConcurrentAccess(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}

	var wg sync.WaitGroup
	for i := range 50 {
		v := i%2 == 0
		wg.Add(4)
		go func() {
			defer wg.Done()
			a.cfgMu.Lock()
			a.cfg.Memory.MemoryEnabled = v
			a.cfgMu.Unlock()
		}()
		go func() {
			defer wg.Done()
			a.cfgMu.Lock()
			a.cfg.WebSearch.Enabled = v
			a.cfgMu.Unlock()
		}()
		go func() {
			defer wg.Done()
			_ = a.GetMemoryEnabled()
		}()
		go func() {
			defer wg.Done()
			_ = a.GetWebSearchEnabled()
		}()
	}
	wg.Wait()
}
