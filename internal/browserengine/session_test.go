package browserengine

import (
	"context"
	"testing"
	"time"
)

// newFakeSession builds a Session that never touches chromedp — its tabCtx/
// allocCtx stay nil, so only lifecycle methods (Close, the idle timer, and
// Manager's bookkeeping around StartSession/GetSession/StopSession) are
// exercised, never Navigate/Screenshot (those would nil-panic on a fake and
// aren't what these tests are about — see session_real_test.go for the
// real-Chromium round trip).
func newFakeSession(onIdle func(*Session)) *Session {
	s := &Session{startedAt: time.Now(), lastUsed: time.Now()}
	s.onIdle = func() { onIdle(s) }
	s.armIdleTimer()
	return s
}

func withFakeNewSession(t *testing.T) *int {
	t.Helper()
	origNewSession := newSession
	origTimeout := sessionIdleTimeout
	t.Cleanup(func() {
		newSession = origNewSession
		sessionIdleTimeout = origTimeout
	})
	calls := 0
	newSession = func(ctx context.Context, onIdle func(*Session)) (*Session, error) {
		calls++
		return newFakeSession(onIdle), nil
	}
	return &calls
}

func TestManager_StartSession_ReusesExistingSession(t *testing.T) {
	calls := withFakeNewSession(t)

	m := New(false)
	s1, err := m.StartSession(context.Background())
	if err != nil {
		t.Fatalf("StartSession #1: %v", err)
	}
	s2, err := m.StartSession(context.Background())
	if err != nil {
		t.Fatalf("StartSession #2: %v", err)
	}
	if s1 != s2 {
		t.Error("StartSession should return the same Session while one is already running")
	}
	if *calls != 1 {
		t.Errorf("newSession called %d times, want exactly 1 (second StartSession should reuse)", *calls)
	}
}

func TestManager_GetSession_ReflectsLifecycle(t *testing.T) {
	withFakeNewSession(t)

	m := New(false)
	if _, ok := m.GetSession(); ok {
		t.Error("GetSession() ok=true before any StartSession call")
	}

	started, err := m.StartSession(context.Background())
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	got, ok := m.GetSession()
	if !ok || got != started {
		t.Error("GetSession() should return the just-started session")
	}
}

func TestManager_StopSession_ClosesAndClearsReference(t *testing.T) {
	withFakeNewSession(t)

	m := New(false)
	s, err := m.StartSession(context.Background())
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if err := m.StopSession(); err != nil {
		t.Fatalf("StopSession: %v", err)
	}
	if _, ok := m.GetSession(); ok {
		t.Error("GetSession() ok=true after StopSession")
	}
	if err := s.checkOpen(); err != errSessionClosed {
		t.Errorf("session.checkOpen() after StopSession = %v, want errSessionClosed", err)
	}
}

func TestManager_StopSession_IdempotentWithNoSessionEverStarted(t *testing.T) {
	m := New(false)
	if err := m.StopSession(); err != nil {
		t.Errorf("StopSession() with no session ever started should be a safe no-op, got: %v", err)
	}
	if err := m.StopSession(); err != nil {
		t.Errorf("second StopSession() call should still be a safe no-op, got: %v", err)
	}
}

func TestManager_StartSession_AfterStopLaunchesAFreshOne(t *testing.T) {
	calls := withFakeNewSession(t)

	m := New(false)
	s1, err := m.StartSession(context.Background())
	if err != nil {
		t.Fatalf("StartSession #1: %v", err)
	}
	if err := m.StopSession(); err != nil {
		t.Fatalf("StopSession: %v", err)
	}
	s2, err := m.StartSession(context.Background())
	if err != nil {
		t.Fatalf("StartSession #2: %v", err)
	}
	if s1 == s2 {
		t.Error("StartSession after StopSession should launch a fresh session, not reuse the closed one")
	}
	if *calls != 2 {
		t.Errorf("newSession called %d times, want exactly 2", *calls)
	}
}

func TestSession_IdleTimeout_ClosesAndNotifiesManager(t *testing.T) {
	origTimeout := sessionIdleTimeout
	sessionIdleTimeout = 20 * time.Millisecond
	defer func() { sessionIdleTimeout = origTimeout }()

	origNewSession := newSession
	defer func() { newSession = origNewSession }()
	newSession = func(ctx context.Context, onIdle func(*Session)) (*Session, error) {
		return newFakeSession(onIdle), nil
	}

	m := New(false)
	s, err := m.StartSession(context.Background())
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := m.GetSession(); !ok {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, ok := m.GetSession(); ok {
		t.Fatal("Manager still reports a session after it should have idle-timed-out")
	}
	if err := s.checkOpen(); err != errSessionClosed {
		t.Errorf("timed-out session.checkOpen() = %v, want errSessionClosed", err)
	}
}

func TestSession_Touch_ResetsIdleTimer(t *testing.T) {
	origTimeout := sessionIdleTimeout
	sessionIdleTimeout = 60 * time.Millisecond
	defer func() { sessionIdleTimeout = origTimeout }()

	idled := make(chan struct{})
	s := newFakeSession(func(*Session) { close(idled) })

	// Touch repeatedly for longer than one timeout window — the session
	// must not idle-close as long as it keeps being used.
	stop := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(stop) {
		s.touch()
		time.Sleep(15 * time.Millisecond)
	}
	select {
	case <-idled:
		t.Fatal("session idle-closed despite being touched continuously")
	default:
	}

	// Now stop touching it — it should idle-close shortly after.
	select {
	case <-idled:
	case <-time.After(2 * time.Second):
		t.Fatal("session did not idle-close after touching stopped")
	}
}

func TestSession_Close_IsIdempotent(t *testing.T) {
	s := newFakeSession(func(*Session) {})
	if err := s.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close should be a safe no-op, got: %v", err)
	}
}

func TestLooksLikePNG(t *testing.T) {
	if looksLikePNG(nil) {
		t.Error("looksLikePNG(nil) = true, want false")
	}
	if looksLikePNG([]byte{0x00, 0x00, 0x00, 0x00}) {
		t.Error("looksLikePNG on non-PNG bytes = true, want false")
	}
	if !looksLikePNG(pngMagic) {
		t.Error("looksLikePNG(pngMagic) = false, want true")
	}
}
