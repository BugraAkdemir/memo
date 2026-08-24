// Package telegram implements a minimal Telegram Bot API client: enough to
// long-poll for incoming messages and send replies back. Unlike
// internal/whatsapp (which emulates a full WhatsApp Web multi-device client
// via whatsmeow, giving it visibility into the user's entire existing
// WhatsApp account), a Telegram bot can only ever see messages sent
// directly to it — there is no equivalent of "read my other chats" without
// the much heavier MTProto user API (phone-number login, not a bot token).
// That scope difference is deliberate: this package exists to let a bot
// token from @BotFather act as another chat surface for talking to Memo,
// not to mirror WhatsApp's contact/group/history breadth.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const apiBase = "https://api.telegram.org/bot"

// Message is a simplified incoming Telegram message.
type Message struct {
	ID        int64
	ChatID    int64
	FromID    int64
	FromName  string // "First Last", falling back to "@username" or the numeric ID
	Username  string
	Text      string
	Timestamp time.Time
}

// BotInfo is the subset of Telegram's getMe response Memo cares about.
type BotInfo struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

// Client manages a long-polling connection to the Telegram Bot API.
type Client struct {
	token      string
	httpClient *http.Client

	msgCh chan Message
	errCh chan error

	mu           sync.Mutex
	started      bool
	reconnecting bool
	lastError    string
	offset       int64
	stopCh       chan struct{}
	stopOnce     sync.Once
}

// NewClient creates a client for the given bot token. The token is not
// validated until Start (or GetMe) is called.
func NewClient(token string) *Client {
	return &Client{
		token:      token,
		httpClient: &http.Client{},
		// Channels are never closed so Start() can be called again after Stop().
		msgCh:  make(chan Message, 256),
		errCh:  make(chan error, 4),
		stopCh: make(chan struct{}),
	}
}

// MessageChannel returns the channel that receives incoming messages.
func (c *Client) MessageChannel() <-chan Message { return c.msgCh }

// ErrorChannel returns the channel that receives poll errors.
func (c *Client) ErrorChannel() <-chan error { return c.errCh }

// LastError returns the last poll error string, or "" if none.
func (c *Client) LastError() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastError
}

// IsReconnecting reports whether the poll loop is currently backing off
// after an error.
func (c *Client) IsReconnecting() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reconnecting
}

// IsRunning reports whether the poll loop is active.
func (c *Client) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.started
}

// GetMe validates the token against the Telegram API and returns basic bot
// info (used both to fail fast on a bad token and to show the bot's own
// @username in Settings).
func (c *Client) GetMe(ctx context.Context) (*BotInfo, error) {
	var resp struct {
		OK          bool    `json:"ok"`
		Result      BotInfo `json:"result"`
		Description string  `json:"description"`
	}
	if err := c.call(ctx, "getMe", nil, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("telegram: %s", resp.Description)
	}
	return &resp.Result, nil
}

// Start validates the token and begins long-polling for updates in the
// background. Thread-safe, no-op if already started.
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	if _, err := c.GetMe(ctx); err != nil {
		return fmt.Errorf("invalid bot token: %w", err)
	}

	c.mu.Lock()
	// Mirrors whatsapp.Client.Start's stopCh/stopOnce recreation: Stop()
	// closes stopCh as a one-shot signal, so a later Start() must hand the
	// poll loop a fresh channel or every future Stop() after the first one
	// would find it already closed and no-op.
	c.stopCh = make(chan struct{})
	c.stopOnce = sync.Once{}
	c.lastError = ""
	c.reconnecting = false
	c.started = true
	c.mu.Unlock()

	// Drain stale entries from a previous session.
	for len(c.msgCh) > 0 {
		<-c.msgCh
	}
	for len(c.errCh) > 0 {
		<-c.errCh
	}

	go c.pollLoop()
	return nil
}

// Stop halts the poll loop. The bot token and offset are kept, so a later
// Start() resumes cleanly.
func (c *Client) Stop() {
	c.mu.Lock()
	stopCh := c.stopCh
	once := &c.stopOnce
	c.mu.Unlock()
	once.Do(func() { close(stopCh) })
}

func (c *Client) pollLoop() {
	defer func() {
		c.mu.Lock()
		c.started = false
		c.mu.Unlock()
	}()

	c.mu.Lock()
	stopCh := c.stopCh
	c.mu.Unlock()

	backoff := time.Second
	const maxBackoff = 30 * time.Second
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		updates, err := c.getUpdates(ctx)
		cancel()

		if err != nil {
			select {
			case <-stopCh:
				return
			default:
			}
			c.mu.Lock()
			c.lastError = err.Error()
			c.reconnecting = true
			c.mu.Unlock()
			select {
			case c.errCh <- err:
			default:
			}
			select {
			case <-time.After(backoff):
			case <-stopCh:
				return
			}
			if backoff < maxBackoff {
				backoff *= 2
			}
			continue
		}

		backoff = time.Second
		c.mu.Lock()
		c.reconnecting = false
		c.mu.Unlock()

		for _, u := range updates {
			c.mu.Lock()
			if u.UpdateID >= c.offset {
				c.offset = u.UpdateID + 1
			}
			c.mu.Unlock()

			if u.Message == nil || u.Message.From == nil || strings.TrimSpace(u.Message.Text) == "" {
				continue
			}
			msg := Message{
				ID:        u.Message.MessageID,
				ChatID:    u.Message.Chat.ID,
				FromID:    u.Message.From.ID,
				FromName:  displayName(u.Message.From),
				Username:  u.Message.From.Username,
				Text:      u.Message.Text,
				Timestamp: time.Unix(u.Message.Date, 0),
			}
			select {
			case c.msgCh <- msg:
			case <-stopCh:
				return
			}
		}
	}
}

// SendMessage sends text to chatID. Telegram caps a single message at 4096
// UTF-16 code units; a longer reply is split into multiple sends (on rune
// boundaries — Turkish/Unicode-safe) rather than silently truncated or
// rejected.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	const maxRunes = 4000
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	for len(runes) > 0 {
		n := len(runes)
		if n > maxRunes {
			n = maxRunes
		}
		chunk := string(runes[:n])
		runes = runes[n:]

		var resp struct {
			OK          bool   `json:"ok"`
			Description string `json:"description"`
		}
		if err := c.call(ctx, "sendMessage", map[string]any{
			"chat_id": chatID,
			"text":    chunk,
		}, &resp); err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("telegram sendMessage: %s", resp.Description)
		}
	}
	return nil
}

// SendDocument uploads the file at filePath to chatID via Telegram's
// sendDocument endpoint, under the given filename (independent of the
// file's actual on-disk name — see whatsapp.Client.SendDocument's doc
// comment for why). Unlike SendMessage/call, this needs a real
// multipart/form-data request (Telegram's Bot API requires it for actual
// file bytes, as opposed to a file_id/URL reference), so it can't reuse
// call's plain JSON POST.
func (c *Client) SendDocument(ctx context.Context, chatID int64, filePath, filename string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("telegram: open file: %w", err)
	}
	defer f.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		return fmt.Errorf("telegram: write chat_id field: %w", err)
	}
	part, err := writer.CreateFormFile("document", filename)
	if err != nil {
		return fmt.Errorf("telegram: create form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return fmt.Errorf("telegram: copy file into request: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("telegram: close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+c.token+"/sendDocument", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var out struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return fmt.Errorf("telegram: decode sendDocument response: %w", err)
	}
	if !out.OK {
		return fmt.Errorf("telegram sendDocument: %s", out.Description)
	}
	return nil
}

// SetTyping sends Telegram's "typing…" chat action. Telegram clears it
// client-side after ~5s if not refreshed, so a caller wanting a longer-lived
// indicator (see startTelegramComposing in internal/app) must resend it
// periodically for as long as generation is in progress — same shape as
// WhatsApp's composing indicator.
func (c *Client) SetTyping(ctx context.Context, chatID int64) error {
	var resp struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := c.call(ctx, "sendChatAction", map[string]any{
		"chat_id": chatID,
		"action":  "typing",
	}, &resp); err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("telegram sendChatAction: %s", resp.Description)
	}
	return nil
}

func (c *Client) getUpdates(ctx context.Context) ([]tgUpdate, error) {
	c.mu.Lock()
	offset := c.offset
	c.mu.Unlock()

	params := map[string]any{
		"timeout":         30,
		"allowed_updates": []string{"message"},
	}
	if offset > 0 {
		params["offset"] = offset
	}

	var resp struct {
		OK          bool       `json:"ok"`
		Result      []tgUpdate `json:"result"`
		Description string     `json:"description"`
	}
	if err := c.call(ctx, "getUpdates", params, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("telegram getUpdates: %s", resp.Description)
	}
	return resp.Result, nil
}

func (c *Client) call(ctx context.Context, method string, params map[string]any, out any) error {
	var body io.Reader
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+c.token+"/"+method, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("telegram: decode %s response: %w", method, err)
	}
	return nil
}

type tgUpdate struct {
	UpdateID int64      `json:"update_id"`
	Message  *tgMessage `json:"message"`
}

type tgMessage struct {
	MessageID int64   `json:"message_id"`
	From      *tgUser `json:"from"`
	Chat      tgChat  `json:"chat"`
	Date      int64   `json:"date"`
	Text      string  `json:"text"`
}

type tgUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type tgChat struct {
	ID int64 `json:"id"`
}

func displayName(u *tgUser) string {
	if u == nil {
		return ""
	}
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name != "" {
		return name
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	return fmt.Sprintf("%d", u.ID)
}
