package whatsapp

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestSanitizeJID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"123@s.whatsapp.net", "123_s_whatsapp_net"},
		{"alice@example.com", "alice_example_com"},
		{"simple", "simple"},
		{"test-123_abc", "test_123_abc"},
		{"", ""},
		{"@#$%", "____"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeJID(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeJID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractText(t *testing.T) {
	tests := []struct {
		name string
		msg  *waE2E.Message
		want string
	}{
		{"nil message", nil, ""},
		{"conversation", &waE2E.Message{Conversation: proto.String("hello")}, "hello"},
		{"extended text", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("extended")}}, "extended"},
		{"image caption", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{Caption: proto.String("image caption")}}, "image caption"},
		{"video caption", &waE2E.Message{VideoMessage: &waE2E.VideoMessage{Caption: proto.String("video caption")}}, "video caption"},
		{"document caption", &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{Caption: proto.String("doc caption")}}, "doc caption"},
		{"prefers conversation over extended", &waE2E.Message{
			Conversation:       proto.String("conv"),
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("ext")},
		}, "conv"},
		{"empty message", &waE2E.Message{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractText(tt.msg)
			if got != tt.want {
				t.Errorf("extractText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBoolToInt(t *testing.T) {
	tests := []struct {
		input bool
		want  int
	}{
		{true, 1},
		{false, 0},
	}
	for _, tt := range tests {
		got := boolToInt(tt.input)
		if got != tt.want {
			t.Errorf("boolToInt(%v) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestPartsBeforeAt(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"123@s.whatsapp.net", "123"},
		{"alice@example.com", "alice"},
		{"noat", "noat"},
		{"", ""},
		{"a@b@c", "a"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := partsBeforeAt(tt.input)
			if got != tt.want {
				t.Errorf("partsBeforeAt(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
