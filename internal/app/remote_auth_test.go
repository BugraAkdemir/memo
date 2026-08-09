package app

import (
	"path/filepath"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/remoteauth"
)

const testSigningKey = "test-signing-key-32-bytes-long!!"

func TestSetRemoteAuthConfig_RejectsInvalidMode(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	if err := a.SetRemoteAuthConfig("bogus", "admin", "hunter2"); err == nil {
		t.Fatal("expected an error for an unknown auth mode")
	}
}

func TestSetRemoteAuthConfig_PasswordModeRequiresUsername(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	if err := a.SetRemoteAuthConfig("password", "", "hunter2"); err == nil {
		t.Fatal("expected an error when username is empty in password mode")
	}
}

func TestSetRemoteAuthConfig_PasswordModeRequiresPasswordWhenNoneStored(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	if err := a.SetRemoteAuthConfig("password", "admin", ""); err == nil {
		t.Fatal("expected an error when no password is set and none was ever stored")
	}
}

func TestSetRemoteAuthConfig_TokenModeNeedsNoCredentials(t *testing.T) {
	// "token" and "none" never touch Username/PasswordHash, so this must
	// succeed with both empty — and since it's a value that never reaches
	// config.Save's error path from validation, it exercises the actual
	// persisted method rather than being skipped for Save-avoidance.
	dir := t.TempDir()
	if _, err := config.Load(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	a := &App{cfg: config.Get(), events: &eventRing{}}
	if err := a.SetRemoteAuthConfig("token", "", ""); err != nil {
		t.Fatalf("SetRemoteAuthConfig: %v", err)
	}
	if a.cfg.RemoteAccess.AuthMode != "token" {
		t.Errorf("AuthMode = %q, want %q", a.cfg.RemoteAccess.AuthMode, "token")
	}
}

func TestSetRemoteAuthConfig_SuccessPersistsHashNotPlaintext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if _, err := config.Load(path); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	a := &App{cfg: config.Get(), events: &eventRing{}}

	if err := a.SetRemoteAuthConfig("password", "admin", "hunter2"); err != nil {
		t.Fatalf("SetRemoteAuthConfig: %v", err)
	}
	if a.cfg.RemoteAccess.Username != "admin" {
		t.Errorf("Username = %q, want %q", a.cfg.RemoteAccess.Username, "admin")
	}
	if a.cfg.RemoteAccess.PasswordHash == "" || a.cfg.RemoteAccess.PasswordHash == "hunter2" {
		t.Errorf("expected PasswordHash to be a hash, not empty or plaintext, got %q", a.cfg.RemoteAccess.PasswordHash)
	}
	ok, err := remoteauth.VerifyPassword(a.cfg.RemoteAccess.PasswordHash, "hunter2")
	if err != nil || !ok {
		t.Errorf("expected stored hash to verify against the original password, ok=%v err=%v", ok, err)
	}

	// Reload from disk to confirm it was actually persisted, not just
	// mutated in memory.
	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.RemoteAccess.Username != "admin" {
		t.Errorf("persisted Username = %q, want %q", reloaded.RemoteAccess.Username, "admin")
	}
}

func TestSetRemoteAuthConfig_EmptyPasswordKeepsExistingHash(t *testing.T) {
	dir := t.TempDir()
	if _, err := config.Load(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	a := &App{cfg: config.Get(), events: &eventRing{}}
	if err := a.SetRemoteAuthConfig("password", "admin", "hunter2"); err != nil {
		t.Fatalf("initial SetRemoteAuthConfig: %v", err)
	}
	firstHash := a.cfg.RemoteAccess.PasswordHash

	// Re-saving with the same mode/username and an empty password (e.g. the
	// Settings form re-submitted without retyping the password) must not
	// wipe the existing hash.
	if err := a.SetRemoteAuthConfig("password", "admin", ""); err != nil {
		t.Fatalf("second SetRemoteAuthConfig: %v", err)
	}
	if a.cfg.RemoteAccess.PasswordHash != firstHash {
		t.Error("expected an empty password on a later call to leave the existing hash untouched")
	}
}

func TestCreateAndListAndRevokeRemoteDevice(t *testing.T) {
	dir := t.TempDir()
	if _, err := config.Load(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	a := &App{cfg: config.Get(), events: &eventRing{}}

	plain, err := a.CreateRemoteDevice("Laptop")
	if err != nil {
		t.Fatalf("CreateRemoteDevice: %v", err)
	}
	if plain == "" {
		t.Fatal("expected a non-empty plaintext token")
	}

	devices := a.ListRemoteDevices().([]RemoteDeviceInfo)
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].Name != "Laptop" {
		t.Errorf("device name = %q, want %q", devices[0].Name, "Laptop")
	}

	// Pre-seed LastSeenAt so the verify call below doesn't also trip
	// VerifyRemoteDeviceToken's staleness threshold and fire its async
	// config.Save — that path is covered on its own, without a real
	// config.Load, by TestVerifyRemoteDeviceToken_UpdatesLastSeenInMemory;
	// mixing the two here would race an out-of-band goroutine against this
	// test's own t.TempDir() cleanup.
	a.cfg.RemoteAccess.Devices[0].LastSeenAt = time.Now()

	if !a.VerifyRemoteDeviceToken(plain) {
		t.Error("expected the freshly created device token to verify")
	}
	if a.VerifyRemoteDeviceToken("memo-not-a-real-token-at-all") {
		t.Error("expected a bogus token to fail verification")
	}

	if err := a.RevokeRemoteDevice(devices[0].ID); err != nil {
		t.Fatalf("RevokeRemoteDevice: %v", err)
	}
	if len(a.ListRemoteDevices().([]RemoteDeviceInfo)) != 0 {
		t.Error("expected device list to be empty after revocation")
	}
	if a.VerifyRemoteDeviceToken(plain) {
		t.Error("expected a revoked device's token to no longer verify")
	}
}

func TestRevokeRemoteDevice_UnknownIDFails(t *testing.T) {
	dir := t.TempDir()
	if _, err := config.Load(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	a := &App{cfg: config.Get(), events: &eventRing{}}
	if err := a.RevokeRemoteDevice("does-not-exist"); err == nil {
		t.Fatal("expected an error revoking a device ID that doesn't exist")
	}
}

func TestVerifyRemoteDeviceToken_UpdatesLastSeenInMemory(t *testing.T) {
	// Deliberately does NOT go through config.Load/Save (see the package's
	// other tests for why: config.Save is a process-wide global). A brand
	// new device's zero-value LastSeenAt would trip VerifyRemoteDeviceToken's
	// staleness threshold and fire an async config.Save — avoided here by
	// asserting only the synchronous in-memory mutation, which is what this
	// test actually targets.
	plain := remoteauth.GenerateDeviceToken()
	a := &App{cfg: &config.AppConfig{
		RemoteAccess: config.RemoteAccessConfig{
			Devices: []config.RemoteDevice{{
				ID:         "dev1",
				Name:       "Phone",
				TokenHash:  remoteauth.HashToken(plain),
				LastSeenAt: time.Now(), // not stale — no background Save triggered
			}},
		},
	}}
	if !a.VerifyRemoteDeviceToken(plain) {
		t.Fatal("expected token to verify")
	}
	if time.Since(a.cfg.RemoteAccess.Devices[0].LastSeenAt) > time.Second {
		t.Error("expected LastSeenAt to be updated to approximately now")
	}
}

func TestLoginRemotePassword_WrongModeRejected(t *testing.T) {
	a := &App{cfg: &config.AppConfig{RemoteAccess: config.RemoteAccessConfig{AuthMode: "token"}}}
	if _, err := a.LoginRemotePassword("1.2.3.4", "admin", "hunter2"); err == nil {
		t.Fatal("expected login to be rejected when auth mode doesn't support passwords")
	}
}

func newPasswordApp(t *testing.T, username, password string) *App {
	t.Helper()
	hash, err := remoteauth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return &App{
		cfg: &config.AppConfig{RemoteAccess: config.RemoteAccessConfig{
			AuthMode:     "password",
			Username:     username,
			PasswordHash: hash,
		}},
		sessionKey: []byte(testSigningKey),
		events:     &eventRing{},
	}
}

func TestLoginRemotePassword_CorrectCredentialsIssueValidSession(t *testing.T) {
	a := newPasswordApp(t, "admin", "hunter2")
	token, err := a.LoginRemotePassword("1.2.3.4", "admin", "hunter2")
	if err != nil {
		t.Fatalf("LoginRemotePassword: %v", err)
	}
	if token == "" {
		t.Fatal("expected a non-empty session token")
	}
	if !a.ValidateRemoteSession(token) {
		t.Error("expected the issued session token to validate")
	}
}

func TestLoginRemotePassword_WrongPasswordFails(t *testing.T) {
	a := newPasswordApp(t, "admin", "hunter2")
	if _, err := a.LoginRemotePassword("1.2.3.4", "admin", "wrongpassword"); err == nil {
		t.Fatal("expected wrong password to fail")
	}
}

func TestLoginRemotePassword_WrongUsernameFails(t *testing.T) {
	a := newPasswordApp(t, "admin", "hunter2")
	if _, err := a.LoginRemotePassword("1.2.3.4", "notadmin", "hunter2"); err == nil {
		t.Fatal("expected wrong username to fail")
	}
}

func TestLoginRemotePassword_LocksOutAfterRepeatedFailures(t *testing.T) {
	a := newPasswordApp(t, "admin", "hunter2")
	var lastErr error
	for i := 0; i < 10; i++ {
		_, lastErr = a.LoginRemotePassword("1.2.3.4", "admin", "wrongpassword")
	}
	le, ok := lastErr.(*remoteLoginError)
	if !ok || le.LockedFor() <= 0 {
		t.Fatalf("expected a lockout error after repeated failures, got %v", lastErr)
	}
	// Even the correct password must be rejected while locked out.
	if _, err := a.LoginRemotePassword("1.2.3.4", "admin", "hunter2"); err == nil {
		t.Error("expected correct credentials to still be rejected during lockout")
	}
}

func TestValidateRemoteSession_RejectsGarbage(t *testing.T) {
	a := &App{
		cfg:        &config.AppConfig{RemoteAccess: config.RemoteAccessConfig{Username: "admin"}},
		sessionKey: []byte(testSigningKey),
	}
	if a.ValidateRemoteSession("not-a-real-token") {
		t.Error("expected a garbage token to fail validation")
	}
}

func TestValidateRemoteSession_RejectsTokenForRenamedUser(t *testing.T) {
	a := newPasswordApp(t, "admin", "hunter2")
	token, err := a.LoginRemotePassword("1.2.3.4", "admin", "hunter2")
	if err != nil {
		t.Fatalf("LoginRemotePassword: %v", err)
	}
	a.cfg.RemoteAccess.Username = "someoneelse"
	if a.ValidateRemoteSession(token) {
		t.Error("expected a session issued for the old username to fail after a rename")
	}
}
