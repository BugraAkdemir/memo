// SPDX-License-Identifier: AGPL-3.0-or-later

package geminisub

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
)

// geminiCLICredsPath is where the official gemini-cli caches its OAuth token
// after a `gemini` login. A var so tests can point it elsewhere.
var geminiCLICredsPath = defaultGeminiCLICredsPath()

func defaultGeminiCLICredsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".gemini", "oauth_creds.json")
}

// geminiCLICreds is the shape gemini-cli writes (Google's oauth2 credentials
// JSON): expiry is epoch milliseconds under "expiry_date".
type geminiCLICreds struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiryDate   int64  `json:"expiry_date"`
}

// loadGeminiCLIToken reads ~/.gemini/oauth_creds.json if present and returns
// it as an oauth2.Token. This is the same idea as claude-code-proxy falling
// back to ~/.claude/.credentials.json — if the user already logged in with
// the `gemini` CLI, reuse that token (it was obtained with the very same
// public client id, so our own refresh works against it). Returns
// (nil, false) when the file is missing, unreadable, malformed, or carries
// no refresh token.
func loadGeminiCLIToken() (*oauth2.Token, bool) {
	if geminiCLICredsPath == "" {
		return nil, false
	}
	data, err := os.ReadFile(geminiCLICredsPath)
	if err != nil {
		return nil, false
	}
	var c geminiCLICreds
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, false
	}
	if c.RefreshToken == "" && c.AccessToken == "" {
		return nil, false
	}
	tt := c.TokenType
	if tt == "" {
		tt = "Bearer"
	}
	tok := &oauth2.Token{
		AccessToken:  c.AccessToken,
		RefreshToken: c.RefreshToken,
		TokenType:    tt,
	}
	if c.ExpiryDate > 0 {
		tok.Expiry = time.UnixMilli(c.ExpiryDate)
	}
	return tok, true
}
