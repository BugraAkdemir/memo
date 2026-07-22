package whatsapp

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	waEvent "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// TestHandleMessage_SavesImageCaption is the regression test for BUG-H6:
// handleMessage (the live-message path) only ever checked
// GetConversation()/GetExtendedTextMessage(), silently returning — no save,
// no queue push, no log — for any message whose only text was an
// image/video/document caption. The package's own extractText helper
// already handled those correctly but was, before this fix, only ever
// called by handleHistorySync, never by handleMessage — so the exact same
// message arriving live behaved completely differently than one arriving
// via a post-reconnect history sync.
func TestHandleMessage_SavesImageCaption(t *testing.T) {
	store := newTestStore(t)
	c := &Client{store: store, msgCh: make(chan Message, 1)}

	chatJID := types.NewJID("123", types.DefaultUserServer)
	evt := &waEvent.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:   chatJID,
				Sender: types.NewJID("456", types.DefaultUserServer),
			},
			ID:        "test-image-caption",
			PushName:  "Ali",
			Timestamp: time.Now(),
		},
		Message: &waE2E.Message{
			ImageMessage: &waE2E.ImageMessage{Caption: proto.String("bakin bu fotograf")},
		},
	}

	c.handleMessage(evt)

	select {
	case msg := <-c.msgCh:
		if msg.Text != "bakin bu fotograf" {
			t.Errorf("msg.Text = %q, want %q", msg.Text, "bakin bu fotograf")
		}
	default:
		t.Fatal("expected a message on msgCh, got none — live image-caption message was silently dropped")
	}

	saved, err := store.GetChatMessages(chatJID.String(), 10)
	if err != nil {
		t.Fatalf("GetChatMessages() error = %v", err)
	}
	if len(saved) != 1 {
		t.Fatalf("len(saved) = %d, want 1 — the image-caption message must be persisted, not silently dropped", len(saved))
	}
	if saved[0].Text != "bakin bu fotograf" {
		t.Errorf("saved[0].Text = %q, want %q", saved[0].Text, "bakin bu fotograf")
	}
}

// TestHandleMessage_NoTextAtAllIsIgnored confirms the fix didn't overreach:
// a message with genuinely no text content anywhere (e.g. a reaction, a
// receipt-only event) must still be silently ignored, not turned into an
// empty saved message.
func TestHandleMessage_NoTextAtAllIsIgnored(t *testing.T) {
	store := newTestStore(t)
	c := &Client{store: store, msgCh: make(chan Message, 1)}

	evt := &waEvent.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:   types.NewJID("123", types.DefaultUserServer),
				Sender: types.NewJID("456", types.DefaultUserServer),
			},
			ID:        "test-empty",
			Timestamp: time.Now(),
		},
		Message: &waE2E.Message{},
	}

	c.handleMessage(evt)

	select {
	case msg := <-c.msgCh:
		t.Fatalf("expected no message on msgCh for an empty message, got %+v", msg)
	default:
	}
}
