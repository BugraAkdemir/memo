package app

import (
	"context"
	"fmt"
	"memo/internal/logx"
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

// AppendMessage appends role/content to the active session's history and
// persists it, without triggering any LLM turn — the raw primitive Live
// Mode's transcript display needs (frontend/lib/providers/
// live_realtime_session_provider.dart's _handleControlFrame): reported
// live, transcript bubbles added via the Flutter-only
// messagesProvider.addMessage() looked permanent but actually lived only
// in that session's in-memory client state — switching chats and back, or
// restarting the app, re-fetched from the backend and the transcript was
// simply gone, since nothing had ever told the backend about it. role
// should be "user" or "assistant" (mirrors sessions.ChatMessage.Role,
// same as every other message).
func (a *App) AppendMessage(role, content string) error {
	sm := a.getSessionManager()
	if sm == nil {
		return fmt.Errorf("no session manager")
	}
	sm.AddMessage(role, content, "", "")

	// A Live Mode conversation is a real conversation — it should feed
	// long-term (RAG) memory the same way a typed chat turn does, not just
	// sit in the session transcript. The frontend appends the user's
	// transcript bubble and then the model's; on the model's, pair it with
	// the most recent user utterance and hand the turn to saveMemoryAsync.
	// That path already drops empty/error replies, respects MemoryEnabled,
	// filters pure acks/greetings (BUG-L1 IsLowValueTurn), and skips
	// near-duplicates — so no extra gating is needed here beyond Incognito,
	// whose whole contract is "nothing persisted, nothing recalled".
	if role == "assistant" && content != "" && !a.GetIncognito() {
		if userMsg := lastUserMessage(sm.GetActiveMessages()); userMsg != "" {
			a.saveMemoryAsync(userMsg, content)
		}
	}
	return nil
}

// lastUserMessage returns the content of the most recent "user" message in
// msgs, or "" if there is none. Used to pair a just-appended assistant
// transcript with the user utterance it answered.
func lastUserMessage(msgs []sessions.ChatMessage) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i].Content
		}
	}
	return ""
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

// GenerateChatTitle asks the LLM to produce a short title from the first
// exchange of the currently active chat. Exposed to the frontend as a
// manual "regenerate title" action, where "whatever chat the user is
// currently viewing" is exactly the right target.
func (a *App) GenerateChatTitle() string {
	sm := a.getSessionManager()
	if sm == nil {
		return ""
	}
	return a.generateChatTitleForSession(sm.GetActiveID())
}

// generateChatTitleForSession is like GenerateChatTitle but names a specific
// session rather than whatever happens to be active. finishStream calls this
// with the session ID the stream actually wrote to — which is not
// necessarily the session the user is currently viewing (e.g. a background
// or task-loop stream running against a non-active chat). Calling
// GenerateChatTitle() there would title whatever chat is active *right now*
// instead of the one the stream was actually for.
func (a *App) generateChatTitleForSession(sessionID string) string {
	sm := a.getSessionManager()
	if sm == nil || sessionID == "" {
		return ""
	}
	msgs := sm.GetActiveMessagesForSession(sessionID)
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

	title := strings.TrimSpace(a.callLLM(context.Background(), prompt, categoryTitle))
	if title == "" || strings.HasPrefix(title, "⚠️") {
		return ""
	}
	title = strings.Trim(title, `"'`)
	runes := []rune(title)
	if len(runes) > 60 {
		title = string(runes[:60])
	}

	if err := sm.RenameChat(sessionID, title); err != nil {
		logx.Printf("auto-title rename: %v", err)
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
