package app

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"memo/internal/api"
	"memo/internal/sessions"
)

func (a *App) getSessionManager() *sessions.Manager {
	a.sessionsMu.RLock()
	defer a.sessionsMu.RUnlock()
	return a.sessions
}

// NewChat creates a new chat session.
func (a *App) NewChat() string {
	sm := a.getSessionManager()
	if sm == nil {
		return ""
	}
	return sm.NewChat()
}

// NewAgentChat creates a new agent chat session tied to a project path.
func (a *App) NewAgentChat(projectPath string) string {
	sm := a.getSessionManager()
	if sm == nil {
		return ""
	}
	return sm.NewAgentChat(projectPath)
}

// ListChats returns a list of all sessions.
func (a *App) ListChats() []sessions.SessionInfo {
	sm := a.getSessionManager()
	if sm == nil {
		return nil
	}
	return sm.ListChats()
}

// SwitchChat switches the active session.
func (a *App) SwitchChat(id string) error {
	sm := a.getSessionManager()
	if sm == nil {
		return fmt.Errorf("no session manager")
	}
	return sm.SwitchChat(id)
}

// DeleteChat removes a session.
func (a *App) DeleteChat(id string) error {
	sm := a.getSessionManager()
	if sm == nil {
		return fmt.Errorf("no session manager")
	}
	return sm.DeleteChat(id)
}

// RenameChat changes the title of a session.
func (a *App) RenameChat(id, title string) error {
	sm := a.getSessionManager()
	if sm == nil {
		return fmt.Errorf("no session manager")
	}
	return sm.RenameChat(id, title)
}

// UpdateMessage edits a message in the active session.
func (a *App) UpdateMessage(index int, content string) error {
	sm := a.getSessionManager()
	if sm == nil {
		return fmt.Errorf("no session manager")
	}
	return sm.UpdateMessage(index, content)
}

// DeleteMessage removes a message from the active session.
func (a *App) DeleteMessage(index int) error {
	sm := a.getSessionManager()
	if sm == nil {
		return fmt.Errorf("no session manager")
	}
	return sm.DeleteMessage(index)
}

// GetActiveMessages returns messages in the current session.
func (a *App) GetActiveMessages() []sessions.ChatMessage {
	sm := a.getSessionManager()
	if sm == nil {
		return nil
	}
	return sm.GetActiveMessages()
}

// GetActiveChatID returns the ID of the current session.
func (a *App) GetActiveChatID() string {
	sm := a.getSessionManager()
	if sm == nil {
		return ""
	}
	return sm.GetActiveID()
}

// ExportChat returns the active chat as a Markdown string.
func (a *App) ExportChat() string {
	sm := a.getSessionManager()
	if sm == nil {
		return ""
	}
	msgs := sm.GetActiveMessages()
	if len(msgs) == 0 {
		return ""
	}

	var sb strings.Builder
	chatID := sm.GetActiveID()
	title := "Memo Chat"
	for _, s := range sm.ListChats() {
		if s.ID == chatID {
			title = s.Title
			break
		}
	}
	sb.WriteString("# " + title + "\n\n")
	sb.WriteString("_Exported from Memo — " + time.Now().Format("2006-01-02 15:04") + "_\n\n---\n\n")

	for _, m := range msgs {
		switch m.Role {
		case "user":
			sb.WriteString("**You** · " + m.Timestamp + "\n\n")
		case "assistant":
			sb.WriteString("**Memo** · " + m.Timestamp + "\n\n")
		}
		sb.WriteString(m.Content + "\n\n---\n\n")
	}
	return sb.String()
}

// GenerateChatTitle asks the LLM to produce a short title from the first exchange.
func (a *App) GenerateChatTitle() string {
	sm := a.getSessionManager()
	if sm == nil {
		return ""
	}
	msgs := sm.GetActiveMessages()
	if len(msgs) < 2 {
		return ""
	}

	first := msgs[0].Content
	if len(first) > 300 {
		first = first[:300]
	}
	second := msgs[1].Content
	if len(second) > 300 {
		second = second[:300]
	}

	prompt := []api.Message{
		api.NewTextMessage("user", fmt.Sprintf(
			"Based on this conversation excerpt, generate a very short chat title (3–6 words max, no quotes, no punctuation at end):\n\nUser: %s\nAssistant: %s\n\nTitle:",
			first, second,
		)),
	}

	title := strings.TrimSpace(a.callLLM(context.Background(), prompt))
	if title == "" || strings.HasPrefix(title, "⚠️") {
		return ""
	}
	title = strings.Trim(title, `"'`)
	runes := []rune(title)
	if len(runes) > 60 {
		title = string(runes[:60])
	}

	chatID := sm.GetActiveID()
	if err := sm.RenameChat(chatID, title); err != nil {
		log.Printf("auto-title rename: %v", err)
		return ""
	}
	return title
}

// ClearHistory deletes the active chat session.
func (a *App) ClearHistory() {
	sm := a.getSessionManager()
	if sm != nil {
		sm.DeleteChat(sm.GetActiveID())
	}
}
