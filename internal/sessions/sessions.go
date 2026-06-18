package sessions

import (
	"encoding/json"
	"fmt"
	"log"
	"memo/internal/truncate"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ChatMessage struct {
	Role        string        `json:"role"`
	Content     string        `json:"content"`
	ImagePath   string        `json:"image_path,omitempty"`
	FilePath    string        `json:"file_path,omitempty"`
	Timestamp   string        `json:"timestamp"`
	AgentEvents []interface{} `json:"agent_events,omitempty"`
}

type Session struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	CreatedAt   string        `json:"created_at"`
	UpdatedAt   string        `json:"updated_at"`
	Messages    []ChatMessage `json:"messages"`
	ProjectPath string        `json:"project_path,omitempty"`
}

type Manager struct {
	dir      string
	sessions map[string]*Session
	active   string
	mu       sync.RWMutex
}

func NewManager(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("sessions: mkdir: %w", err)
	}

	m := &Manager{
		dir:      dir,
		sessions: make(map[string]*Session),
	}

	if err := m.loadAll(); err != nil {
		return nil, err
	}

	// Create default session if none exist
	if len(m.sessions) == 0 {
		s := m.newSession("New Chat")
		m.active = s.ID
	} else {
		// Set most recent as active
		list := m.sortedList()
		m.active = list[0].ID
	}

	return m, nil
}

func (m *Manager) newSession(title string) *Session {
	now := time.Now().Format("2006-01-02 15:04")
	s := &Session{
		ID:        uuid.New().String(),
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  []ChatMessage{},
	}
	m.sessions[s.ID] = s
	if err := m.save(s); err != nil {
		log.Printf("sessions: save new session %s: %v", s.ID, err)
	}
	return s
}



func (m *Manager) NewChat() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.newSession("New Chat")
	m.active = s.ID
	return s.ID
}

func (m *Manager) NewAgentChat(projectPath string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.newSession("Agent Chat")
	s.ProjectPath = projectPath
	m.active = s.ID
	return s.ID
}

func (m *Manager) GetProjectPath(id string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	if !ok {
		return ""
	}
	return s.ProjectPath
}

func (m *Manager) SwitchChat(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[id]; !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	m.active = id
	return nil
}

func (m *Manager) DeleteChat(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[id]; !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	delete(m.sessions, id)
	os.Remove(filepath.Join(m.dir, id+".json"))

	if m.active == id {
		if len(m.sessions) == 0 {
			s := m.newSession("New Chat")
			m.active = s.ID
		} else {
			list := m.sortedList()
			m.active = list[0].ID
		}
	}
	return nil
}

func (m *Manager) RenameChat(id, title string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	s.Title = title
	return m.save(s)
}

func (m *Manager) AddMessage(role, content, imagePath, filePath string, agentEvents ...[]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[m.active]
	if s == nil {
		return
	}
	msg := ChatMessage{
		Role:      role,
		Content:   content,
		ImagePath: imagePath,
		FilePath:  filePath,
		Timestamp: time.Now().Format("15:04"),
	}
	if len(agentEvents) > 0 && len(agentEvents[0]) > 0 {
		msg.AgentEvents = agentEvents[0]
	}
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now().Format("2006-01-02 15:04")

	// Auto-title from first user message
	if s.Title == "New Chat" && role == "user" && len(content) > 0 {
		title := content
		if len(title) > 40 {
			title = title[:40] + "..."
		}
		s.Title = title
	}

	if err := m.save(s); err != nil {
		log.Printf("sessions: save message %s: %v", s.ID, err)
	}
}

func (m *Manager) UpdateMessage(index int, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[m.active]
	if s == nil {
		return fmt.Errorf("no active session")
	}
	if index < 0 || index >= len(s.Messages) {
		return fmt.Errorf("message index %d out of range", index)
	}
	s.Messages[index].Content = content
	s.Messages[index].Timestamp = time.Now().Format("15:04")
	s.UpdatedAt = time.Now().Format("2006-01-02 15:04")
	return m.save(s)
}

func (m *Manager) DeleteMessage(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[m.active]
	if s == nil {
		return fmt.Errorf("no active session")
	}
	if index < 0 || index >= len(s.Messages) {
		return fmt.Errorf("message index %d out of range", index)
	}
	s.Messages = append(s.Messages[:index], s.Messages[index+1:]...)
	s.UpdatedAt = time.Now().Format("2006-01-02 15:04")
	return m.save(s)
}

func (m *Manager) GetActiveMessages() []ChatMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.sessions[m.active]
	if s == nil {
		return nil
	}
	out := make([]ChatMessage, len(s.Messages))
	copy(out, s.Messages)
	return out
}

func (m *Manager) GetActiveID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

type SessionInfo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	MsgCount    int    `json:"msg_count"`
	ProjectPath string `json:"project_path,omitempty"`
}

func (m *Manager) IsAgentChat(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return ok && s.ProjectPath != ""
}

func (m *Manager) ListChats() []SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := m.sortedList()
	out := make([]SessionInfo, len(list))
	for i, s := range list {
		out[i] = SessionInfo{
			ID:          s.ID,
			Title:       s.Title,
			CreatedAt:   s.CreatedAt,
			UpdatedAt:   s.UpdatedAt,
			MsgCount:    len(s.Messages),
			ProjectPath: s.ProjectPath,
		}
	}
	return out
}

func (m *Manager) sortedList() []*Session {
	list := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, s)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt > list[j].UpdatedAt
	})
	return list
}

// GetHistoryForAPI returns history in api.Message compatible format, truncated by message count.
func (m *Manager) GetHistoryForAPI(maxMessages int) []map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.sessions[m.active]
	if s == nil {
		return nil
	}
	msgs := s.Messages
	if len(msgs) > maxMessages {
		msgs = msgs[len(msgs)-maxMessages:]
	}
	out := make([]map[string]string, len(msgs))
	for i, msg := range msgs {
		out[i] = map[string]string{"role": msg.Role, "content": msg.Content}
	}
	return out
}

// GetHistoryForAPITokenAware returns history truncated to fit within maxTokens.
// Preserves system prompt, drops oldest messages first. More accurate than
// GetHistoryForAPI for long conversations with varying message sizes.
func (m *Manager) GetHistoryForAPITokenAware(maxTokens int) []map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.sessions[m.active]
	if s == nil {
		return nil
	}

	msgs := s.Messages
	if len(msgs) == 0 {
		return nil
	}

	truncMsgs := make([]truncate.Message, len(msgs))
	for i, msg := range msgs {
		truncMsgs[i] = truncate.Message{Role: msg.Role, Content: msg.Content}
	}

	truncMsgs = truncate.TruncateMessages(truncMsgs, maxTokens)

	out := make([]map[string]string, len(truncMsgs))
	for i, msg := range truncMsgs {
		out[i] = map[string]string{"role": msg.Role, "content": msg.Content}
	}
	return out
}

func (m *Manager) save(s *Session) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(m.dir, s.ID+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("sessions: write tmp %s: %w", s.ID, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("sessions: rename %s: %w", s.ID, err)
	}
	return nil
}

func (m *Manager) loadAll() error {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.dir, e.Name()))
		if err != nil {
			log.Printf("sessions: read %s: %v", e.Name(), err)
			continue
		}
		var s Session
		if err := json.Unmarshal(data, &s); err != nil {
			log.Printf("sessions: decode %s: %v", e.Name(), err)
			continue
		}
		m.sessions[s.ID] = &s
	}
	return nil
}
