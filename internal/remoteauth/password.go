// SPDX-License-Identifier: AGPL-3.0-or-later

// Package remoteauth implements the authentication primitives for Memo's
// self-hosted remote access (Faz 2 of yapacam.md): argon2id password
// hashing, per-device token generation/hashing, signed session tokens, and
// login brute-force protection. Deliberately independent of internal/config
// and internal/app — this package only deals in plain values (strings,
// bytes, durations) so it stays unit-testable without any app/webserver
// wiring, matching how internal/tunnel and internal/ngrok are scoped.
package remoteauth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters. Chosen from OWASP's published "minimum recommended"
// profile (m=19MiB, t=2, p=1) rather than the heavier alternatives — Memo's
// stated self-hosting target includes a Raspberry Pi, and login is an
// infrequent, low-throughput operation (nothing here is hashed in a hot
// loop), so there's no reason to spend a Pi's limited RAM/CPU on a profile
// sized for a high-traffic multi-tenant service.
const (
	argon2Time    = 2
	argon2Memory  = 19 * 1024 // KiB
	argon2Threads = 1
	argon2KeyLen  = 32
	saltLen       = 16
)

// ErrInvalidHash is returned by VerifyPassword when the stored hash isn't in
// the format HashPassword produces (corrupted config, or hand-edited YAML).
var ErrInvalidHash = errors.New("remoteauth: invalid password hash format")

// HashPassword returns an encoded argon2id hash of plain, in the standard
// PHC-like string format ($argon2id$v=19$m=...,t=...,p=...$salt$hash) —
// self-describing, so a future parameter change doesn't break verification
// of hashes already persisted in an existing user's config.yaml.
func HashPassword(plain string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	hash := argon2.IDKey([]byte(plain), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argon2Memory, argon2Time, argon2Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, nil
}

// VerifyPassword checks plain against an encoded hash produced by
// HashPassword. Returns (false, nil) for a wrong password, (false, err) for
// a malformed/unparseable hash — callers should treat both as "auth failed"
// but may want to log the latter separately (it indicates config corruption,
// not a bad guess).
func VerifyPassword(encoded, plain string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, ErrInvalidHash
	}
	var mem uint32
	var t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &t, &p); err != nil {
		return false, ErrInvalidHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrInvalidHash
	}
	got := argon2.IDKey([]byte(plain), salt, t, mem, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
