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
	storeDB      *sqlstore.Container
	store        *Store
	qrCodes      []string
	msgCh        chan Message
	errCh        chan error
	started      bool
	reconnecting bool
	lastError    string
	startMu      sync.Mutex
	stopCh       chan struct{}
	stopOnce     sync.Once

	// selfSentIDs remembers message IDs this client itself just sent via
	// SendMessage, so a caller reacting to incoming self-chat messages (see
	// OwnJID's doc comment) can recognize and skip its own outgoing replies
	// instead of treating them as a new incoming command — regardless of
	// whether the multi-device sync protocol ever actually echoes a send
	// back through the event stream, this is a correct guard either way.
	selfSentMu  sync.Mutex
	selfSentIDs map[string]time.Time
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
	// whatsmeow's sqlstore creates the file itself (Signal/Noise session keys)
	// at the process umask — harden it, since this probe may be what creates
	// it on a fresh install.
	if err := os.Chmod(sessionDB, 0o600); err != nil {
		logx.Printf("whatsapp: chmod session db %q: %v", sessionDB, err)
	}

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
		msgCh:  make(chan Message, 256),
		errCh:  make(chan error, 4),
		stopCh: make(chan struct{}),
	}
}

// SetStore attaches a message store to the client.
func (c *Client) SetStore(s *Store) { c.store = s }

// QRCodes returns the most recent batch of QR codes for pairing.
func (c *Client) QRCodes() []string {
	c.startMu.Lock()
	defer c.startMu.Unlock()
	return c.qrCodes
}

// MessageChannel returns the channel that receives incoming messages.
func (c *Client) MessageChannel() <-chan Message { return c.msgCh }

// ErrorChannel returns the channel that receives connection errors.
func (c *Client) ErrorChannel() <-chan error { return c.errCh }

// LastError returns the last connection error string.
func (c *Client) LastError() string {
	c.startMu.Lock()
	defer c.startMu.Unlock()
	return c.lastError
}

// IsReconnecting returns true while an auto-reconnect is in progress.
func (c *Client) IsReconnecting() bool {
	c.startMu.Lock()
	defer c.startMu.Unlock()
	return c.reconnecting
}

// Start connects to WhatsApp Web. Thread-safe, no-op if already started.
func (c *Client) Start(ctx context.Context) error {
	c.startMu.Lock()
	defer c.startMu.Unlock()
	if c.started {
		return nil
	}

	// Stop() closes stopCh via stopOnce as a one-shot "please abort" signal
	// to any in-flight autoReconnect goroutine. Both are recreated here so a
	// Stop() *after* this Start() can signal a fresh autoReconnect the same
	// way — without this, stopCh stayed permanently closed from the very
	// first Stop() ever called, so any autoReconnect spawned after a later
	// Stop()+Start() cycle saw an already-closed channel and returned
	// immediately, permanently disabling auto-reconnect until the process
	// restarts.
	c.stopCh = make(chan struct{})
	c.stopOnce = sync.Once{}

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
	// whatsmeow's sqlstore creates the file itself (Signal/Noise session keys)
	// at the process umask — harden it.
	if err := os.Chmod(c.config.SessionDB, 0o600); err != nil {
		logx.Printf("whatsapp: chmod session db %q: %v", c.config.SessionDB, err)
	}

	deviceStore, err := storeDB.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("whatsapp: get device: %w", err)
	}
	c.storeDB = storeDB

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
	logx.GoRecover("whatsapp.Client.importContacts", c.importContacts)
	logx.GoRecover("whatsapp.Client.importGroups", c.importGroups)

	return nil
}

// Stop disconnects from WhatsApp Web and cancels auto-reconnect.
// Channels are not closed so Start() can be called again.
func (c *Client) Stop() {
	c.startMu.Lock()
	defer c.startMu.Unlock()
	// Under startMu so this can never race Start()'s reassignment of
	// stopCh/stopOnce (see the comment there) — both fields are now only
	// ever touched while holding this lock.
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	if c.waClient != nil {
		c.waClient.Disconnect()
		c.waClient = nil
	}
	// storeDB (whatsmeow's session.db) is opened in Start() and otherwise
	// never closed anywhere — on Windows an open handle into data/whatsapp/
	// blocks a full-data-wipe's os.RemoveAll outright ("used by another
	// process"), unlike Linux where unlinking an open file just works.
	if c.storeDB != nil {
		if err := c.storeDB.Close(); err != nil {
			logx.Printf("whatsapp: session db close: %v", err)
		}
		c.storeDB = nil
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
	c.startMu.Lock()
	defer c.startMu.Unlock()
	return c.waClient != nil && c.waClient.IsConnected()
}

// IsLoggedIn returns whether a valid session exists (no QR needed).
func (c *Client) IsLoggedIn() bool {
	c.startMu.Lock()
	defer c.startMu.Unlock()
	return c.waClient != nil && c.waClient.IsLoggedIn()
}

// SendMessage sends a text message to a JID and saves it to the local store.
func (c *Client) SendMessage(ctx context.Context, jid, text string) (string, error) {
	c.startMu.Lock()
	wa := c.waClient
	store := c.store
	c.startMu.Unlock()
	if wa == nil || !wa.IsConnected() || !wa.IsLoggedIn() {
		return "", fmt.Errorf("whatsapp: not connected")
	}
	parsedJID, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("whatsapp: invalid JID: %w", err)
	}
	resp, err := wa.SendMessage(ctx, parsedJID, &waE2E.Message{
		Conversation: proto.String(text),
	})
	if err != nil {
		return "", fmt.Errorf("whatsapp: send: %w", err)
	}

	c.markSelfSent(resp.ID)

	if store != nil {
		myJID := ""
		if wa.Store != nil {
			myJID = wa.Store.ID.String()
		}
		// BUG-M5: this used to discard the error entirely (`_ = ...`) —
		// unlike handleMessage's identical SaveMessage call for the
		// receive path, which logs a failure. The WhatsApp send itself
		// already succeeded (resp.ID is real) by this point, so a local
		// save failure here (disk full, WAL lock contention, permissions)
		// silently lost the sent message from local history/search with
		// zero diagnostic trail, while the exact same failure on receive
		// at least left a log line to investigate.
		if err := store.SaveMessage(Message{
			ID:         resp.ID,
			ChatJID:    jid,
			SenderJID:  myJID,
			SenderName: "Ben",
			Text:       text,
			Timestamp:  time.Now(),
			FromMe:     true,
		}); err != nil {
			logx.Printf("WhatsApp: save sent message error: %v", err)
		}
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
	c.startMu.Lock()
	wa := c.waClient
	c.startMu.Unlock()
	if wa == nil {
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

	info, err := wa.GetProfilePictureInfo(ctx, parsed, &whatsmeow.GetProfilePictureParams{Preview: preview})
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

// OwnJIDs returns every bare (non-AD) JID form the connected account's
// self-chat ("Message Yourself") might carry as its ChatJID.
//
// wa.Store.ID alone is not enough: confirmed live against a real account,
// a genuine self-chat message's ChatJID came back as "<n>@lid" — WhatsApp's
// newer privacy-preserving Linked-ID addressing — not
// "<phone>@s.whatsapp.net" like Store.ID (itself also in AD form, carrying
// this device's own :N suffix, e.g. "9055...:42@s.whatsapp.net", which
// never equals any Chat JID either way). whatsmeow's store.Device exposes
// both identities (ID = phone-number JID, LID = Linked-ID JID) — a caller
// recognizing the self-chat must check the incoming ChatJID against both
// non-AD forms, not just one. Returns nil if not connected/logged in yet.
func (c *Client) OwnJIDs() []string {
	c.startMu.Lock()
	wa := c.waClient
	c.startMu.Unlock()
	if wa == nil || wa.Store == nil {
		return nil
	}
	var out []string
	if wa.Store.ID != nil {
		if s := wa.Store.ID.ToNonAD().String(); s != "" {
			out = append(out, s)
		}
	}
	if s := wa.Store.LID.ToNonAD().String(); s != "" {
		out = append(out, s)
	}
	return out
}

// markSelfSent records a just-sent message's ID so IsSelfSentRecently can
// recognize it later. Bounded opportunistically rather than via a separate
// sweep goroutine — this map only ever holds a handful of short-lived IDs.
func (c *Client) markSelfSent(id string) {
	if id == "" {
		return
	}
	c.selfSentMu.Lock()
	defer c.selfSentMu.Unlock()
	if c.selfSentIDs == nil {
		c.selfSentIDs = make(map[string]time.Time)
	}
	c.selfSentIDs[id] = time.Now()
	if len(c.selfSentIDs) > 64 {
		cutoff := time.Now().Add(-5 * time.Minute)
		for k, t := range c.selfSentIDs {
			if t.Before(cutoff) {
				delete(c.selfSentIDs, k)
			}
		}
	}
}

// IsSelfSentRecently reports whether id was sent by this client itself via
// SendMessage within roughly the last 5 minutes.
func (c *Client) IsSelfSentRecently(id string) bool {
	c.selfSentMu.Lock()
	defer c.selfSentMu.Unlock()
	_, ok := c.selfSentIDs[id]
	return ok
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
		c.startMu.Lock()
		c.qrCodes = v.Codes
		c.startMu.Unlock()
	case *waEvent.PairSuccess:
		logx.Printf("WhatsApp: paired successfully with %s", v.ID)
		c.startMu.Lock()
		c.qrCodes = nil
		c.lastError = ""
		c.startMu.Unlock()
	case *waEvent.PairError:
		logx.Printf("WhatsApp: pair error: %v", v.Error)
		errStr := v.Error.Error()
		c.startMu.Lock()
		c.lastError = errStr
		c.startMu.Unlock()
		select {
		case c.errCh <- v.Error:
		default:
		}
	case *waEvent.Connected:
		logx.Printf("WhatsApp: connected")
		c.startMu.Lock()
		c.reconnecting = false
		c.lastError = ""
		c.startMu.Unlock()
	case *waEvent.Disconnected:
		logx.Printf("WhatsApp: disconnected")
		c.startMu.Lock()
		c.lastError = "connection lost"
		wasStarted := c.started
		c.startMu.Unlock()
		select {
		case c.errCh <- fmt.Errorf("disconnected"):
		default:
		}
		if wasStarted {
			logx.GoRecover("whatsapp.Client.autoReconnect", c.autoReconnect)
		}
	case *waEvent.LoggedOut:
		logx.Printf("WhatsApp: logged out remotely")
		c.startMu.Lock()
		c.lastError = "logged out"
		c.qrCodes = nil
		c.started = false
		c.startMu.Unlock()
	case *waEvent.StreamReplaced:
		logx.Printf("WhatsApp: stream replaced")
	case *waEvent.HistorySync:
		logx.GoRecover("whatsapp.Client.handleHistorySync", func() { c.handleHistorySync(v) })
	case *waEvent.Message:
		c.handleMessage(v)
	}
}

// autoReconnect attempts to reconnect with exponential backoff, aborting
// immediately when stopCh is closed (e.g. during Shutdown).
func (c *Client) autoReconnect() {
	c.startMu.Lock()
	c.reconnecting = true
	// Snapshot stopCh under the lock rather than reading c.stopCh directly
	// in the select below — Start() reassigns it on every call, and an
	// unsynchronized read here would race that write.
	stopCh := c.stopCh
	c.startMu.Unlock()
	backoff := []time.Duration{5 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second}
	for attempt, delay := range backoff {
		select {
		case <-stopCh:
			c.startMu.Lock()
			c.reconnecting = false
			c.startMu.Unlock()
			return
		case <-time.After(delay):
		}

		c.startMu.Lock()
		wa := c.waClient
		alive := c.started && wa != nil
		c.startMu.Unlock()

		if !alive || wa.IsConnected() {
			c.startMu.Lock()
			c.reconnecting = false
			c.startMu.Unlock()
			return
		}

		logx.Printf("WhatsApp: reconnect attempt %d...", attempt+1)
		if err := wa.Connect(); err != nil {
			logx.Printf("WhatsApp: reconnect error: %v", err)
			errMsg := fmt.Sprintf("reconnect failed (%d/%d)", attempt+1, len(backoff))
			c.startMu.Lock()
			c.lastError = errMsg
			c.startMu.Unlock()
			continue
		}
		logx.Printf("WhatsApp: reconnected")
		c.startMu.Lock()
		c.reconnecting = false
		c.startMu.Unlock()
		return
	}
	c.startMu.Lock()
	c.reconnecting = false
	c.lastError = "reconnect failed — please reconnect manually"
	c.startMu.Unlock()
	logx.Printf("WhatsApp: auto-reconnect exhausted")
}

// handleHistorySync imports historical messages after first pairing.
func (c *Client) handleHistorySync(evt *waEvent.HistorySync) {
	if evt.Data == nil || c.store == nil {
		return
	}
	convs := evt.Data.GetConversations()
	logx.Printf("WhatsApp: history sync — %d conversations", len(convs))
	c.startMu.Lock()
	wa := c.waClient
	c.startMu.Unlock()
	myJID := ""
	if wa != nil && wa.Store != nil {
		myJID = wa.Store.ID.String()
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
	// BUG-H6: this used to inline only the conversation/extended-text
	// cases, silently dropping any live image/video/document message that
	// carried only a caption — no save, no log, no error, just gone. The
	// package's own extractText (below) already handles those correctly
	// and is what handleHistorySync uses, but nothing routed live messages
	// through it, so the exact same message arriving live vs. via a
	// post-reconnect history sync behaved completely differently.
	text := extractText(evt.Message)
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
		c.startMu.Lock()
		wa := c.waClient
		c.startMu.Unlock()
		if wa != nil && wa.Store != nil {
			senderJID = wa.Store.ID.String()
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
	c.startMu.Lock()
	wa := c.waClient
	c.startMu.Unlock()
	if wa != nil && wa.Store != nil {
		parsed, err := types.ParseJID(jid)
		if err == nil {
			contact, err2 := wa.Store.Contacts.GetContact(context.Background(), parsed)
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
	c.startMu.Lock()
	wa := c.waClient
	c.startMu.Unlock()
	if wa == nil || wa.Store == nil || wa.Store.Contacts == nil || c.store == nil {
		return
	}
	contacts, err := wa.Store.Contacts.GetAllContacts(context.Background())
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
	c.startMu.Lock()
	wa := c.waClient
	c.startMu.Unlock()
	if wa == nil || c.store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	groups, err := wa.GetJoinedGroups(ctx)
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
