// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// signContext is prepended to the bytes that are signed. It binds a signature
// to this protocol: a signature made for something else with the same key
// (a future artifact type, say) can never be replayed as a manifest.
const signContext = "memo-update-v1\n"

// ErrBadSignature is returned for any signature that does not verify. The
// reason is deliberately not refined — a caller has nothing useful to do with
// "wrong key" versus "tampered body" except refuse.
var ErrBadSignature = errors.New("updater: signature does not match")

// GenerateKey makes a new signing key pair.
func GenerateKey() (pub ed25519.PublicKey, priv ed25519.PrivateKey, err error) {
	return ed25519.GenerateKey(rand.Reader)
}

// Sign returns the base64 detached signature of body.
func Sign(priv ed25519.PrivateKey, body []byte) (string, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return "", errors.New("updater: bad private key")
	}
	sig := ed25519.Sign(priv, append([]byte(signContext), body...))
	return base64.StdEncoding.EncodeToString(sig), nil
}

// Verify checks sigB64 (as written by Sign) over body against pub.
func Verify(pub ed25519.PublicKey, body []byte, sigB64 string) error {
	if len(pub) != ed25519.PublicKeySize {
		return errors.New("updater: no usable public key")
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sigB64))
	if err != nil || len(sig) != ed25519.SignatureSize {
		return ErrBadSignature
	}
	if !ed25519.Verify(pub, append([]byte(signContext), body...), sig) {
		return ErrBadSignature
	}
	return nil
}

// EncodeKey / DecodeKey use unpadded-free standard base64, one line, so a key
// fits a CI secret and a Go string constant.
func EncodePublic(pub ed25519.PublicKey) string { return base64.StdEncoding.EncodeToString(pub) }

func EncodePrivate(priv ed25519.PrivateKey) string { return base64.StdEncoding.EncodeToString(priv) }

func DecodePublic(s string) (ed25519.PublicKey, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("updater: not an ed25519 public key")
	}
	return ed25519.PublicKey(b), nil
}

func DecodePrivate(s string) (ed25519.PrivateKey, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(b) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("updater: not an ed25519 private key")
	}
	return ed25519.PrivateKey(b), nil
}
