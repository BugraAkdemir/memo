package whatsapp

import (
	"context"
	"strings"
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

func imageEvent(id, caption string) *waEvent.Message {
	im := &waE2E.ImageMessage{Mimetype: proto.String("image/jpeg"), FileLength: proto.Uint64(1234)}
	if caption != "" {
		im.Caption = proto.String(caption)
	}
	return &waEvent.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:   types.NewJID("123", types.DefaultUserServer),
				Sender: types.NewJID("456", types.DefaultUserServer),
			},
			ID:        id,
			PushName:  "Ali",
			Timestamp: time.Now(),
		},
		Message: &waE2E.Message{ImageMessage: im},
	}
}

// A picture with no caption used to be dropped (no text to extract). It must now
// reach the consumer carrying its image handle, but leave nothing in the text
// store (an empty row would be noise in search).
func TestHandleMessage_PictureWithoutCaptionIsDeliveredButNotStored(t *testing.T) {
	store := newTestStore(t)
	c := &Client{store: store, msgCh: make(chan Message, 1)}

	c.handleMessage(imageEvent("pic-1", ""))

	select {
	case msg := <-c.msgCh:
		if msg.Image == nil || msg.Image.MimeType != "image/jpeg" || msg.Image.Size != 1234 {
			t.Fatalf("image handle = %+v", msg.Image)
		}
		if msg.Text != "" {
			t.Errorf("Text = %q, want empty", msg.Text)
		}
	default:
		t.Fatal("a caption-less picture was dropped")
	}
	saved, _ := store.GetChatMessages(types.NewJID("123", types.DefaultUserServer).String(), 10)
	if len(saved) != 0 {
		t.Fatalf("a caption-less picture left %d row(s) in the text store", len(saved))
	}
}

func TestHandleMessage_PictureWithCaptionKeepsBoth(t *testing.T) {
	c := &Client{msgCh: make(chan Message, 1)}
	c.handleMessage(imageEvent("pic-2", "make it a cartoon"))
	select {
	case msg := <-c.msgCh:
		if msg.Text != "make it a cartoon" || msg.Image == nil {
			t.Fatalf("got %+v", msg)
		}
	default:
		t.Fatal("captioned picture dropped")
	}
}

func TestDownloadImage_RefusesWhatHasNoImageOrIsTooBig(t *testing.T) {
	c := &Client{}
	if _, _, err := c.DownloadImage(context.Background(), Message{}); err == nil {
		t.Fatal("a message without an image must be an error")
	}
	big := Message{Image: &ImageRef{Size: MaxImageBytes + 1, msg: &waE2E.ImageMessage{}}}
	if _, _, err := c.DownloadImage(context.Background(), big); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("oversized image: %v", err)
	}
}
