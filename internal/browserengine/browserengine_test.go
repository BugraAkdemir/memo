package browserengine

import (
	"context"
	"errors"
	"testing"

	"github.com/BugraAkdemir/gosearch"
	"github.com/BugraAkdemir/gosearch/browser"
)

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

func TestManager_Install_PassesAllowDownload(t *testing.T) {
	orig := installFn
	defer func() { installFn = orig }()

	var gotOptCount int
	installFn = func(ctx context.Context, opts ...browser.Option) error {
		gotOptCount = len(opts)
		return nil
	}
	m := New(false)
	if err := m.Install(context.Background()); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if gotOptCount == 0 {
		t.Error("Install() should pass AllowDownload(true) through to installFn")
	}
}
