package browserengine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/BugraAkdemir/gosearch"
	"github.com/BugraAkdemir/gosearch/browser"
)

// waitForInstallDone polls until a StartInstall goroutine finishes (or the
// timeout fires) — StartInstall is fire-and-forget, so tests need this
// instead of asserting on it directly.
func waitForInstallDone(t *testing.T, m *Manager) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !m.InstallProgress().Active {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("StartInstall did not finish within 2s")
}

type fakeEngine struct {
	closed     bool
	fetchN     int
	fetchErr   error
	closeErr   error
	pageForURL func(url string) *gosearch.Page
}

func (f *fakeEngine) Fetch(_ context.Context, url string) (*gosearch.Page, error) {
	f.fetchN++
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	if f.pageForURL != nil {
		return f.pageForURL(url), nil
	}
	return &gosearch.Page{URL: url, Content: "rendered"}, nil
}

func (f *fakeEngine) Close() error {
	f.closed = true
	return f.closeErr
}

func TestManager_OnDemand_NewEngineClosedAfterEachFetch(t *testing.T) {
	orig := newEngine
	defer func() { newEngine = orig }()

	var created []*fakeEngine
	newEngine = func(ctx context.Context) (engineHandle, error) {
		e := &fakeEngine{}
		created = append(created, e)
		return e, nil
	}

	m := New(false) // keepAlive off
	if _, err := m.Fetch(context.Background(), "https://a.example"); err != nil {
		t.Fatalf("Fetch #1: %v", err)
	}
	if _, err := m.Fetch(context.Background(), "https://b.example"); err != nil {
		t.Fatalf("Fetch #2: %v", err)
	}

	if len(created) != 2 {
		t.Fatalf("expected 2 engines created (one per fetch), got %d", len(created))
	}
	for i, e := range created {
		if !e.closed {
			t.Errorf("engine %d not closed after its fetch", i)
		}
	}
}

func TestManager_KeepAlive_ReusesEngineAcrossFetches(t *testing.T) {
	orig := newEngine
	defer func() { newEngine = orig }()

	var created []*fakeEngine
	newEngine = func(ctx context.Context) (engineHandle, error) {
		e := &fakeEngine{}
		created = append(created, e)
		return e, nil
	}

	m := New(true) // keepAlive on
	if _, err := m.Fetch(context.Background(), "https://a.example"); err != nil {
		t.Fatalf("Fetch #1: %v", err)
	}
	if _, err := m.Fetch(context.Background(), "https://b.example"); err != nil {
		t.Fatalf("Fetch #2: %v", err)
	}

	if len(created) != 1 {
		t.Fatalf("expected exactly 1 engine created and reused, got %d", len(created))
	}
	if created[0].closed {
		t.Error("the reused engine should still be open after two fetches")
	}
	if created[0].fetchN != 2 {
		t.Errorf("reused engine's Fetch called %d times, want 2", created[0].fetchN)
	}
}

func TestManager_SetKeepAliveFalse_ClosesLiveEngineImmediately(t *testing.T) {
	orig := newEngine
	defer func() { newEngine = orig }()

	fake := &fakeEngine{}
	newEngine = func(ctx context.Context) (engineHandle, error) { return fake, nil }

	m := New(true)
	if _, err := m.Fetch(context.Background(), "https://a.example"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if fake.closed {
		t.Fatal("engine should still be open right after a keep-alive fetch")
	}

	if err := m.SetKeepAlive(false); err != nil {
		t.Fatalf("SetKeepAlive(false): %v", err)
	}
	if !fake.closed {
		t.Error("turning keep-alive off should close the live engine immediately, not wait for the next fetch")
	}
}

func TestManager_Stop_IdempotentWithNoEngineEverStarted(t *testing.T) {
	m := New(true)
	if err := m.Stop(); err != nil {
		t.Errorf("Stop() with no engine ever started should be a safe no-op, got: %v", err)
	}
	if err := m.Stop(); err != nil {
		t.Errorf("second Stop() call should still be a safe no-op, got: %v", err)
	}
}

func TestManager_Stop_ClosesLiveEngine(t *testing.T) {
	orig := newEngine
	defer func() { newEngine = orig }()

	fake := &fakeEngine{}
	newEngine = func(ctx context.Context) (engineHandle, error) { return fake, nil }

	m := New(true)
	if _, err := m.Fetch(context.Background(), "https://a.example"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !fake.closed {
		t.Error("Stop should close the live engine")
	}

	// A Fetch after Stop should start a fresh engine (keepAlive is still
	// true — Stop doesn't change the mode, just tears down the current one).
	fake2 := &fakeEngine{}
	newEngine = func(ctx context.Context) (engineHandle, error) { return fake2, nil }
	if _, err := m.Fetch(context.Background(), "https://b.example"); err != nil {
		t.Fatalf("Fetch after Stop: %v", err)
	}
	if fake2.fetchN != 1 {
		t.Errorf("expected the new engine to be used after Stop, fetchN=%d", fake2.fetchN)
	}
}

func TestManager_IsInstalled(t *testing.T) {
	orig := installFn
	defer func() { installFn = orig }()

	installFn = func(ctx context.Context, opts ...browser.Option) error { return nil }
	m := New(false)
	if !m.IsInstalled(context.Background()) {
		t.Error("IsInstalled() = false, want true when installFn succeeds")
	}

	installFn = func(ctx context.Context, opts ...browser.Option) error { return browser.ErrNoBrowserFound }
	if m.IsInstalled(context.Background()) {
		t.Error("IsInstalled() = true, want false when installFn reports ErrNoBrowserFound")
	}
}

func TestManager_IsInstalled_NeverPassesAllowDownload(t *testing.T) {
	// IsInstalled must be a cheap, no-download check — verified indirectly:
	// a fake installFn that errors unless called with zero options confirms
	// no AllowDownload(true) (or any other option) sneaks in.
	orig := installFn
	defer func() { installFn = orig }()

	installFn = func(ctx context.Context, opts ...browser.Option) error {
		if len(opts) != 0 {
			return errors.New("IsInstalled must call installFn with no options")
		}
		return nil
	}
	m := New(false)
	if !m.IsInstalled(context.Background()) {
		t.Error("IsInstalled unexpectedly failed — see error handling above")
	}
}

func TestManager_StartInstall_PassesAllowDownload(t *testing.T) {
	orig := installFn
	defer func() { installFn = orig }()

	var calls, gotOptCount int
	installFn = func(ctx context.Context, opts ...browser.Option) error {
		calls++
		if calls == 1 {
			// StartInstall's own IsInstalled pre-check (0 opts) — must
			// report "not installed" so the real install path below runs.
			return browser.ErrNoBrowserFound
		}
		gotOptCount = len(opts)
		return nil
	}
	m := New(false)
	m.StartInstall(context.Background())
	waitForInstallDone(t, m)
	if gotOptCount == 0 {
		t.Error("StartInstall() should pass AllowDownload(true) (and WithProgress) through to installFn")
	}
	if p := m.InstallProgress(); p.Percent != 100 || p.Error != "" {
		t.Errorf("InstallProgress() after success = %+v, want Percent 100 and no Error", p)
	}
}

func TestManager_StartInstall_UnsupportedPlatformGetsActionableHint(t *testing.T) {
	orig := installFn
	defer func() { installFn = orig }()

	installFn = func(ctx context.Context, opts ...browser.Option) error {
		return fmt.Errorf("browser: download engine: %w: linux/arm64", browser.ErrUnsupportedPlatform)
	}
	m := New(false)
	m.StartInstall(context.Background())
	waitForInstallDone(t, m)
	errMsg := m.InstallProgress().Error
	if errMsg == "" {
		t.Fatal("InstallProgress().Error is empty, want an error")
	}
	if !strings.Contains(errMsg, "install a system Chromium") {
		t.Errorf("InstallProgress().Error = %q, want an actionable install hint", errMsg)
	}
}

func TestManager_StartInstall_OtherErrorsPassThroughWithoutHint(t *testing.T) {
	orig := installFn
	defer func() { installFn = orig }()

	installFn = func(ctx context.Context, opts ...browser.Option) error {
		return browser.ErrNoBrowserFound
	}
	m := New(false)
	m.StartInstall(context.Background())
	waitForInstallDone(t, m)
	errMsg := m.InstallProgress().Error
	if errMsg == "" {
		t.Fatal("InstallProgress().Error is empty, want an error")
	}
	if strings.Contains(errMsg, "install a system Chromium") {
		t.Error("hint should only be added for ErrUnsupportedPlatform, not every error")
	}
}

func TestManager_StartInstall_AlreadyInstalledShortCircuitsWithoutDownload(t *testing.T) {
	orig := installFn
	defer func() { installFn = orig }()

	var calls int
	installFn = func(ctx context.Context, opts ...browser.Option) error {
		calls++
		if len(opts) > 0 {
			t.Error("StartInstall should never reach the download step when already installed")
		}
		return nil // always "installed"
	}
	m := New(false)
	m.StartInstall(context.Background())
	if p := m.InstallProgress(); p.Active || p.Percent != 100 {
		t.Errorf("InstallProgress() = %+v, want an instant Percent:100 short-circuit", p)
	}
	if calls != 1 {
		t.Errorf("installFn called %d times, want exactly 1 (the IsInstalled pre-check)", calls)
	}
}

func TestManager_StartInstall_AlreadyActiveIsANoOp(t *testing.T) {
	orig := installFn
	defer func() { installFn = orig }()

	started := make(chan struct{})
	release := make(chan struct{})
	var calls int
	installFn = func(ctx context.Context, opts ...browser.Option) error {
		calls++
		if len(opts) == 0 {
			return browser.ErrNoBrowserFound // pre-check: not installed
		}
		close(started)
		<-release
		return nil
	}
	m := New(false)
	m.StartInstall(context.Background())
	<-started // first call is now blocked inside the "download"

	m.StartInstall(context.Background()) // must be a no-op, not a second download
	close(release)
	waitForInstallDone(t, m)
	if calls != 2 {
		t.Errorf("installFn called %d times, want exactly 2 (one pre-check + one real download, second StartInstall call added nothing)", calls)
	}
}
