// SPDX-License-Identifier: AGPL-3.0-or-later

package mail

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager manages connected email accounts, encrypted storage, OAuth2 flows, and IMAP/SMTP operations.
type Manager struct {
	vault    *Vault
	mu       sync.RWMutex
	accounts map[string]*Account
	sessions map[string]*OAuthSession
}

// NewManager initializes a Manager backed by an AES-256-GCM encrypted Vault at storePath.
func NewManager(storePath string, masterKey ...[]byte) (*Manager, error) {
	var key []byte
	if len(masterKey) > 0 {
		key = masterKey[0]
	}
	v := NewVault(storePath, key)
	loaded, err := v.Load()
	if err != nil {
		return nil, err
	}
	m := &Manager{
		vault:    v,
		accounts: make(map[string]*Account, len(loaded)),
		sessions: make(map[string]*OAuthSession),
	}
	for _, a := range loaded {
		if a != nil && a.ID != "" {
			m.accounts[a.ID] = a
		}
	}
	return m, nil
}

func (m *Manager) saveAccountsToDisk() error {
	m.mu.RLock()
	list := make([]*Account, 0, len(m.accounts))
	for _, a := range m.accounts {
		list = append(list, a)
	}
	m.mu.RUnlock()

	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
	return m.vault.Save(list)
}

func (m *Manager) saveOAuthAccount(ctx context.Context, sess *OAuthSession, tok *OAuthToken) (*Account, error) {
	email, name := ExtractEmailFromIDToken(tok.IDToken)
	if email == "" && sess.Provider == ProviderGoogle {
		email, name = fetchGoogleUserInfoEmail(ctx, tok.AccessToken)
	}
	if email == "" && sess.EmailHint != "" {
		email = strings.ToLower(strings.TrimSpace(sess.EmailHint))
	}
	if email == "" {
		return nil, fmt.Errorf("could not determine email address from OAuth response")
	}

	serverCfg := PresetForProvider(sess.Provider)
	acctID := fmt.Sprintf("%s:%s", sess.Provider, email)

	acct := &Account{
		ID:          acctID,
		Email:       email,
		DisplayName: name,
		Provider:    sess.Provider,
		AuthType:    AuthOAuth2,
		Server:      serverCfg,
		Token:       tok,
		ConnectedAt: time.Now(),
	}

	m.mu.Lock()
	if existing, ok := m.accounts[acctID]; ok {
		if tok.RefreshToken == "" && existing.Token != nil {
			acct.Token.RefreshToken = existing.Token.RefreshToken
		}
		acct.LastSeenUID = existing.LastSeenUID
	}
	m.accounts[acctID] = acct
	m.mu.Unlock()

	if err := m.saveAccountsToDisk(); err != nil {
		return nil, err
	}

	if sess.mu != nil {
		sess.mu.Lock()
	}
	sess.Status = "completed"
	sess.AccountID = acct.ID
	sess.Email = acct.Email
	if sess.mu != nil {
		sess.mu.Unlock()
	}

	return acct, nil
}

// AddPasswordAccount uses Autodiscover to find the IMAP/SMTP servers for an email address,
// verifies login via IMAP, and stores the account encrypted at rest.
func (m *Manager) AddPasswordAccount(ctx context.Context, email, password string) (*Account, error) {
	return m.ConnectWithPassword(ctx, email, password, "", nil)
}

// ConnectWithPassword connects an email account via password (either using a
// custom ServerConfig or 3-tier Autodiscover), verifies login over IMAP, and
// stores the credentials encrypted at rest.
func (m *Manager) ConnectWithPassword(ctx context.Context, email, password, displayName string, custom *ServerConfig) (*Account, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return nil, fmt.Errorf("email and password are required")
	}

	var srv ServerConfig
	if custom != nil && custom.IMAPHost != "" && custom.SMTPHost != "" {
		srv = *custom
	} else {
		var err error
		srv, err = Autodiscover(ctx, email)
		if err != nil {
			return nil, err
		}
	}
	srv.AuthType = AuthPassword

	imapClient, err := DialIMAP(srv.IMAPHost, srv.IMAPPort, 15*time.Second)
	if err != nil {
		return nil, err
	}
	defer imapClient.Close()

	if err := imapClient.LoginPassword(email, password); err != nil {
		return nil, err
	}

	acctID := fmt.Sprintf("%s:%s", srv.Provider, email)
	acct := &Account{
		ID:          acctID,
		Email:       email,
		DisplayName: strings.TrimSpace(displayName),
		Provider:    srv.Provider,
		AuthType:    AuthPassword,
		Server:      srv,
		Password:    password,
		ConnectedAt: time.Now(),
	}

	m.mu.Lock()
	m.accounts[acctID] = acct
	m.mu.Unlock()
	if err := m.saveAccountsToDisk(); err != nil {
		return nil, err
	}
	return acct, nil
}

// ListAccounts returns safe copies of all connected accounts (without exposing raw tokens/passwords).
func (m *Manager) ListAccounts() []Account {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Account, 0, len(m.accounts))
	for _, a := range m.accounts {
		cp := *a
		cp.Password = ""
		cp.Token = nil
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

// GetSession returns a thread-safe snapshot of the OAuthSession for the given state.
func (m *Manager) GetSession(state string) (OAuthSession, bool) {
	m.mu.RLock()
	s, ok := m.sessions[state]
	m.mu.RUnlock()
	if !ok {
		return OAuthSession{}, false
	}
	return s.Snapshot(), true
}

// UpdateLastSeenUID persists the High-Water Mark UID for an account.
func (m *Manager) UpdateLastSeenUID(accountID string, uid uint32) error {
	m.mu.Lock()
	acct, ok := m.accounts[accountID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("account not found: %s", accountID)
	}
	if uid <= acct.LastSeenUID {
		m.mu.Unlock()
		return nil
	}
	acct.LastSeenUID = uid
	m.mu.Unlock()
	return m.saveAccountsToDisk()
}

// RemoveAccount deletes a connected account.
func (m *Manager) RemoveAccount(accountID string) error {
	m.mu.Lock()
	delete(m.accounts, accountID)
	m.mu.Unlock()
	return m.saveAccountsToDisk()
}

func (m *Manager) connectAuthenticatedIMAP(ctx context.Context, accountID string) (*Account, *IMAPClient, error) {
	m.mu.RLock()
	acct, ok := m.accounts[accountID]
	m.mu.RUnlock()
	if !ok {
		return nil, nil, fmt.Errorf("account not found: %s", accountID)
	}

	client, err := DialIMAP(acct.Server.IMAPHost, acct.Server.IMAPPort, 20*time.Second)
	if err != nil {
		return nil, nil, err
	}

	if acct.AuthType == AuthOAuth2 {
		token, err := m.EnsureValidToken(ctx, acct)
		if err != nil {
			_ = client.Close()
			return nil, nil, err
		}
		if err := client.AuthenticateXOAUTH2(acct.Email, token); err != nil {
			_ = client.Close()
			return nil, nil, err
		}
	} else {
		if err := client.LoginPassword(acct.Email, acct.Password); err != nil {
			_ = client.Close()
			return nil, nil, err
		}
	}

	return acct, client, nil
}

// FetchInbox connects to the account's INBOX and returns total message count + recent messages.
func (m *Manager) FetchInbox(ctx context.Context, accountID string, limit int) (int, []MessageSummary, error) {
	_, client, err := m.connectAuthenticatedIMAP(ctx, accountID)
	if err != nil {
		return 0, nil, err
	}
	defer client.Close()

	st, err := client.SelectMailboxStatus("INBOX")
	if err != nil {
		return 0, nil, err
	}
	msgs, err := client.FetchRecentSummaries(st.Exists, limit)
	if err != nil {
		return st.Exists, nil, err
	}
	return st.Exists, msgs, nil
}

// FetchMessage fetches the full body, attachments, and structured metadata of a single message.
func (m *Manager) FetchMessage(ctx context.Context, accountID string, seqNum uint32) (*MessageDetail, error) {
	_, client, err := m.connectAuthenticatedIMAP(ctx, accountID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	if _, err := client.SelectMailbox("INBOX"); err != nil {
		return nil, err
	}
	return client.FetchMessageDetail(seqNum)
}

// FetchAttachment fetches a specific attachment from a message by sequence number and filename or part ID.
func (m *Manager) FetchAttachment(ctx context.Context, accountID string, seqNum uint32, partOrName string) (*AttachmentInfo, error) {
	detail, err := m.FetchMessage(ctx, accountID, seqNum)
	if err != nil {
		return nil, err
	}
	for _, att := range detail.Attachments {
		if att.PartID == partOrName || strings.EqualFold(att.Filename, partOrName) {
			cp := att
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("attachment %q not found on message #%d", partOrName, seqNum)
}

// Search searches messages inside INBOX.
func (m *Manager) Search(ctx context.Context, accountID, query string, limit int) ([]MessageSummary, error) {
	_, client, err := m.connectAuthenticatedIMAP(ctx, accountID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	if _, err := client.SelectMailbox("INBOX"); err != nil {
		return nil, err
	}
	return client.SearchMessages(query, limit)
}

// Send sends an email from the specified account.
func (m *Manager) Send(ctx context.Context, req SendRequest) error {
	m.mu.RLock()
	acct, ok := m.accounts[req.AccountID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("account not found: %s", req.AccountID)
	}

	var token string
	if acct.AuthType == AuthOAuth2 {
		var err error
		token, err = m.EnsureValidToken(ctx, acct)
		if err != nil {
			return err
		}
	}
	return SendEmail(acct, token, req)
}

// SaveDraft saves a draft message directly into the remote IMAP Drafts folder.
func (m *Manager) SaveDraft(ctx context.Context, req SendRequest) (string, error) {
	acct, client, err := m.connectAuthenticatedIMAP(ctx, req.AccountID)
	if err != nil {
		return "", err
	}
	defer client.Close()

	return client.AppendDraft(acct.Email, req)
}

// GetAccount returns a safe copy of a single connected account by ID.
func (m *Manager) GetAccount(accountID string) (Account, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	acct, ok := m.accounts[accountID]
	if !ok {
		return Account{}, false
	}
	cp := *acct
	cp.Password = ""
	cp.Token = nil
	return cp, true
}

// SyncAccountIncremental connects to the account's INBOX and performs an
// incremental High-Water Mark sync:
//   - If acct.LastSeenUID == 0 (first poll after connecting a new account),
//     it records the current highest UID (or UIDNEXT-1) as the High-Water Mark
//     and returns coldStart=true with zero messages so pre-existing inbox
//     history never triggers a notification or autonomous reply storm.
//   - If acct.LastSeenUID > 0, it fetches only messages with UID > LastSeenUID
//     (in ascending UID order), updates LastSeenUID on disk, and returns them.
func (m *Manager) SyncAccountIncremental(ctx context.Context, accountID string, maxScan int) (highWaterMark uint32, coldStart bool, newMessages []*MessageDetail, err error) {
	acct, client, err := m.connectAuthenticatedIMAP(ctx, accountID)
	if err != nil {
		return 0, false, nil, err
	}
	defer client.Close()

	st, err := client.SelectMailboxStatus("INBOX")
	if err != nil {
		return 0, false, nil, err
	}

	m.mu.RLock()
	lastUID := acct.LastSeenUID
	m.mu.RUnlock()

	if maxScan <= 0 {
		maxScan = 25
	}

	// Cold-Start High-Water Mark initialization
	if lastUID == 0 {
		var hwm uint32
		if st.UIDNext > 1 {
			hwm = st.UIDNext - 1
		}
		if st.Exists > 0 {
			summaries, ferr := client.FetchRecentSummaries(st.Exists, 5)
			if ferr == nil {
				for _, s := range summaries {
					if s.UID > hwm {
						hwm = s.UID
					}
					if s.UID == 0 && s.SeqNum > hwm {
						hwm = s.SeqNum
					}
				}
			}
		}
		if hwm == 0 {
			// Empty inbox on initial connect: set sentinel 1 so subsequent
			// messages (UID >= 1) are treated as incremental if UID > 0, or if
			// UIDNext == 1 then any first message gets UID >= 1. Wait: if inbox
			// is completely empty and first message will get UID 1, we only
			// advance if hwm > 0.
			return 0, true, nil, nil
		}
		_ = m.UpdateLastSeenUID(accountID, hwm)
		return hwm, true, nil, nil
	}

	if st.Exists == 0 {
		return lastUID, false, nil, nil
	}
	if st.UIDNext > 1 && st.UIDNext-1 <= lastUID {
		return lastUID, false, nil, nil
	}

	summaries, err := client.FetchRecentSummaries(st.Exists, maxScan)
	if err != nil {
		return lastUID, false, nil, err
	}

	// Collect messages with UID > lastUID and sort ascending by UID so we
	// process older messages before newer ones.
	var unseen []MessageSummary
	for _, s := range summaries {
		uid := s.UID
		if uid == 0 {
			uid = s.SeqNum
		}
		if uid > lastUID {
			s.UID = uid
			unseen = append(unseen, s)
		}
	}
	if len(unseen) == 0 {
		return lastUID, false, nil, nil
	}

	sort.Slice(unseen, func(i, j int) bool {
		return unseen[i].UID < unseen[j].UID
	})

	highestUID := lastUID
	for _, s := range unseen {
		detail, derr := client.FetchMessageDetail(s.SeqNum)
		if derr != nil {
			continue
		}
		if detail.UID == 0 {
			detail.UID = s.UID
		}
		if detail.UID > highestUID {
			highestUID = detail.UID
		}
		newMessages = append(newMessages, detail)
	}

	if highestUID > lastUID {
		_ = m.UpdateLastSeenUID(accountID, highestUID)
	}
	return highestUID, false, newMessages, nil
}
