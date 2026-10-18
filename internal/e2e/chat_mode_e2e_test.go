// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func chatInfo(t *testing.T, h *Harness, id string) (title, project string) {
	t.Helper()
	var chats []struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		ProjectPath string `json:"project_path"`
	}
	decodeInto(t, h.getJSON("/api/chats"), &chats)
	for _, c := range chats {
		if c.ID == id {
			return c.Title, c.ProjectPath
		}
	}
	t.Fatalf("chat %s not in the list", id)
	return "", ""
}

// The new-chat screen starts every chat as a plain one and lets the person pick
// Chat or Agent before typing. That choice is made once: it is refused after the
// first message.
func TestChatProject_AnEmptyChatCanBecomeAnAgentChatAndBack_ButNotAfterAMessage(t *testing.T) {
	h := NewHarness(t)
	h.Fake.Script = func(int, FakeChatRequest) FakeChatResponse { return FakeChatResponse{Text: "tamam"} }
	chat := h.NewChat()
	dir := t.TempDir()

	resp := h.postJSON("/api/chats/project", map[string]string{"chat_id": chat, "project_path": dir})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("empty chat -> agent: status %d", resp.StatusCode)
	}
	if title, project := chatInfo(t, h, chat); project != dir || title != "Agent Chat" {
		t.Fatalf("after switching to agent: title=%q project=%q", title, project)
	}

	resp = h.postJSON("/api/chats/project", map[string]string{"chat_id": chat, "project_path": ""})
	resp.Body.Close()
	if title, project := chatInfo(t, h, chat); project != "" || title != "New Chat" {
		t.Fatalf("after switching back: title=%q project=%q", title, project)
	}

	// Once there is a message, the kind is fixed.
	h.SendMessageStream(chat, "merhaba")
	resp = h.postJSON("/api/chats/project", map[string]string{"chat_id": chat, "project_path": dir})
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("switching a chat that has messages: status %d, want 409", resp.StatusCode)
	}
	if _, project := chatInfo(t, h, chat); project != "" {
		t.Fatal("the refused request changed the chat anyway")
	}

	resp = h.postJSON("/api/chats/project", map[string]string{"chat_id": "nope", "project_path": dir})
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown chat: status %d, want 404", resp.StatusCode)
	}
}

// An agent chat used to keep "Agent Chat" forever (Code Mode skips the LLM title
// call), so the list became a column of identical names. It is now named from the
// person's first words, with no extra model call.
func TestAgentChat_IsTitledFromTheFirstWordsWithoutAnExtraModelCall(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	calls := 0
	h.Fake.Script = func(int, FakeChatRequest) FakeChatResponse {
		calls++
		return FakeChatResponse{Text: "tamam"}
	}
	chat := h.NewAgentChat(t.TempDir())
	h.SendMessageStream(chat, "@AGENTS.md @handoff.md bu projede en son neyi yapmışız peki bakalım")

	deadline := time.Now().Add(3 * time.Second)
	var title string
	for time.Now().Before(deadline) {
		title, _ = chatInfo(t, h, chat)
		if title != "Agent Chat" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if title != "bu projede en son neyi yapmışız" {
		t.Fatalf("title = %q, want the first words without the @mentions", title)
	}
	time.Sleep(200 * time.Millisecond)
	if calls != 1 {
		t.Errorf("model calls = %d, want 1 (the turn itself; no title request)", calls)
	}
	if strings.Contains(title, "@") {
		t.Error("mentions leaked into the title")
	}
}

// A name the person already gave is never replaced.
func TestAgentChat_TitleNeverReplacesAUserRename(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	h.Fake.Script = func(int, FakeChatRequest) FakeChatResponse { return FakeChatResponse{Text: "tamam"} }
	chat := h.NewAgentChat(t.TempDir())
	h.postJSON("/api/chats/rename", map[string]string{"id": chat, "title": "Benim adım"}).Body.Close()
	h.SendMessageStream(chat, "bir şeyler yap")
	time.Sleep(400 * time.Millisecond)
	if title, _ := chatInfo(t, h, chat); title != "Benim adım" {
		t.Fatalf("title = %q, the user's name must survive", title)
	}
}
