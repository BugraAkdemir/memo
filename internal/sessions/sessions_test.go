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
	// NewManager's own bootstrap default session never receives a message
	// and is deliberately never persisted (see newSession's doc comment) —
	// only the chat created and used below survives a reload.
	m1.NewChat()
	m1.AddMessage("user", "hello", "", "")

	m2, err := NewManager(dir)
	if err != nil {
		t.Fatalf("reload NewManager() error = %v", err)
	}
	chats := m2.ListChats()
	if len(chats) != 1 {
		t.Fatalf("len(chats) = %d, want 1 (the empty bootstrap default must not survive a reload, only the chat with a real message)", len(chats))
	}
}

// TestEmptyChatsDoNotSurviveReload is the regression test for the "empty
// Agent Chat / New Chat clutters the sidebar forever" bug: creating chats
// and never sending a message into them (the CLI's startFreshChat does
// exactly this on every `memo` launch) used to persist each one to disk
// immediately on creation, so opening and closing the CLI without ever
// typing anything still left a permanent, empty entry in every future
// chat list — indistinguishable from a chat the user actually meant to
// keep.
func TestEmptyChatsDoNotSurviveReload(t *testing.T) {
	dir := t.TempDir()
	m1, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	m1.NewChat()
	m1.NewAgentChat("/home/user/project")
	m1.NewAgentChat("/home/user/other-project")
	if got := len(m1.ListChats()); got != 4 {
		t.Fatalf("len(ListChats()) before reload = %d, want 4 (bootstrap default + NewChat + 2 NewAgentChat, all live in memory even though unsaved)", got)
	}

	m2, err := NewManager(dir)
	if err != nil {
		t.Fatalf("reload NewManager() error = %v", err)
	}
	if got := len(m2.ListChats()); got != 1 {
		t.Fatalf("len(ListChats()) after reload = %d, want 1 (only m2's own fresh bootstrap default — none of m1's empty, message-less chats should have left a file on disk)", got)
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

// TestSessionScopedHistory_IsolatedBetweenSessions is Phase 1's completion
// test for PLAN_chatid_refactor.md: two sessions in the same manager, write
// to one via the *ForSession variants, and confirm the other's history is
// completely untouched — the foundation the rest of the refactor (explicit
// chatID threaded through the send pipeline instead of the global "active"
// session) depends on.
func TestSessionScopedHistory_IsolatedBetweenSessions(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	first := m.GetActiveID()
	second := m.NewChat()

	m.AddMessageToSession(first, "user", "hello from first", "", "")
	m.AddMessageToSession(second, "user", "hello from second", "", "")
	m.AddMessageToSession(second, "assistant", "reply in second", "", "")

	firstMsgs := m.GetActiveMessagesForSession(first)
	if len(firstMsgs) != 1 || firstMsgs[0].Content != "hello from first" {
		t.Fatalf("first session messages = %+v, want exactly [hello from first]", firstMsgs)
	}

	secondMsgs := m.GetActiveMessagesForSession(second)
	if len(secondMsgs) != 2 {
		t.Fatalf("second session messages = %+v, want 2", secondMsgs)
	}

	firstHistory := m.GetHistoryForAPIForSession(first, 10)
	if len(firstHistory) != 1 {
		t.Fatalf("GetHistoryForAPIForSession(first) = %+v, want 1 message", firstHistory)
	}
	secondHistory := m.GetHistoryForAPIForSession(second, 10)
	if len(secondHistory) != 2 {
		t.Fatalf("GetHistoryForAPIForSession(second) = %+v, want 2 messages", secondHistory)
	}

	firstTokenHistory := m.GetHistoryForAPITokenAwareForSession(first, 100_000)
	if len(firstTokenHistory) != 1 {
		t.Fatalf("GetHistoryForAPITokenAwareForSession(first) = %+v, want 1 message", firstTokenHistory)
	}
	secondTokenHistory := m.GetHistoryForAPITokenAwareForSession(second, 100_000)
	if len(secondTokenHistory) != 2 {
		t.Fatalf("GetHistoryForAPITokenAwareForSession(second) = %+v, want 2 messages", secondTokenHistory)
	}
}

// TestGetHistoryForAPI_MatchesActiveSessionVariant confirms the global
// GetHistoryForAPI/GetHistoryForAPITokenAware wrappers still behave exactly
// like before now that they delegate to the *ForSession variants keyed by
// GetActiveID() — same public contract, deduplicated implementation.
func TestGetHistoryForAPI_MatchesActiveSessionVariant(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	m.AddMessage("user", "msg", "", "")
	m.AddMessage("assistant", "resp", "", "")

	global := m.GetHistoryForAPI(10)
	scoped := m.GetHistoryForAPIForSession(m.GetActiveID(), 10)
	if len(global) != len(scoped) || len(global) != 2 {
		t.Fatalf("GetHistoryForAPI() = %+v, GetHistoryForAPIForSession() = %+v", global, scoped)
	}

	globalTok := m.GetHistoryForAPITokenAware(100_000)
	scopedTok := m.GetHistoryForAPITokenAwareForSession(m.GetActiveID(), 100_000)
	if len(globalTok) != len(scopedTok) || len(globalTok) != 2 {
		t.Fatalf("GetHistoryForAPITokenAware() = %+v, GetHistoryForAPITokenAwareForSession() = %+v", globalTok, scopedTok)
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

	// A chat with no messages yet is deliberately NOT persisted (see
	// newSession's doc comment — this is what stops an opened-and-abandoned
	// CLI/agent chat from cluttering every future chat list forever), so
	// ProjectPath must survive a restart only once the chat actually has
	// content.
	m.AddMessage("user", "hello", "", "")

	m2, err := NewManager(dir)
	if err != nil {
		t.Fatalf("reload NewManager() error = %v", err)
	}
	got = m2.GetProjectPath(id)
	if got != projectPath {
		t.Errorf("after reload GetProjectPath() = %q, want %q", got, projectPath)
	}
}

// TestNewAgentChatProjectPathNotPersistedWithoutAMessage is
// TestNewAgentChatPersistsProjectPath's counterpart, confirming the new
// deliberate behavior directly: an agent chat that never received a
// message must NOT reappear after a restart.
func TestNewAgentChatProjectPathNotPersistedWithoutAMessage(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	id := m.NewAgentChat("/home/user/project")
	if id == "" {
		t.Fatal("NewAgentChat() returned empty ID")
	}

	m2, err := NewManager(dir)
	if err != nil {
		t.Fatalf("reload NewManager() error = %v", err)
	}
	if got := m2.GetProjectPath(id); got != "" {
		t.Errorf("after reload GetProjectPath() = %q, want empty — a message-less agent chat must not survive a restart", got)
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

func TestCLIProvider_DefaultsEmptyAndIsPerChat(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	chatA := m.NewChat()
	chatB := m.NewChat()

	if got := m.GetCLIProvider(chatA); got != "" {
		t.Errorf("new chat CLIProvider = %q, want empty", got)
	}

	if err := m.SetCLIProvider(chatA, "claude-code-cli"); err != nil {
		t.Fatalf("SetCLIProvider: %v", err)
	}
	if got := m.GetCLIProvider(chatA); got != "claude-code-cli" {
		t.Errorf("chatA CLIProvider = %q, want claude-code-cli", got)
	}
	if got := m.GetCLIProvider(chatB); got != "" {
		t.Errorf("chatB CLIProvider = %q, want empty (setting chatA must not affect chatB)", got)
	}
}

func TestCLIProvider_UnknownChatReturnsError(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	if err := m.SetCLIProvider("does-not-exist", "claude-code-cli"); err == nil {
		t.Error("expected an error for an unknown chat id")
	}
}

func TestCLISessionID_RoundTripsPerProvider(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	chat := m.NewChat()

	if got := m.GetCLISessionID(chat, "claude-code-cli"); got != "" {
		t.Errorf("initial CLISessionID = %q, want empty", got)
	}

	if err := m.SetCLISessionID(chat, "claude-code-cli", "sess-abc"); err != nil {
		t.Fatalf("SetCLISessionID: %v", err)
	}
	if got := m.GetCLISessionID(chat, "claude-code-cli"); got != "sess-abc" {
		t.Errorf("CLISessionID = %q, want sess-abc", got)
	}
	// A different provider on the same chat must not see the first one's id.
	if got := m.GetCLISessionID(chat, "codex-cli"); got != "" {
		t.Errorf("codex-cli CLISessionID = %q, want empty (per-provider isolation)", got)
	}
}

func TestCLIWorkdir_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	chat := m.NewChat()

	if got := m.GetCLIWorkdir(chat); got != "" {
		t.Errorf("initial CLIWorkdir = %q, want empty", got)
	}
	if err := m.SetCLIWorkdir(chat, "/home/user/project"); err != nil {
		t.Fatalf("SetCLIWorkdir: %v", err)
	}
	if got := m.GetCLIWorkdir(chat); got != "/home/user/project" {
		t.Errorf("CLIWorkdir = %q, want /home/user/project", got)
	}
}

func TestCLIModel_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	chat := m.NewChat()

	if got := m.GetCLIModel(chat); got != "" {
		t.Errorf("initial CLIModel = %q, want empty", got)
	}
	if err := m.SetCLIModel(chat, "opus"); err != nil {
		t.Fatalf("SetCLIModel: %v", err)
	}
	if got := m.GetCLIModel(chat); got != "opus" {
		t.Errorf("CLIModel = %q, want opus", got)
	}
}

func TestCLIFields_SurviveReload(t *testing.T) {
	dir := t.TempDir()
	m1, _ := NewManager(dir)
	chat := m1.NewChat()
	m1.AddMessageToSession(chat, "user", "hi", "", "") // must be persisted to survive reload
	if err := m1.SetCLIProvider(chat, "claude-code-cli"); err != nil {
		t.Fatalf("SetCLIProvider: %v", err)
	}
	if err := m1.SetCLISessionID(chat, "claude-code-cli", "sess-xyz"); err != nil {
		t.Fatalf("SetCLISessionID: %v", err)
	}
	if err := m1.SetCLIWorkdir(chat, "/tmp/proj"); err != nil {
		t.Fatalf("SetCLIWorkdir: %v", err)
	}
	if err := m1.SetCLIModel(chat, "opus"); err != nil {
		t.Fatalf("SetCLIModel: %v", err)
	}

	m2, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager (reload): %v", err)
	}
	if got := m2.GetCLIProvider(chat); got != "claude-code-cli" {
		t.Errorf("reloaded CLIProvider = %q", got)
	}
	if got := m2.GetCLISessionID(chat, "claude-code-cli"); got != "sess-xyz" {
		t.Errorf("reloaded CLISessionID = %q", got)
	}
	if got := m2.GetCLIWorkdir(chat); got != "/tmp/proj" {
		t.Errorf("reloaded CLIWorkdir = %q", got)
	}
	if got := m2.GetCLIModel(chat); got != "opus" {
		t.Errorf("reloaded CLIModel = %q", got)
	}
}
