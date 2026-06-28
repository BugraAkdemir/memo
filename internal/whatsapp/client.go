package whatsapp

import (
	"context"
	"fmt"
	"io"
	"memo/internal/logx"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waEvent "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// profilePicTTL is how long a cached avatar (or "no picture" marker) stays
// valid before we re-query WhatsApp for it.
const profilePicTTL = 7 * 24 * time.Hour

// Config holds WhatsApp integration settings.
type Config struct {
	DataDir        string
	MessageStoreDB string
	SessionDB      string
	AutoIndex      bool
	MaxHistoryDays int
}

// Message represents a simplified WhatsApp message for storage.
type Message struct {
	ID         string    `json:"id"`
	ChatJID    string    `json:"chat_jid"`
	SenderJID  string    `json:"sender_jid"`
	SenderName string    `json:"sender_name"`
	Text       string    `json:"text"`
	Timestamp  time.Time `json:"timestamp"`
	FromMe     bool      `json:"from_me"`
}

// Client manages the WhatsApp Web connection.
type Client struct {
	config       Config
	waClient     *whatsmeow.Client
	store        *Store
	qrCodes      []string
	msgCh        chan Message
	errCh        chan error
	started      bool
	reconnecting bool
	lastError    string
	startMu      sync.Mutex
}

// HasRegisteredDevice reports whether a paired WhatsApp session already exists
// on disk (i.e. the user previously scanned the QR and has not logged out).
// Used at startup to auto-reconnect without making the user click "Connect"
// again — a fresh/never-paired install returns false so the welcome screen
// still shows.
func HasRegisteredDevice(ctx context.Context, sessionDB string) bool {
	storeDB, err := sqlstore.New(ctx, "sqlite3",
		"file:"+sessionDB+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", nil)
	if err != nil {
		return false
	}
	defer storeDB.Close()

	device, err := storeDB.GetFirstDevice(ctx)
	if err != nil {
		return false
	}
	// GetFirstDevice returns a fresh, unregistered device (ID == nil) when no
	// session is stored; a non-nil ID means a real paired account.
	return device != nil && device.ID != nil
}

func NewClient(cfg Config) *Client {
	return &Client{
		config: cfg,
		// Channels are never closed so Start() can be called multiple times safely.
		msgCh: make(chan Message, 256),
		errCh: make(chan error, 4),
	}
}

// SetStore attaches a message store to the client.
func (c *Client) SetStore(s *Store) { c.store = s }

// QRCodes returns the most recent batch of QR codes for pairing.
func (c *Client) QRCodes() []string { return c.qrCodes }

// MessageChannel returns the channel that receives incoming messages.
func (c *Client) MessageChannel() <-chan Message { return c.msgCh }

// ErrorChannel returns the channel that receives connection errors.
func (c *Client) ErrorChannel() <-chan error { return c.errCh }

// LastError returns the last connection error string.
func (c *Client) LastError() string { return c.lastError }

// IsReconnecting returns true while an auto-reconnect is in progress.
func (c *Client) IsReconnecting() bool { return c.reconnecting }

// Start connects to WhatsApp Web. Thread-safe, no-op if already started.
func (c *Client) Start(ctx context.Context) error {
	c.startMu.Lock()
	defer c.startMu.Unlock()
	if c.started {
		return nil
	}

	// Drain stale entries from a previous session.
	for len(c.msgCh) > 0 {
		<-c.msgCh
	}
	for len(c.errCh) > 0 {
		<-c.errCh
	}
	c.qrCodes = nil
	c.lastError = ""

	storeDB, err := sqlstore.New(ctx, "sqlite3",
		"file:"+c.config.SessionDB+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", nil)
	if err != nil {
		return fmt.Errorf("whatsapp: session db: %w", err)
	}

	deviceStore, err := storeDB.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("whatsapp: get device: %w", err)
	}

	c.waClient = whatsmeow.NewClient(deviceStore, nil)
	c.waClient.AddEventHandler(c.handleEvent)

	if err := c.waClient.Connect(); err != nil {
		return fmt.Errorf("whatsapp: connect: %w", err)
	}

	c.started = true

	// If we have a stored session, wait for the Connected event so
	// IsLoggedIn() is accurate before the first status poll hits.
	if deviceStore != nil {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if c.waClient.IsLoggedIn() {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			time.Sleep(250 * time.Millisecond)
		}
	}

	// Import contacts and group names without blocking Start().
	go c.importContacts()
	go c.importGroups()

	return nil
}

// Stop disconnects from WhatsApp Web without closing channels (so Start can be called again).
func (c *Client) Stop() {
	c.startMu.Lock()
	defer c.startMu.Unlock()
	if c.waClient != nil {
		c.waClient.Disconnect()
		c.waClient = nil
	}
	c.started = false
	c.reconnecting = false
}

// Logout disconnects and removes the local session so the next Start() will show a QR code.
func (c *Client) Logout() error {
	c.startMu.Lock()
	defer c.startMu.Unlock()
	if c.waClient == nil {
		c.started = false
		c.reconnecting = false
		c.qrCodes = nil
		return nil
	}

	// whatsmeow's Logout() first sends a network IQ to deregister the device and
	// only deletes the local session afterwards. With no deadline that call can
	// hang indefinitely on a flaky network — and because startMu is held here it
	// would freeze every WhatsApp operation and the HTTP handler behind it. Bound
	// it so logout always returns promptly.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store := c.waClient.Store
	if err := c.waClient.Logout(ctx); err != nil {
		// Remote deregister failed/timed out, so the local session was NOT
		// cleared. Delete it ourselves — otherwise the next Start() would
		// silently reuse a dead session instead of showing a fresh QR code.
		logx.Printf("WhatsApp: remote logout failed (%v); clearing local session", err)
		c.waClient.Disconnect()
		if store != nil {
			delCtx, delCancel := context.WithTimeout(context.Background(), 5*time.Second)
			if derr := store.Delete(delCtx); derr != nil {
				logx.Printf("WhatsApp: local session delete error: %v", derr)
			}
			delCancel()
		}
	}

	c.waClient = nil
	c.started = false
	c.reconnecting = false
	c.qrCodes = nil
	return nil
}

// IsConnected returns whether the client is connected.
func (c *Client) IsConnected() bool {
	return c.waClient != nil && c.waClient.IsConnected()
}

// IsLoggedIn returns whether a valid session exists (no QR needed).
func (c *Client) IsLoggedIn() bool {
	return c.waClient != nil && c.waClient.IsLoggedIn()
}

// SendMessage sends a text message to a JID and saves it to the local store.
func (c *Client) SendMessage(ctx context.Context, jid, text string) (string, error) {
	if !c.IsConnected() || !c.IsLoggedIn() {
		return "", fmt.Errorf("whatsapp: not connected")
	}
	parsedJID, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("whatsapp: invalid JID: %w", err)
	}
	resp, err := c.waClient.SendMessage(ctx, parsedJID, &waE2E.Message{
		Conversation: proto.String(text),
	})
	if err != nil {
		return "", fmt.Errorf("whatsapp: send: %w", err)
	}

	if c.store != nil {
		myJID := ""
		if c.waClient.Store != nil {
			myJID = c.waClient.Store.ID.String()
		}
		_ = c.store.SaveMessage(Message{
			ID:         resp.ID,
			ChatJID:    jid,
			SenderJID:  myJID,
			SenderName: "Ben",
			Text:       text,
			Timestamp:  time.Now(),
			FromMe:     true,
		})
	}

	return resp.ID, nil
}

// SearchMessages searches WhatsApp messages via the store.
func (c *Client) SearchMessages(query string, limit int) ([]Message, error) {
	if c.store == nil {
		return nil, fmt.Errorf("whatsapp: store not available")
	}
	return c.store.SearchMessages(query, limit)
}

// GetChatList returns the chat list from the store.
func (c *Client) GetChatList() ([]ChatSummary, error) {
	if c.store == nil {
		return nil, fmt.Errorf("whatsapp: store not available")
	}
	return c.store.GetChatList()
}

// GetChatMessages returns messages for a specific chat from the store.
func (c *Client) GetChatMessages(chatJID string, limit int) ([]Message, error) {
	if c.store == nil {
		return nil, fmt.Errorf("whatsapp: store not available")
	}
	return c.store.GetChatMessages(chatJID, limit)
}

// GetProfilePicture returns the JPEG bytes of a contact/group profile picture,
// caching the result on disk. It returns (nil, nil) when the JID has no picture
// or it is hidden — callers fall back to a letter avatar without retrying every
// time. Cached entries (and "no picture" markers) refresh after profilePicTTL.
// When preview is true a small thumbnail is fetched (fast, for list avatars);
// when false the full-resolution photo is fetched (for the enlarged preview).
func (c *Client) GetProfilePicture(ctx context.Context, jid string, preview bool) ([]byte, error) {
	if c.waClient == nil {
		return nil, fmt.Errorf("whatsapp: not connected")
	}

	dir := filepath.Join(c.config.DataDir, "avatars")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("whatsapp: avatar dir: %w", err)
	}
	safe := sanitizeJID(jid)
	// Thumbnail and full-res are cached under separate names.
	suffix := "_full"
	if preview {
		suffix = ""
	}
	imgPath := filepath.Join(dir, safe+suffix+".jpg")
	nonePath := filepath.Join(dir, safe+suffix+".none")

	// Serve from cache while fresh.
	if fi, err := os.Stat(imgPath); err == nil && time.Since(fi.ModTime()) < profilePicTTL {
		return os.ReadFile(imgPath)
	}
	if fi, err := os.Stat(nonePath); err == nil && time.Since(fi.ModTime()) < profilePicTTL {
		return nil, nil
	}

	parsed, err := types.ParseJID(jid)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: invalid JID: %w", err)
	}

	info, err := c.waClient.GetProfilePictureInfo(ctx, parsed, &whatsmeow.GetProfilePictureParams{Preview: preview})
	if err != nil {
		return nil, fmt.Errorf("whatsapp: profile pic info: %w", err)
	}
	if info == nil || info.URL == "" {
		// No picture available — drop a marker so we don't hammer the server.
		if err := os.WriteFile(nonePath, nil, 0644); err != nil {
			logx.Printf("whatsapp: write none marker: %v", err)
		}
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: download avatar: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("whatsapp: download avatar: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(imgPath, data, 0644); err != nil {
		logx.Printf("WhatsApp: cache avatar write error: %v", err)
	}
	return data, nil
}

// sanitizeJID turns a JID into a filesystem-safe cache filename.
func sanitizeJID(jid string) string {
	var b strings.Builder
	for _, r := range jid {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// handleEvent processes incoming WhatsApp events.
func (c *Client) handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *waEvent.QR:
		c.qrCodes = v.Codes
	case *waEvent.PairSuccess:
		logx.Printf("WhatsApp: paired successfully with %s", v.ID)
		c.qrCodes = nil
		c.lastError = ""
	case *waEvent.PairError:
		logx.Printf("WhatsApp: pair error: %v", v.Error)
		c.lastError = v.Error.Error()
		select {
		case c.errCh <- v.Error:
		default:
		}
	case *waEvent.Connected:
		logx.Printf("WhatsApp: connected")
		c.reconnecting = false
		c.lastError = ""
	case *waEvent.Disconnected:
		logx.Printf("WhatsApp: disconnected")
		c.lastError = "connection lost"
		select {
		case c.errCh <- fmt.Errorf("disconnected"):
		default:
		}
		// Auto-reconnect if Start() was called (session exists or QR flow).
		if c.started {
			go c.autoReconnect()
		}
	case *waEvent.LoggedOut:
		logx.Printf("WhatsApp: logged out remotely")
		c.lastError = "logged out"
		c.qrCodes = nil
		c.started = false
	case *waEvent.StreamReplaced:
		logx.Printf("WhatsApp: stream replaced")
	case *waEvent.HistorySync:
		go c.handleHistorySync(v)
	case *waEvent.Message:
		c.handleMessage(v)
	}
}

// autoReconnect attempts to reconnect with exponential backoff.
func (c *Client) autoReconnect() {
	c.reconnecting = true
	backoff := []time.Duration{5 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second}
	for attempt, delay := range backoff {
		time.Sleep(delay)

		c.startMu.Lock()
		alive := c.started && c.waClient != nil
		c.startMu.Unlock()

		if !alive || c.IsConnected() {
			c.reconnecting = false
			return
		}

		logx.Printf("WhatsApp: reconnect attempt %d...", attempt+1)
		if err := c.waClient.Connect(); err != nil {
			logx.Printf("WhatsApp: reconnect error: %v", err)
			c.lastError = fmt.Sprintf("reconnect failed (%d/%d)", attempt+1, len(backoff))
			continue
		}
		logx.Printf("WhatsApp: reconnected")
		c.reconnecting = false
		return
	}
	c.reconnecting = false
	c.lastError = "reconnect failed — please reconnect manually"
	logx.Printf("WhatsApp: auto-reconnect exhausted")
}

// handleHistorySync imports historical messages after first pairing.
func (c *Client) handleHistorySync(evt *waEvent.HistorySync) {
	if evt.Data == nil || c.store == nil {
		return
	}
	convs := evt.Data.GetConversations()
	logx.Printf("WhatsApp: history sync — %d conversations", len(convs))
	myJID := ""
	if c.waClient != nil && c.waClient.Store != nil {
		myJID = c.waClient.Store.ID.String()
	}
	count := 0
	for _, conv := range convs {
		for _, hsm := range conv.GetMessages() {
			if hsm == nil || hsm.Message == nil {
				continue
			}
			wmi := hsm.Message
			key := wmi.GetKey()
			if key == nil {
				continue
			}
			msgText := extractText(wmi.GetMessage())
			if msgText == "" {
				continue
			}
			fromMe := key.GetFromMe()
			senderJID := key.GetRemoteJID()
			senderName := c.resolveDisplayName(senderJID)
			if fromMe {
				senderJID = myJID
				senderName = "Ben"
			}
			msg := Message{
				ID:         key.GetID(),
				ChatJID:    key.GetRemoteJID(),
				SenderJID:  senderJID,
				SenderName: senderName,
				Text:       msgText,
				Timestamp:  time.Unix(int64(wmi.GetMessageTimestamp()), 0),
				FromMe:     fromMe,
			}
			if err := c.store.SaveMessage(msg); err != nil {
				logx.Printf("WhatsApp: save history msg error: %v", err)
			}
			count++
		}
	}
	logx.Printf("WhatsApp: history sync saved %d messages", count)
}

// extractText extracts the text content from a WhatsApp message.
func extractText(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}
	if t := msg.GetConversation(); t != "" {
		return t
	}
	if t := msg.GetExtendedTextMessage().GetText(); t != "" {
		return t
	}
	if t := msg.GetImageMessage().GetCaption(); t != "" {
		return t
	}
	if t := msg.GetVideoMessage().GetCaption(); t != "" {
		return t
	}
	return msg.GetDocumentMessage().GetCaption()
}

// handleMessage processes an incoming live message.
func (c *Client) handleMessage(evt *waEvent.Message) {
	info := evt.Info
	text := evt.Message.GetConversation()
	if text == "" {
		text = evt.Message.GetExtendedTextMessage().GetText()
	}
	if text == "" {
		return
	}

	senderJID := info.Sender.String()
	senderName := info.PushName

	if senderName != "" && c.store != nil {
		_ = c.store.SaveContact(senderJID, senderName)
	}
	if senderName == "" {
		senderName = c.resolveDisplayName(senderJID)
	}
	if info.IsFromMe {
		senderName = "Ben"
		if c.waClient != nil && c.waClient.Store != nil {
			senderJID = c.waClient.Store.ID.String()
		}
	}

	msg := Message{
		ID:         info.ID,
		ChatJID:    info.Chat.String(),
		SenderJID:  senderJID,
		SenderName: senderName,
		Text:       text,
		Timestamp:  info.Timestamp,
		FromMe:     info.IsFromMe,
	}

	if c.store != nil {
		if err := c.store.SaveMessage(msg); err != nil {
			logx.Printf("WhatsApp: save message error: %v", err)
		}
	}

	select {
	case c.msgCh <- msg:
	default:
	}
}

// resolveDisplayName resolves a JID to a human-readable name.
func (c *Client) resolveDisplayName(jid string) string {
	if c.store != nil {
		if name := c.store.GetContactName(jid); name != "" {
			return name
		}
	}
	if c.waClient != nil && c.waClient.Store != nil {
		parsed, err := types.ParseJID(jid)
		if err == nil {
			contact, err2 := c.waClient.Store.Contacts.GetContact(context.Background(), parsed)
			if err2 == nil {
				if contact.FullName != "" {
					return contact.FullName
				}
				if contact.FirstName != "" {
					return contact.FirstName
				}
				if contact.PushName != "" {
					return contact.PushName
				}
			}
		}
	}
	return partsBeforeAt(jid)
}

// importContacts copies all contacts from whatsmeow into the local contacts table.
func (c *Client) importContacts() {
	if c.waClient == nil || c.waClient.Store == nil || c.waClient.Store.Contacts == nil || c.store == nil {
		return
	}
	contacts, err := c.waClient.Store.Contacts.GetAllContacts(context.Background())
	if err != nil || len(contacts) == 0 {
		return
	}
	count := 0
	for jid, contact := range contacts {
		name := contact.FullName
		if name == "" {
			name = contact.FirstName
		}
		if name == "" {
			name = contact.PushName
		}
		if name == "" {
			continue
		}
		if err := c.store.SaveContact(jid.String(), name); err != nil {
			logx.Printf("WhatsApp: save contact %s error: %v", jid, err)
		}
		count++
	}
	logx.Printf("WhatsApp: imported %d contacts", count)
}

// importGroups stores the name of every joined group in the contacts table so
// group chats resolve to a readable name instead of their raw numeric JID
// (e.g. "120363...@g.us"). Individual contacts come from importContacts; groups
// are not contacts, so without this they show as bare numbers in the chat list.
func (c *Client) importGroups() {
	if c.waClient == nil || c.store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	groups, err := c.waClient.GetJoinedGroups(ctx)
	if err != nil {
		logx.Printf("WhatsApp: get joined groups error: %v", err)
		return
	}
	count := 0
	for _, g := range groups {
		if g == nil || g.Name == "" {
			continue
		}
		if err := c.store.SaveContact(g.JID.String(), g.Name); err != nil {
			logx.Printf("WhatsApp: save group %s error: %v", g.JID, err)
			continue
		}
		count++
	}
	logx.Printf("WhatsApp: imported %d group names", count)
}
