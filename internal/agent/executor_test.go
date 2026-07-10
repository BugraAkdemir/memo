package agent

import (
	"sync"
	"testing"
)

// TestExecutor_PermissionFlagsConcurrentAccess guards against the bug where
// RunStream read e.bypassPermissions/e.autoPermission directly instead of
// through GetBypassPermissions/GetAutoPermission — SetBypassPermissions/
// SetAutoPermission write under e.mu (an HTTP handler goroutine), so an
// unsynchronized read elsewhere has no happens-before guarantee and can
// observe a stale value. Run with -race to catch a regression.
func TestExecutor_PermissionFlagsConcurrentAccess(t *testing.T) {
	e := NewExecutor(t.TempDir(), nil, nil)

	var wg sync.WaitGroup
	for i := range 100 {
		v := i%2 == 0
		wg.Add(4)
		go func() {
			defer wg.Done()
			e.SetAutoPermission(v)
		}()
		go func() {
			defer wg.Done()
			e.SetBypassPermissions(v)
		}()
		go func() {
			defer wg.Done()
			_ = e.GetAutoPermission()
		}()
		go func() {
			defer wg.Done()
			_ = e.GetBypassPermissions()
		}()
	}
	wg.Wait()
}
