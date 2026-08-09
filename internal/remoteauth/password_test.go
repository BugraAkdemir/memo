// SPDX-License-Identifier: AGPL-3.0-or-later

package remoteauth

import "testing"

func TestHashPassword_RoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := VerifyPassword(hash, "correct horse battery staple")
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Fatal("expected correct password to verify")
	}
}

func TestVerifyPassword_WrongPasswordFails(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := VerifyPassword(hash, "wrong guess")
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if ok {
		t.Fatal("expected wrong password to fail verification")
	}
}

func TestHashPassword_SamePasswordDifferentHashes(t *testing.T) {
	h1, err := HashPassword("same password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	h2, err := HashPassword("same password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if h1 == h2 {
		t.Fatal("expected random salts to produce different encoded hashes for the same password")
	}
}

func TestVerifyPassword_MalformedHashReturnsError(t *testing.T) {
	cases := []string{
		"",
		"not-a-hash-at-all",
		"$argon2id$v=19$m=19456,t=2,p=1$onlyonefield",
		"$bcrypt$v=1$abc$def",
	}
	for _, c := range cases {
		ok, err := VerifyPassword(c, "anything")
		if ok {
			t.Errorf("case %q: expected verification to fail", c)
		}
		if err == nil {
			t.Errorf("case %q: expected an error for malformed hash", c)
		}
	}
}

func TestVerifyPassword_EmptyPasswordAgainstEmptyHashIsNotSpecialCased(t *testing.T) {
	// An empty stored hash (password mode never configured) must never
	// authenticate, even against an empty candidate password.
	ok, err := VerifyPassword("", "")
	if ok || err == nil {
		t.Fatal("expected empty stored hash to fail closed, not authenticate")
	}
}
