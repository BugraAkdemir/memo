package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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
	if _, _, err := a.LoginRemotePassword("1.2.3.4", "admin", "hunter2", false); err == nil {
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
	token, role, err := a.LoginRemotePassword("1.2.3.4", "admin", "hunter2", false)
	if err != nil {
		t.Fatalf("LoginRemotePassword: %v", err)
	}
	if token == "" {
		t.Fatal("expected a non-empty session token")
	}
	if role != "admin" {
		t.Errorf("role = %q, want %q (legacy single-credential path)", role, "admin")
	}
	if !a.ValidateRemoteSession(token) {
		t.Error("expected the issued session token to validate")
	}
}

func TestLoginRemotePassword_WrongPasswordFails(t *testing.T) {
	a := newPasswordApp(t, "admin", "hunter2")
	if _, _, err := a.LoginRemotePassword("1.2.3.4", "admin", "wrongpassword", false); err == nil {
		t.Fatal("expected wrong password to fail")
	}
}

// TestLoginRemotePassword_RememberIssuesLongLivedToken is the "beni hatırla"
// contract: with remember=true the issued token must carry the 30-day
// lifetime (RememberSessionTTL), not the default 12h one — otherwise the
// checkbox is a no-op and the client is re-prompted daily.
func TestLoginRemotePassword_RememberIssuesLongLivedToken(t *testing.T) {
	a := newPasswordApp(t, "admin", "hunter2")
	token, _, err := a.LoginRemotePassword("1.2.3.4", "admin", "hunter2", true)
	if err != nil {
		t.Fatalf("LoginRemotePassword(remember): %v", err)
	}
	claims := &jwt.RegisteredClaims{}
	if _, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		return a.sessionKey, nil
	}); err != nil {
		t.Fatalf("parse token: %v", err)
	}
	lifetime := claims.ExpiresAt.Sub(claims.IssuedAt.Time)
	if lifetime < remoteauth.RememberSessionTTL-time.Minute {
		t.Errorf("lifetime = %v, want ~%v (remembered session)", lifetime, remoteauth.RememberSessionTTL)
	}
	if lifetime <= remoteauth.SessionTTL {
		t.Errorf("lifetime = %v, must exceed the default %v", lifetime, remoteauth.SessionTTL)
	}
}

func TestLoginRemotePassword_WrongUsernameFails(t *testing.T) {
	a := newPasswordApp(t, "admin", "hunter2")
	if _, _, err := a.LoginRemotePassword("1.2.3.4", "notadmin", "hunter2", false); err == nil {
		t.Fatal("expected wrong username to fail")
	}
}

func TestLoginRemotePassword_LocksOutAfterRepeatedFailures(t *testing.T) {
	a := newPasswordApp(t, "admin", "hunter2")
	var lastErr error
	for i := 0; i < 10; i++ {
		_, _, lastErr = a.LoginRemotePassword("1.2.3.4", "admin", "wrongpassword", false)
	}
	le, ok := lastErr.(*remoteLoginError)
	if !ok || le.LockedFor() <= 0 {
		t.Fatalf("expected a lockout error after repeated failures, got %v", lastErr)
	}
	// Even the correct password must be rejected while locked out.
	if _, _, err := a.LoginRemotePassword("1.2.3.4", "admin", "hunter2", false); err == nil {
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
	token, _, err := a.LoginRemotePassword("1.2.3.4", "admin", "hunter2", false)
	if err != nil {
		t.Fatalf("LoginRemotePassword: %v", err)
	}
	a.cfg.RemoteAccess.Username = "someoneelse"
	if a.ValidateRemoteSession(token) {
		t.Error("expected a session issued for the old username to fail after a rename")
	}
}

// ── Faz 5.1 (yapacam.md): multi-account / role model ───────────────────

func newAccountsApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	if _, err := config.Load(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return &App{cfg: config.Get(), events: &eventRing{}, sessionKey: []byte(testSigningKey)}
}

func TestNeedsSetup_TrueOnFreshInstall(t *testing.T) {
	a := newAccountsApp(t)
	if !a.NeedsSetup() {
		t.Error("expected a fresh install with no accounts and no legacy password to need setup")
	}
}

func TestNeedsSetup_FalseWhenLegacyPasswordAlreadyConfigured(t *testing.T) {
	a := newPasswordApp(t, "admin", "hunter2")
	if a.NeedsSetup() {
		t.Error("expected a pre-Faz-5.1 install with password auth already configured to not need setup")
	}
}

func TestCreateAdminAccount_Succeeds(t *testing.T) {
	a := newAccountsApp(t)
	token, err := a.CreateAdminAccount("alice", "hunter2")
	if err != nil {
		t.Fatalf("CreateAdminAccount: %v", err)
	}
	if token == "" {
		t.Fatal("expected a non-empty session token")
	}
	if a.NeedsSetup() {
		t.Error("expected NeedsSetup to be false immediately after bootstrap")
	}
	if a.cfg.RemoteAccess.AuthMode != "password" {
		t.Errorf("AuthMode = %q, want %q (should upgrade from default token mode)", a.cfg.RemoteAccess.AuthMode, "password")
	}
	role, ok := a.SessionRole(token)
	if !ok || role != "admin" {
		t.Errorf("SessionRole = (%q, %v), want (\"admin\", true)", role, ok)
	}
}

func TestCreateAdminAccount_FailsWhenAlreadySetUp(t *testing.T) {
	a := newAccountsApp(t)
	if _, err := a.CreateAdminAccount("alice", "hunter2"); err != nil {
		t.Fatalf("first CreateAdminAccount: %v", err)
	}
	if _, err := a.CreateAdminAccount("mallory", "hunter3"); err == nil {
		t.Fatal("expected a second bootstrap call to be rejected")
	}
}

func TestCreateAdminAccount_PreservesTokenPasswordMode(t *testing.T) {
	a := newAccountsApp(t)
	a.cfg.RemoteAccess.AuthMode = "token_password"
	if _, err := a.CreateAdminAccount("alice", "hunter2"); err != nil {
		t.Fatalf("CreateAdminAccount: %v", err)
	}
	if a.cfg.RemoteAccess.AuthMode != "token_password" {
		t.Errorf("AuthMode = %q, want unchanged %q", a.cfg.RemoteAccess.AuthMode, "token_password")
	}
}

func TestLoginRemotePassword_UsesAccountsAndReturnsRole(t *testing.T) {
	a := newAccountsApp(t)
	if _, err := a.CreateAdminAccount("alice", "adminpass"); err != nil {
		t.Fatalf("CreateAdminAccount: %v", err)
	}
	if err := a.CreateAccount("bob", "userpass", "user"); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	_, role, err := a.LoginRemotePassword("1.2.3.4", "alice", "adminpass", false)
	if err != nil || role != "admin" {
		t.Errorf("alice login: role=%q err=%v, want role=admin err=nil", role, err)
	}
	_, role, err = a.LoginRemotePassword("1.2.3.4", "bob", "userpass", false)
	if err != nil || role != "user" {
		t.Errorf("bob login: role=%q err=%v, want role=user err=nil", role, err)
	}
	if _, _, err := a.LoginRemotePassword("1.2.3.4", "bob", "wrongpass", false); err == nil {
		t.Error("expected wrong password to fail even though the username exists")
	}
}

func TestSessionRole_UnrecognizedTokenReturnsFalse(t *testing.T) {
	a := newAccountsApp(t)
	if _, ok := a.SessionRole("not-a-real-token"); ok {
		t.Error("expected an unrecognized token to return ok=false")
	}
}

func TestSessionRole_DeletedAccountInvalidatesSession(t *testing.T) {
	a := newAccountsApp(t)
	if _, err := a.CreateAdminAccount("alice", "adminpass"); err != nil {
		t.Fatalf("CreateAdminAccount: %v", err)
	}
	if err := a.CreateAccount("bob", "userpass", "user"); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	token, role, err := a.LoginRemotePassword("1.2.3.4", "bob", "userpass", false)
	if err != nil || role != "user" {
		t.Fatalf("bob login: role=%q err=%v", role, err)
	}
	if _, ok := a.SessionRole(token); !ok {
		t.Fatal("expected bob's session to validate before deletion")
	}

	accounts := a.ListAccounts().([]AccountInfo)
	var bobID string
	for _, acc := range accounts {
		if acc.Username == "bob" {
			bobID = acc.ID
		}
	}
	if err := a.DeleteAccount(bobID); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if _, ok := a.SessionRole(token); ok {
		t.Error("expected bob's session to stop validating after his account was deleted")
	}
}

func TestCreateAccount_DuplicateUsernameRejected(t *testing.T) {
	a := newAccountsApp(t)
	if err := a.CreateAccount("bob", "pass1", "user"); err != nil {
		t.Fatalf("first CreateAccount: %v", err)
	}
	if err := a.CreateAccount("bob", "pass2", "admin"); err == nil {
		t.Error("expected a duplicate username to be rejected")
	}
}

func TestCreateAccount_InvalidRoleRejected(t *testing.T) {
	a := newAccountsApp(t)
	if err := a.CreateAccount("bob", "pass1", "superuser"); err == nil {
		t.Error("expected an unrecognized role to be rejected")
	}
}

func TestDeleteAccount_CannotRemoveLastAdmin(t *testing.T) {
	a := newAccountsApp(t)
	token, err := a.CreateAdminAccount("alice", "adminpass")
	if err != nil {
		t.Fatalf("CreateAdminAccount: %v", err)
	}
	_, ok := a.SessionRole(token)
	if !ok {
		t.Fatal("expected the bootstrap session to validate")
	}
	accounts := a.ListAccounts().([]AccountInfo)
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if err := a.DeleteAccount(accounts[0].ID); err == nil {
		t.Error("expected deleting the last admin account to be rejected")
	}
}

func TestDeleteAccount_AllowsRemovingNonLastAdmin(t *testing.T) {
	a := newAccountsApp(t)
	if _, err := a.CreateAdminAccount("alice", "adminpass"); err != nil {
		t.Fatalf("CreateAdminAccount: %v", err)
	}
	if err := a.CreateAccount("carol", "carolpass", "admin"); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	accounts := a.ListAccounts().([]AccountInfo)
	var aliceID string
	for _, acc := range accounts {
		if acc.Username == "alice" {
			aliceID = acc.ID
		}
	}
	if aliceID == "" {
		t.Fatal("expected to find alice's account")
	}
	if err := a.DeleteAccount(aliceID); err != nil {
		t.Errorf("expected deleting one of two admins to succeed, got %v", err)
	}
}

func TestDeleteAccount_RemovingUserAccountNeverBlocked(t *testing.T) {
	a := newAccountsApp(t)
	if _, err := a.CreateAdminAccount("alice", "adminpass"); err != nil {
		t.Fatalf("CreateAdminAccount: %v", err)
	}
	if err := a.CreateAccount("bob", "userpass", "user"); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	accounts := a.ListAccounts().([]AccountInfo)
	var bobID string
	for _, acc := range accounts {
		if acc.Username == "bob" {
			bobID = acc.ID
		}
	}
	if err := a.DeleteAccount(bobID); err != nil {
		t.Errorf("expected deleting a non-admin account to succeed, got %v", err)
	}
}

func hashTestPassword(t *testing.T, pw string) string {
	t.Helper()
	h, err := remoteauth.HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return h
}

func sessionTokenFor(t *testing.T, a *App, username, role string) string {
	t.Helper()
	tok, err := remoteauth.IssueSessionToken([]byte(testSigningKey), username, role)
	if err != nil {
		t.Fatalf("IssueSessionToken: %v", err)
	}
	return tok
}

func accountsApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	if _, err := config.Load(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	a := &App{cfg: config.Get(), events: &eventRing{}, sessionKey: []byte(testSigningKey)}
	a.cfg.RemoteAccess.Accounts = []config.Account{
		{ID: "a-admin", Username: "admin", PasswordHash: hashTestPassword(t, "adminpw"), Role: "admin", CreatedAt: time.Now()},
		{ID: "a-user", Username: "kaya", PasswordHash: hashTestPassword(t, "userpw"), Role: "user", CreatedAt: time.Now()},
	}
	return a
}

func TestSessionSubject_ValidSessionReturnsUsername(t *testing.T) {
	a := accountsApp(t)
	tok := sessionTokenFor(t, a, "admin", "admin")
	got, ok := a.SessionSubject(tok)
	if !ok || got != "admin" {
		t.Errorf("SessionSubject = (%q, %v), want (\"admin\", true)", got, ok)
	}
}

func TestSessionSubject_GarbageTokenFails(t *testing.T) {
	a := accountsApp(t)
	if got, ok := a.SessionSubject("garbage"); ok || got != "" {
		t.Errorf("SessionSubject(garbage) = (%q, %v), want (\"\", false)", got, ok)
	}
}

func TestChangeAccountPassword_SelfServiceNeedsCurrentPassword(t *testing.T) {
	a := accountsApp(t)
	tok := sessionTokenFor(t, a, "kaya", "user")
	err := a.ChangeAccountPassword(tok, "a-user", "wrongpw", "newpw")
	if err == nil || err.Error() != "current password is incorrect" {
		t.Fatalf("wrong current password: err = %v, want 'current password is incorrect'", err)
	}
	if err := a.ChangeAccountPassword(tok, "a-user", "userpw", "newpw"); err != nil {
		t.Fatalf("self-service change: %v", err)
	}
	ok, verr := remoteauth.VerifyPassword(a.cfg.RemoteAccess.Accounts[1].PasswordHash, "newpw")
	if verr != nil || !ok {
		t.Errorf("expected new password to verify, ok=%v err=%v", ok, verr)
	}
}

func TestChangeAccountPassword_AdminChangesOtherWithoutCurrentPassword(t *testing.T) {
	a := accountsApp(t)
	tok := sessionTokenFor(t, a, "admin", "admin")
	if err := a.ChangeAccountPassword(tok, "a-user", "", "freshpw"); err != nil {
		t.Fatalf("admin change: %v", err)
	}
	ok, verr := remoteauth.VerifyPassword(a.cfg.RemoteAccess.Accounts[1].PasswordHash, "freshpw")
	if verr != nil || !ok {
		t.Errorf("expected freshpw to verify, ok=%v err=%v", ok, verr)
	}
}

func TestChangeAccountPassword_UserCannotChangeOthers(t *testing.T) {
	a := accountsApp(t)
	tok := sessionTokenFor(t, a, "kaya", "user")
	err := a.ChangeAccountPassword(tok, "a-admin", "", "hacked")
	if err == nil || err.Error() != "only admins can change another account's password" {
		t.Fatalf("err = %v, want 'only admins can change another account's password'", err)
	}
}

func TestChangeAccountPassword_UnknownAccountFails(t *testing.T) {
	a := accountsApp(t)
	tok := sessionTokenFor(t, a, "admin", "admin")
	if err := a.ChangeAccountPassword(tok, "nope", "", "x"); err == nil || err.Error() != "account not found: nope" {
		t.Fatalf("err = %v, want 'account not found: nope'", err)
	}
}

func TestChangeAccountPassword_RequiresValidSessionAndNewPassword(t *testing.T) {
	a := accountsApp(t)
	if err := a.ChangeAccountPassword("garbage", "a-admin", "", "x"); err == nil || err.Error() != "valid session token is required" {
		t.Fatalf("garbage session: err = %v, want 'valid session token is required'", err)
	}
	tok := sessionTokenFor(t, a, "admin", "admin")
	if err := a.ChangeAccountPassword(tok, "a-admin", "", ""); err == nil || err.Error() != "new password is required" {
		t.Fatalf("empty new password: err = %v, want 'new password is required'", err)
	}
	if err := a.ChangeAccountPassword(tok, "a-admin", "", "xl"); err != nil {
		t.Fatalf("plain change: %v", err)
	}
}

func TestChangeAccountPassword_LegacyOnlyFails(t *testing.T) {
	a := accountsApp(t)
	a.cfg.RemoteAccess.Accounts = nil
	a.cfg.RemoteAccess.Username = "admin"
	a.cfg.RemoteAccess.PasswordHash = hashTestPassword(t, "pw")
	tok := sessionTokenFor(t, a, "admin", "admin")
	err := a.ChangeAccountPassword(tok, "a-admin", "", "x")
	if err == nil || err.Error() != "password change requires accounts" {
		t.Fatalf("err = %v, want 'password change requires accounts'", err)
	}
}
