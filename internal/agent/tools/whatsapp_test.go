package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type fakeWhatsAppClient struct {
	sendCtx       context.Context
	searchLimit   int
	messagesLimit int
	chatsLimit    int
	chats         []WhatsAppChat
}

func (f *fakeWhatsAppClient) SendMessage(ctx context.Context, jid, text string) (string, error) {
	f.sendCtx = ctx
	return "msg-id", nil
}

func (f *fakeWhatsAppClient) SearchMessages(query string, limit int) ([]WhatsAppMsg, error) {
	f.searchLimit = limit
	return nil, nil
}

func (f *fakeWhatsAppClient) GetChatList() ([]WhatsAppChat, error) {
	return f.chats, nil
}

func (f *fakeWhatsAppClient) GetChatMessages(chatJID string, limit int) ([]WhatsAppMsg, error) {
	f.messagesLimit = limit
	return nil, nil
}

func withFakeWhatsAppClient(t *testing.T, f *fakeWhatsAppClient) {
	t.Helper()
	old := WhatsAppClient
	WhatsAppClient = f
	t.Cleanup(func() { WhatsAppClient = old })
}

// TestSendWhatsApp_ForwardsCallerContext is the regression test for a real
// gap found in a 2026-09-23 security/reliability audit: SendWhatsApp used
// to discard the tool call's own ctx and substitute a fresh
// context.Background() for the one network call in this file — the only
// such call among the audited agent tool files. Cancelling the turn (user
// stops the chat, the turn's own deadline fires) used to leave the send
// running regardless. Verified here via a cancelled context: the fake
// client must observe it as already Done, proving it is the real caller
// ctx and not a substitute.
func TestSendWhatsApp_ForwardsCallerContext(t *testing.T) {
	f := &fakeWhatsAppClient{}
	withFakeWhatsAppClient(t, f)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	args, _ := json.Marshal(map[string]string{"jid": "123@s.whatsapp.net", "text": "hi"})
	if _, err := SendWhatsApp(ctx, args, "", nil); err != nil {
		t.Fatalf("SendWhatsApp() error = %v", err)
	}
	if f.sendCtx == nil {
		t.Fatal("WhatsAppClient.SendMessage was not called")
	}
	select {
	case <-f.sendCtx.Done():
	default:
		t.Error("SendMessage received a context that is not the (cancelled) caller ctx — still using context.Background()")
	}
}

// TestWhatsAppTools_ClampUnboundedLimit is the regression test for a
// suspected gap the same audit flagged: search_whatsapp/whatsapp_messages/
// whatsapp_chats only ever raised a non-positive Limit up to a small
// default, never capped a large one — a model-requested (or prompt-
// injected) limit of e.g. 100000 would have pulled that many rows straight
// into the tool result and from there into the LLM context.
func TestWhatsAppTools_ClampUnboundedLimit(t *testing.T) {
	bigChats := make([]WhatsAppChat, maxWhatsAppResultLimit+50)
	for i := range bigChats {
		bigChats[i] = WhatsAppChat{JID: "x", LastTime: time.Now()}
	}

	t.Run("search_whatsapp", func(t *testing.T) {
		f := &fakeWhatsAppClient{}
		withFakeWhatsAppClient(t, f)
		args, _ := json.Marshal(map[string]any{"query": "x", "limit": 100000})
		if _, err := SearchWhatsApp(context.Background(), args, "", nil); err != nil {
			t.Fatalf("SearchWhatsApp() error = %v", err)
		}
		if f.searchLimit != maxWhatsAppResultLimit {
			t.Errorf("SearchMessages limit = %d, want clamped to %d", f.searchLimit, maxWhatsAppResultLimit)
		}
	})

	t.Run("whatsapp_messages", func(t *testing.T) {
		f := &fakeWhatsAppClient{}
		withFakeWhatsAppClient(t, f)
		args, _ := json.Marshal(map[string]any{"jid": "123@s.whatsapp.net", "limit": 100000})
		if _, err := GetWhatsAppMessages(context.Background(), args, "", nil); err != nil {
			t.Fatalf("GetWhatsAppMessages() error = %v", err)
		}
		if f.messagesLimit != maxWhatsAppResultLimit {
			t.Errorf("GetChatMessages limit = %d, want clamped to %d", f.messagesLimit, maxWhatsAppResultLimit)
		}
	})

	t.Run("whatsapp_chats", func(t *testing.T) {
		f := &fakeWhatsAppClient{chats: bigChats}
		withFakeWhatsAppClient(t, f)
		args, _ := json.Marshal(map[string]any{"limit": 100000})
		out, err := LatestWhatsAppChats(context.Background(), args, "", nil)
		if err != nil {
			t.Fatalf("LatestWhatsAppChats() error = %v", err)
		}
		// One line per chat in the output — count newlines+1 as a proxy for
		// how many chats were actually rendered, and confirm it's capped.
		lines := 1
		for _, r := range out {
			if r == '\n' {
				lines++
			}
		}
		if lines > maxWhatsAppResultLimit {
			t.Errorf("LatestWhatsAppChats rendered %d lines, want <= %d", lines, maxWhatsAppResultLimit)
		}
	})
}

func TestSearchWhatsApp_DefaultsNonPositiveLimit(t *testing.T) {
	f := &fakeWhatsAppClient{}
	withFakeWhatsAppClient(t, f)
	args, _ := json.Marshal(map[string]any{"query": "x", "limit": 0})
	if _, err := SearchWhatsApp(context.Background(), args, "", nil); err != nil {
		t.Fatalf("SearchWhatsApp() error = %v", err)
	}
	if f.searchLimit != 10 {
		t.Errorf("SearchMessages limit = %d, want default 10", f.searchLimit)
	}
}
