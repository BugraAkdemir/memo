package app

import (
	"context"
	"fmt"
	"memo/internal/logx"
	"os"
	"strings"
	"time"

	"memo/internal/cloudsync"
	"memo/internal/config"
	"memo/internal/sessions"
)

func (a *App) resolveSyncCredentials() (string, string) {
	clientID := strings.TrimSpace(a.cfg.Sync.ClientID)
	clientSecret := strings.TrimSpace(a.cfg.Sync.ClientSecret)
	if clientID == "" {
		clientID = strings.TrimSpace(os.Getenv("MEMO_GOOGLE_CLIENT_ID"))
	}
	if clientSecret == "" {
		clientSecret = strings.TrimSpace(os.Getenv("MEMO_GOOGLE_CLIENT_SECRET"))
	}
	return clientID, clientSecret
}

func (a *App) ensureSyncManager() error {
	a.syncMu.RLock()
	sm := a.syncManager
	a.syncMu.RUnlock()
	if sm != nil {
		return nil
	}
	clientID, clientSecret := a.resolveSyncCredentials()
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("cloud sync OAuth credentials missing (set MEMO_GOOGLE_CLIENT_ID and MEMO_GOOGLE_CLIENT_SECRET in app environment)")
	}
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	if a.syncManager != nil {
		return nil
	}
	a.syncManager = cloudsync.New(
		a.lifecycleCtx,
		a.cfg.Memory.PersistDir,
		a.cfg.Sync.Passphrase,
		a.cfg.Sync.IntervalMessages,
		clientID,
		clientSecret,
		a.cfg.Sync.TokenPath,
	)
	a.wireSyncRestoreHooks(a.syncManager)
	return nil
}

// wireSyncRestoreHooks closes the live memory store before a cloud restore
// overwrites memory.db and re-opens it afterwards. Without this the open SQLite
// connection's stale WAL/SHM survive the rename, corrupting the restored DB.
func (a *App) wireSyncRestoreHooks(sm *cloudsync.Manager) {
	if sm == nil {
		return
	}
	sm.BeforeRestore = func() {
		a.storeMu.Lock()
		if a.store != nil {
			if err := a.store.Close(); err != nil {
				logx.Printf("WARN: memory store close before restore: %v", err)
			}
			a.store = nil
		}
		a.storeMu.Unlock()
	}
	sm.AfterRestore = func() {
		a.clientMu.RLock()
		client := a.client
		a.clientMu.RUnlock()
		a.reinitMemoryStore(client, a.cfg.API.EmbeddingModel)

		// Cloud restore also overwrites sessions/*.json, providers.json and
		// orchestra.json (see sync_manager.go's restoreZip), but until now
		// only the memory store was reloaded — the running app kept using
		// its pre-restore in-memory sessions manager, provider router and
		// orchestra conductor until the next full restart. Same reinit
		// ImportData calls after a local .memo import.
		a.sessionsMu.Lock()
		if a.sessions != nil {
			if newSm, err := sessions.NewManager(config.DataPath("sessions")); err == nil {
				a.sessions = newSm
			} else {
				logx.Printf("WARN: cloud restore: sessions reinit: %v", err)
			}
		}
		a.sessionsMu.Unlock()

		a.reinitProviderAndOrchestra()
	}
}

// CheckSyncAuth reports whether the cloud sync manager is authenticated.
func (a *App) CheckSyncAuth() bool {
	if err := a.ensureSyncManager(); err != nil {
		return false
	}
	a.syncMu.RLock()
	sm := a.syncManager
	a.syncMu.RUnlock()
	if sm == nil {
		return false
	}
	return sm.IsAuthenticated()
}

// CheckAuth is an alias for CheckSyncAuth exposed for cloud sync UI logic.
func (a *App) CheckAuth() bool {
	return a.CheckSyncAuth()
}

// StartSyncAuth starts the OAuth2 loopback flow and returns the URL to open.
func (a *App) StartSyncAuth() (string, error) {
	if err := a.ensureSyncManager(); err != nil {
		return "", err
	}
	a.syncMu.RLock()
	sm := a.syncManager
	a.syncMu.RUnlock()
	if sm == nil {
		return "", fmt.Errorf("sync manager not initialized")
	}
	url, err := sm.StartAuthFlow()
	if err != nil {
		return "", err
	}
	return url, nil
}

// TriggerSync forces an immediate backup upload.
func (a *App) TriggerSync() {
	if err := a.ensureSyncManager(); err != nil {
		a.emitEvent("sync:error", err.Error())
		return
	}
	a.syncMu.RLock()
	sm := a.syncManager
	a.syncMu.RUnlock()
	if sm != nil {
		sm.TriggerNow()
	}
}

// PullSync downloads the latest cloud backup and restores local .gob files.
func (a *App) PullSync() {
	if err := a.ensureSyncManager(); err != nil {
		a.emitEvent("sync:error", err.Error())
		return
	}
	a.syncMu.RLock()
	sm := a.syncManager
	a.syncMu.RUnlock()
	if sm != nil {
		sm.TriggerPullNow()
	}
}

// SyncNow runs push then pull in background.
func (a *App) SyncNow() {
	if err := a.ensureSyncManager(); err != nil {
		a.emitEvent("sync:error", err.Error())
		return
	}
	a.syncMu.RLock()
	sm := a.syncManager
	a.syncMu.RUnlock()
	if sm != nil {
		sm.TriggerFullSyncNow()
	}
}

// GetSyncAccount returns Google account identity for the connected sync session.
func (a *App) GetSyncAccount() interface{} {
	if err := a.ensureSyncManager(); err != nil {
		return SyncAccount{Authenticated: false}
	}
	a.syncMu.RLock()
	sm := a.syncManager
	a.syncMu.RUnlock()
	if sm == nil {
		return SyncAccount{Authenticated: false}
	}
	if !sm.IsAuthenticated() {
		return SyncAccount{Authenticated: false}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	acc, err := sm.GetAccountInfo(ctx)
	if err != nil {
		logx.Printf("cloud sync account info: %v", err)
		return SyncAccount{Authenticated: true}
	}
	return SyncAccount{
		Authenticated: true,
		Name:          acc.Name,
		Email:         acc.Email,
	}
}

// GetSyncSettings returns the current sync configuration.
func (a *App) GetSyncSettings() interface{} {
	return a.cfg.Sync
}

// UpdateSyncSettings saves updated sync credentials and re-creates the sync manager.
func (a *App) UpdateSyncSettings(enabled bool, clientID, clientSecret, passphrase, tokenPath string, intervalMessages int) error {
	if tokenPath == "" {
		tokenPath = config.DataPath("sync_token.json")
	}
	if intervalMessages <= 0 {
		intervalMessages = 50
	}

	a.cfg.Sync.Enabled = enabled
	a.cfg.Sync.ClientID = strings.TrimSpace(clientID)
	a.cfg.Sync.ClientSecret = strings.TrimSpace(clientSecret)
	a.cfg.Sync.Passphrase = passphrase
	a.cfg.Sync.TokenPath = strings.TrimSpace(tokenPath)
	a.cfg.Sync.IntervalMessages = intervalMessages

	if err := config.Save(a.cfg); err != nil {
		return err
	}

	resolvedClientID, resolvedClientSecret := a.resolveSyncCredentials()
	a.syncMu.Lock()
	oldSM := a.syncManager
	if a.cfg.Sync.Enabled && resolvedClientID != "" && resolvedClientSecret != "" {
		a.syncManager = cloudsync.New(
			a.lifecycleCtx,
			a.cfg.Memory.PersistDir,
			a.cfg.Sync.Passphrase,
			a.cfg.Sync.IntervalMessages,
			resolvedClientID,
			resolvedClientSecret,
			a.cfg.Sync.TokenPath,
		)
		a.wireSyncRestoreHooks(a.syncManager)
	} else {
		a.syncManager = nil
	}
	a.syncMu.Unlock()
	if oldSM != nil {
		oldSM.Stop()
	}
	return nil
}

// DisconnectSync revokes the local OAuth token and resets the sync manager.
func (a *App) DisconnectSync() error {
	tokenPath := a.cfg.Sync.TokenPath
	if tokenPath == "" {
		tokenPath = config.DataPath("sync_token.json")
	}
	if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("disconnect sync: remove token: %w", err)
	}
	a.syncMu.Lock()
	oldSM := a.syncManager
	a.syncManager = nil
	a.syncMu.Unlock()
	if oldSM != nil {
		oldSM.Stop()
	}
	return nil
}
