package sessions

import (
	"encoding/json"
	"fmt"
	"memo/internal/fileutil"
	"memo/internal/logx"
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
	// MemoryUsed is how many memories were retrieved and injected into the
	// system prompt for the turn that produced this (assistant) message —
	// 0/omitted means either memory was off, or nothing relevant enough was
	// found. Set via SetLastMessageMemoryUsed right after the message is
	// added, not passed through AddMessageToSession itself: the count is
	// only known at buildMessagesForSession time (before the LLM call even
	// starts), while AddMessageToSession for the assistant reply happens
	// after it — see memoryUsedCtxKey in internal/app/llm.go for how it
	// travels between the two.
	MemoryUsed int `json:"memory_used,omitempty"`
}

type Session struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	CreatedAt   string        `json:"created_at"`
	UpdatedAt   string        `json:"updated_at"`
	Messages    []ChatMessage `json:"messages"`
	ProjectPath string        `json:"project_path,omitempty"`

	// CLIProvider is this chat's own active provider (a provider.ProviderType
	// value, e.g. "claude-code-cli"), independent of the app-wide active
	// provider — each chat remembers its own, so switching provider in one
	// chat never affects another. Empty means this chat isn't CLI-backed and
	// uses the app-wide provider/local-model routing as before.
	CLIProvider string `json:"cli_provider,omitempty"`
	// CLISessionIDs maps a CLIProvider value to that CLI's own session id
	// for this chat (keyed by provider, not just one string, since a chat
	// could in principle be switched between claude-code-cli and codex-cli
	// and each needs its own continuity). Empty/missing means the next
	// message starts a fresh CLI session rather than resuming one.
	CLISessionIDs map[string]string `json:"cli_session_ids,omitempty"`
	// CLIWorkdir is the filesystem directory a CLI provider runs in for this
	// chat. Deliberately separate from ProjectPath (which already means
	// something else — "is this an agent chat" — elsewhere in this
	// codebase) so the two concepts don't get conflated.
	CLIWorkdir string `json:"cli_workdir,omitempty"`
	// CLIModel overrides which model the CLI itself uses for this chat (e.g.
	// "opus", "sonnet" for Claude Code; a model id for Codex). Empty means no
	// override is passed — the CLI uses its own configured default, exactly
	// as it did before this field existed.
	CLIModel string `json:"cli_model,omitempty"`
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

// newSession creates a session in memory only — deliberately NOT persisted
// to disk here. Every caller used to save immediately on creation, so a chat
// that was opened and abandoned with zero messages (e.g. the CLI's
// startFreshChat, called unconditionally on every `memo` launch since
// 2026-07-12, or the GUI's "+ New Chat" button clicked and never used)
// still left a permanent, empty "Agent Chat"/"New Chat" entry cluttering
// every future chat list. AddMessage's own save (below) is what actually
// commits a session to disk, the first time it receives real content — an
// unused chat now simply vanishes if the process exits/restarts before
// that happens, instead of persisting forever with nothing in it.
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
	return s
}

func (m *Manager) NewChat() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.newSession("New Chat")
	m.active = s.ID
	return s.ID
}

// NewBackgroundChat creates a new session without switching the active
// chat — for sessions a background process needs (like the WhatsApp
// self-chat bridge), where hijacking whatever chat the user currently has
// open in the UI would be a jarring, unrelated side effect of a message
// arriving on a completely different surface.
func (m *Manager) NewBackgroundChat(title string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.newSession(title)
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

// SetProjectPath updates a session's persisted working-directory override —
// the write-side counterpart to GetProjectPath. Used by the change_directory
// agent tool so a directory switch survives past the turn that made it,
// across every surface (Flutter chat, WhatsApp, Telegram) that later reads
// GetProjectPath before calling Executor.RunStream.
func (m *Manager) SetProjectPath(id, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	s.ProjectPath = path
	return m.save(s)
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
		// Truncate by rune, not byte: a byte slice can split a multi-byte UTF-8
		// rune (e.g. Turkish ç/ş/ğ/ı), persisting a corrupted title.
		if r := []rune(title); len(r) > 40 {
			title = string(r[:40]) + "..."
		}
		s.Title = title
	}

	if err := m.save(s); err != nil {
		logx.Printf("sessions: save message %s: %v", s.ID, err)
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

// AddMessageToSession adds a message to a specific session by ID.
func (m *Manager) AddMessageToSession(sessionID, role, content, imagePath, filePath string, agentEvents ...[]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[sessionID]
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

	if s.Title == "New Chat" && role == "user" && len(content) > 0 {
		title := content
		if r := []rune(title); len(r) > 40 {
			title = string(r[:40]) + "..."
		}
		s.Title = title
	}

	if err := m.save(s); err != nil {
		logx.Printf("sessions: save message %s: %v", s.ID, err)
	}
}

// SetLastMessageMemoryUsed sets MemoryUsed on the most recently added
// message in sessionID. Meant to be called immediately after
// AddMessageToSession for the assistant reply it's annotating — see
// ChatMessage.MemoryUsed's doc comment for why this is a separate call
// instead of a parameter on AddMessageToSession itself. No-ops (including
// skipping the extra disk write) when count is 0, so a turn where memory
// contributed nothing costs nothing extra over today's behavior.
func (m *Manager) SetLastMessageMemoryUsed(sessionID string, count int) {
	if count <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[sessionID]
	if s == nil || len(s.Messages) == 0 {
		return
	}
	s.Messages[len(s.Messages)-1].MemoryUsed = count
	if err := m.save(s); err != nil {
		logx.Printf("sessions: save memory-used %s: %v", s.ID, err)
	}
}

// GetActiveMessagesForSession returns messages for a specific session.
func (m *Manager) GetActiveMessagesForSession(sessionID string) []ChatMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.sessions[sessionID]
	if s == nil {
		return nil
	}
	out := make([]ChatMessage, len(s.Messages))
	copy(out, s.Messages)
	return out
}

// SessionExists checks if a session exists.
func (m *Manager) SessionExists(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.sessions[id]
	return ok
}

// LastActivity returns the session's last-activity wall-clock time —
// Session.UpdatedAt ("2006-01-02 15:04", local), which newSession seeds and
// AddMessageToSession refreshes on every message — parsed as time.Time.
// Zero time when the session doesn't exist or UpdatedAt is empty or
// unparseable; callers treat that as "no signal", not an error (sole
// consumer today: timeContextBlockForChat in internal/app/helpers.go).
//
// ChatMessage.Timestamp can't serve this purpose: it stores only "15:04"
// (hour:minute, no date) for display rendering, so it cannot express
// "the last message was 3 days ago".
func (m *Manager) LastActivity(sessionID string) time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.sessions[sessionID]
	if s == nil || s.UpdatedAt == "" {
		return time.Time{}
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", s.UpdatedAt, time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
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

// GetCLIProvider returns id's own active CLI provider (empty if this chat
// isn't CLI-backed, or id doesn't exist).
func (m *Manager) GetCLIProvider(id string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	if !ok {
		return ""
	}
	return s.CLIProvider
}

// SetCLIProvider sets id's own active CLI provider, independent of every
// other chat's provider (see Session.CLIProvider's doc comment). Passing ""
// clears it, returning the chat to normal app-wide provider/local-model
// routing.
func (m *Manager) SetCLIProvider(id, provider string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	s.CLIProvider = provider
	s.UpdatedAt = time.Now().Format("2006-01-02 15:04")
	return m.save(s)
}

// GetCLISessionID returns the CLI's own session id for id+cliProvider, or ""
// if none is recorded yet (the next message should start a fresh session
// rather than trying to resume one).
func (m *Manager) GetCLISessionID(id, cliProvider string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	if !ok || s.CLISessionIDs == nil {
		return ""
	}
	return s.CLISessionIDs[cliProvider]
}

// SetCLISessionID records the CLI's own session id for id+cliProvider, so
// the next message to this chat resumes it instead of starting fresh.
func (m *Manager) SetCLISessionID(id, cliProvider, cliSessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	if s.CLISessionIDs == nil {
		s.CLISessionIDs = make(map[string]string)
	}
	s.CLISessionIDs[cliProvider] = cliSessionID
	return m.save(s)
}

// GetCLIWorkdir returns id's CLI working directory, or "" if unset (the
// caller should prompt the user to pick one — see yapacam.md §3).
func (m *Manager) GetCLIWorkdir(id string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	if !ok {
		return ""
	}
	return s.CLIWorkdir
}

// SetCLIWorkdir sets id's CLI working directory, persisted so it's only
// ever asked once per chat.
func (m *Manager) SetCLIWorkdir(id, dir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	s.CLIWorkdir = dir
	return m.save(s)
}

func (m *Manager) GetCLIModel(id string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	if !ok {
		return ""
	}
	return s.CLIModel
}

// SetCLIModel sets id's CLI model override, persisted like CLIWorkdir.
func (m *Manager) SetCLIModel(id, model string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	s.CLIModel = model
	return m.save(s)
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

// GetHistoryForAPI returns the active session's history in api.Message
// compatible format, truncated by message count. Thin wrapper around
// GetHistoryForAPIForSession — see PLAN_chatid_refactor.md Phase 1.
func (m *Manager) GetHistoryForAPI(maxMessages int) []map[string]string {
	return m.GetHistoryForAPIForSession(m.GetActiveID(), maxMessages)
}

// GetHistoryForAPIForSession returns sessionID's history in api.Message
// compatible format, truncated by message count.
func (m *Manager) GetHistoryForAPIForSession(sessionID string, maxMessages int) []map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.sessions[sessionID]
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

// GetHistoryForAPITokenAware returns the active session's history truncated
// to fit within maxTokens. Thin wrapper around
// GetHistoryForAPITokenAwareForSession — see PLAN_chatid_refactor.md Phase 1.
func (m *Manager) GetHistoryForAPITokenAware(maxTokens int) []map[string]string {
	return m.GetHistoryForAPITokenAwareForSession(m.GetActiveID(), maxTokens)
}

// GetHistoryForAPITokenAwareForSession returns sessionID's history
// truncated to fit within maxTokens. Preserves system prompt, drops oldest
// messages first. More accurate than the message-count variant for long
// conversations with varying message sizes.
func (m *Manager) GetHistoryForAPITokenAwareForSession(sessionID string, maxTokens int) []map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.sessions[sessionID]
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
	if err := fileutil.AtomicWrite(path, data, 0600); err != nil {
		return fmt.Errorf("sessions: save %s: %w", s.ID, err)
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
			logx.Printf("sessions: read %s: %v", e.Name(), err)
			continue
		}
		var s Session
		if err := json.Unmarshal(data, &s); err != nil {
			logx.Printf("sessions: decode %s: %v", e.Name(), err)
			continue
		}
		m.sessions[s.ID] = &s
	}
	return nil
}
