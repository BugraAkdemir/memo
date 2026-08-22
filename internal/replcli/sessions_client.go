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

// ChatMessage mirrors the fields of sessions.ChatMessage the REPL (and the
// `-chat <id> -list`/`-memory` scripting flags, cli_chat.go) need. Timestamp/
// MemoryUsed weren't needed until -memory added a per-message memory-usage
// view — see sessions.ChatMessage's own doc comment for what MemoryUsed
// actually counts (retrieved-and-injected memories for that turn, not the
// specific memories themselves — no per-message record of *which* memories
// were used is persisted anywhere).
type ChatMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	Timestamp  string `json:"timestamp"`
	MemoryUsed int    `json:"memory_used,omitempty"`
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
