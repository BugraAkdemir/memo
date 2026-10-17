// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import "crypto/ed25519"

// publicKeyB64 is the ed25519 public key every manifest must be signed with. The
// matching private key lives only in CI (the MEMO_UPDATE_SIGNING_KEY secret) and
// in the maintainer's release tooling; it is never in this repository.
//
// Rotating it means shipping a release that carries the new key signed by the
// old one — there is deliberately no remote way to change it.
const publicKeyB64 = "7RC5TBa92+dh2CPyuDUVUyLqpVq4Er/rLSaMJb4tcQI="

// PublicKey returns the compiled-in key, or nil if the build has none. A nil key
// makes Verify refuse everything, so a build without a key can never apply an
// update (it fails closed).
func PublicKey() ed25519.PublicKey {
	if publicKeyB64 == "" {
		return nil
	}
	k, err := DecodePublic(publicKeyB64)
	if err != nil {
		return nil
	}
	return k
}
