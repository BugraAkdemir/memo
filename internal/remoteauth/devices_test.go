// SPDX-License-Identifier: AGPL-3.0-or-later

package remoteauth

import (
	"strings"
	"testing"
)

func TestGenerateDeviceToken_FormatAndUniqueness(t *testing.T) {
	a := GenerateDeviceToken()
	b := GenerateDeviceToken()
	if !strings.HasPrefix(a, "memo-") {
		t.Fatalf("expected memo- prefix, got %q", a)
	}
	if a == b {
		t.Fatal("expected two calls to produce different tokens")
	}
}

func TestGenerateDeviceID_Uniqueness(t *testing.T) {
	a := GenerateDeviceID()
	b := GenerateDeviceID()
	if a == "" || a == b {
		t.Fatalf("expected non-empty, unique device IDs, got %q and %q", a, b)
	}
}

func TestVerifyTokenHash_RoundTrip(t *testing.T) {
	token := GenerateDeviceToken()
	hash := HashToken(token)
	if !VerifyTokenHash(hash, token) {
		t.Fatal("expected matching token to verify against its own hash")
	}
}

func TestVerifyTokenHash_WrongTokenFails(t *testing.T) {
	hash := HashToken(GenerateDeviceToken())
	if VerifyTokenHash(hash, "memo-attackerguess00000000") {
		t.Fatal("expected mismatched token to fail")
	}
}

func TestVerifyTokenHash_EmptyInputsFailClosed(t *testing.T) {
	if VerifyTokenHash("", "") {
		t.Fatal("expected empty hash and empty token to never verify")
	}
	if VerifyTokenHash(HashToken("memo-real"), "") {
		t.Fatal("expected empty candidate token to fail against a real hash")
	}
	if VerifyTokenHash("", "memo-real") {
		t.Fatal("expected empty stored hash to fail against a real candidate")
	}
}
