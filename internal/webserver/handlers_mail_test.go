package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"memo/internal/config"
	"memo/internal/mail"
)

type fakeMailBridge struct {
	mockBridge
	accounts []mail.Account
	cfg      config.MailConfig
	sent     []mail.SendRequest
	drafts   []mail.SendRequest
}

func (f *fakeMailBridge) MailListAccounts() []mail.Account { return f.accounts }
func (f *fakeMailBridge) MailRemoveAccount(accountID string) error {
	for i, a := range f.accounts {
		if a.ID == accountID {
			f.accounts = append(f.accounts[:i], f.accounts[i+1:]...)
			return nil
		}
	}
	return nil
}
func (f *fakeMailBridge) MailAutodiscover(ctx context.Context, emailAddr string) mail.ServerConfig {
	cfg, _ := mail.Autodiscover(ctx, emailAddr)
	return cfg
}
func (f *fakeMailBridge) MailStartOAuth(ctx context.Context, provider, emailAddr string) (mail.OAuthSession, error) {
	return mail.OAuthSession{
		State:    "st-123",
		Provider: mail.ProviderGoogle,
		Status:   "pending",
		AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth?state=st-123",
	}, nil
}
func (f *fakeMailBridge) MailGetOAuthStatus(state string) (mail.OAuthSession, bool) {
	if state == "st-123" {
		return mail.OAuthSession{State: "st-123", Status: "connected", Email: "bugra@gmail.com"}, true
	}
	return mail.OAuthSession{}, false
}
func (f *fakeMailBridge) MailCompleteManualOAuth(ctx context.Context, state, callbackURL, emailAddr string) (*mail.Account, error) {
	return &mail.Account{ID: "google:bugra@gmail.com", Email: "bugra@gmail.com"}, nil
}
func (f *fakeMailBridge) MailConnectPassword(ctx context.Context, emailAddr, password, displayName, imapHost string, imapPort int, smtpHost string, smtpPort int) (*mail.Account, error) {
	acct := mail.Account{ID: "custom:" + emailAddr, Email: emailAddr, DisplayName: displayName}
	f.accounts = append(f.accounts, acct)
	return &acct, nil
}
func (f *fakeMailBridge) MailFetchInbox(ctx context.Context, accountID string, limit int) (int, []mail.MessageSummary, error) {
	return 1, []mail.MessageSummary{
		{SeqNum: 1, UID: 101, Subject: "Test Mail", From: "can@arkadas.com"},
	}, nil
}
func (f *fakeMailBridge) MailFetchMessage(ctx context.Context, accountID string, seqNum uint32) (*mail.MessageDetail, error) {
	return &mail.MessageDetail{
		SeqNum:   seqNum,
		UID:      101,
		Subject:  "Test Mail",
		From:     "can@arkadas.com",
		TextBody: "Kanka xxx projesini merge ettin mi?",
	}, nil
}
func (f *fakeMailBridge) MailFetchAttachment(ctx context.Context, accountID string, seqNum uint32, partOrName string) (*mail.AttachmentInfo, error) {
	return &mail.AttachmentInfo{
		PartID:      "2",
		Filename:    "invite.ics",
		ContentType: "text/calendar",
		Data:        []byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR"),
	}, nil
}
func (f *fakeMailBridge) MailSearch(ctx context.Context, accountID, query string, limit int) ([]mail.MessageSummary, error) {
	return []mail.MessageSummary{
		{SeqNum: 1, UID: 101, Subject: "Test Mail", From: "can@arkadas.com"},
	}, nil
}
func (f *fakeMailBridge) MailSend(ctx context.Context, req mail.SendRequest) error {
	f.sent = append(f.sent, req)
	return nil
}
func (f *fakeMailBridge) MailSaveDraft(ctx context.Context, req mail.SendRequest) (string, error) {
	f.drafts = append(f.drafts, req)
	return "[Gmail]/Taslaklar", nil
}
func (f *fakeMailBridge) MailSyncNow(ctx context.Context) error { return nil }
func (f *fakeMailBridge) MailGetWatcherStats() mail.WatcherStats {
	return mail.WatcherStats{TotalEvaluated: 5, DroppedMarketing: 3, IndexedInRAG: 2}
}
func (f *fakeMailBridge) GetMailConfig() config.MailConfig             { return f.cfg }
func (f *fakeMailBridge) UpdateMailConfig(cfg config.MailConfig) error { f.cfg = cfg; return nil }

func TestMailHTTPHandlers(t *testing.T) {
	fb := &fakeMailBridge{
		accounts: []mail.Account{{ID: "google:bugra@gmail.com", Email: "bugra@gmail.com"}},
		cfg:      config.MailConfig{Enabled: true, PollIntervalSeconds: 60, AutoIndexMemory: true},
	}
	srv := New(fb)

	// 1. GET /api/mail/accounts
	req := httptest.NewRequest(http.MethodGet, "/api/mail/accounts", nil)
	rec := httptest.NewRecorder()
	srv.handleMailAccounts(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/mail/accounts status=%d", rec.Code)
	}

	// 2. GET /api/mail/autodiscover
	req = httptest.NewRequest(http.MethodGet, "/api/mail/autodiscover?email=bugra@gmail.com", nil)
	rec = httptest.NewRecorder()
	srv.handleMailAutodiscover(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/mail/autodiscover status=%d", rec.Code)
	}
	var sc mail.ServerConfig
	_ = json.Unmarshal(rec.Body.Bytes(), &sc)
	if sc.IMAPHost != "imap.gmail.com" {
		t.Fatalf("unexpected autodiscover host: %+v", sc)
	}

	// 3. POST /api/mail/oauth/start & GET /api/mail/oauth/status
	req = httptest.NewRequest(http.MethodPost, "/api/mail/oauth/start", bytes.NewBufferString(`{"provider":"google","email":"bugra@gmail.com"}`))
	rec = httptest.NewRecorder()
	srv.handleMailOAuthStart(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/mail/oauth/start status=%d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/mail/oauth/status?state=st-123", nil)
	rec = httptest.NewRecorder()
	srv.handleMailOAuthStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/mail/oauth/status status=%d", rec.Code)
	}

	// 4. GET /api/mail/inbox & /api/mail/message & /api/mail/attachment
	req = httptest.NewRequest(http.MethodGet, "/api/mail/inbox?account_id=google:bugra@gmail.com", nil)
	rec = httptest.NewRecorder()
	srv.handleMailInbox(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/mail/inbox status=%d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/mail/message?account_id=google:bugra@gmail.com&seq_num=1", nil)
	rec = httptest.NewRecorder()
	srv.handleMailMessage(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/mail/message status=%d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/mail/attachment?account_id=google:bugra@gmail.com&seq_num=1&filename=invite.ics", nil)
	rec = httptest.NewRecorder()
	srv.handleMailAttachment(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("BEGIN:VCALENDAR")) {
		t.Fatalf("GET /api/mail/attachment failed: code=%d body=%q", rec.Code, rec.Body.String())
	}

	// 5. POST /api/mail/draft & POST /api/mail/send
	draftPayload := `{"account_id":"google:bugra@gmail.com","to":["can@arkadas.com"],"subject":"Re: Test","body":"Taslak metni"}`
	req = httptest.NewRequest(http.MethodPost, "/api/mail/draft", bytes.NewBufferString(draftPayload))
	rec = httptest.NewRecorder()
	srv.handleMailDraft(rec, req)
	if rec.Code != http.StatusOK || len(fb.drafts) != 1 {
		t.Fatalf("POST /api/mail/draft failed: code=%d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/mail/send", bytes.NewBufferString(draftPayload))
	rec = httptest.NewRecorder()
	srv.handleMailSend(rec, req)
	if rec.Code != http.StatusOK || len(fb.sent) != 1 {
		t.Fatalf("POST /api/mail/send failed: code=%d", rec.Code)
	}

	// 6. POST /api/mail/sync & GET /api/mail/stats
	req = httptest.NewRequest(http.MethodPost, "/api/mail/sync", nil)
	rec = httptest.NewRecorder()
	srv.handleMailSync(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/mail/sync status=%d", rec.Code)
	}
}
