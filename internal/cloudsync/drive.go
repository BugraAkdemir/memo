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
	backupMimeType    = "application/octet-stream"
	backupNamePrefix  = "memo_backup_"
	driveFolderName   = "Memo Backups"
	driveFolderMime   = "application/vnd.google-apps.folder"
)

// driveClient wraps the Google Drive API with automatic token refresh and persistence.
type driveClient struct {
	mu        sync.Mutex
	cfg       *oauth2.Config
	token     *oauth2.Token
	tokenPath string
	folderID  string // cached "Memo Backups" folder ID

	// authDone is closed once the OAuth loopback flow completes.
	authDone       chan struct{}
	authDoneClosed bool
}

func newDriveClient(clientID, clientSecret, tokenPath string) *driveClient {
	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		// drive.file scope: access only to files created by this app; no verification required.
		Scopes: []string{
			drive.DriveFileScope,
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
		fmt.Fprint(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Memo — Connected</title>
<style>
  *{margin:0;padding:0;box-sizing:border-box}
  body{min-height:100vh;display:flex;align-items:center;justify-content:center;background:#0a0a0a;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;color:#e5e5e5}
  .card{background:#141414;border:1px solid #2a2a2a;border-radius:16px;padding:48px 40px;max-width:400px;width:90%;text-align:center;box-shadow:0 24px 64px rgba(0,0,0,.6)}
  .icon{width:64px;height:64px;background:linear-gradient(135deg,#a78bfa,#7c3aed);border-radius:50%;display:flex;align-items:center;justify-content:center;margin:0 auto 24px}
  .icon svg{width:32px;height:32px;stroke:#fff;fill:none;stroke-width:2.5;stroke-linecap:round;stroke-linejoin:round}
  h1{font-size:20px;font-weight:600;color:#f5f5f5;margin-bottom:8px}
  p{font-size:14px;color:#888;line-height:1.6}
  .badge{display:inline-block;margin-top:24px;padding:6px 16px;background:#1e1e1e;border:1px solid #333;border-radius:999px;font-size:12px;color:#666;letter-spacing:.04em}
</style>
</head>
<body>
<div class="card">
  <div class="icon">
    <svg viewBox="0 0 24 24"><polyline points="20 6 9 17 4 12"/></svg>
  </div>
  <h1>Drive Connected</h1>
  <p>Authorization complete. You can close this tab and return to Memo.</p>
  <span class="badge">MEMO · LOCAL AI</span>
</div>
</body>
</html>`)

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

// getOrCreateFolder returns the Drive folder ID for "Memo Backups", creating it if needed.
// The ID is cached so subsequent calls are free.
func (dc *driveClient) getOrCreateFolder(svc *drive.Service) (string, error) {
	dc.mu.Lock()
	cached := dc.folderID
	dc.mu.Unlock()
	if cached != "" {
		return cached, nil
	}

	// Search for existing folder.
	q := fmt.Sprintf("name = '%s' and mimeType = '%s' and trashed = false", driveFolderName, driveFolderMime)
	resp, err := svc.Files.List().
		Spaces("drive").
		Q(q).
		Fields("files(id)").
		PageSize(1).
		Do()
	if err != nil {
		return "", fmt.Errorf("cloudsync: folder search: %w", err)
	}

	var folderID string
	if len(resp.Files) > 0 {
		folderID = resp.Files[0].Id
	} else {
		// Create the folder.
		f, err := svc.Files.Create(&drive.File{
			Name:     driveFolderName,
			MimeType: driveFolderMime,
		}).Fields("id").Do()
		if err != nil {
			return "", fmt.Errorf("cloudsync: folder create: %w", err)
		}
		folderID = f.Id
		log.Printf("cloudsync: created Drive folder %q (%s)", driveFolderName, folderID)
	}

	dc.mu.Lock()
	dc.folderID = folderID
	dc.mu.Unlock()
	return folderID, nil
}

// UploadBackup uploads an already-encrypted blob into the "Memo Backups" Drive folder.
func (dc *driveClient) UploadBackup(data []byte) (string, error) {
	svc, err := dc.service()
	if err != nil {
		return "", err
	}

	folderID, err := dc.getOrCreateFolder(svc)
	if err != nil {
		return "", err
	}

	name := fmt.Sprintf("%s%s.enc", backupNamePrefix, time.Now().UTC().Format("20060102_150405"))
	meta := &drive.File{
		Name:    name,
		Parents: []string{folderID},
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

	folderID, err := dc.getOrCreateFolder(svc)
	if err != nil {
		return "", nil, err
	}

	q := fmt.Sprintf("'%s' in parents and name contains '%s' and trashed = false", folderID, backupNamePrefix)
	resp, err := svc.Files.List().
		Spaces("drive").
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

	folderID, err := dc.getOrCreateFolder(svc)
	if err != nil {
		return err
	}

	q := fmt.Sprintf("'%s' in parents and name contains '%s' and trashed = false", folderID, backupNamePrefix)
	resp, err := svc.Files.List().
		Spaces("drive").
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
