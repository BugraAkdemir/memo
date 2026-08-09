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
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at,omitempty"`
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
		out = append(out, RemoteDeviceInfo{
			ID: d.ID, Name: d.Name, CreatedAt: d.CreatedAt, LastSeenAt: d.LastSeenAt,
		})
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

// LoginRemotePassword validates username/password against the configured
// remote-access credentials and, on success, issues a signed session
// token. remoteAddr should be the raw connecting IP (used only to key the
// brute-force limiter — never persisted).
func (a *App) LoginRemotePassword(remoteAddr, username, password string) (string, error) {
	mode := a.cfg.RemoteAccess.AuthMode
	if mode != "password" && mode != "token_password" {
		return "", fmt.Errorf("password login is not enabled (auth mode is %q)", mode)
	}

	limiter := a.authLimiter()
	key := remoteAddr + "|" + username
	if allowed, retryAfter := limiter.Allowed(key); !allowed {
		return "", &remoteLoginError{lockedFor: retryAfter}
	}

	valid := username != "" && username == a.cfg.RemoteAccess.Username
	if valid {
		ok, err := remoteauth.VerifyPassword(a.cfg.RemoteAccess.PasswordHash, password)
		valid = ok && err == nil
	}
	if !valid {
		lockout := limiter.RecordFailure(key)
		a.emitEvent("remote_auth:login_failed", username)
		return "", &remoteLoginError{lockedFor: lockout}
	}
	limiter.RecordSuccess(key)

	signingKey, err := a.sessionSigningKey()
	if err != nil {
		return "", fmt.Errorf("session signing key: %w", err)
	}
	token, err := remoteauth.IssueSessionToken(signingKey, username)
	if err != nil {
		return "", fmt.Errorf("issue session token: %w", err)
	}
	a.emitEvent("remote_auth:login_success", username)
	return token, nil
}

// ValidateRemoteSession reports whether token is a currently-valid session
// token issued for the currently-configured username. Checking the
// username (not just signature+expiry) means renaming the account
// immediately invalidates every session issued under the old name. A
// *password* change alone does not rotate the signing key or otherwise
// invalidate outstanding sessions — known limitation, not a design goal:
// a compromised session token still self-expires within SessionTTL (12h)
// regardless, same as noted on remoteauth.SessionTTL.
func (a *App) ValidateRemoteSession(token string) bool {
	signingKey, err := a.sessionSigningKey()
	if err != nil {
		return false
	}
	subject, err := remoteauth.ValidateSessionToken(signingKey, token)
	if err != nil {
		return false
	}
	return subject != "" && subject == a.cfg.RemoteAccess.Username
}
