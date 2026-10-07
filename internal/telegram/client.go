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

// apiBase is a var (not const) so tests can point it at an httptest.Server
// instead of the real Telegram API — same pattern the other provider packages use
// for their own endpoint constants.
var apiBase = "https://api.telegram.org/bot"

// fileBase is where getFile's file_path is downloaded from (the Bot API serves
// file bytes from a different path than its methods). A var for the same reason
// as apiBase.
var fileBase = "https://api.telegram.org/file/bot"

// EnvAPIBase points the client at another Bot API server instead of
// api.telegram.org — a self-hosted telegram-bot-api (which lifts the 20 MB file
// limit), or a fake one in an end-to-end test. The value is the server's origin,
// e.g. "http://127.0.0.1:8081"; the "/bot<token>" and "/file/bot<token>" parts
// are added here.
const EnvAPIBase = "MEMO_TELEGRAM_API_BASE"

// MaxDownloadBytes caps a file fetched from Telegram. The Bot API itself refuses
// files over 20 MB through getFile.
const MaxDownloadBytes = 20 << 20

// Message is a simplified incoming Telegram message.
type Message struct {
	ID        int64
	ChatID    int64
	FromID    int64
	FromName  string // "First Last", falling back to "@username" or the numeric ID
	Username  string
	Text      string // the message text, or the caption of a photo/image file
	Timestamp time.Time
	// Image is set when the message carries a picture (a photo, or a file sent as
	// an image document). Fetch its bytes with Client.DownloadFile.
	Image *Media
}

// Media points at a file held by Telegram.
type Media struct {
	FileID   string
	MimeType string // "image/jpeg" for a photo; the document's own type otherwise
	Size     int64
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

	// apiOverride/fileOverride replace the package-level bases for this client
	// (see EnvAPIBase); empty means "use apiBase / fileBase".
	apiOverride  string
	fileOverride string

	msgCh chan Message
	errCh chan error

	// startMu serializes Start() end-to-end (see Start's own comment) —
	// separate from mu so it doesn't also block unrelated concurrent reads
	// (GetMe, Stop, message polling) for the whole duration of a Start call.
	startMu sync.Mutex

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
	c := &Client{
		token:      token,
		httpClient: &http.Client{},
		// Channels are never closed so Start() can be called again after Stop().
		msgCh:  make(chan Message, 256),
		errCh:  make(chan error, 4),
		stopCh: make(chan struct{}),
	}
	if origin := strings.TrimRight(strings.TrimSpace(os.Getenv(EnvAPIBase)), "/"); origin != "" {
		c.apiOverride = origin + "/bot"
		c.fileOverride = origin + "/file/bot"
	}
	return c
}

func (c *Client) apiURL() string {
	if c.apiOverride != "" {
		return c.apiOverride
	}
	return apiBase
}

func (c *Client) fileURL() string {
	if c.fileOverride != "" {
		return c.fileOverride
	}
	return fileBase
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
	// Mirrors whatsapp.Client.Start's startMu: held for this whole call so
	// two concurrent Start()s can't both observe !c.started, both dial
	// GetMe, and both end up launching their own pollLoop — the second
	// clobbering the first's stopCh/stopOnce (see below) and leaving two
	// goroutines racing to consume the same msgCh/errCh. No known caller
	// actually does this today, but nothing before this fix made it safe.
	c.startMu.Lock()
	defer c.startMu.Unlock()

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

			if u.Message == nil || u.Message.From == nil {
				continue
			}
			text := u.Message.Text
			if strings.TrimSpace(text) == "" {
				text = u.Message.Caption
			}
			image := imageOf(u.Message)
			if strings.TrimSpace(text) == "" && image == nil {
				continue
			}
			msg := Message{
				ID:        u.Message.MessageID,
				ChatID:    u.Message.Chat.ID,
				FromID:    u.Message.From.ID,
				FromName:  displayName(u.Message.From),
				Username:  u.Message.From.Username,
				Text:      text,
				Timestamp: time.Unix(u.Message.Date, 0),
				Image:     image,
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL()+c.token+"/sendDocument", &body)
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

// DownloadFile fetches a file Telegram holds (getFile, then the file URL) and
// returns its bytes, refusing anything over MaxDownloadBytes.
func (c *Client) DownloadFile(ctx context.Context, fileID string) ([]byte, error) {
	var resp struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Result      struct {
			FilePath string `json:"file_path"`
			FileSize int64  `json:"file_size"`
		} `json:"result"`
	}
	if err := c.call(ctx, "getFile", map[string]any{"file_id": fileID}, &resp); err != nil {
		return nil, err
	}
	if !resp.OK || resp.Result.FilePath == "" {
		return nil, fmt.Errorf("telegram getFile: %s", resp.Description)
	}
	if resp.Result.FileSize > MaxDownloadBytes {
		return nil, fmt.Errorf("telegram: file is %d bytes, over the %d byte limit", resp.Result.FileSize, MaxDownloadBytes)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.fileURL()+c.token+"/"+resp.Result.FilePath, nil)
	if err != nil {
		return nil, err
	}
	r, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram: file download answered %d", r.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, MaxDownloadBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxDownloadBytes {
		return nil, fmt.Errorf("telegram: file is over the %d byte limit", MaxDownloadBytes)
	}
	return data, nil
}

// SendPhoto uploads image bytes to chatID as a photo (sendPhoto), with an
// optional caption. Telegram recompresses photos and rejects extreme sizes or
// proportions; a caller that wants the picture to arrive regardless falls back
// to SendDocumentBytes on error.
func (c *Client) SendPhoto(ctx context.Context, chatID int64, data []byte, filename, caption string) error {
	return c.sendUpload(ctx, "sendPhoto", "photo", chatID, data, filename, caption)
}

// SendDocumentBytes uploads in-memory bytes as a document (no recompression).
func (c *Client) SendDocumentBytes(ctx context.Context, chatID int64, data []byte, filename, caption string) error {
	return c.sendUpload(ctx, "sendDocument", "document", chatID, data, filename, caption)
}

func (c *Client) sendUpload(ctx context.Context, method, field string, chatID int64, data []byte, filename, caption string) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		return err
	}
	if caption != "" {
		// Telegram caps a caption at 1024 characters.
		if r := []rune(caption); len(r) > 1000 {
			caption = string(r[:1000])
		}
		if err := writer.WriteField("caption", caption); err != nil {
			return err
		}
	}
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL()+c.token+"/"+method, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var out struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("telegram: decode %s response: %w", method, err)
	}
	if !out.OK {
		return fmt.Errorf("telegram %s: %s", method, out.Description)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL()+c.token+"/"+method, body)
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
	MessageID int64         `json:"message_id"`
	From      *tgUser       `json:"from"`
	Chat      tgChat        `json:"chat"`
	Date      int64         `json:"date"`
	Text      string        `json:"text"`
	Caption   string        `json:"caption"`
	Photo     []tgPhotoSize `json:"photo"`
	Document  *tgDocument   `json:"document"`
}

type tgPhotoSize struct {
	FileID   string `json:"file_id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	FileSize int64  `json:"file_size"`
}

type tgDocument struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	FileSize int64  `json:"file_size"`
}

// imageOf returns the picture a message carries, if any: the largest size of a
// photo, or a document whose type says it is an image (a picture sent "as a
// file" to keep it uncompressed).
func imageOf(m *tgMessage) *Media {
	if len(m.Photo) > 0 {
		best := m.Photo[0]
		for _, p := range m.Photo[1:] {
			if p.Width*p.Height > best.Width*best.Height {
				best = p
			}
		}
		return &Media{FileID: best.FileID, MimeType: "image/jpeg", Size: best.FileSize}
	}
	if d := m.Document; d != nil && strings.HasPrefix(strings.ToLower(d.MimeType), "image/") {
		return &Media{FileID: d.FileID, MimeType: strings.ToLower(d.MimeType), Size: d.FileSize}
	}
	return nil
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
