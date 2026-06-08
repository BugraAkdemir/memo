package whatsapp

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waEvent "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// Config holds WhatsApp integration settings.
type Config struct {
	DataDir        string // e.g. "./data/whatsapp"
	MessageStoreDB string // e.g. "./data/whatsapp/messages.db"
	SessionDB      string // e.g. "./data/whatsapp/session.db"
	AutoIndex      bool   // index messages for RAG
	MaxHistoryDays int    // days of history to fetch on first connect
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
	config   Config
	waClient *whatsmeow.Client
	store    *Store
	qrCodes  []string     // current batch of QR codes
	msgCh    chan Message // incoming messages for RAG indexing
	errCh    chan error
	started  bool
	startMu  sync.Mutex
}

func NewClient(cfg Config) *Client {
	return &Client{
		config: cfg,
		msgCh:  make(chan Message, 256),
		errCh:  make(chan error, 4),
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

// Start connects to WhatsApp Web. Thread-safe, no-op if already started.
func (c *Client) Start(ctx context.Context) error {
	c.startMu.Lock()
	defer c.startMu.Unlock()
	if c.started {
		return nil
	}

	// Use SQLite store for session persistence
	storeDB, err := sqlstore.New(ctx, "sqlite3", "file:"+c.config.SessionDB+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", nil)
	if err != nil {
		return fmt.Errorf("whatsapp: session db: %w", err)
	}

	// Load or create device store
	deviceStore, err := storeDB.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("whatsapp: get device: %w", err)
	}

	c.waClient = whatsmeow.NewClient(deviceStore, nil)
	c.waClient.AddEventHandler(c.handleEvent)

	// Connect
	if err := c.waClient.Connect(); err != nil {
		return fmt.Errorf("whatsapp: connect: %w", err)
	}

	c.started = true
	return nil
}

// Stop disconnects from WhatsApp Web.
func (c *Client) Stop() {
	c.startMu.Lock()
	defer c.startMu.Unlock()
	if c.waClient != nil {
		c.waClient.Disconnect()
	}
	close(c.msgCh)
	close(c.errCh)
	c.started = false
}

// IsConnected returns whether the client is connected.
func (c *Client) IsConnected() bool {
	return c.waClient != nil && c.waClient.IsConnected()
}

// IsLoggedIn returns whether a valid session exists (no QR needed).
func (c *Client) IsLoggedIn() bool {
	return c.waClient != nil && c.waClient.IsLoggedIn()
}

// SendMessage sends a text message to a JID.
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

// handleEvent processes incoming WhatsApp events.
func (c *Client) handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *waEvent.QR:
		c.qrCodes = v.Codes
	case *waEvent.PairSuccess:
		log.Printf("WhatsApp: paired successfully with %s", v.ID)
	case *waEvent.PairError:
		log.Printf("WhatsApp: pair error: %v", v.Error)
		c.errCh <- v.Error
	case *waEvent.Connected:
		log.Printf("WhatsApp: connected")
	case *waEvent.Disconnected:
		log.Printf("WhatsApp: disconnected")
		select {
		case c.errCh <- fmt.Errorf("disconnected"):
		default:
		}
	case *waEvent.LoggedOut:
		log.Printf("WhatsApp: logged out")
	case *waEvent.StreamReplaced:
		log.Printf("WhatsApp: stream replaced (another client connected?)")
	case *waEvent.HistorySync:
		c.handleHistorySync(v)
	case *waEvent.Message:
		c.handleMessage(v)
	}
}

// handleHistorySync processes the initial history sync after pairing,
// importing historical messages into the local store.
func (c *Client) handleHistorySync(evt *waEvent.HistorySync) {
	if evt.Data == nil || c.store == nil {
		return
	}
	convs := evt.Data.GetConversations()
	log.Printf("WhatsApp: history sync received %d conversations", len(convs))
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
			msg := Message{
				ID:        key.GetID(),
				ChatJID:   key.GetRemoteJID(),
				SenderJID: key.GetRemoteJID(),
				Text:      msgText,
				Timestamp: time.Unix(int64(wmi.GetMessageTimestamp()), 0),
				FromMe:    key.GetFromMe(),
			}
			if msg.FromMe {
				msg.SenderName = "Ben"
			}
			if err := c.store.SaveMessage(msg); err != nil {
				log.Printf("WhatsApp: save history msg error: %v", err)
			}
			count++
		}
	}
	log.Printf("WhatsApp: history sync saved %d messages", count)
}

// extractText extracts the text content from a WhatsApp message.
func extractText(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}
	text := msg.GetConversation()
	if text == "" {
		text = msg.GetExtendedTextMessage().GetText()
	}
	if text == "" {
		text = msg.GetImageMessage().GetCaption()
	}
	if text == "" {
		text = msg.GetVideoMessage().GetCaption()
	}
	if text == "" {
		text = msg.GetDocumentMessage().GetCaption()
	}
	return text
}

// handleMessage processes an incoming message event.
func (c *Client) handleMessage(evt *waEvent.Message) {
	info := evt.Info
	text := evt.Message.GetConversation()
	if text == "" {
		text = evt.Message.GetExtendedTextMessage().GetText()
	}
	if text == "" {
		return // skip non-text messages
	}

	msg := Message{
		ID:        info.ID,
		ChatJID:   info.Chat.String(),
		SenderJID: info.Sender.String(),
		Text:      text,
		Timestamp: info.Timestamp,
		FromMe:    info.IsFromMe,
	}

	// Get sender name from pushname
	if info.PushName != "" {
		msg.SenderName = info.PushName
	} else if info.IsFromMe {
		msg.SenderName = "Ben"
	}

	// Save to store
	if c.store != nil {
		if err := c.store.SaveMessage(msg); err != nil {
			log.Printf("WhatsApp: save message error: %v", err)
		}
	}

	// Forward to RAG channel
	select {
	case c.msgCh <- msg:
	default:
	}
}
