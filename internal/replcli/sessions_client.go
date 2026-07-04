package replcli

import (
	"context"
	"net/http"
)

// SessionInfo mirrors sessions.SessionInfo — the fields the REPL needs to
// list and pick chats by project, title and recency.
type SessionInfo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	MsgCount    int    `json:"msg_count"`
	ProjectPath string `json:"project_path,omitempty"`
}

// ChatMessage mirrors the fields of sessions.ChatMessage the REPL needs to
// replay a resumed chat's history in the terminal.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ListChats returns every chat session the backend knows about, most
// recently updated first.
func (c *Client) ListChats(ctx context.Context) ([]SessionInfo, error) {
	var chats []SessionInfo
	if err := c.doJSON(ctx, http.MethodGet, "/api/chats", nil, &chats); err != nil {
		return nil, err
	}
	return chats, nil
}

// Messages returns the message history of the currently active chat.
func (c *Client) Messages(ctx context.Context) ([]ChatMessage, error) {
	var msgs []ChatMessage
	if err := c.doJSON(ctx, http.MethodGet, "/api/messages", nil, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}
