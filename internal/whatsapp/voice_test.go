package whatsapp

import (
	"context"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	waEvent "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
)

func TestLiveVoiceIsDeliveredWithoutPersistingAudio(t *testing.T) {
	c := NewClient(Config{})
	c.handleMessage(&waEvent.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: types.NewJID("90555", "s.whatsapp.net"), Sender: types.NewJID("90555", "s.whatsapp.net")}, ID: "voice", Timestamp: time.Now()}, Message: &waE2E.Message{AudioMessage: &waE2E.AudioMessage{Mimetype: proto.String("audio/ogg"), FileLength: proto.Uint64(50)}}})
	select {
	case m := <-c.MessageChannel():
		if m.Audio == nil || m.Audio.Size != 50 || m.Text != "" {
			t.Fatal(m)
		}
	case <-time.After(time.Second):
		t.Fatal("voice dropped")
	}
	if _, err := c.DownloadAudio(context.Background(), Message{Audio: &AudioRef{Size: 21 << 20}}); err == nil {
		t.Fatal("oversized voice accepted")
	}
}
