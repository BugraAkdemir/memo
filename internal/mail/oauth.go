// SPDX-License-Identifier: AGPL-3.0-or-later

package mail

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Public desktop OAuth2 client profiles (Mozilla Thunderbird open-source desktop profiles).
var (
	GoogleClientID     = "406964657835-aq8lmia8j95dhl1a2bvharmfk3t1hgqj.apps.googleusercontent.com"
	GoogleClientSecret = "kSmqreRr0qwBWJgbf5Y-PjSU"
	GoogleAuthURL      = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL     = "https://oauth2.googleapis.com/token"
	GoogleUserInfoURL  = "https://www.googleapis.com/oauth2/v3/userinfo"

	MicrosoftClientID  = "9e5f94bc-e8a4-4e73-b8be-63364c29d753"
	MicrosoftAuthURL   = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	MicrosoftTokenURL  = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	MicrosoftDeviceURL = "https://login.microsoftonline.com/common/oauth2/v2.0/devicecode"
)

// OAuthSession tracks an in-flight OAuth2 browser login flow.
type OAuthSession struct {
	State        string       `json:"state"`
	Provider     ProviderType `json:"provider"`
	EmailHint    string       `json:"email_hint,omitempty"`
	AuthURL      string       `json:"auth_url"`
	RedirectURI  string       `json:"redirect_uri"`
	CodeVerifier string       `json:"-"`
	DeviceCode   string       `json:"-"`
	UserCode     string       `json:"user_code,omitempty"`
	VerifyURL    string       `json:"verify_url,omitempty"`
	IntervalSec  int          `json:"interval_sec,omitempty"`
	Status       string       `json:"status"` // "pending", "completed", "error"
	Error        string       `json:"error,omitempty"`
	AccountID    string       `json:"account_id,omitempty"`
	Email        string       `json:"email,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`

	listener net.Listener
	server   *http.Server
	mu       sync.Mutex
}

// Snapshot returns a thread-safe copy of OAuthSession for JSON serialization.
func (s *OAuthSession) Snapshot() OAuthSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return OAuthSession{
		State:       s.State,
		Provider:    s.Provider,
		EmailHint:   s.EmailHint,
		AuthURL:     s.AuthURL,
		RedirectURI: s.RedirectURI,
		UserCode:    s.UserCode,
		VerifyURL:   s.VerifyURL,
		IntervalSec: s.IntervalSec,
		Status:      s.Status,
		Error:       s.Error,
		AccountID:   s.AccountID,
		Email:       s.Email,
		CreatedAt:   s.CreatedAt,
	}
}

func randomBase64URL(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceS256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// StartOAuthFlow spins up a loopback listener (for Google) or Device Code / PKCE flow (for Microsoft).
func (m *Manager) StartOAuthFlow(provider ProviderType, emailHint string) (*OAuthSession, error) {
	verifier, err := randomBase64URL(32)
	if err != nil {
		return nil, err
	}
	state, err := randomBase64URL(16)
	if err != nil {
		return nil, err
	}

	sess := &OAuthSession{
		State:        state,
		Provider:     provider,
		EmailHint:    strings.TrimSpace(emailHint),
		CodeVerifier: verifier,
		Status:       "pending",
		CreatedAt:    time.Now(),
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("open loopback listener: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	sess.listener = ln

	switch provider {
	case ProviderGoogle:
		sess.RedirectURI = fmt.Sprintf("http://localhost:%d", port)
		params := url.Values{
			"client_id":             {GoogleClientID},
			"redirect_uri":          {sess.RedirectURI},
			"response_type":         {"code"},
			"scope":                 {"https://mail.google.com/ openid email profile"},
			"access_type":           {"offline"},
			"prompt":                {"consent"},
			"state":                 {state},
			"code_challenge":        {pkceS256Challenge(verifier)},
			"code_challenge_method": {"S256"},
		}
		if sess.EmailHint != "" {
			params.Set("login_hint", sess.EmailHint)
		}
		sess.AuthURL = GoogleAuthURL + "?" + params.Encode()

	case ProviderMicrosoft:
		_ = ln.Close()
		sess.listener = nil
		if err := m.initMicrosoftDeviceFlow(sess); err != nil {
			sess.RedirectURI = "https://localhost"
			params := url.Values{
				"client_id":             {MicrosoftClientID},
				"redirect_uri":          {sess.RedirectURI},
				"response_type":         {"code"},
				"scope":                 {"offline_access openid email profile https://outlook.office.com/IMAP.AccessAsUser.All https://outlook.office.com/SMTP.Send"},
				"state":                 {state},
				"code_challenge":        {pkceS256Challenge(verifier)},
				"code_challenge_method": {"S256"},
			}
			if sess.EmailHint != "" {
				params.Set("login_hint", sess.EmailHint)
			}
			sess.AuthURL = MicrosoftAuthURL + "?" + params.Encode()
		}

	default:
		_ = ln.Close()
		return nil, fmt.Errorf("unsupported OAuth provider: %s", provider)
	}

	m.mu.Lock()
	m.sessions[state] = sess
	m.mu.Unlock()

	if sess.listener != nil {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			code := r.URL.Query().Get("code")
			errParam := r.URL.Query().Get("error")
			if errParam != "" {
				sess.mu.Lock()
				sess.Status = "error"
				sess.Error = errParam + ": " + r.URL.Query().Get("error_description")
				errMsg := sess.Error
				sess.mu.Unlock()
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprintf(w, "<html><body style='font-family:sans-serif;text-align:center;padding:40px;background:#0f172a;color:#f8fafc;'><h2>❌ Giriş İptal Edildi / Sign-in Cancelled</h2><p>%s</p></body></html>", errMsg)
				go sess.closeServer()
				return
			}
			if code == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			acct, err := m.finalizeOAuthCode(r.Context(), sess, code)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err != nil {
				sess.mu.Lock()
				sess.Status = "error"
				sess.Error = err.Error()
				sess.mu.Unlock()
				fmt.Fprintf(w, "<html><body style='font-family:sans-serif;text-align:center;padding:40px;background:#0f172a;color:#f8fafc;'><h2>❌ Bağlantı Hatası / Connection Error</h2><p>%s</p></body></html>", err.Error())
			} else {
				fmt.Fprintf(w, "<html><body style='font-family:sans-serif;text-align:center;padding:40px;background:#0f172a;color:#f8fafc;'><h2>✅ Memo Mail Bağlantısı Başarılı!</h2><p><b>%s</b> hesabı bağlandı. Bu pencereyi kapatabilirsiniz.</p><script>setTimeout(()=>window.close(),1500);</script></body></html>", acct.Email)
			}
			go sess.closeServer()
		})
		sess.server = &http.Server{Handler: mux}
		go func() {
			_ = sess.server.Serve(sess.listener)
		}()
	}

	return sess, nil
}

func (s *OAuthSession) closeServer() {
	time.Sleep(250 * time.Millisecond)
	s.mu.Lock()
	srv := s.server
	s.mu.Unlock()
	if srv != nil {
		_ = srv.Close()
	}
}

func (m *Manager) initMicrosoftDeviceFlow(sess *OAuthSession) error {
	form := url.Values{
		"client_id": {MicrosoftClientID},
		"scope":     {"offline_access openid email profile https://outlook.office.com/IMAP.AccessAsUser.All https://outlook.office.com/SMTP.Send"},
	}
	req, err := http.NewRequest(http.MethodPost, MicrosoftDeviceURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("devicecode status %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		Interval                int    `json:"interval"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return err
	}
	if parsed.DeviceCode == "" {
		return fmt.Errorf("empty device_code")
	}

	sess.DeviceCode = parsed.DeviceCode
	sess.UserCode = parsed.UserCode
	sess.VerifyURL = parsed.VerificationURI
	if parsed.VerificationURIComplete != "" {
		sess.AuthURL = parsed.VerificationURIComplete
	} else {
		sess.AuthURL = parsed.VerificationURI
	}
	if parsed.Interval <= 0 {
		parsed.Interval = 5
	}
	sess.IntervalSec = parsed.Interval

	go m.pollMicrosoftDeviceLoop(sess)
	return nil
}

func (m *Manager) pollMicrosoftDeviceLoop(sess *OAuthSession) {
	interval := time.Duration(sess.IntervalSec) * time.Second
	if interval < time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(10 * time.Minute)
	client := &http.Client{Timeout: 10 * time.Second}

	for time.Now().Before(deadline) {
		time.Sleep(interval)
		sess.mu.Lock()
		status := sess.Status
		sess.mu.Unlock()
		if status != "pending" {
			return
		}

		form := url.Values{
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"client_id":   {MicrosoftClientID},
			"device_code": {sess.DeviceCode},
		}
		resp, err := client.PostForm(MicrosoftTokenURL, form)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			tok, err := parseTokenResponse(body)
			if err != nil {
				sess.mu.Lock()
				sess.Status = "error"
				sess.Error = err.Error()
				sess.mu.Unlock()
				return
			}
			if _, err := m.saveOAuthAccount(context.Background(), sess, tok); err != nil {
				sess.mu.Lock()
				sess.Status = "error"
				sess.Error = err.Error()
				sess.mu.Unlock()
			}
			return
		}

		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &errResp)
		if errResp.Error == "authorization_pending" {
			continue
		}
		if errResp.Error == "slow_down" {
			interval += 5 * time.Second
			continue
		}
		sess.mu.Lock()
		sess.Status = "error"
		sess.Error = errResp.Error + ": " + errResp.ErrorDescription
		sess.mu.Unlock()
		return
	}
}

// CompleteManualCallback completes an OAuth flow from a pasted redirect URL or authorization code.
func (m *Manager) CompleteManualCallback(ctx context.Context, state, rawURLOrCode string) (*Account, error) {
	m.mu.RLock()
	sess, ok := m.sessions[state]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("oauth session not found: %s", state)
	}

	code := strings.TrimSpace(rawURLOrCode)
	if strings.Contains(code, "code=") {
		if u, err := url.Parse(code); err == nil {
			if c := u.Query().Get("code"); c != "" {
				code = c
			}
		}
	}
	if code == "" {
		return nil, fmt.Errorf("missing authorization code")
	}

	return m.finalizeOAuthCode(ctx, sess, code)
}

func (m *Manager) finalizeOAuthCode(ctx context.Context, sess *OAuthSession, code string) (*Account, error) {
	var tokenURL string
	form := url.Values{
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {sess.RedirectURI},
		"code_verifier": {sess.CodeVerifier},
	}

	switch sess.Provider {
	case ProviderGoogle:
		tokenURL = GoogleTokenURL
		form.Set("client_id", GoogleClientID)
		form.Set("client_secret", GoogleClientSecret)
	case ProviderMicrosoft:
		tokenURL = MicrosoftTokenURL
		form.Set("client_id", MicrosoftClientID)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", sess.Provider)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	tok, err := parseTokenResponse(body)
	if err != nil {
		return nil, err
	}

	return m.saveOAuthAccount(ctx, sess, tok)
}

func parseTokenResponse(body []byte) (*OAuthToken, error) {
	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		IDToken      string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if raw.AccessToken == "" {
		return nil, fmt.Errorf("empty access_token in response")
	}
	expiry := time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second)
	if raw.ExpiresIn <= 0 {
		expiry = time.Now().Add(55 * time.Minute)
	}
	return &OAuthToken{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		TokenType:    raw.TokenType,
		Expiry:       expiry,
		IDToken:      raw.IDToken,
	}, nil
}

// ExtractEmailFromIDToken decodes the JWT payload to discover the authenticated email address.
func ExtractEmailFromIDToken(idToken string) (email, name string) {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return "", ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ""
	}
	var claims struct {
		Email             string `json:"email"`
		PreferredUsername string `json:"preferred_username"`
		UPN               string `json:"upn"`
		Name              string `json:"name"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", ""
	}
	email = claims.Email
	if email == "" && strings.Contains(claims.PreferredUsername, "@") {
		email = claims.PreferredUsername
	}
	if email == "" && strings.Contains(claims.UPN, "@") {
		email = claims.UPN
	}
	return strings.ToLower(strings.TrimSpace(email)), claims.Name
}

func fetchGoogleUserInfoEmail(ctx context.Context, accessToken string) (string, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GoogleUserInfoURL, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", ""
	}
	var info struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&info)
	return strings.ToLower(strings.TrimSpace(info.Email)), info.Name
}

// EnsureValidToken refreshes the OAuth2 token automatically if expired.
func (m *Manager) EnsureValidToken(ctx context.Context, acct *Account) (string, error) {
	if acct.AuthType != AuthOAuth2 || acct.Token == nil {
		return "", fmt.Errorf("account does not use OAuth2")
	}
	if !acct.Token.Expired() {
		return acct.Token.AccessToken, nil
	}
	if acct.Token.RefreshToken == "" {
		return acct.Token.AccessToken, nil
	}

	var tokenURL string
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {acct.Token.RefreshToken},
	}

	switch acct.Provider {
	case ProviderGoogle:
		tokenURL = GoogleTokenURL
		form.Set("client_id", GoogleClientID)
		form.Set("client_secret", GoogleClientSecret)
	case ProviderMicrosoft:
		tokenURL = MicrosoftTokenURL
		form.Set("client_id", MicrosoftClientID)
	default:
		return acct.Token.AccessToken, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token refresh failed (%d): %s", resp.StatusCode, string(body))
	}

	newTok, err := parseTokenResponse(body)
	if err != nil {
		return "", err
	}
	if newTok.RefreshToken == "" {
		newTok.RefreshToken = acct.Token.RefreshToken
	}

	m.mu.Lock()
	acct.Token = newTok
	m.mu.Unlock()
	_ = m.saveAccountsToDisk()
	return newTok.AccessToken, nil
}
