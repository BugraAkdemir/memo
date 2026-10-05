// SPDX-License-Identifier: AGPL-3.0-or-later

package claudesub

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"memo/internal/fileutil"
	"memo/internal/logx"

	"golang.org/x/oauth2"
)

// Adoption — signing in without a browser.
//
// The whole point of using Anthropic's public Claude Code OAuth client is that
// the user may ALREADY be signed into it. If they are, "Connect" should not
// send them through a browser and a copy-paste round trip at all; it should
// find the login that is already there.
//
// Sources, in priority order:
//
//  1. MEMO_CLAUDE_TOKEN / CLAUDE_CODE_OAUTH_TOKEN — a long-lived token from
//     `claude setup-token`. Officially supported by Claude Code, and the
//     documented way to use a subscription from a script or CI.
//  2. ~/.claude/.credentials.json — the Claude Code CLI's own store on Linux
//     and Windows. This is exactly what claude-code-proxy reads.
//  3. The macOS Keychain entry "Claude Code-credentials", which is where the
//     same CLI keeps them on macOS instead of a file.
//
// NOT a source: Claude Desktop's ~/.config/Claude/config.json. It carries
// oauth:tokenCache / oauth:tokenCacheV2, but those are base64 wrappers around
// Electron's safeStorage ("v11"-prefixed) ciphertext keyed by the OS keyring.
// Reading them means reverse-engineering a platform keychain, which is fragile,
// varies per OS, and would break the first time Electron changes the scheme —
// for no gain, since Claude Desktop and Claude Code are the same account. If
// someone is signed into Desktop, signing into Claude Code once covers this
// feature too, and the UI says so.

// envTokenVars are checked in order; the first non-empty wins.
var envTokenVars = []string{
	"MEMO_CLAUDE_TOKEN",
	"CLAUDE_CODE_OAUTH_TOKEN",
}

// claudeCredentialsPath is Claude Code's credential file on Linux/Windows.
func claudeCredentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", ".credentials.json")
}

// keychainService is the macOS Keychain service name Claude Code registers.
const keychainService = "Claude Code-credentials"

// cliCredentials is the shape of ~/.claude/.credentials.json.
type cliCredentials struct {
	ClaudeAiOauth *struct {
		AccessToken  string   `json:"accessToken"`
		RefreshToken string   `json:"refreshToken"`
		ExpiresAt    int64    `json:"expiresAt"`
		Scopes       []string `json:"scopes"`
	} `json:"claudeAiOauth"`
}

// AdoptResult says where a token came from, so the UI can tell the user what
// happened instead of a bare "connected".
type AdoptResult struct {
	Token *oauth2.Token
	// Source is SourceEnv, SourceClaudeCodeFile or SourceMacKeychain — empty
	// means nothing was found.
	Source string
	// ExpiresAt is the CLI-reported expiry in Unix millis, zero when unknown.
	ExpiresAt int64
}

// AdoptLocal looks for a usable Claude Code login on this machine. It never
// performs I/O beyond reading small files and (on macOS) one keychain query,
// never writes anything, and returns a zero result rather than an error when
// there is simply nothing to adopt — "not signed in" is the common case, not a
// failure.
func AdoptLocal() AdoptResult {
	if tok := fromEnv(); tok != nil {
		// "env", not the variable's name: the UI and config document the
		// source as one of a fixed set of values, and the frontend's
		// adoptedLocally check never matched "MEMO_CLAUDE_TOKEN".
		return AdoptResult{Token: tok, Source: SourceEnv}
	}
	if tok, exp := fromClaudeCodeFile(); tok != nil {
		return AdoptResult{Token: tok, Source: SourceClaudeCodeFile, ExpiresAt: exp}
	}
	if tok := fromMacKeychain(); tok != nil {
		return AdoptResult{Token: tok, Source: SourceMacKeychain}
	}
	return AdoptResult{}
}

// fromEnv reads a long-lived setup token. These have no refresh token — that
// is the point of setup-token — so the token is stored with a far-future
// expiry and, when it does eventually stop working, the user reconnects.
func fromEnv() *oauth2.Token {
	for _, name := range envTokenVars {
		v := strings.TrimSpace(os.Getenv(name))
		if v == "" {
			continue
		}
		return &oauth2.Token{
			AccessToken: v,
			TokenType:   "Bearer",
			// setup-token tokens are documented as long-lived. One year is well
			// past any realistic session; a wrong guess here costs one
			// reconnect, never a wrong answer.
			Expiry: time.Now().Add(365 * 24 * time.Hour),
		}
	}
	return nil
}

// fromClaudeCodeFile reads ~/.claude/.credentials.json.
func fromClaudeCodeFile() (*oauth2.Token, int64) {
	path := claudeCredentialsPath()
	if path == "" {
		return nil, 0
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0
	}
	var c cliCredentials
	if err := json.Unmarshal(raw, &c); err != nil {
		logx.Printf("claudesub: %s did not parse, ignoring it: %v", path, err)
		return nil, 0
	}
	if c.ClaudeAiOauth == nil || c.ClaudeAiOauth.AccessToken == "" {
		return nil, 0
	}
	t := &oauth2.Token{
		AccessToken:  c.ClaudeAiOauth.AccessToken,
		RefreshToken: c.ClaudeAiOauth.RefreshToken,
		TokenType:    "Bearer",
	}
	if c.ClaudeAiOauth.ExpiresAt > 0 {
		t.Expiry = time.UnixMilli(c.ClaudeAiOauth.ExpiresAt)
	}
	return t, c.ClaudeAiOauth.ExpiresAt
}

// fromMacKeychain shells out to `security`, which is the only supported way to
// read a Keychain item without linking Security.framework. It is also what
// claude-code-proxy's macOS instructions tell users to do by hand, so the
// mechanism is already the community's answer to this exact problem.
//
// Costs a subprocess, which is why it is last and macOS-only: on Linux and
// Windows the file above covers it, and this code never runs there at all.
func fromMacKeychain() *oauth2.Token {
	if runtime.GOOS != "darwin" {
		return nil
	}
	// -w prints only the secret. A missing entry exits non-zero with nothing on
	// stdout, which is the normal "not signed in" path.
	out, err := exec.Command("security", "find-generic-password", "-s", keychainService, "-w").Output()
	if err != nil {
		return nil
	}
	body := strings.TrimSpace(string(out))
	if body == "" {
		return nil
	}
	// The Keychain item holds the same JSON document as the file store.
	var c cliCredentials
	if err := json.Unmarshal([]byte(body), &c); err != nil || c.ClaudeAiOauth == nil || c.ClaudeAiOauth.AccessToken == "" {
		return nil
	}
	t := &oauth2.Token{
		AccessToken:  c.ClaudeAiOauth.AccessToken,
		RefreshToken: c.ClaudeAiOauth.RefreshToken,
		TokenType:    "Bearer",
	}
	if c.ClaudeAiOauth.ExpiresAt > 0 {
		t.Expiry = time.UnixMilli(c.ClaudeAiOauth.ExpiresAt)
	}
	return t
}

// freshLocalLogin re-reads the store a shared token was adopted from and
// returns its token when that one is still valid and differs from current —
// i.e. when Claude Code itself has refreshed in the meantime.
//
// This is the common way a shared login goes stale. Claude Code and Memo hold
// the same refresh token; whichever refreshes first gets a new one and, if
// Anthropic rotates, kills the other's. Reading Claude Code's store before
// spending our own refresh token (and again after a refused refresh) costs one
// small file read and turns "sign-in expired — reconnect" into nothing at all.
// Only for sources that ARE a shared store; an env token has nothing to re-read.
func freshLocalLogin(source string, current *oauth2.Token) *oauth2.Token {
	var t *oauth2.Token
	switch source {
	case SourceClaudeCodeFile:
		t, _ = fromClaudeCodeFile()
	case SourceMacKeychain:
		t = fromMacKeychain()
	default:
		return nil
	}
	if t == nil || !t.Valid() {
		return nil
	}
	if current != nil && t.AccessToken == current.AccessToken {
		return nil
	}
	return t
}

// writeBackRefreshed hands a refreshed login back to the user's own Claude
// Code install. The caller only does this for a token adopted FROM that file
// (Source == SourceClaudeCodeFile) — a token from our own browser flow is a
// different session and must never overwrite the CLI's.
//
// Anthropic does not document whether a refresh rotates the refresh token.
// claude-code-proxy handles the rotating case (`response.refresh_token ||
// tokens.refresh_token`), and this does too, because the failure is silent and
// bad in a specific way: if Anthropic DOES rotate and the file keeps the old
// one, the user's next `claude` invocation fails to refresh and they blame
// Claude Code for a change Memo made. The access token and its expiry are
// written alongside it so the file never pairs a new refresh token with an
// access token from the previous generation.
//
// Only those three keys of the claudeAiOauth subtree are rewritten; every other
// key in the file is preserved.
func writeBackRefreshed(t *oauth2.Token) {
	if t == nil || t.RefreshToken == "" {
		return
	}
	path := claudeCredentialsPath()
	if path == "" {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return // the token did not come from here
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return
	}
	oauthRaw, ok := doc["claudeAiOauth"]
	if !ok {
		return
	}
	var oauth map[string]json.RawMessage
	if err := json.Unmarshal(oauthRaw, &oauth); err != nil {
		return
	}
	// Nothing to do if the file already holds this token.
	if cur, ok := oauth["refreshToken"]; ok && string(cur) == mustJSONString(t.RefreshToken) {
		return
	}
	oauth["refreshToken"] = json.RawMessage(mustJSONString(t.RefreshToken))
	oauth["accessToken"] = json.RawMessage(mustJSONString(t.AccessToken))
	if !t.Expiry.IsZero() {
		oauth["expiresAt"] = json.RawMessage(fmt.Sprint(t.Expiry.UnixMilli()))
	}
	newOAuth, err := json.Marshal(oauth)
	if err != nil {
		return
	}
	doc["claudeAiOauth"] = newOAuth
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return
	}
	if err := fileutil.AtomicWrite(path, out, 0600); err != nil {
		logx.Printf("claudesub: could not update %s with a rotated refresh token: %v", path, err)
		return
	}
	logx.Printf("claudesub: Anthropic rotated the refresh token; updated %s so Claude Code keeps working", path)
}

func mustJSONString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return string(b)
}
