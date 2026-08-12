package app

import (
	"fmt"
	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/remoteauth"
	"time"
)

// RemoteDeviceInfo is the device list DTO exposed to clients — deliberately
// excludes TokenHash (the whole point of hashing at rest is that the secret
// never leaves the moment it was generated).
type RemoteDeviceInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	// LastSeenAt is a pointer, not a plain time.Time — encoding/json's
	// omitempty has no effect on struct-typed fields (a well-known Go
	// gotcha: the "empty" check only applies to bools/numbers/strings/nil
	// pointers-slices-maps-interfaces, never to a zero-value struct), so a
	// never-verified device would otherwise always serialize
	// "0001-01-01T00:00:00Z" instead of being omitted.
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
}

// AccountInfo is the account list DTO exposed to clients (Faz 5.1,
// yapacam.md) — deliberately excludes PasswordHash, same reasoning as
// RemoteDeviceInfo excluding TokenHash.
type AccountInfo struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// validAccountRoles lists every value Account.Role may hold.
var validAccountRoles = map[string]bool{
	"admin": true,
	"user":  true,
}

// validAuthModes lists every value RemoteAccess.AuthMode may hold — kept in
// one place so SetRemoteAuthConfig's validation and any future switch over
// modes (see remoteAuthOK in internal/webserver) can't silently drift apart.
var validAuthModes = map[string]bool{
	"none":           true,
	"token":          true,
	"password":       true,
	"token_password": true,
}

// sessionSigningKey lazily loads (or, on first use, generates) the HMAC key
// that signs password-login session tokens. Lazy rather than eager at
// Startup because most installs never touch password auth at all (default
// AuthMode is "token") — no reason to read/create a key file on every boot
// for a feature that may never be used.
func (a *App) sessionSigningKey() ([]byte, error) {
	a.remoteAuthMu.Lock()
	defer a.remoteAuthMu.Unlock()
	if a.sessionKey != nil {
		return a.sessionKey, nil
	}
	key, err := remoteauth.LoadOrCreateSigningKey(config.DataPath("session.key"))
	if err != nil {
		return nil, err
	}
	a.sessionKey = key
	return key, nil
}

// authLimiter lazily creates the brute-force limiter backing password
// login. A process-lifetime singleton — its whole purpose is to remember
// failures across requests, so it must outlive any single call.
func (a *App) authLimiter() *remoteauth.Limiter {
	a.remoteAuthMu.Lock()
	defer a.remoteAuthMu.Unlock()
	if a.remoteAuthLimiter == nil {
		a.remoteAuthLimiter = remoteauth.NewLimiter()
	}
	return a.remoteAuthLimiter
}

// GetRemoteAuthMode returns the configured remote-access auth mode
// ("none"/"token"/"password"/"token_password" — validate() guarantees this
// is never empty).
func (a *App) GetRemoteAuthMode() string {
	return a.cfg.RemoteAccess.AuthMode
}

// SetRemoteAuthConfig updates the auth mode and, for password-backed modes,
// the username/password. An empty password leaves any existing
// PasswordHash untouched (so re-saving the mode/username from Settings
// without retyping the password doesn't wipe it) — but a password/token
// mode with no password ever set at all (no prior hash, empty input) is
// rejected outright, since that would silently produce a mode no one could
// ever log into.
func (a *App) SetRemoteAuthConfig(mode, username, password string) error {
	if !validAuthModes[mode] {
		return fmt.Errorf("invalid auth mode: %q", mode)
	}
	needsPassword := mode == "password" || mode == "token_password"
	if needsPassword {
		if username == "" {
			return fmt.Errorf("username is required for %q mode", mode)
		}
		if password == "" && a.cfg.RemoteAccess.PasswordHash == "" {
			return fmt.Errorf("password is required to enable %q mode", mode)
		}
	}
	if password != "" {
		hash, err := remoteauth.HashPassword(password)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		a.cfg.RemoteAccess.PasswordHash = hash
	}
	a.cfg.RemoteAccess.AuthMode = mode
	if username != "" {
		a.cfg.RemoteAccess.Username = username
	}
	if err := config.Save(a.cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

// ListRemoteDevices returns every paired device's metadata (never the
// token itself — see RemoteDeviceInfo). Returns interface{}, not
// []RemoteDeviceInfo, so this satisfies webserver.FullBridge without that
// package needing to import internal/app (see GetRemoteAccessStatus's
// identical reasoning) — an interface method's return type has to match
// exactly, so the concrete slice type can't stand in for interface{}
// implicitly at the interface-satisfaction level even though it does at
// any individual call site.
func (a *App) ListRemoteDevices() interface{} {
	a.remoteDevicesMu.Lock()
	defer a.remoteDevicesMu.Unlock()
	devices := a.cfg.RemoteAccess.Devices
	out := make([]RemoteDeviceInfo, 0, len(devices))
	for _, d := range devices {
		info := RemoteDeviceInfo{ID: d.ID, Name: d.Name, CreatedAt: d.CreatedAt}
		if !d.LastSeenAt.IsZero() {
			lastSeen := d.LastSeenAt
			info.LastSeenAt = &lastSeen
		}
		out = append(out, info)
	}
	return out
}

// CreateRemoteDevice generates a new device token, persists only its hash,
// and returns the plaintext exactly once — this is the caller's one and
// only chance to see/copy it, matching how the original single shared
// token was handed to the client (see GetRemoteAccessStatus's doc comment
// in the pre-Faz-2 code).
func (a *App) CreateRemoteDevice(name string) (string, error) {
	if name == "" {
		name = "Device"
	}
	plain := remoteauth.GenerateDeviceToken()
	a.remoteDevicesMu.Lock()
	a.cfg.RemoteAccess.Devices = append(a.cfg.RemoteAccess.Devices, config.RemoteDevice{
		ID:        remoteauth.GenerateDeviceID(),
		Name:      name,
		TokenHash: remoteauth.HashToken(plain),
		CreatedAt: time.Now(),
	})
	err := config.Save(a.cfg)
	a.remoteDevicesMu.Unlock()
	if err != nil {
		return "", fmt.Errorf("save config: %w", err)
	}
	a.emitEvent("remote_auth:device_created", name)
	return plain, nil
}

// RevokeRemoteDevice removes a paired device's access permanently.
func (a *App) RevokeRemoteDevice(id string) error {
	a.remoteDevicesMu.Lock()
	devices := a.cfg.RemoteAccess.Devices
	idx := -1
	for i, d := range devices {
		if d.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		a.remoteDevicesMu.Unlock()
		return fmt.Errorf("device not found: %s", id)
	}
	name := devices[idx].Name
	a.cfg.RemoteAccess.Devices = append(devices[:idx], devices[idx+1:]...)
	err := config.Save(a.cfg)
	a.remoteDevicesMu.Unlock()

	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	a.emitEvent("remote_auth:device_revoked", name)
	return nil
}

// deviceLastSeenSaveThreshold throttles how often a VerifyRemoteDeviceToken
// hit persists its LastSeenAt update — this runs on every authenticated
// remote request, and a config.Save() (full YAML re-marshal + atomic file
// write) on every single one of those would be needless disk wear for a
// display-only "last seen" timestamp. The in-memory value is still updated
// on every call; only the persisted copy is throttled.
const deviceLastSeenSaveThreshold = time.Minute

// VerifyRemoteDeviceToken reports whether token matches any paired
// device's hash, and if so records that device as just-seen.
func (a *App) VerifyRemoteDeviceToken(token string) bool {
	a.remoteDevicesMu.Lock()
	devices := a.cfg.RemoteAccess.Devices
	matched := -1
	for i := range devices {
		if remoteauth.VerifyTokenHash(devices[i].TokenHash, token) {
			matched = i
			break
		}
	}
	if matched == -1 {
		a.remoteDevicesMu.Unlock()
		return false
	}
	now := time.Now()
	stale := now.Sub(devices[matched].LastSeenAt) > deviceLastSeenSaveThreshold
	devices[matched].LastSeenAt = now
	a.remoteDevicesMu.Unlock()

	if stale {
		// Best-effort, off the request path: a lost "last seen" update is
		// not worth failing (or even delaying) the request over. Re-taking
		// remoteDevicesMu here (rather than holding it across the save)
		// keeps this serialized against other device-list mutators
		// (Create/Revoke/another VerifyRemoteDeviceToken call) without
		// blocking the request that triggered it on disk I/O.
		go func() {
			a.remoteDevicesMu.Lock()
			err := config.Save(a.cfg)
			a.remoteDevicesMu.Unlock()
			if err != nil {
				logx.Printf("remote_auth: failed to persist device last-seen: %v", err)
			}
		}()
	}
	return true
}

// remoteLoginError distinguishes a brute-force lockout (callers should
// surface Retry-After) from a plain bad-credentials rejection, without
// exposing which specific check failed.
type remoteLoginError struct {
	lockedFor time.Duration
}

func (e *remoteLoginError) Error() string {
	if e.lockedFor > 0 {
		return "too many failed attempts"
	}
	return "invalid credentials"
}

// LockedFor returns how long the caller must wait before retrying, or 0 if
// this failure was a plain bad-credentials rejection rather than a
// lockout.
func (e *remoteLoginError) LockedFor() time.Duration { return e.lockedFor }

// findAccount returns a pointer into a.cfg.RemoteAccess.Accounts matching
// username, or nil. Callers must hold remoteAccountsMu.
func (a *App) findAccount(username string) *config.Account {
	for i := range a.cfg.RemoteAccess.Accounts {
		if a.cfg.RemoteAccess.Accounts[i].Username == username {
			return &a.cfg.RemoteAccess.Accounts[i]
		}
	}
	return nil
}

// LoginRemotePassword validates username/password and, on success, issues
// a signed session token carrying the matched account's role. remoteAddr
// should be the raw connecting IP (used only to key the brute-force
// limiter — never persisted). remember requests the longer "remember me"
// lifetime (RememberSessionTTL) instead of the default short one — the
// login screen's checkbox maps straight onto this parameter.
//
// Once Accounts (Faz 5.1, yapacam.md) has at least one entry, it is the
// sole source of truth: the legacy single Username/PasswordHash pair is
// ignored for login purposes even if still set (see config.
// migrateLegacyRemoteAccount's doc comment — an existing single-credential
// install is migrated into Accounts' first, "admin" entry, so this switch
// is invisible to a user who never touches the new multi-account
// features). Only when Accounts is still empty does this fall back to the
// legacy pair, with an implicit "admin" role.
func (a *App) LoginRemotePassword(remoteAddr, username, password string, remember bool) (token, role string, err error) {
	mode := a.cfg.RemoteAccess.AuthMode
	if mode != "password" && mode != "token_password" {
		return "", "", fmt.Errorf("password login is not enabled (auth mode is %q)", mode)
	}

	limiter := a.authLimiter()
	key := remoteAddr + "|" + username

	a.remoteAccountsMu.Lock()
	if allowed, retryAfter := limiter.Allowed(key); !allowed {
		a.remoteAccountsMu.Unlock()
		return "", "", &remoteLoginError{lockedFor: retryAfter}
	}

	valid := false
	if len(a.cfg.RemoteAccess.Accounts) > 0 {
		if acc := a.findAccount(username); acc != nil {
			ok, verr := remoteauth.VerifyPassword(acc.PasswordHash, password)
			valid = ok && verr == nil
			role = acc.Role
		}
	} else {
		valid = username != "" && username == a.cfg.RemoteAccess.Username
		if valid {
			ok, verr := remoteauth.VerifyPassword(a.cfg.RemoteAccess.PasswordHash, password)
			valid = ok && verr == nil
		}
		role = "admin"
	}
	a.remoteAccountsMu.Unlock()

	if !valid {
		lockout := limiter.RecordFailure(key)
		a.emitEvent("remote_auth:login_failed", username)
		return "", "", &remoteLoginError{lockedFor: lockout}
	}
	limiter.RecordSuccess(key)

	signingKey, err := a.sessionSigningKey()
	if err != nil {
		return "", "", fmt.Errorf("session signing key: %w", err)
	}
	ttl := remoteauth.SessionTTL
	if remember {
		ttl = remoteauth.RememberSessionTTL
	}
	token, err = remoteauth.IssueSessionTokenWithTTL(signingKey, username, role, ttl)
	if err != nil {
		return "", "", fmt.Errorf("issue session token: %w", err)
	}
	a.emitEvent("remote_auth:login_success", username)
	return token, role, nil
}

// ValidateRemoteSession reports whether token is a currently-valid session
// token issued for a still-existing identity — either an Accounts entry
// (Faz 5.1) or, when Accounts is empty, the legacy single username (see
// LoginRemotePassword's doc comment for which one is authoritative).
// Checking against the *current* list (not just signature+expiry) means
// renaming/deleting an account, or renaming the legacy username,
// immediately invalidates every session issued under the old name. A
// *password* change alone does not rotate the signing key or otherwise
// invalidate outstanding sessions — known limitation, not a design goal:
// a compromised session token still self-expires within SessionTTL (12h)
// regardless, same as noted on remoteauth.SessionTTL.
func (a *App) ValidateRemoteSession(token string) bool {
	_, _, ok := a.sessionSubjectRole(token)
	return ok
}

// SessionRole returns the role a currently-valid session token's subject
// currently holds (re-checked against the live account list, not just
// trusted from the JWT claim — see ValidateRemoteSession), and whether the
// token validated at all. Used by admin-only endpoints (internal/webserver)
// to distinguish a "user"-role session from an "admin" one; a device token
// or an unrecognized/expired credential returns ok=false, which callers
// treat as "no role information available" rather than "forbidden" — see
// the gating logic's own doc comment for why.
func (a *App) SessionRole(token string) (string, bool) {
	_, role, ok := a.sessionSubjectRole(token)
	return role, ok
}

// sessionSubjectRole is the shared implementation behind
// ValidateRemoteSession/SessionRole/SessionSubject.
func (a *App) sessionSubjectRole(token string) (subject, role string, ok bool) {
	signingKey, err := a.sessionSigningKey()
	if err != nil {
		return "", "", false
	}
	subject, _, err = remoteauth.ValidateSessionToken(signingKey, token)
	if err != nil || subject == "" {
		return "", "", false
	}

	a.remoteAccountsMu.Lock()
	defer a.remoteAccountsMu.Unlock()
	if len(a.cfg.RemoteAccess.Accounts) > 0 {
		if acc := a.findAccount(subject); acc != nil {
			return subject, acc.Role, true
		}
		return "", "", false
	}
	if subject == a.cfg.RemoteAccess.Username {
		return subject, "admin", true
	}
	return "", "", false
}

// NeedsSetup reports whether the server has never had a password-based
// identity configured at all — no Accounts entries and no legacy
// Username/PasswordHash pair. While true, POST /api/setup/create-admin is
// reachable without any credential (see internal/webserver's
// isSetupBootstrapPath) and the minimal web UI shows a first-run "create
// your account" screen instead of a token/login prompt (Faz 5.1,
// yapacam.md). An install that already had password auth configured
// before Faz 5.1 shipped is treated as already set up — see config.
// migrateLegacyRemoteAccount — so upgrading never re-surfaces this screen
// for an existing user.
func (a *App) NeedsSetup() bool {
	a.remoteAccountsMu.Lock()
	defer a.remoteAccountsMu.Unlock()
	return len(a.cfg.RemoteAccess.Accounts) == 0 &&
		a.cfg.RemoteAccess.Username == "" &&
		!a.cfg.RemoteAccess.SetupBootstrapped
}

// CreateAdminAccount creates the very first Accounts entry (always Role
// "admin") and immediately logs it in, returning a session token — the
// bootstrap flow the minimal web UI's first-run screen drives. Only
// reachable while NeedsSetup() is true; the second call (and every call
// after) fails, permanently, regardless of server restarts, matching
// yapacam.md's "ilk başarılı çağrıdan sonra kalıcı olarak kapanır"
// requirement — there is no code path that ever clears Accounts back to
// empty other than DeleteAccount, which itself refuses to remove the last
// admin (see its doc comment).
//
// Also upgrades AuthMode when needed: a fresh install defaults to "token"
// (no password login checked at all — see remoteAuthOK), which would make
// the freshly created account's password silently unusable. "none" and
// "token" both become "password"; "token_password" is left alone (already
// password-capable, and downgrading would break any already-paired device
// token).
func (a *App) CreateAdminAccount(username, password string) (string, error) {
	if !a.NeedsSetup() {
		return "", fmt.Errorf("setup already completed")
	}
	if username == "" {
		return "", fmt.Errorf("username is required")
	}
	if password == "" {
		return "", fmt.Errorf("password is required")
	}
	hash, err := remoteauth.HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	a.remoteAccountsMu.Lock()
	// Re-check under lock: NeedsSetup() above raced no lock at all, so two
	// concurrent bootstrap requests could otherwise both pass it and both
	// append an account. SetupBootstrapped is included too, guarding against
	// a race with the token-only bootstrap path (BootstrapTokenAuth) below.
	if len(a.cfg.RemoteAccess.Accounts) > 0 || a.cfg.RemoteAccess.Username != "" ||
		a.cfg.RemoteAccess.SetupBootstrapped {
		a.remoteAccountsMu.Unlock()
		return "", fmt.Errorf("setup already completed")
	}
	a.cfg.RemoteAccess.Accounts = append(a.cfg.RemoteAccess.Accounts, config.Account{
		ID:           remoteauth.GenerateDeviceID(),
		Username:     username,
		PasswordHash: hash,
		Role:         "admin",
		CreatedAt:    time.Now(),
	})
	if a.cfg.RemoteAccess.AuthMode == "none" || a.cfg.RemoteAccess.AuthMode == "token" || a.cfg.RemoteAccess.AuthMode == "" {
		a.cfg.RemoteAccess.AuthMode = "password"
	}
	err = config.Save(a.cfg)
	a.remoteAccountsMu.Unlock()
	if err != nil {
		return "", fmt.Errorf("save config: %w", err)
	}
	a.emitEvent("remote_auth:admin_bootstrapped", username)

	signingKey, err := a.sessionSigningKey()
	if err != nil {
		return "", fmt.Errorf("session signing key: %w", err)
	}
	token, err := remoteauth.IssueSessionToken(signingKey, username, "admin")
	if err != nil {
		return "", fmt.Errorf("issue session token: %w", err)
	}
	return token, nil
}

// BootstrapTokenAuth is the token-only counterpart to CreateAdminAccount:
// the first-run "just give me a device token, no account" setup choice has
// no account/username of its own to flip NeedsSetup() false, so without
// this it would depend on NeedsSetup() staying true forever (see
// SetupBootstrapped's doc comment on RemoteAccessConfig). Sets AuthMode to
// "token" and creates the very first device token as one atomic operation,
// self-gated on NeedsSetup() the same way CreateAdminAccount is — reachable
// with no credential (POST /api/setup/create-device, see
// isSetupBootstrapPath) only because it can never succeed a second time.
//
// Before this existed, the token-only setup screen's two calls
// (PUT /api/remote-access then POST /api/remote-access/devices) went
// through the normal, authenticated remoteAuthMiddleware — which a
// first-run client with no credential at all can only pass from a loopback
// source. Every non-loopback attempt (a phone/desktop client setting up a
// remote server over LAN, the exact case the other two setup methods are
// for) 401'd on the very first call, deterministically.
func (a *App) BootstrapTokenAuth(deviceName string) (string, error) {
	a.remoteAccountsMu.Lock()
	// Same re-check-under-lock reasoning as CreateAdminAccount: NeedsSetup()
	// above (if a caller checked it) races no lock, so two concurrent
	// bootstrap requests — of either kind — could otherwise both pass.
	if len(a.cfg.RemoteAccess.Accounts) > 0 || a.cfg.RemoteAccess.Username != "" ||
		a.cfg.RemoteAccess.SetupBootstrapped {
		a.remoteAccountsMu.Unlock()
		return "", fmt.Errorf("setup already completed")
	}
	a.cfg.RemoteAccess.AuthMode = "token"
	a.cfg.RemoteAccess.SetupBootstrapped = true
	err := config.Save(a.cfg)
	a.remoteAccountsMu.Unlock()
	if err != nil {
		return "", fmt.Errorf("save config: %w", err)
	}
	a.emitEvent("remote_auth:token_bootstrapped", deviceName)
	// CreateRemoteDevice takes its own, separate lock (remoteDevicesMu) — no
	// deadlock risk, and this call happens after remoteAccountsMu is already
	// released above.
	return a.CreateRemoteDevice(deviceName)
}

// ListAccounts returns every account's metadata (never the password hash
// — see AccountInfo). Returns interface{} for the same FullBridge
// decoupling reason as ListRemoteDevices.
func (a *App) ListAccounts() interface{} {
	a.remoteAccountsMu.Lock()
	defer a.remoteAccountsMu.Unlock()
	accounts := a.cfg.RemoteAccess.Accounts
	out := make([]AccountInfo, 0, len(accounts))
	for _, acc := range accounts {
		out = append(out, AccountInfo{ID: acc.ID, Username: acc.Username, Role: acc.Role, CreatedAt: acc.CreatedAt})
	}
	return out
}

// CreateAccount adds a new account. Callers (internal/webserver's
// admin-only handlers) are responsible for verifying the caller is
// actually an admin — this method itself performs no authorization check,
// matching CreateRemoteDevice's existing convention in this file.
func (a *App) CreateAccount(username, password, role string) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if password == "" {
		return fmt.Errorf("password is required")
	}
	if !validAccountRoles[role] {
		return fmt.Errorf("invalid role: %q", role)
	}
	hash, err := remoteauth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	a.remoteAccountsMu.Lock()
	if a.findAccount(username) != nil {
		a.remoteAccountsMu.Unlock()
		return fmt.Errorf("an account named %q already exists", username)
	}
	a.cfg.RemoteAccess.Accounts = append(a.cfg.RemoteAccess.Accounts, config.Account{
		ID:           remoteauth.GenerateDeviceID(),
		Username:     username,
		PasswordHash: hash,
		Role:         role,
		CreatedAt:    time.Now(),
	})
	err = config.Save(a.cfg)
	a.remoteAccountsMu.Unlock()
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	a.emitEvent("remote_auth:account_created", username)
	return nil
}

// DeleteAccount removes an account permanently. Refuses to remove the
// last remaining "admin"-role account — without this guard, an admin
// could lock every account (including themselves) out of every
// admin-only endpoint with no recovery path short of hand-editing
// config.yaml, since NeedsSetup()/the bootstrap endpoint only ever
// re-opens when Accounts *and* the legacy Username are both empty, not
// merely when no admin remains.
func (a *App) DeleteAccount(id string) error {
	a.remoteAccountsMu.Lock()
	accounts := a.cfg.RemoteAccess.Accounts
	idx := -1
	for i, acc := range accounts {
		if acc.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		a.remoteAccountsMu.Unlock()
		return fmt.Errorf("account not found: %s", id)
	}
	if accounts[idx].Role == "admin" {
		adminCount := 0
		for _, acc := range accounts {
			if acc.Role == "admin" {
				adminCount++
			}
		}
		if adminCount <= 1 {
			a.remoteAccountsMu.Unlock()
			return fmt.Errorf("cannot remove the last admin account")
		}
	}
	name := accounts[idx].Username
	a.cfg.RemoteAccess.Accounts = append(accounts[:idx], accounts[idx+1:]...)
	err := config.Save(a.cfg)
	a.remoteAccountsMu.Unlock()
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	a.emitEvent("remote_auth:account_deleted", name)
	return nil
}

// SessionSubject returns the username a currently-valid session token was
// issued for, re-checked against the live account list (same resolution
// as sessionSubjectRole — a deleted/renamed account invalidates its
// outstanding sessions immediately). ok=false for device tokens, expired
// or garbage credentials.
func (a *App) SessionSubject(token string) (string, bool) {
	subject, _, ok := a.sessionSubjectRole(token)
	return subject, ok
}

// ChangeAccountPassword updates an account's password hash. Self-service
// (the account the session was issued for — matched by id or the
// account's username) requires the current password; changing someone
// else's account requires an admin session. The password change does NOT
// invalidate outstanding sessions (JWT TTL 12h applies, matching
// ValidateRemoteSession's documented limitation).
func (a *App) ChangeAccountPassword(sessionToken, id, currentPassword, newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("new password is required")
	}
	subject, subjectRole, ok := a.sessionSubjectRole(sessionToken)
	if !ok {
		return fmt.Errorf("valid session token is required")
	}

	a.remoteAccountsMu.Lock()
	defer a.remoteAccountsMu.Unlock()
	if len(a.cfg.RemoteAccess.Accounts) == 0 {
		return fmt.Errorf("password change requires accounts")
	}
	var acc *config.Account
	for i := range a.cfg.RemoteAccess.Accounts {
		if a.cfg.RemoteAccess.Accounts[i].ID == id {
			acc = &a.cfg.RemoteAccess.Accounts[i]
			break
		}
	}
	if acc == nil {
		return fmt.Errorf("account not found: %s", id)
	}
	if id == subject || (acc.Username == subject && subjectRole != "admin") {
		ok, err := remoteauth.VerifyPassword(acc.PasswordHash, currentPassword)
		if err != nil || !ok {
			return fmt.Errorf("current password is incorrect")
		}
	} else if subjectRole != "admin" {
		return fmt.Errorf("only admins can change another account's password")
	}
	hash, err := remoteauth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	acc.PasswordHash = hash
	if err := config.Save(a.cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	a.emitEvent("remote_auth:password_changed", acc.Username)
	return nil
}
