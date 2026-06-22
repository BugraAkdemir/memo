package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// WhatsAppClient is the interface tools use to interact with WhatsApp.
// Set by App after initialization.
var WhatsAppClient interface {
	SendMessage(ctx context.Context, jid, text string) (string, error)
	SearchMessages(query string, limit int) ([]WhatsAppMsg, error)
	GetChatList() ([]WhatsAppChat, error)
	GetChatMessages(chatJID string, limit int) ([]WhatsAppMsg, error)
}

type WhatsAppMsg struct {
	ID         string    `json:"id"`
	ChatJID    string    `json:"chat_jid"`
	SenderJID  string    `json:"sender_jid"`
	SenderName string    `json:"sender_name"`
	Text       string    `json:"text"`
	Timestamp  time.Time `json:"timestamp"`
	FromMe     bool      `json:"from_me"`
}

type WhatsAppChat struct {
	JID         string    `json:"jid"`
	DisplayName string    `json:"display_name"`
	LastMessage string    `json:"last_message"`
	LastTime    time.Time `json:"last_time"`
	Unread      int       `json:"unread"`
}

type WhatsAppSendArgs struct {
	JID  string `json:"jid"`
	Text string `json:"text"`
}

type WhatsAppSearchArgs struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type WhatsAppLatestArgs struct {
	Limit int `json:"limit"`
}

type WhatsAppMessagesArgs struct {
	JID   string `json:"jid"`
	Limit int    `json:"limit"`
}

func SendWhatsApp(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args WhatsAppSendArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if WhatsAppClient == nil {
		return "WhatsApp bağlı değil (config.yaml'de whatsapp.enabled: true olmalı)", nil
	}
	jid := resolveWhatsAppJID(args.JID)
	msgID, err := WhatsAppClient.SendMessage(context.Background(), jid, args.Text)
	if err != nil {
		return "", fmt.Errorf("WhatsApp gönderilemedi: %w", err)
	}
	return fmt.Sprintf("Mesaj gönderildi (ID: %s)", msgID), nil
}

func SearchWhatsApp(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args WhatsAppSearchArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Limit <= 0 {
		args.Limit = 10
	}
	if WhatsAppClient == nil {
		return "WhatsApp bağlı değil", nil
	}
	msgs, err := WhatsAppClient.SearchMessages(args.Query, args.Limit)
	if err != nil {
		return "", fmt.Errorf("WhatsApp arama hatası: %w", err)
	}
	if len(msgs) == 0 {
		return "Mesaj bulunamadı", nil
	}
	var lines []string
	for _, m := range msgs {
		from := m.SenderName
		if from == "" {
			from = partsBeforeAt(m.SenderJID)
		}
		ts := m.Timestamp.Format("02/01 15:04")
		lines = append(lines, fmt.Sprintf("[%s] %s: %s", ts, from, m.Text))
	}
	return strings.Join(lines, "\n"), nil
}

func LatestWhatsAppChats(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args WhatsAppLatestArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Limit <= 0 {
		args.Limit = 10
	}
	if WhatsAppClient == nil {
		return "WhatsApp bağlı değil", nil
	}
	chats, err := WhatsAppClient.GetChatList()
	if err != nil {
		return "", fmt.Errorf("WhatsApp sohbet listesi hatası: %w", err)
	}
	if len(chats) == 0 {
		return "Henüz hiçbir sohbet yok", nil
	}
	if args.Limit > len(chats) {
		args.Limit = len(chats)
	}
	chats = chats[:args.Limit]
	var lines []string
	for _, c := range chats {
		displayName := c.DisplayName
		if displayName == "" {
			displayName = partsBeforeAt(c.JID)
		}
		ts := c.LastTime.Format("02/01 15:04")
		// Include the JID so the model can pass it verbatim to whatsapp_send /
		// whatsapp_messages instead of guessing a name as the JID.
		lines = append(lines, fmt.Sprintf("%s (jid: %s) — %s [%s]", displayName, c.JID, c.LastMessage, ts))
	}
	return strings.Join(lines, "\n"), nil
}

func GetWhatsAppMessages(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args WhatsAppMessagesArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Limit <= 0 {
		args.Limit = 20
	}
	if WhatsAppClient == nil {
		return "WhatsApp bağlı değil", nil
	}
	jid := resolveWhatsAppJID(args.JID)
	msgs, err := WhatsAppClient.GetChatMessages(jid, args.Limit)
	if err != nil {
		return "", fmt.Errorf("WhatsApp mesaj hatası: %w", err)
	}
	if len(msgs) == 0 {
		return "Bu sohbette henüz mesaj yok", nil
	}
	var lines []string
	for _, m := range msgs {
		from := m.SenderName
		if from == "" {
			from = partsBeforeAt(m.SenderJID)
		}
		ts := m.Timestamp.Format("02/01 15:04")
		lines = append(lines, fmt.Sprintf("[%s] %s: %s", ts, from, m.Text))
	}
	return strings.Join(lines, "\n"), nil
}

// resolveWhatsAppJID maps a display name (or partial) to a real JID using the
// chat list, so the model can refer to a contact by name. If the input already
// looks like a JID (contains '@'), it is returned unchanged. Falls back to the
// original input when no match is found, so the caller surfaces a clear error.
func resolveWhatsAppJID(nameOrJID string) string {
	if strings.Contains(nameOrJID, "@") || WhatsAppClient == nil {
		return nameOrJID
	}
	chats, err := WhatsAppClient.GetChatList()
	if err != nil {
		return nameOrJID
	}
	q := strings.ToLower(strings.TrimSpace(nameOrJID))
	if q == "" {
		return nameOrJID
	}
	// Exact display-name match first, then substring, then phone-number prefix.
	for _, c := range chats {
		if strings.ToLower(c.DisplayName) == q {
			return c.JID
		}
	}
	for _, c := range chats {
		if c.DisplayName != "" && strings.Contains(strings.ToLower(c.DisplayName), q) {
			return c.JID
		}
	}
	for _, c := range chats {
		if strings.Contains(strings.ToLower(partsBeforeAt(c.JID)), q) {
			return c.JID
		}
	}
	return nameOrJID
}

func partsBeforeAt(s string) string {
	if parts := strings.SplitN(s, "@", 2); len(parts) > 0 {
		return parts[0]
	}
	return s
}
