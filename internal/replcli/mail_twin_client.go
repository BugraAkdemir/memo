package replcli

import (
	"context"
	"fmt"
	"memo/internal/mail"
	"memo/internal/twin"
	"net/http"
	"net/url"
	"strconv"
)

type MailInboxResponse struct {
	Total    int                   `json:"total"`
	Messages []mail.MessageSummary `json:"messages"`
}

type TwinCommitmentView struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Channel     string `json:"channel"`
	Account     string `json:"account"`
	ContactID   string `json:"contact_id"`
	ContactName string `json:"contact_name"`
	Address     string `json:"address"`
	Subject     string `json:"subject"`
	Summary     string `json:"summary"`
	AgeHours    int    `json:"age_hours"`
	Drafted     bool   `json:"drafted"`
}

type TwinStateResponse struct {
	Contacts    []twin.Contact       `json:"contacts"`
	Actions     []twin.Action        `json:"actions"`
	Commitments []TwinCommitmentView `json:"commitments"`
	Briefings   []twin.Briefing      `json:"briefings"`
	AuditLog    []twin.AuditEntry    `json:"audit_log"`
	Preferences twin.Preferences     `json:"preferences"`
}

func (c *Client) MailAccounts(ctx context.Context) ([]mail.Account, error) {
	var resp struct {
		Accounts []mail.Account `json:"accounts"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/mail/accounts", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Accounts, nil
}

func (c *Client) MailInbox(ctx context.Context, accountID string, limit int) (*MailInboxResponse, error) {
	if limit <= 0 {
		limit = 10
	}
	q := url.Values{}
	q.Set("account_id", accountID)
	q.Set("limit", strconv.Itoa(limit))
	var resp MailInboxResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/mail/inbox?"+q.Encode(), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) MailRead(ctx context.Context, accountID string, seqNum int) (*mail.MessageDetail, error) {
	q := url.Values{}
	q.Set("account_id", accountID)
	q.Set("seq_num", strconv.Itoa(seqNum))
	var detail mail.MessageDetail
	if err := c.doJSON(ctx, http.MethodGet, "/api/mail/message?"+q.Encode(), nil, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func (c *Client) MailSearch(ctx context.Context, accountID, query string, limit int) ([]mail.MessageSummary, error) {
	if limit <= 0 {
		limit = 10
	}
	q := url.Values{}
	q.Set("account_id", accountID)
	q.Set("q", query)
	q.Set("limit", strconv.Itoa(limit))
	var resp struct {
		Messages []mail.MessageSummary `json:"messages"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/mail/search?"+q.Encode(), nil, &resp); err != nil {
		return nil, err
	}
	return resp.Messages, nil
}

func (c *Client) MailSync(ctx context.Context) (*mail.WatcherStats, error) {
	var resp struct {
		OK    bool              `json:"ok"`
		Stats mail.WatcherStats `json:"stats"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/mail/sync", map[string]any{}, &resp); err != nil {
		return nil, err
	}
	return &resp.Stats, nil
}

func (c *Client) MailStats(ctx context.Context) (*mail.WatcherStats, error) {
	var stats mail.WatcherStats
	if err := c.doJSON(ctx, http.MethodGet, "/api/mail/stats", nil, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

func (c *Client) TwinState(ctx context.Context) (*TwinStateResponse, error) {
	var st TwinStateResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/twin/state", nil, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func (c *Client) TwinSavePreferences(ctx context.Context, prefs twin.Preferences) error {
	return c.doJSON(ctx, http.MethodPost, "/api/twin/preferences", prefs, nil)
}

func (c *Client) TwinActionOp(ctx context.Context, op, idOrCode string) error {
	switch op {
	case "approve_action", "cancel_action", "send_action":
	default:
		return fmt.Errorf("unsupported twin op: %s", op)
	}
	return c.doJSON(ctx, http.MethodPost, "/api/twin/"+op, map[string]any{"id": idOrCode, "code": idOrCode}, nil)
}
