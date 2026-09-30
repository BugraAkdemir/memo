package claudesub

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"memo/internal/provider"

	"golang.org/x/oauth2"
)

// newTestManager builds a Manager on a temp token file with no adoption, and
// points the package-level endpoints at t.
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m := newManager(newTokenStoreAt(filepath.Join(t.TempDir(), "token.enc"), []byte("0123456789abcdef0123456789abcdef")))
	m.flow = nil
	return m
}

// withEndpoints redirects the OAuth endpoints for one test.
func withEndpoints(t *testing.T, authorize, token string) {
	t.Helper()
	oldA, oldT := authorizeEndpoint, tokenEndpoint
	authorizeEndpoint, tokenEndpoint = authorize, token
	t.Cleanup(func() { authorizeEndpoint, tokenEndpoint = oldA, oldT })
}

// tokenServer serves a token endpoint that answers with the given body.
func tokenServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestStartAuth_AuthorizeURLCarriesEverythingAnthropicNeeds(t *testing.T) {
	withEndpoints(t, "https://claude.test/authorize", "https://claude.test/token")
	m := newTestManager(t)

	authURL, state, redirect, err := m.StartAuth(0)
	if err != nil {
		t.Fatalf("StartAuth: %v", err)
	}
	if state == "" {
		t.Error("no state issued")
	}
	if !strings.HasPrefix(redirect, "https://") {
		t.Errorf("redirect = %q", redirect)
	}

	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("authorize URL is not parseable: %v", err)
	}
	q := u.Query()
	if got := q.Get("code"); got != "true" {
		t.Errorf("code param = %q; without it Anthropic shows no code at all", got)
	}
	if q.Get("client_id") == "" {
		t.Error("client_id missing")
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", q.Get("code_challenge_method"))
	}
	if q.Get("code_challenge") == "" {
		t.Error("code_challenge missing")
	}
	if !strings.Contains(q.Get("scope"), "user:inference") {
		t.Errorf("scope = %q, want user:inference", q.Get("scope"))
	}
	if q.Get("state") != state {
		t.Errorf("state in URL %q != returned state %q", q.Get("state"), state)
	}
	if q.Get("redirect_uri") != redirect {
		t.Errorf("redirect_uri in URL %q != returned redirect %q", q.Get("redirect_uri"), redirect)
	}
}

func TestStartAuth_SecondCallReplacesTheFirstFlow(t *testing.T) {
	// A token endpoint that always succeeds: every failure below must come
	// from the state check, never from the network. Without this the test
	// passes for the wrong reason — it did, against the real Anthropic API.
	tok := tokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"a","refresh_token":"r","expires_in":3600}`))
	})
	withEndpoints(t, "https://claude.test/authorize", tok)
	m := newTestManager(t)

	_, first, _, err := m.StartAuth(0)
	if err != nil {
		t.Fatalf("StartAuth: %v", err)
	}
	_, second, _, err := m.StartAuth(0)
	if err != nil {
		t.Fatalf("StartAuth: %v", err)
	}
	if first == second {
		t.Fatal("two flows issued the same state")
	}
	err = m.CompleteAuth(context.Background(), "somecode", first)
	if err == nil {
		t.Fatal("CompleteAuth accepted a code for a superseded flow")
	}
	if !strings.Contains(err.Error(), "state mismatch") {
		t.Errorf("error = %v, want a state mismatch", err)
	}
	if m.Connected() {
		t.Error("a superseded code still connected the account")
	}
	// ...and the flow the user is actually looking at still works.
	if err := m.CompleteAuth(context.Background(), "somecode", second); err != nil {
		t.Errorf("the current flow was rejected too: %v", err)
	}
}

func TestCompleteAuth_ExchangesCodeAndStoresToken(t *testing.T) {
	var gotPayload map[string]any
	tok := tokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotPayload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"sk-ant-oat01-new","refresh_token":"r-new","token_type":"Bearer","expires_in":3600}`))
	})
	withEndpoints(t, "https://claude.test/authorize", tok)

	m := newTestManager(t)
	_, state, redirect, err := m.StartAuth(0)
	if err != nil {
		t.Fatalf("StartAuth: %v", err)
	}
	if err := m.CompleteAuth(context.Background(), "the-code", state); err != nil {
		t.Fatalf("CompleteAuth: %v", err)
	}

	if !m.Connected() {
		t.Error("Connected() is false after a successful exchange")
	}
	for _, k := range []string{"grant_type", "code", "code_verifier", "redirect_uri", "client_id", "state"} {
		if gotPayload[k] == nil || gotPayload[k] == "" {
			t.Errorf("token request omitted %q (payload: %v)", k, gotPayload)
		}
	}
	if gotPayload["grant_type"] != "authorization_code" {
		t.Errorf("grant_type = %v", gotPayload["grant_type"])
	}
	if gotPayload["redirect_uri"] != redirect {
		t.Errorf("redirect_uri = %v, want %v", gotPayload["redirect_uri"], redirect)
	}

	// And it survives a reload of the store.
	reloaded := newManager(newTokenStoreAt(m.tok.path, m.tok.key))
	if !reloaded.Connected() {
		t.Error("token was not persisted to disk")
	}
}

func TestCompleteAuth_AcceptsFullRedirectURL(t *testing.T) {
	var gotState string
	tok := tokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p map[string]any
		_ = json.Unmarshal(body, &p)
		gotState, _ = p["state"].(string)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"a","refresh_token":"r","expires_in":3600}`))
	})
	withEndpoints(t, "https://claude.test/authorize", tok)

	m := newTestManager(t)
	_, state, _, err := m.StartAuth(0)
	if err != nil {
		t.Fatalf("StartAuth: %v", err)
	}
	// The user pastes whatever is in front of them: the whole URL the browser
	// ended on, with no separate state argument.
	pasted := "https://platform.claude.com/oauth/code/callback?code=abc123&state=" + url.QueryEscape(state)
	if err := m.CompleteAuth(context.Background(), pasted, ""); err != nil {
		t.Fatalf("CompleteAuth with a pasted redirect URL: %v", err)
	}
	if gotState != state {
		t.Errorf("token request carried state %q, want %q (taken from the pasted URL)", gotState, state)
	}
}

func TestCompleteAuth_Rejections(t *testing.T) {
	tok := tokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"code already used"}}`))
	})
	withEndpoints(t, "https://claude.test/authorize", tok)

	t.Run("no flow in progress", func(t *testing.T) {
		m := newTestManager(t)
		if err := m.CompleteAuth(context.Background(), "c", "s"); err == nil {
			t.Error("CompleteAuth succeeded with no flow open")
		}
	})

	t.Run("state mismatch", func(t *testing.T) {
		m := newTestManager(t)
		if _, _, _, err := m.StartAuth(0); err != nil {
			t.Fatalf("StartAuth: %v", err)
		}
		// The stub endpoint answers 400, so an error alone proves nothing —
		// the state check has to be what fired.
		err := m.CompleteAuth(context.Background(), "c", "someone-elses-state")
		if err == nil {
			t.Fatal("CompleteAuth accepted a foreign state")
		}
		if !strings.Contains(err.Error(), "state mismatch") {
			t.Errorf("error = %v, want a state mismatch", err)
		}
	})

	t.Run("empty paste", func(t *testing.T) {
		m := newTestManager(t)
		_, state, _, _ := m.StartAuth(0)
		if err := m.CompleteAuth(context.Background(), "   ", state); err == nil {
			t.Error("CompleteAuth accepted an empty paste")
		}
	})

	t.Run("denied in the pasted URL", func(t *testing.T) {
		m := newTestManager(t)
		_, state, _, _ := m.StartAuth(0)
		err := m.CompleteAuth(context.Background(), "https://platform.claude.com/oauth/code/callback?error=access_denied&state="+state, "")
		if err == nil {
			t.Fatal("CompleteAuth ignored an error= in the pasted URL")
		}
		if !strings.Contains(err.Error(), "access_denied") {
			t.Errorf("error %q does not mention access_denied", err)
		}
	})

	t.Run("anthropic rejects the exchange", func(t *testing.T) {
		m := newTestManager(t)
		_, state, _, _ := m.StartAuth(0)
		err := m.CompleteAuth(context.Background(), "c", state)
		if err == nil {
			t.Fatal("CompleteAuth ignored a 400 from Anthropic")
		}
		if m.Connected() {
			t.Error("a rejected exchange still marked the account connected")
		}
	})

	t.Run("expired flow", func(t *testing.T) {
		m := newTestManager(t)
		_, state, _, _ := m.StartAuth(0)
		m.mu.Lock()
		m.flow.expiresAt = time.Now().Add(-time.Minute)
		m.mu.Unlock()
		err := m.CompleteAuth(context.Background(), "c", state)
		if err == nil {
			t.Fatal("CompleteAuth accepted a code for an expired flow")
		}
		if !strings.Contains(err.Error(), "expired") {
			t.Errorf("error = %v, want an expiry message", err)
		}
	})
}

// The token source must refresh once, not once per caller. Two expired tokens
// observed concurrently must produce exactly one refresh call — and if
// Anthropic ever rotates the access token on refresh, two calls would
// invalidate each other.
func TestTokenSource_ConcurrentCallersRefreshExactlyOnce(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	tok := tokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"sk-ant-oat01-refreshed","refresh_token":"r","expires_in":3600}`))
	})
	withEndpoints(t, "https://claude.test/authorize", tok)

	expired := time.Now().Add(-time.Hour)
	m := newTestManager(t)
	m.token = tokenAt(expired)

	const callers = 8
	sources := make([]*persistingTokenSource, callers)
	for i := range sources {
		sources[i] = &persistingTokenSource{m: m, last: m.token}
	}
	results := make(chan string, callers)
	for _, s := range sources {
		go func(s *persistingTokenSource) {
			tok, err := s.Token()
			if err != nil {
				results <- "err:" + err.Error()
				return
			}
			results <- tok.AccessToken
		}(s)
	}
	seen := map[string]bool{}
	for i := 0; i < callers; i++ {
		seen[<-results] = true
	}
	if len(seen) != 1 {
		t.Fatalf("callers disagreed on the token: %v", seen)
	}
	for v := range seen {
		if strings.HasPrefix(v, "err:") {
			t.Fatalf("a caller failed: %v", v)
		}
		if v != "sk-ant-oat01-refreshed" {
			t.Errorf("token = %q", v)
		}
	}
	if calls != 1 {
		t.Errorf("Anthropic was asked to refresh %d times, want exactly 1", calls)
	}
}

func TestTokenSource_RefreshWithoutTokenIsActionable(t *testing.T) {
	withEndpoints(t, "https://claude.test/authorize", "https://claude.test/token")
	m := newTestManager(t)
	m.token = tokenAt(time.Now().Add(-time.Hour))
	m.token.RefreshToken = ""
	ts, err := m.TokenSource(context.Background())
	if err != nil {
		t.Fatalf("TokenSource: %v", err)
	}
	_, err = ts.Token()
	if err == nil {
		t.Fatal("refresh succeeded with no refresh token")
	}
	if !strings.Contains(err.Error(), "no refresh token") {
		t.Errorf("error is not actionable: %v", err)
	}
}

func TestDecorateRequest_HeadersAndNotConnected(t *testing.T) {
	m := newTestManager(t)

	t.Run("disconnected reports ErrNotConnected", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
		err := m.decorateRequest(context.Background(), req)
		if err == nil {
			t.Fatal("decorateRequest succeeded while disconnected")
		}
		// The router must not read this as a 401 or a rate limit — that would
		// make it back off and fall through to a provider the user did not pick.
		var pe *provider.ProviderError
		if !errors.As(err, &pe) {
			t.Fatalf("error is not a *provider.ProviderError: %#v", err)
		}
		if pe.Err != ErrNotConnected {
			t.Errorf("wrapped err = %v, want ErrNotConnected", pe.Err)
		}
	})

	t.Run("connected sends a Bearer token and the required betas", func(t *testing.T) {
		m.token = tokenAt(time.Now().Add(time.Hour))
		req, _ := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
		if err := m.decorateRequest(context.Background(), req); err != nil {
			t.Fatalf("decorateRequest: %v", err)
		}
		if h := req.Header.Get("Authorization"); !strings.HasPrefix(h, "Bearer ") {
			t.Errorf("Authorization = %q", h)
		}
		if h := req.Header.Get("x-api-key"); h != "" {
			t.Errorf("x-api-key = %q; a subscription account has no key to send", h)
		}
		beta := req.Header.Get("anthropic-beta")
		for _, want := range []string{"oauth-2025-04-20", "claude-code-20250219", "interleaved-thinking-2025-05-14"} {
			if !strings.Contains(beta, want) {
				t.Errorf("anthropic-beta %q is missing %q", beta, want)
			}
		}
		if ua := req.Header.Get("user-agent"); !strings.HasPrefix(ua, "claude-cli/") {
			t.Errorf("user-agent = %q", ua)
		}
		if req.Header.Get("x-app") != "cli" {
			t.Errorf("x-app = %q", req.Header.Get("x-app"))
		}
	})
}

func TestDisconnect_ClearsEverything(t *testing.T) {
	m := newTestManager(t)
	m.token = tokenAt(time.Now().Add(time.Hour))
	m.storeModels([]string{"claude-x"})
	if !m.Connected() {
		t.Fatal("setup failed")
	}
	if err := m.Disconnect(); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if m.Connected() {
		t.Error("still connected after Disconnect")
	}
	if _, ok := m.cachedModels(); ok {
		t.Error("model cache survived Disconnect")
	}
	if _, ok := m.tok.load(); ok {
		t.Error("token file survived Disconnect")
	}
}

func TestAdoptLocal_ReadsClaudeCodeCredentialsFile(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(2 * time.Hour).UnixMilli()
	doc := map[string]any{
		"claudeAiOauth": map[string]any{
			"accessToken":  "sk-ant-oat01-fromfile",
			"refreshToken": "r-fromfile",
			"expiresAt":    expiry,
			"scopes":       []string{"user:inference"},
		},
		"someOtherClaudeCodeKey": "must survive a write-back",
	}
	raw, _ := json.Marshal(doc)
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	// Any ambient env token must not win over the file in this sub-case.
	os.Unsetenv("MEMO_CLAUDE_TOKEN")
	os.Unsetenv("CLAUDE_CODE_OAUTH_TOKEN")

	res := AdoptLocal()
	if res.Source != "claude-code-file" {
		t.Skipf("running on %s, where the file path is not the store (source=%q)", hostOS(), res.Source)
	}
	if res.Token.AccessToken != "sk-ant-oat01-fromfile" {
		t.Errorf("access token = %q", res.Token.AccessToken)
	}
	if res.Token.RefreshToken != "r-fromfile" {
		t.Errorf("refresh token = %q", res.Token.RefreshToken)
	}
	if res.Token.Expiry.UnixMilli() != expiry {
		t.Errorf("expiry = %v, want %v", res.Token.Expiry.UnixMilli(), expiry)
	}
}

func TestAdoptLocal_EnvTokenWinsAndIsLongLived(t *testing.T) {
	t.Setenv("MEMO_CLAUDE_TOKEN", "sk-ant-oat01-fromenv")
	res := AdoptLocal()
	if res.Source != "MEMO_CLAUDE_TOKEN" {
		t.Fatalf("source = %q", res.Source)
	}
	if res.Token.AccessToken != "sk-ant-oat01-fromenv" {
		t.Errorf("token = %q", res.Token.AccessToken)
	}
	if res.Token.Expiry.Before(time.Now().Add(300 * 24 * time.Hour)) {
		t.Errorf("expiry %v is not long-lived; a setup-token does not expire", res.Token.Expiry)
	}
}

func TestAdoptLocal_EnvBeatsAFileFoundOnDisk(t *testing.T) {
	// Both sources present. The environment variable is deliberate — the user
	// ran `claude setup-token` and exported it (or set it in .env) — so it has
	// to win over a login that merely happens to be lying around in
	// ~/.claude/.credentials.json, which may belong to a different account.
	home := t.TempDir()
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	doc := `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-fromfile","refreshToken":"r","expiresAt":99999999999999}}`
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(doc), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("MEMO_CLAUDE_TOKEN", "sk-ant-oat01-fromenv")

	res := AdoptLocal()
	if res.Source != "MEMO_CLAUDE_TOKEN" {
		t.Fatalf("source = %q, want the explicit environment token", res.Source)
	}
	if res.Token.AccessToken != "sk-ant-oat01-fromenv" {
		t.Errorf("access token = %q", res.Token.AccessToken)
	}
}

func TestAdoptLocal_NoLoginIsNotAnError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	os.Unsetenv("MEMO_CLAUDE_TOKEN")
	os.Unsetenv("CLAUDE_CODE_OAUTH_TOKEN")

	res := AdoptLocal()
	if res.Token != nil {
		t.Errorf("found a token where there is none: %#v", res)
	}
	if res.Source != "" {
		t.Errorf("source = %q for an empty result", res.Source)
	}
}

// A rotated refresh token has to reach the user's own Claude Code install, or
// their CLI breaks on a change Memo made — and nothing else in the file may be
// disturbed.
func TestWriteBackRefreshed_PreservesEverythingButTheToken(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".credentials.json")
	orig := `{"claudeAiOauth":{"accessToken":"a","refreshToken":"OLD","expiresAt":123,"scopes":["user:inference"]},"oauthAccount":{"accountUuid":"u-1"},"other":{"nested":true}}`
	if err := os.WriteFile(path, []byte(orig), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	writeBackRefreshed(refreshTokenOnly("NEW"))

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("credentials file unreadable after write-back: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("write-back produced invalid JSON: %v", err)
	}
	oauth, _ := got["claudeAiOauth"].(map[string]any)
	if oauth["refreshToken"] != "NEW" {
		t.Errorf("refreshToken = %v, want NEW", oauth["refreshToken"])
	}
	if oauth["accessToken"] != "a" {
		t.Errorf("accessToken was disturbed: %v", oauth["accessToken"])
	}
	if oauth["scopes"] == nil {
		t.Error("scopes were dropped")
	}
	if got["oauthAccount"] == nil || got["other"] == nil {
		t.Errorf("unrelated top-level keys were dropped: %v", got)
	}
}

func TestWriteBackRefreshed_NoFileToWriteIsSilent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	// Must not panic or error out — an env-sourced token has no file.
	writeBackRefreshed(refreshTokenOnly("anything"))
}

func TestFallbackModels_AreNeverMoreThanALastResort(t *testing.T) {
	if len(fallbackModels) == 0 {
		t.Fatal("the fallback list is empty, so an offline account has no models at all")
	}
	for _, m := range fallbackModels {
		if strings.TrimSpace(m) == "" {
			t.Errorf("blank entry in fallbackModels: %q", m)
		}
	}
}

func hostOS() string {
	return runtime.GOOS
}

// oauthToken builds a token valid until expiry, for tests that only care about
// whether the token source refreshes.
func tokenAt(expiry time.Time) *oauth2.Token {
	return &oauth2.Token{AccessToken: "sk-ant-oat01-stale", RefreshToken: "r-1", TokenType: "Bearer", Expiry: expiry}
}

// oauthTokenRefreshed builds a token carrying only a refresh token, for the
// write-back tests.
func refreshTokenOnly(refresh string) *oauth2.Token {
	return &oauth2.Token{RefreshToken: refresh}
}
