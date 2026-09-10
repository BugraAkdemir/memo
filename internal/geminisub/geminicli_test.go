// SPDX-License-Identifier: AGPL-3.0-or-later

package geminisub

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestLoadGeminiCLIToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "oauth_creds.json")
	prev := geminiCLICredsPath
	geminiCLICredsPath = path
	defer func() { geminiCLICredsPath = prev }()

	// Missing file -> (nil, false).
	if tok, ok := loadGeminiCLIToken(); ok || tok != nil {
		t.Fatalf("missing file: got (%v, %v), want (nil, false)", tok, ok)
	}

	exp := time.Now().Add(30 * time.Minute)
	body := `{"access_token":"ya29.aaa","refresh_token":"1//rrr","token_type":"Bearer","scope":"x","expiry_date":` +
		itoa(exp.UnixMilli()) + `}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	tok, ok := loadGeminiCLIToken()
	if !ok {
		t.Fatal("valid creds file: ok=false")
	}
	if tok.AccessToken != "ya29.aaa" || tok.RefreshToken != "1//rrr" || tok.TokenType != "Bearer" {
		t.Errorf("token = %+v", tok)
	}
	if d := tok.Expiry.Sub(exp); d > time.Second || d < -time.Second {
		t.Errorf("expiry = %v, want ~%v", tok.Expiry, exp)
	}

	// Malformed -> (nil, false), no panic.
	_ = os.WriteFile(path, []byte("not json"), 0o600)
	if tok, ok := loadGeminiCLIToken(); ok || tok != nil {
		t.Errorf("malformed file: got (%v, %v), want (nil, false)", tok, ok)
	}

	// No tokens at all -> (nil, false).
	_ = os.WriteFile(path, []byte(`{"scope":"x"}`), 0o600)
	if tok, ok := loadGeminiCLIToken(); ok || tok != nil {
		t.Errorf("empty creds: got (%v, %v), want (nil, false)", tok, ok)
	}
}



// TestNewManagerAdoptsGeminiCLIToken: with no store of our own but a
// gemini-cli creds file present, newManager seeds from it.
func TestNewManagerAdoptsGeminiCLIToken(t *testing.T) {
	dir := t.TempDir()
	credsPath := filepath.Join(dir, "oauth_creds.json")
	prev := geminiCLICredsPath
	geminiCLICredsPath = credsPath
	defer func() { geminiCLICredsPath = prev }()

	_ = os.WriteFile(credsPath, []byte(
		`{"access_token":"ya29.seed","refresh_token":"1//seed","expiry_date":`+
			itoa(time.Now().Add(time.Hour).UnixMilli())+`}`), 0o600)

	m := newManager(newTokenStoreAt(filepath.Join(dir, "token.enc"), testKey()))
	if !m.Connected() {
		t.Fatal("newManager did not adopt the gemini-cli token")
	}
}
