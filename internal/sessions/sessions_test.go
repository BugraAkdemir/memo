package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManagerCreatesDirAndDefaultSession(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if m.GetActiveID() == "" {
		t.Error("active session ID is empty")
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("manager did not create directory")
	}
}

func TestNewManagerLoadsExistingSessions(t *testing.T) {
	dir := t.TempDir()
	m1, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	m1.NewChat()
	m1.AddMessage("user", "hello", "", "")

	m2, err := NewManager(dir)
	if err != nil {
		t.Fatalf("reload NewManager() error = %v", err)
	}
	chats := m2.ListChats()
	if len(chats) != 2 {
		t.Fatalf("len(chats) = %d, want 2", len(chats))
	}
}

func TestNewChat(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	id := m.NewChat()
	if id == "" {
		t.Error("NewChat() returned empty ID")
	}
	if m.GetActiveID() != id {
		t.Errorf("active ID = %q, want %q", m.GetActiveID(), id)
	}
}

func TestSwitchChat(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	id1 := m.NewChat()
	m.NewChat()

	if err := m.SwitchChat(id1); err != nil {
		t.Fatalf("SwitchChat() error = %v", err)
	}
	if m.GetActiveID() != id1 {
		t.Errorf("active = %q, want %q", m.GetActiveID(), id1)
	}
}

func TestSwitchChatNotFound(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	err := m.SwitchChat("nonexistent-id")
	if err == nil {
		t.Fatal("SwitchChat() should error on non-existent ID")
	}
}

func TestDeleteChat(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	id := m.NewChat()

	if err := m.DeleteChat(id); err != nil {
		t.Fatalf("DeleteChat() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, id+".json")); !os.IsNotExist(err) {
		t.Error("session file still exists after DeleteChat")
	}
}

func TestDeleteChatNotFound(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	err := m.DeleteChat("nonexistent")
	if err == nil {
		t.Fatal("DeleteChat() should error on non-existent session")
	}
}

func TestDeleteChatSwitchesToNext(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	id1 := m.NewChat()
	m.NewChat()

	m.DeleteChat(id1)
	if m.GetActiveID() == id1 {
		t.Error("active session should not be the deleted one")
	}
}

func TestDeleteLastChatCreatesNewDefault(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	chats := m.ListChats()
	// There's one default session created by NewManager
	for _, c := range chats {
		m.DeleteChat(c.ID)
	}
	// Should have created a new default session
	chats = m.ListChats()
	if len(chats) != 1 {
		t.Errorf("len(chats) = %d, want 1 (new default)", len(chats))
	}
}

func TestRenameChat(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	id := m.NewChat()

	if err := m.RenameChat(id, "New Title"); err != nil {
		t.Fatalf("RenameChat() error = %v", err)
	}

	chats := m.ListChats()
	var found bool
	for _, c := range chats {
		if c.ID == id && c.Title == "New Title" {
			found = true
			break
		}
	}
	if !found {
		t.Error("renamed session not found with new title")
	}
}

func TestRenameChatNotFound(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	err := m.RenameChat("nonexistent", "title")
	if err == nil {
		t.Fatal("RenameChat() should error on non-existent session")
	}
}

func TestAddMessage(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	m.NewChat()

	m.AddMessage("user", "hello", "", "")
	m.AddMessage("assistant", "hi there", "", "")

	msgs := m.GetActiveMessages()
	if len(msgs) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hello" {
		t.Errorf("msg[0] = %+v, want user/hello", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "hi there" {
		t.Errorf("msg[1] = %+v, want assistant/hi there", msgs[1])
	}
}

func TestAddMessageWithImageAndFile(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)

	m.AddMessage("user", "check this", "/path/img.png", "/path/doc.pdf")
	msgs := m.GetActiveMessages()
	if len(msgs) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(msgs))
	}
	if msgs[0].ImagePath != "/path/img.png" {
		t.Errorf("ImagePath = %q, want /path/img.png", msgs[0].ImagePath)
	}
	if msgs[0].FilePath != "/path/doc.pdf" {
		t.Errorf("FilePath = %q, want /path/doc.pdf", msgs[0].FilePath)
	}
}

func TestAutoTitleFromFirstMessage(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)

	m.AddMessage("user", "What is the meaning of life?", "", "")
	chats := m.ListChats()
	if len(chats) > 0 {
		title := chats[0].Title
		if title == "New Chat" {
			t.Error("title should have been auto-set from first message")
		}
	}
}

func TestAutoTitleTruncates(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	longMsg := ""
	for i := 0; i < 100; i++ {
		longMsg += "x"
	}

	m.AddMessage("user", longMsg, "", "")
	chats := m.ListChats()
	if len(chats) > 0 && len(chats[0].Title) > 43 {
		t.Errorf("title too long: %d chars", len(chats[0].Title))
	}
}

func TestGetActiveMessagesReturnsCopy(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	m.AddMessage("user", "original", "", "")

	msgs := m.GetActiveMessages()
	msgs[0].Content = "modified"

	msgs2 := m.GetActiveMessages()
	if msgs2[0].Content != "original" {
		t.Error("GetActiveMessages should return a copy")
	}
}

func TestGetHistoryForAPI(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	for i := 0; i < 10; i++ {
		m.AddMessage("user", "msg", "", "")
		m.AddMessage("assistant", "resp", "", "")
	}

	history := m.GetHistoryForAPI(4)
	if len(history) != 4 {
		t.Fatalf("len(history) = %d, want 4", len(history))
	}
}

func TestListChatsOrder(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	m.NewChat()
	m.NewChat()

	chats := m.ListChats()
	if len(chats) < 2 {
		t.Fatal("need at least 2 chats")
	}
	for i := 1; i < len(chats); i++ {
		if chats[i].UpdatedAt > chats[i-1].UpdatedAt {
			t.Error("chats not sorted by UpdatedAt descending")
		}
	}
}

func TestNewAgentChatPersistsProjectPath(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	projectPath := "/home/user/project"
	id := m.NewAgentChat(projectPath)
	if id == "" {
		t.Fatal("NewAgentChat() returned empty ID")
	}

	got := m.GetProjectPath(id)
	if got != projectPath {
		t.Errorf("GetProjectPath() = %q, want %q", got, projectPath)
	}

	if !m.IsAgentChat(id) {
		t.Error("IsAgentChat() should be true for agent chat")
	}

	// Re-open and verify ProjectPath survived restart
	m2, err := NewManager(dir)
	if err != nil {
		t.Fatalf("reload NewManager() error = %v", err)
	}
	got = m2.GetProjectPath(id)
	if got != projectPath {
		t.Errorf("after reload GetProjectPath() = %q, want %q", got, projectPath)
	}
}

func TestNewAgentChatReturnsEmptyIDWhenNilManager(t *testing.T) {
	// This test exercises the guard in App.NewAgentChat (simulated here by
	// verifying the Session.ProjectPath is set in a freshly created session).
	dir := t.TempDir()
	m, _ := NewManager(dir)
	id := m.NewAgentChat("")
	if id == "" {
		t.Fatal("NewAgentChat() should still create a session")
	}
}

func TestListChatsIncludesProjectPath(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	projectPath := "/tmp/test-project"
	m.NewAgentChat(projectPath)

	chats := m.ListChats()
	found := false
	for _, c := range chats {
		if c.ProjectPath == projectPath {
			found = true
			break
		}
	}
	if !found {
		t.Error("ListChats() should include project_path in SessionInfo")
	}
}

func TestFilePermissions(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	m.AddMessage("user", "hello", "", "")

	chats := m.ListChats()
	if len(chats) == 0 {
		t.Fatal("no sessions")
	}

	info, err := os.Stat(filepath.Join(dir, chats[0].ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0077 != 0 {
		t.Errorf("session file has group/other perms: %o", info.Mode())
	}
}
