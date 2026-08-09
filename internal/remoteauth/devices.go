// SPDX-License-Identifier: AGPL-3.0-or-later

package remoteauth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

// GenerateDeviceToken returns a new random per-device access token, in the
// same "memo-<24 hex chars>" shape the single shared token used before Faz 2
// (internal/app/remote.go's generateToken) — existing clients that already
// parse/store a token in this format need no changes.
func GenerateDeviceToken() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "memo-" + hex.EncodeToString(b)
}

// GenerateDeviceID returns a short random identifier for a device record —
// distinct from the token itself so a device can be listed/named/revoked in
// the UI without ever displaying (or re-deriving) its secret token.
func GenerateDeviceID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// HashToken returns the persisted-at-rest form of a device token. Unlike
// HashPassword, this deliberately does NOT use argon2id: the input here is
// already a high-entropy, uniformly random 96-bit value (never a
// human-chosen, guessable string), so slow/memory-hard hashing defends
// against nothing an attacker could actually exploit and would only add
// needless CPU cost to every authenticated request. A single fast SHA-256
// pass is standard practice for hashing pre-existing high-entropy secrets
// (the same reasoning API-key-style credentials at GitHub/Stripe etc. use).
func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// VerifyTokenHash reports whether plain hashes to want, in constant time.
func VerifyTokenHash(want, plain string) bool {
	if want == "" || plain == "" {
		return false
	}
	got := HashToken(plain)
	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}
