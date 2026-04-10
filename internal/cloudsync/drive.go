package cloudsync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const (
	backupMimeType   = "application/octet-stream"
	backupNamePrefix = "memo_backup_"
	driveAppFolder   = "appDataFolder"
)

// driveClient wraps the Google Drive API with automatic token refresh and persistence.
type driveClient struct {
	mu        sync.Mutex
	cfg       *oauth2.Config
	token     *oauth2.Token
	tokenPath string

	// authDone is closed once the OAuth loopback flow completes.
	authDone       chan struct{}
	authDoneClosed bool
}

func newDriveClient(clientID, clientSecret, tokenPath string) *driveClient {
	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		// appdata scope: only files this app creates; user cannot see them in Drive UI.
		Scopes: []string{
			drive.DriveAppdataScope,
			"openid",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
	dc := &driveClient{
		cfg:       cfg,
		tokenPath: tokenPath,
		authDone:  make(chan struct{}),
	}
	if t, err := dc.loadToken(); err == nil {
		dc.token = t
		dc.authDoneClosed = true
		close(dc.authDone) // already authenticated
	}
	return dc
}

// IsAuthenticated reports whether a valid (or refreshable) token is available.
func (dc *driveClient) IsAuthenticated() bool {
	dc.mu.Lock()
	token := dc.token
	cfg := dc.cfg
	dc.mu.Unlock()

	if token == nil {
		return false
	}
	ts := cfg.TokenSource(context.Background(), token)
	t, err := ts.Token()
	if err != nil {
		return false
	}
	// Persist refreshed token if it changed.
	if t.AccessToken != token.AccessToken || t.RefreshToken != token.RefreshToken || !t.Expiry.Equal(token.Expiry) {
		dc.mu.Lock()
		dc.token = t
		_ = dc.saveToken(t)
		dc.mu.Unlock()
	}
	return t.Valid()
}

// StartAuthFlow starts a loopback HTTP server, returns the URL the user must
// open in a browser. Call WaitForAuth to block until the flow completes.
func (dc *driveClient) StartAuthFlow() (string, error) {
	// Reset the done channel so the flow can be restarted.
	dc.mu.Lock()
	dc.authDone = make(chan struct{})
	dc.authDoneClosed = false
	dc.mu.Unlock()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("cloudsync: oauth listener: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)
	dc.mu.Lock()
	dc.cfg.RedirectURL = redirectURL
	authURL := dc.cfg.AuthCodeURL("memo-sync", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	dc.mu.Unlock()

	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `<html><body style="font-family:sans-serif;padding:2rem">
			<h2>Memo — Authorization complete</h2>
			<p>You can close this tab and return to the app.</p>
		</body></html>`)

		// Exchange in background so the HTTP response can flush.
		go func() {
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_ = srv.Shutdown(ctx)
			}()

			t, err := dc.cfg.Exchange(context.Background(), code)
			if err != nil {
				log.Printf("cloudsync: oauth exchange: %v", err)
				return
			}
			dc.mu.Lock()
			dc.token = t
			if err := dc.saveToken(t); err != nil {
				log.Printf("cloudsync: save token: %v", err)
			}
			dc.closeAuthDoneLocked()
			dc.mu.Unlock()
		}()
	})

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("cloudsync: auth server: %v", err)
		}
	}()

	return authURL, nil
}

// WaitForAuth blocks until the OAuth flow finishes or the context is cancelled.
func (dc *driveClient) WaitForAuth(ctx context.Context) error {
	dc.mu.Lock()
	done := dc.authDone
	dc.mu.Unlock()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// service returns an authenticated Drive service, refreshing the token as needed.
func (dc *driveClient) service() (*drive.Service, error) {
	dc.mu.Lock()
	token := dc.token
	cfg := dc.cfg
	dc.mu.Unlock()

	if token == nil {
		return nil, fmt.Errorf("cloudsync: not authenticated")
	}
	ctx := context.Background()
	ts := cfg.TokenSource(ctx, token)

	// Force a token refresh check and persist if changed.
	t, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("cloudsync: token refresh: %w", err)
	}
	if t.AccessToken != token.AccessToken || t.RefreshToken != token.RefreshToken || !t.Expiry.Equal(token.Expiry) {
		dc.mu.Lock()
		dc.token = t
		_ = dc.saveToken(t)
		dc.mu.Unlock()
	}

	svc, err := drive.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("cloudsync: drive service: %w", err)
	}
	return svc, nil
}

// GetAccountInfo returns the authenticated Google account's display name and email.
func (dc *driveClient) GetAccountInfo(ctx context.Context) (string, string, error) {
	dc.mu.Lock()
	token := dc.token
	cfg := dc.cfg
	dc.mu.Unlock()

	if token == nil {
		return "", "", fmt.Errorf("cloudsync: not authenticated")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ts := cfg.TokenSource(ctx, token)
	t, err := ts.Token()
	if err != nil {
		return "", "", fmt.Errorf("cloudsync: token refresh: %w", err)
	}
	if t.AccessToken != token.AccessToken || t.RefreshToken != token.RefreshToken || !t.Expiry.Equal(token.Expiry) {
		dc.mu.Lock()
		dc.token = t
		_ = dc.saveToken(t)
		dc.mu.Unlock()
	}

	client := oauth2.NewClient(ctx, ts)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return "", "", fmt.Errorf("cloudsync: userinfo request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("cloudsync: userinfo call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", "", fmt.Errorf("cloudsync: userinfo status %d: %s", resp.StatusCode, string(body))
	}

	var u struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return "", "", fmt.Errorf("cloudsync: userinfo decode: %w", err)
	}
	if u.Name == "" && u.Email == "" {
		return "", "", fmt.Errorf("cloudsync: userinfo empty")
	}
	return u.Name, u.Email, nil
}

// UploadBackup uploads an already-encrypted blob to Drive's appDataFolder.
func (dc *driveClient) UploadBackup(data []byte) (string, error) {
	svc, err := dc.service()
	if err != nil {
		return "", err
	}

	name := fmt.Sprintf("%s%s.enc", backupNamePrefix, time.Now().UTC().Format("20060102_150405"))
	meta := &drive.File{
		Name:    name,
		Parents: []string{driveAppFolder},
	}

	f, err := svc.Files.Create(meta).
		Media(bytes.NewReader(data), googleapi.ContentType(backupMimeType)).
		Fields("id, name").
		Do()
	if err != nil {
		return "", fmt.Errorf("cloudsync: upload: %w", err)
	}

	log.Printf("cloudsync: uploaded %s (%d bytes, id=%s)", f.Name, len(data), f.Id)
	return f.Id, nil
}

// DownloadLatestBackup downloads the most recent encrypted backup blob.
func (dc *driveClient) DownloadLatestBackup() (string, []byte, error) {
	svc, err := dc.service()
	if err != nil {
		return "", nil, err
	}

	q := fmt.Sprintf("name contains '%s' and trashed = false", backupNamePrefix)
	resp, err := svc.Files.List().
		Spaces(driveAppFolder).
		Q(q).
		Fields("files(id, name, createdTime)").
		OrderBy("createdTime desc").
		PageSize(1).
		Do()
	if err != nil {
		return "", nil, fmt.Errorf("cloudsync: latest backup list: %w", err)
	}
	if len(resp.Files) == 0 {
		return "", nil, fmt.Errorf("cloudsync: no backups found")
	}
	f := resp.Files[0]

	dl, err := svc.Files.Get(f.Id).Download()
	if err != nil {
		return "", nil, fmt.Errorf("cloudsync: download %s: %w", f.Name, err)
	}
	defer dl.Body.Close()

	data, err := io.ReadAll(dl.Body)
	if err != nil {
		return "", nil, fmt.Errorf("cloudsync: read backup %s: %w", f.Name, err)
	}
	return f.Name, data, nil
}

// PruneOldBackups deletes all but the `keep` most-recent backup files.
func (dc *driveClient) PruneOldBackups(keep int) error {
	svc, err := dc.service()
	if err != nil {
		return err
	}

	q := fmt.Sprintf("name contains '%s' and trashed = false", backupNamePrefix)
	resp, err := svc.Files.List().
		Spaces(driveAppFolder).
		Q(q).
		Fields("files(id, name, createdTime)").
		OrderBy("createdTime desc").
		Do()
	if err != nil {
		return fmt.Errorf("cloudsync: prune list: %w", err)
	}

	files := resp.Files
	// Ensure descending order (API may not guarantee it).
	sort.Slice(files, func(i, j int) bool {
		return files[i].CreatedTime > files[j].CreatedTime
	})

	for i := keep; i < len(files); i++ {
		if err := svc.Files.Delete(files[i].Id).Do(); err != nil {
			return fmt.Errorf("cloudsync: prune delete %s: %w", files[i].Name, err)
		}
		log.Printf("cloudsync: pruned old backup %s", files[i].Name)
	}
	return nil
}

// ─── Token persistence ────────────────────────────────────────────────────────

func (dc *driveClient) loadToken() (*oauth2.Token, error) {
	data, err := os.ReadFile(dc.tokenPath)
	if err != nil {
		return nil, err
	}
	var t oauth2.Token
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (dc *driveClient) saveToken(t *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(dc.tokenPath), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	// 0600 — only the owner can read the token file.
	return os.WriteFile(dc.tokenPath, data, 0600)
}

func (dc *driveClient) closeAuthDoneLocked() {
	if dc.authDoneClosed {
		return
	}
	close(dc.authDone)
	dc.authDoneClosed = true
}
