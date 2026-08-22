package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"memo/internal/replcli"
)

func TestOneLine_CollapsesWhitespaceAndTruncates(t *testing.T) {
	got := oneLine("hello\n\nworld   with   spaces")
	if got != "hello world with spaces" {
		t.Errorf("oneLine collapsed whitespace wrong: %q", got)
	}

	long := strings.Repeat("a", 150)
	got = oneLine(long)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("oneLine(150 chars) = %q, want it to end with an ellipsis", got)
	}
	if trimmed := strings.TrimSuffix(got, "…"); len(trimmed) != 100 {
		t.Errorf("oneLine(150 chars) kept %d chars before the ellipsis, want 100", len(trimmed))
	}
}

func TestChatListCmd_SwitchesThenListsMessages(t *testing.T) {
	var switchedTo string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/chats/switch":
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			switchedTo = body["id"]
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/messages":
			json.NewEncoder(w).Encode([]replcli.ChatMessage{
				{Role: "user", Content: "merhaba", Timestamp: "2026-08-22T10:00:00Z"},
				{Role: "assistant", Content: "selam!", Timestamp: "2026-08-22T10:00:01Z", MemoryUsed: 3},
			})
		}
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := chatListCmd(ctx, client, "chat-123"); code != 0 {
		t.Fatalf("chatListCmd returned %d, want 0", code)
	}
	if switchedTo != "chat-123" {
		t.Errorf("switched to chat %q, want %q", switchedTo, "chat-123")
	}
}

func TestChatListCmd_EmptyChatSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/messages" {
			json.NewEncoder(w).Encode([]replcli.ChatMessage{})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := chatListCmd(ctx, client, "chat-empty"); code != 0 {
		t.Fatalf("chatListCmd returned %d, want 0 for an empty chat", code)
	}
}

func TestChatListCmd_FailsWhenSwitchFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := chatListCmd(ctx, client, "does-not-exist"); code == 0 {
		t.Fatal("expected a non-zero exit code when SwitchChat fails")
	}
}

func TestChatMemoryUsageCmd_SumsMemoryUsedAcrossMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/chats/switch":
			w.WriteHeader(http.StatusOK)
		case "/api/messages":
			json.NewEncoder(w).Encode([]replcli.ChatMessage{
				{Role: "user", Content: "soru 1", Timestamp: "t1"},
				{Role: "assistant", Content: "cevap 1", Timestamp: "t2", MemoryUsed: 2},
				{Role: "user", Content: "soru 2", Timestamp: "t3"},
				{Role: "assistant", Content: "cevap 2", Timestamp: "t4", MemoryUsed: 5},
			})
		}
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got := captureStdout(t, func() {
		if code := chatMemoryUsageCmd(ctx, client, "chat-1"); code != 0 {
			t.Fatalf("chatMemoryUsageCmd returned %d, want 0", code)
		}
	})
	if !strings.Contains(got, "Toplam / total: 7") {
		t.Errorf("expected the printed total to be 7, got: %q", got)
	}
}

func TestChatMemoryUsageCmd_NoMemoryUsedAnywhereSaysSo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/chats/switch":
			w.WriteHeader(http.StatusOK)
		case "/api/messages":
			json.NewEncoder(w).Encode([]replcli.ChatMessage{
				{Role: "user", Content: "merhaba", Timestamp: "t1"},
				{Role: "assistant", Content: "selam", Timestamp: "t2"},
			})
		}
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got := captureStdout(t, func() {
		if code := chatMemoryUsageCmd(ctx, client, "chat-1"); code != 0 {
			t.Fatalf("chatMemoryUsageCmd returned %d, want 0", code)
		}
	})
	if !strings.Contains(got, "No message in this chat used memory") {
		t.Errorf("expected the no-memory-used message, got: %q", got)
	}
}

// TestValidateMemoryQuery_UsageProceeds is the one case that must NOT be
// handled inline — the caller goes on to make the real network calls.
func TestValidateMemoryQuery_UsageProceeds(t *testing.T) {
	code, handled := validateMemoryQuery("usage")
	if handled {
		t.Errorf("validateMemoryQuery(\"usage\") handled = true, want false (caller must proceed)")
	}
	_ = code
}

// TestValidateMemoryQuery_SavedRefusesHonestly is the actual point of this
// whole helper: "saved" must fail loudly with an explanation rather than
// silently return an empty/misleading result, since internal/memory.Store
// entries have no source-chat-ID to filter by.
func TestValidateMemoryQuery_SavedRefusesHonestly(t *testing.T) {
	got := captureStderr(t, func() {
		code, handled := validateMemoryQuery("saved")
		if !handled || code == 0 {
			t.Errorf("validateMemoryQuery(\"saved\") = (%d, %v), want a non-zero code and handled=true", code, handled)
		}
	})
	if !strings.Contains(got, "chat_id") {
		t.Errorf("expected the refusal to explain the missing chat_id field, got: %q", got)
	}
}

func TestValidateMemoryQuery_UnknownValueRejected(t *testing.T) {
	code, handled := validateMemoryQuery("bogus")
	if !handled || code == 0 {
		t.Errorf("validateMemoryQuery(\"bogus\") = (%d, %v), want a non-zero code and handled=true", code, handled)
	}
}
