package whatsapp

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "wa_test.db")
	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestNewStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer s.Close()

	if s.db == nil {
		t.Error("db is nil")
	}
}

func TestNewStore_InvalidPath(t *testing.T) {
	_, err := NewStore("/nonexistent/dir/store.db")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestSaveAndGetMessage(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Second)

	msg := Message{
		ID:         "msg1",
		ChatJID:    "123@s.whatsapp.net",
		SenderJID:  "456@s.whatsapp.net",
		SenderName: "Alice",
		Text:       "Hello!",
		Timestamp:  now,
		FromMe:     false,
	}

	if err := s.SaveMessage(msg); err != nil {
		t.Fatalf("SaveMessage failed: %v", err)
	}

	msgs, err := s.GetChatMessages("123@s.whatsapp.net", 10)
	if err != nil {
		t.Fatalf("GetChatMessages failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].ID != "msg1" {
		t.Errorf("id = %q, want %q", msgs[0].ID, "msg1")
	}
	if msgs[0].Text != "Hello!" {
		t.Errorf("text = %q, want %q", msgs[0].Text, "Hello!")
	}
	if !msgs[0].Timestamp.Equal(now) {
		t.Errorf("timestamp = %v, want %v", msgs[0].Timestamp, now)
	}
	if msgs[0].FromMe != false {
		t.Errorf("fromMe = %v, want false", msgs[0].FromMe)
	}
}

func TestSaveDuplicateMessage(t *testing.T) {
	s := newTestStore(t)
	msg := Message{
		ID:        "dup1",
		ChatJID:   "123@s.whatsapp.net",
		SenderJID: "456@s.whatsapp.net",
		Text:      "first",
		Timestamp: time.Now(),
	}
	if err := s.SaveMessage(msg); err != nil {
		t.Fatalf("first SaveMessage failed: %v", err)
	}

	msg.Text = "second"
	if err := s.SaveMessage(msg); err != nil {
		t.Fatalf("second SaveMessage failed: %v", err)
	}

	msgs, _ := s.GetChatMessages("123@s.whatsapp.net", 10)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message (duplicate ignored), got %d", len(msgs))
	}
	if msgs[0].Text != "first" {
		t.Errorf("text = %q, want %q (INSERT OR IGNORE should keep original)", msgs[0].Text, "first")
	}
}

func TestSaveAndGetContact(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveContact("123@s.whatsapp.net", "Alice"); err != nil {
		t.Fatalf("SaveContact failed: %v", err)
	}

	name := s.GetContactName("123@s.whatsapp.net")
	if name != "Alice" {
		t.Errorf("name = %q, want %q", name, "Alice")
	}
}

func TestUpdateContact(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveContact("123@s.whatsapp.net", "Alice"); err != nil {
		t.Fatalf("save Alice failed: %v", err)
	}
	if err := s.SaveContact("123@s.whatsapp.net", "Alice Updated"); err != nil {
		t.Fatalf("save Alice Updated failed: %v", err)
	}

	name := s.GetContactName("123@s.whatsapp.net")
	if name != "Alice Updated" {
		t.Errorf("name = %q, want %q", name, "Alice Updated")
	}
}

func TestGetContactName_NotFound(t *testing.T) {
	s := newTestStore(t)
	name := s.GetContactName("nonexistent@s.whatsapp.net")
	if name != "" {
		t.Errorf("expected empty, got %q", name)
	}
}

func TestSearchMessages(t *testing.T) {
	s := newTestStore(t)
	msgs := []Message{
		{ID: "m1", ChatJID: "c1", SenderJID: "s1", Text: "hello world", Timestamp: time.Unix(1000, 0)},
		{ID: "m2", ChatJID: "c1", SenderJID: "s1", Text: "goodbye moon", Timestamp: time.Unix(2000, 0)},
		{ID: "m3", ChatJID: "c2", SenderJID: "s1", Text: "hello again", Timestamp: time.Unix(3000, 0)},
	}
	for _, m := range msgs {
		if err := s.SaveMessage(m); err != nil {
			t.Fatalf("SaveMessage failed: %v", err)
		}
	}

	tests := []struct {
		query string
		want  int
	}{
		{"hello", 2},
		{"moon", 1},
		{"nonexistent", 0},
		{"", 3},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results, err := s.SearchMessages(tt.query, 10)
			if err != nil {
				t.Fatalf("SearchMessages failed: %v", err)
			}
			if len(results) != tt.want {
				t.Errorf("got %d results, want %d", len(results), tt.want)
			}
		})
	}
}

func TestSearchMessages_Limit(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 10; i++ {
		m := Message{
			ID:        fmt.Sprintf("m%d", i),
			ChatJID:   "c1",
			SenderJID: "s1",
			Text:      "hello",
			Timestamp: time.Unix(int64(i*100), 0),
		}
		s.SaveMessage(m)
	}

	results, err := s.SearchMessages("hello", 3)
	if err != nil {
		t.Fatalf("SearchMessages failed: %v", err)
	}
	if len(results) > 3 {
		t.Errorf("got %d results, want <= 3", len(results))
	}
}

func TestSearchMessages_DefaultLimit(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 60; i++ {
		m := Message{
			ID:        fmt.Sprintf("m%d", i),
			ChatJID:   "c1",
			SenderJID: "s1",
			Text:      "data",
			Timestamp: time.Unix(int64(i*100), 0),
		}
		s.SaveMessage(m)
	}

	results, err := s.SearchMessages("data", 0)
	if err != nil {
		t.Fatalf("SearchMessages failed: %v", err)
	}
	if len(results) > 50 {
		t.Errorf("got %d results, want <= 50 (default limit)", len(results))
	}
}

func TestGetMessagesSince(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)

	msgs := []Message{
		{ID: "m1", ChatJID: "c1", SenderJID: "s1", Text: "old", Timestamp: base.Add(-1 * time.Hour)},
		{ID: "m2", ChatJID: "c1", SenderJID: "s1", Text: "new", Timestamp: base.Add(1 * time.Hour)},
	}
	for _, m := range msgs {
		s.SaveMessage(m)
	}

	results, err := s.GetMessagesSince(base, 10)
	if err != nil {
		t.Fatalf("GetMessagesSince failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 message after base, got %d", len(results))
	}
	if results[0].ID != "m2" {
		t.Errorf("id = %q, want %q", results[0].ID, "m2")
	}
}

func TestGetMessagesSince_DefaultLimit(t *testing.T) {
	s := newTestStore(t)
	base := time.Unix(0, 0)
	for i := 0; i < 120; i++ {
		m := Message{
			ID:        fmt.Sprintf("m%d", i),
			ChatJID:   "c1",
			SenderJID: "s1",
			Text:      "x",
			Timestamp: time.Unix(int64(i*100), 0),
		}
		s.SaveMessage(m)
	}

	results, err := s.GetMessagesSince(base, 0)
	if err != nil {
		t.Fatalf("GetMessagesSince failed: %v", err)
	}
	if len(results) > 100 {
		t.Errorf("got %d results, want <= 100 (default limit)", len(results))
	}
}

func TestGetChatList(t *testing.T) {
	s := newTestStore(t)
	msgs := []Message{
		{ID: "m1", ChatJID: "c1@s.whatsapp.net", SenderJID: "s1", Text: "first chat msg", Timestamp: time.Unix(100, 0), FromMe: false},
		{ID: "m2", ChatJID: "c2@s.whatsapp.net", SenderJID: "s1", Text: "second chat msg", Timestamp: time.Unix(200, 0), FromMe: true},
	}
	for _, m := range msgs {
		s.SaveMessage(m)
	}

	chats, err := s.GetChatList()
	if err != nil {
		t.Fatalf("GetChatList failed: %v", err)
	}
	if len(chats) != 2 {
		t.Fatalf("expected 2 chats, got %d", len(chats))
	}
}

func TestGetChatList_Empty(t *testing.T) {
	s := newTestStore(t)
	chats, err := s.GetChatList()
	if err != nil {
		t.Fatalf("GetChatList failed: %v", err)
	}
	if len(chats) != 0 {
		t.Errorf("expected empty list, got %d chats", len(chats))
	}
}

func TestGetChatMessages(t *testing.T) {
	s := newTestStore(t)
	msgs := []Message{
		{ID: "m1", ChatJID: "c1", SenderJID: "s1", Text: "a", Timestamp: time.Unix(1, 0)},
		{ID: "m2", ChatJID: "c1", SenderJID: "s1", Text: "b", Timestamp: time.Unix(2, 0)},
		{ID: "m3", ChatJID: "c2", SenderJID: "s1", Text: "c", Timestamp: time.Unix(3, 0)},
	}
	for _, m := range msgs {
		s.SaveMessage(m)
	}

	results, err := s.GetChatMessages("c1", 10)
	if err != nil {
		t.Fatalf("GetChatMessages failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 messages for c1, got %d", len(results))
	}
}

func TestGetChatMessages_DefaultLimit(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 60; i++ {
		m := Message{
			ID:        fmt.Sprintf("m%d", i),
			ChatJID:   "c1",
			SenderJID: "s1",
			Text:      "x",
			Timestamp: time.Unix(int64(i*100), 0),
		}
		s.SaveMessage(m)
	}

	results, err := s.GetChatMessages("c1", 0)
	if err != nil {
		t.Fatalf("GetChatMessages failed: %v", err)
	}
	if len(results) > 50 {
		t.Errorf("got %d results, want <= 50 (default limit)", len(results))
	}
}

func TestStats(t *testing.T) {
	s := newTestStore(t)
	msgs := []Message{
		{ID: "m1", ChatJID: "c1", SenderJID: "s1", Text: "a", Timestamp: time.Now().Add(-2 * 24 * time.Hour)},
		{ID: "m2", ChatJID: "c1", SenderJID: "s1", Text: "b", Timestamp: time.Now().Add(-1 * time.Hour)},
		{ID: "m3", ChatJID: "c1", SenderJID: "s1", Text: "c", Timestamp: time.Now()},
	}
	for _, m := range msgs {
		s.SaveMessage(m)
	}

	total, last24h, err := s.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if last24h != 2 {
		t.Errorf("last24h = %d, want 2", last24h)
	}
}

func TestStats_Empty(t *testing.T) {
	s := newTestStore(t)
	total, last24h, err := s.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if last24h != 0 {
		t.Errorf("last24h = %d, want 0", last24h)
	}
}

func TestClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "close_test.db")
	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	// Closing again should be safe (dereferenced db)
	if err := s.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
}

func TestConcurrentWrites(t *testing.T) {
	s := newTestStore(t)
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			msg := Message{
				ID:        fmt.Sprintf("concurrent-%d", n),
				ChatJID:   "chat1",
				SenderJID: "sender1",
				Text:      fmt.Sprintf("msg %d", n),
				Timestamp: time.Now(),
			}
			if err := s.SaveMessage(msg); err != nil {
				t.Errorf("concurrent save %d failed: %v", n, err)
			}
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	msgs, err := s.GetChatMessages("chat1", 20)
	if err != nil {
		t.Fatalf("GetChatMessages failed: %v", err)
	}
	if len(msgs) != 10 {
		t.Errorf("expected 10 messages, got %d", len(msgs))
	}
}

func TestPersistData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "persist.db")

	func() {
		s, err := NewStore(path)
		if err != nil {
			t.Fatalf("NewStore failed: %v", err)
		}
		defer s.Close()
		s.SaveMessage(Message{ID: "p1", ChatJID: "c1", SenderJID: "s1", Text: "persist", Timestamp: time.Now()})
		s.SaveContact("c1@s.whatsapp.net", "Contact1")
	}()

	s2, err := NewStore(path)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer s2.Close()

	msgs, _ := s2.GetChatMessages("c1", 10)
	if len(msgs) != 1 {
		t.Errorf("expected 1 message after reopen, got %d", len(msgs))
	}
	if name := s2.GetContactName("c1@s.whatsapp.net"); name != "Contact1" {
		t.Errorf("contact name = %q, want %q", name, "Contact1")
	}
}

func TestGetChatListDisplayName(t *testing.T) {
	s := newTestStore(t)
	s.SaveMessage(Message{ID: "m1", ChatJID: "alice@s.whatsapp.net", SenderJID: "s1", Text: "hi", Timestamp: time.Now()})
	s.SaveContact("alice@s.whatsapp.net", "Alice")

	chats, err := s.GetChatList()
	if err != nil {
		t.Fatalf("GetChatList failed: %v", err)
	}
	if len(chats) != 1 {
		t.Fatalf("expected 1 chat, got %d", len(chats))
	}
	if chats[0].DisplayName != "Alice" {
		t.Errorf("display name = %q, want %q", chats[0].DisplayName, "Alice")
	}
}

func TestNewStore_DBFileCreated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "created.db")
	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	s.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}
