// SPDX-License-Identifier: AGPL-3.0-or-later

package main

// Claude subscription (claude-sub) stand-in.
//
// Memo's claude-sub provider talks to three Anthropic surfaces that a normal
// API key never touches: the hosted OAuth authorize page, the OAuth token
// endpoint, and the Messages API under an OAuth Bearer token, where an
// undocumented entitlement gate applies. Without a real Claude Pro/Max account
// none of that can be exercised by hand, so this file fakes all three, with the
// gate's KNOWN rules (see AGENTS.md "Claude subscription"):
//
//   - the Claude Code identity sentence must be the FIRST system block, or the
//     answer is a 429 rate_limit_error with NO anthropic-ratelimit-* headers and
//     no Retry-After — the shape that is indistinguishable from a quota wall
//   - temperature != 1 is refused with an opaque 400 whose message is "Error"
//   - the oauth beta flag must be present
//   - Haiku refuses adaptive thinking and the 1M-context beta, the way a model
//     without those capabilities does (a concrete 400, NOT the gate)
//
// The authorize page DISPLAYS "code#state" instead of redirecting, exactly like
// Anthropic's hosted callback page, so the paste-back step is tested with the
// string a real user would copy.
//
// Point Memo at it with, for example:
//
//	MEMO_CLAUDE_AUTH_URL=http://127.0.0.1:4959/oauth/authorize
//	MEMO_CLAUDE_TOKEN_URL=http://127.0.0.1:4959/v1/oauth/token
//	MEMO_CLAUDE_API_URL=http://127.0.0.1:4959
//
// A setup token (what `claude setup-token` prints) is any Bearer value starting
// with "fake-setup-" — use it as MEMO_CLAUDE_TOKEN to test the browserless path.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/oauth2"
)

var (
	subExpires     = flag.Int("sub-expires", 3600, "claude-sub: access token lifetime in seconds")
	subRotate      = flag.Bool("sub-rotate", true, "claude-sub: rotate (and invalidate) the refresh token on every refresh")
	subGatePremium = flag.Bool("sub-gate-premium", false, "claude-sub: refuse tool-carrying turns on non-Haiku models with the headerless 429 (an account not entitled to agent traffic)")
)

const subIdentity = "You are Claude Code, Anthropic's official CLI for Claude."

type subPending struct {
	challenge, redirect, state string
}

var (
	subMu       sync.Mutex
	subCodes    = map[string]subPending{} // authorization code -> PKCE data
	subAccess   = map[string]bool{}       // live access tokens
	subRefresh  = map[string]bool{}       // live refresh tokens
	subCounters int
)

func subRand() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// handleSubAuthorize is the stand-in for claude.ai/oauth/authorize: it checks
// the parameters Memo must send and then shows the code the way Anthropic's
// hosted callback page does, as "code#state".
func handleSubAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var v []string
	for _, k := range []string{"client_id", "redirect_uri", "code_challenge", "state"} {
		if q.Get(k) == "" {
			v = append(v, "missing "+k)
		}
	}
	if q.Get("code") != "true" {
		v = append(v, "code=true missing: the hosted page would show no code")
	}
	if q.Get("response_type") != "code" || q.Get("code_challenge_method") != "S256" {
		v = append(v, "response_type=code and code_challenge_method=S256 are required")
	}
	if len(v) > 0 {
		record("/oauth/authorize", nil, v, 400)
		http.Error(w, strings.Join(v, "; "), http.StatusBadRequest)
		return
	}
	code := "fakecode-" + subRand()
	subMu.Lock()
	subCodes[code] = subPending{challenge: q.Get("code_challenge"), redirect: q.Get("redirect_uri"), state: q.Get("state")}
	subMu.Unlock()
	record("/oauth/authorize", map[string]any{"redirect_uri": q.Get("redirect_uri")}, nil, 200)

	shown := code + "#" + q.Get("state")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><title>Fake Anthropic authorize</title>
<body style="font-family:sans-serif;max-width:40em;margin:3em auto">
<h1>Authentication Code</h1>
<p>Paste this into the application you are signing in to:</p>
<pre id="code" style="padding:1em;background:#eee;word-break:break-all;white-space:pre-wrap">%s</pre>
</body>`, html.EscapeString(shown))
}

// handleSubToken is the stand-in for the OAuth token endpoint: the
// authorization_code grant (PKCE-checked) and the refresh_token grant.
func handleSubToken(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]string
	if err := json.Unmarshal(raw, &body); err != nil {
		record("/v1/oauth/token", nil, []string{"token request is not a JSON object: " + err.Error()}, 400)
		oauthErr(w, 400, "invalid_request", "body must be JSON")
		return
	}
	logged := map[string]any{"grant_type": body["grant_type"]}
	switch body["grant_type"] {
	case "authorization_code":
		subMu.Lock()
		p, ok := subCodes[body["code"]]
		delete(subCodes, body["code"]) // single use, like the real thing
		subMu.Unlock()
		var v []string
		switch {
		case !ok:
			v = append(v, fmt.Sprintf("unknown or reused authorization code %q", clip(body["code"], 60)))
		case oauth2.S256ChallengeFromVerifier(body["code_verifier"]) != p.challenge:
			v = append(v, "code_verifier does not match code_challenge")
		case body["redirect_uri"] != p.redirect:
			v = append(v, "redirect_uri differs from the one used to authorize")
		case body["state"] != p.state:
			v = append(v, "state differs from the one used to authorize")
		}
		if len(v) > 0 {
			record("/v1/oauth/token", logged, v, 400)
			oauthErr(w, 400, "invalid_grant", strings.Join(v, "; "))
			return
		}
	case "refresh_token":
		subMu.Lock()
		ok := subRefresh[body["refresh_token"]]
		if ok && *subRotate {
			delete(subRefresh, body["refresh_token"])
		}
		subMu.Unlock()
		if !ok {
			record("/v1/oauth/token", logged, []string{"refresh token unknown or already rotated away"}, 400)
			oauthErr(w, 400, "invalid_grant", "Refresh token not found or invalid")
			return
		}
	default:
		record("/v1/oauth/token", logged, []string{"unsupported grant_type"}, 400)
		oauthErr(w, 400, "unsupported_grant_type", "unsupported grant_type")
		return
	}

	subMu.Lock()
	subCounters++
	access := fmt.Sprintf("fake-oat-%d-%s", subCounters, subRand())
	subAccess[access] = true
	refresh := body["refresh_token"]
	if body["grant_type"] == "authorization_code" || *subRotate {
		refresh = fmt.Sprintf("fake-ort-%d-%s", subCounters, subRand())
	}
	subRefresh[refresh] = true
	subMu.Unlock()
	record("/v1/oauth/token", logged, nil, 200)
	writeJSON(w, map[string]any{
		"access_token":  access,
		"refresh_token": refresh,
		"token_type":    "Bearer",
		"expires_in":    *subExpires,
		"scope":         "user:inference user:profile",
	})
}

func oauthErr(w http.ResponseWriter, code int, kind, desc string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	b, _ := json.Marshal(map[string]any{"error": kind, "error_description": desc})
	w.Write(b)
}

// subBearer returns the OAuth Bearer token on a request, "" for API-key traffic.
func subBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

// subAuthOK reports whether a Bearer token is one this fake issued (or a setup
// token), writing a 401 when it is not.
func subAuthOK(w http.ResponseWriter, r *http.Request, route string, body map[string]any) bool {
	tok := subBearer(r)
	subMu.Lock()
	live := subAccess[tok]
	subMu.Unlock()
	if live || strings.HasPrefix(tok, "fake-setup-") {
		return true
	}
	record(route, body, []string{"Bearer token is not a live access token (expired, rotated away, or never issued)"}, 401)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(401)
	io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"OAuth token has expired."}}`)
	return false
}

// subGate applies the subscription endpoint's rules to an OAuth-authenticated
// /v1/messages request. Reports true when it already wrote the response.
func subGate(w http.ResponseWriter, r *http.Request, body map[string]any) bool {
	if subBearer(r) == "" {
		return false // API-key traffic: not ours
	}
	if r.Header.Get("x-api-key") != "" {
		record("/v1/messages", body, []string{"x-api-key sent alongside an OAuth Bearer token"}, 400)
		anError(w, 400, "x-api-key and Authorization both set")
		return true
	}
	if !subAuthOK(w, r, "/v1/messages", body) {
		return true
	}
	betas := r.Header.Get("anthropic-beta")
	if !strings.Contains(betas, "oauth-2025-04-20") {
		record("/v1/messages", body, []string{"anthropic-beta lacks oauth-2025-04-20"}, 401)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"OAuth authentication is currently not supported."}}`)
		return true
	}
	model, _ := body["model"].(string)
	haiku := strings.Contains(model, "haiku")

	// The gate: identity sentence as the first system block, by position.
	if first := firstSystemText(body["system"]); !strings.HasPrefix(first, subIdentity) {
		record("/v1/messages", body, []string{"first system block is not the Claude Code identity sentence (gate: headerless 429)"}, 429)
		gate429(w)
		return true
	}
	if t, ok := body["temperature"].(float64); ok && t != 1 {
		record("/v1/messages", body, []string{fmt.Sprintf("temperature %v != 1 (gate: opaque 400)", t)}, 400)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":"Error"}}`)
		return true
	}
	if *subGatePremium && !haiku && body["tools"] != nil {
		record("/v1/messages", body, []string{"tool-carrying turn on a premium model (gate: headerless 429)"}, 429)
		gate429(w)
		return true
	}
	// Model capability refusals: concrete, named, NOT the gate.
	if haiku && body["thinking"] != nil {
		record("/v1/messages", body, []string{"adaptive thinking on a model that lacks it"}, 400)
		anError(w, 400, "thinking.type: adaptive thinking is not supported on this model")
		return true
	}
	if haiku && strings.Contains(betas, "context-1m") {
		record("/v1/messages", body, []string{"1M-context beta on a model without it"}, 400)
		anError(w, 400, "The long context beta is not supported for this model.")
		return true
	}
	return false
}

func gate429(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(429)
	io.WriteString(w, `{"type":"error","error":{"type":"rate_limit_error","message":"Error"}}`)
}

func firstSystemText(sys any) string {
	switch s := sys.(type) {
	case string:
		return s
	case []any:
		if len(s) == 0 {
			return ""
		}
		if b, ok := s[0].(map[string]any); ok {
			t, _ := b["text"].(string)
			return t
		}
	}
	return ""
}
