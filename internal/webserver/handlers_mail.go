package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"memo/internal/autonomy"
	"net/http"
	"strconv"
	"strings"

	"memo/internal/config"
	"memo/internal/mail"
)

func writeMailJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) mailBridge() (MailBridge, bool) {
	if s.fullBridge != nil {
		return s.fullBridge, true
	}
	mb, ok := s.bridge.(MailBridge)
	return mb, ok
}

func (s *Server) handleMailAccounts(w http.ResponseWriter, r *http.Request) {
	mb, ok := s.mailBridge()
	if !ok {
		http.Error(w, "mail subsystem not available", http.StatusNotImplemented)
		return
	}

	switch r.Method {
	case http.MethodGet:
		accts := mb.MailListAccounts()
		if accts == nil {
			accts = []mail.Account{}
		}
		writeMailJSON(w, http.StatusOK, map[string]any{"accounts": accts})

	case http.MethodDelete:
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			var body struct {
				ID string `json:"id"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			id = strings.TrimSpace(body.ID)
		}
		if id == "" {
			writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": "missing account id"})
			return
		}
		if err := mb.MailRemoveAccount(id); err != nil {
			writeMailJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeMailJSON(w, http.StatusOK, map[string]any{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleMailAutodiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mb, ok := s.mailBridge()
	if !ok {
		http.Error(w, "mail subsystem not available", http.StatusNotImplemented)
		return
	}
	emailAddr := strings.TrimSpace(r.URL.Query().Get("email"))
	if emailAddr == "" {
		writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": "missing email parameter"})
		return
	}
	cfg := mb.MailAutodiscover(r.Context(), emailAddr)
	writeMailJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleMailOAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mb, ok := s.mailBridge()
	if !ok {
		http.Error(w, "mail subsystem not available", http.StatusNotImplemented)
		return
	}
	var req struct {
		Provider string `json:"provider"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	sess, err := mb.MailStartOAuth(r.Context(), req.Provider, req.Email)
	if err != nil {
		writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeMailJSON(w, http.StatusOK, sess)
}

func (s *Server) handleMailOAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mb, ok := s.mailBridge()
	if !ok {
		http.Error(w, "mail subsystem not available", http.StatusNotImplemented)
		return
	}
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if state == "" {
		writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": "missing state parameter"})
		return
	}
	sess, found := mb.MailGetOAuthStatus(state)
	if !found {
		writeMailJSON(w, http.StatusNotFound, map[string]string{"error": "oauth session not found"})
		return
	}
	writeMailJSON(w, http.StatusOK, sess)
}

func (s *Server) handleMailOAuthManual(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mb, ok := s.mailBridge()
	if !ok {
		http.Error(w, "mail subsystem not available", http.StatusNotImplemented)
		return
	}
	var req struct {
		State       string `json:"state"`
		CallbackURL string `json:"callback_url"`
		Email       string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	acct, err := mb.MailCompleteManualOAuth(r.Context(), req.State, req.CallbackURL, req.Email)
	if err != nil {
		writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeMailJSON(w, http.StatusOK, map[string]any{"account": acct})
}

func (s *Server) handleMailPasswordConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mb, ok := s.mailBridge()
	if !ok {
		http.Error(w, "mail subsystem not available", http.StatusNotImplemented)
		return
	}
	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		IMAPHost    string `json:"imap_host"`
		IMAPPort    int    `json:"imap_port"`
		SMTPHost    string `json:"smtp_host"`
		SMTPPort    int    `json:"smtp_port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	acct, err := mb.MailConnectPassword(
		r.Context(),
		req.Email,
		req.Password,
		req.DisplayName,
		req.IMAPHost,
		req.IMAPPort,
		req.SMTPHost,
		req.SMTPPort,
	)
	if err != nil {
		writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeMailJSON(w, http.StatusOK, map[string]any{"account": acct})
}

func (s *Server) handleMailInbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mb, ok := s.mailBridge()
	if !ok {
		http.Error(w, "mail subsystem not available", http.StatusNotImplemented)
		return
	}
	accountID := strings.TrimSpace(r.URL.Query().Get("account_id"))
	if accountID == "" {
		writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": "missing account_id parameter"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	total, msgs, err := mb.MailFetchInbox(r.Context(), accountID, limit)
	if err != nil {
		writeMailJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if msgs == nil {
		msgs = []mail.MessageSummary{}
	}
	writeMailJSON(w, http.StatusOK, map[string]any{
		"total":    total,
		"messages": msgs,
	})
}

func (s *Server) handleMailMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mb, ok := s.mailBridge()
	if !ok {
		http.Error(w, "mail subsystem not available", http.StatusNotImplemented)
		return
	}
	accountID := strings.TrimSpace(r.URL.Query().Get("account_id"))
	seqNum, _ := strconv.Atoi(r.URL.Query().Get("seq_num"))
	if accountID == "" || seqNum <= 0 {
		writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": "account_id and positive seq_num are required"})
		return
	}
	detail, err := mb.MailFetchMessage(r.Context(), accountID, uint32(seqNum))
	if err != nil {
		writeMailJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeMailJSON(w, http.StatusOK, detail)
}

func (s *Server) handleMailAttachment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mb, ok := s.mailBridge()
	if !ok {
		http.Error(w, "mail subsystem not available", http.StatusNotImplemented)
		return
	}
	accountID := strings.TrimSpace(r.URL.Query().Get("account_id"))
	seqNum, _ := strconv.Atoi(r.URL.Query().Get("seq_num"))
	part := strings.TrimSpace(r.URL.Query().Get("part"))
	if part == "" {
		part = strings.TrimSpace(r.URL.Query().Get("filename"))
	}
	if accountID == "" || seqNum <= 0 || part == "" {
		writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": "account_id, seq_num, and part/filename are required"})
		return
	}
	att, err := mb.MailFetchAttachment(r.Context(), accountID, uint32(seqNum), part)
	if err != nil {
		writeMailJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	ct := att.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	if att.Filename != "" {
		safeName := strings.ReplaceAll(att.Filename, `"`, "")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, safeName))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(att.Data)
}

func (s *Server) handleMailSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mb, ok := s.mailBridge()
	if !ok {
		http.Error(w, "mail subsystem not available", http.StatusNotImplemented)
		return
	}
	accountID := strings.TrimSpace(r.URL.Query().Get("account_id"))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	if accountID == "" || query == "" {
		writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": "account_id and q are required"})
		return
	}
	msgs, err := mb.MailSearch(r.Context(), accountID, query, limit)
	if err != nil {
		writeMailJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if msgs == nil {
		msgs = []mail.MessageSummary{}
	}
	writeMailJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (s *Server) handleMailSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mb, ok := s.mailBridge()
	if !ok {
		http.Error(w, "mail subsystem not available", http.StatusNotImplemented)
		return
	}
	var req mail.SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := mb.MailSend(r.Context(), req); err != nil {
		writeMailSendError(w, err)
		return
	}
	writeMailJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMailDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mb, ok := s.mailBridge()
	if !ok {
		http.Error(w, "mail subsystem not available", http.StatusNotImplemented)
		return
	}
	var req mail.SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	mailbox, err := mb.MailSaveDraft(r.Context(), req)
	if err != nil {
		writeMailSendError(w, err)
		return
	}
	writeMailJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"mailbox": mailbox,
	})
}

func (s *Server) handleMailSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mb, ok := s.mailBridge()
	if !ok {
		http.Error(w, "mail subsystem not available", http.StatusNotImplemented)
		return
	}
	if err := mb.MailSyncNow(r.Context()); err != nil {
		writeMailJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeMailJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"stats": mb.MailGetWatcherStats(),
	})
}

func (s *Server) handleMailStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mb, ok := s.mailBridge()
	if !ok {
		http.Error(w, "mail subsystem not available", http.StatusNotImplemented)
		return
	}
	writeMailJSON(w, http.StatusOK, mb.MailGetWatcherStats())
}

func (s *Server) handleMailSettings(w http.ResponseWriter, r *http.Request) {
	mb, ok := s.mailBridge()
	if !ok {
		http.Error(w, "mail subsystem not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeMailJSON(w, http.StatusOK, mb.GetMailConfig())
	case http.MethodPut, http.MethodPost:
		var cfg config.MailConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := mb.UpdateMailConfig(cfg); err != nil {
			writeMailJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeMailJSON(w, http.StatusOK, mb.GetMailConfig())
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeMailSendError(w http.ResponseWriter, err error) {
	var denied *autonomy.BlockedError
	if errors.As(err, &denied) {
		writeMailJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "mail_security_blocked", "code": denied.Code})
		return
	}
	writeMailJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
}

type mailSecurityBridge interface {
	MailCheckSecurity(context.Context, string) autonomy.SecurityReport
}

func (s *Server) handleMailSecurityCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var b mailSecurityBridge
	if s.fullBridge != nil {
		b, _ = any(s.fullBridge).(mailSecurityBridge)
	} else {
		b, _ = s.bridge.(mailSecurityBridge)
	}
	if b == nil {
		http.Error(w, "mail security not available", http.StatusNotImplemented)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, autonomy.MaxCheckBytes+1024)
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Text) > autonomy.MaxCheckBytes {
		writeMailJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid security check input"})
		return
	}
	writeMailJSON(w, http.StatusOK, b.MailCheckSecurity(r.Context(), req.Text))
}
