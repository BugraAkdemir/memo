// SPDX-License-Identifier: AGPL-3.0-or-later

package remoteauth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// SessionTTL is how long a session token issued by a successful password
// login stays valid before the client must log in again. Short-lived by
// design (yapacam.md Faz 2: "kısa ömürlü... oturum token'ı") — there is
// deliberately no server-side revocation list or refresh-token flow yet;
// a compromised session token self-expires within this window regardless.
const SessionTTL = 12 * time.Hour

// ErrInvalidSessionToken covers every way ValidateSessionToken can fail
// (expired, wrong signature, malformed) — callers only need to distinguish
// "valid" from "not," never why, so a single sentinel keeps the auth gate's
// error handling from leaking which failure mode occurred.
var ErrInvalidSessionToken = errors.New("remoteauth: invalid or expired session token")

type sessionClaims struct {
	jwt.RegisteredClaims
}

// IssueSessionToken signs a new short-lived session token for subject
// (typically the configured remote-access username) using signingKey.
func IssueSessionToken(signingKey []byte, subject string) (string, error) {
	return issueSessionTokenAt(signingKey, subject, time.Now(), SessionTTL)
}

func issueSessionTokenAt(signingKey []byte, subject string, issuedAt time.Time, ttl time.Duration) (string, error) {
	claims := sessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(issuedAt.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

// ValidateSessionToken verifies tokenString's signature and expiry against
// signingKey and returns the subject it was issued for.
func ValidateSessionToken(signingKey []byte, tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &sessionClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return signingKey, nil
	})
	if err != nil || !token.Valid {
		return "", ErrInvalidSessionToken
	}
	claims, ok := token.Claims.(*sessionClaims)
	if !ok || claims.Subject == "" {
		return "", ErrInvalidSessionToken
	}
	return claims.Subject, nil
}

// LoadOrCreateSigningKey reads a 32-byte HMAC signing key from path, or
// generates and persists a new random one (0600) if the file doesn't exist
// yet. Mirrors internal/provider's defaultMachineKey generate/persist/reuse
// pattern, kept as a separate file/key rather than reusing machine.key so
// rotating the session-signing key (e.g. to force-invalidate every
// outstanding session) never has side effects on provider API key
// decryption, which machine.key is also used for.
func LoadOrCreateSigningKey(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil && len(data) == 32 {
		return data, nil
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create signing key dir: %w", err)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("persist signing key: %w", err)
	}
	return key, nil
}
