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

func SendWhatsApp(argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args WhatsAppSendArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if WhatsAppClient == nil {
		return "WhatsApp bağlı değil (config.yaml'de whatsapp.enabled: true olmalı)", nil
	}
	msgID, err := WhatsAppClient.SendMessage(context.Background(), args.JID, args.Text)
	if err != nil {
		return "", fmt.Errorf("WhatsApp gönderilemedi: %w", err)
	}
	return fmt.Sprintf("Mesaj gönderildi (ID: %s)", msgID), nil
}

func SearchWhatsApp(argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
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
			from = m.SenderJID
		}
		ts := m.Timestamp.Format("02/01 15:04")
		lines = append(lines, fmt.Sprintf("[%s] %s: %s", ts, from, m.Text))
	}
	return strings.Join(lines, "\n"), nil
}

func LatestWhatsAppChats(argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
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
		displayName := partsBeforeAt(c.JID)
		ts := c.LastTime.Format("02/01 15:04")
		lines = append(lines, fmt.Sprintf("%s: %s [%s]", displayName, c.LastMessage, ts))
	}
	return strings.Join(lines, "\n"), nil
}

func GetWhatsAppMessages(argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
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
	msgs, err := WhatsAppClient.GetChatMessages(args.JID, args.Limit)
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
			from = m.SenderJID
		}
		ts := m.Timestamp.Format("02/01 15:04")
		lines = append(lines, fmt.Sprintf("[%s] %s: %s", ts, from, m.Text))
	}
	return strings.Join(lines, "\n"), nil
}

func partsBeforeAt(s string) string {
	if parts := strings.SplitN(s, "@", 2); len(parts) > 0 {
		return parts[0]
	}
	return s
}
