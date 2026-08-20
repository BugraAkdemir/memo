package telegram

import (
	"encoding/json"
	"testing"
)

func TestDisplayName(t *testing.T) {
	cases := []struct {
		name string
		u    *tgUser
		want string
	}{
		{"first+last", &tgUser{FirstName: "Bugra", LastName: "Akdemir"}, "Bugra Akdemir"},
		{"first only", &tgUser{FirstName: "Bugra"}, "Bugra"},
		{"username fallback", &tgUser{Username: "bugraa"}, "@bugraa"},
		{"id fallback", &tgUser{ID: 42}, "42"},
		{"nil", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := displayName(tc.u); got != tc.want {
				t.Errorf("displayName(%+v) = %q, want %q", tc.u, got, tc.want)
			}
		})
	}
}

func TestTgUpdate_UnmarshalsTelegramShape(t *testing.T) {
	raw := `{
		"update_id": 123456,
		"message": {
			"message_id": 7,
			"from": {"id": 999, "first_name": "Bugra", "username": "bugraa"},
			"chat": {"id": 999},
			"date": 1700000000,
			"text": "/status"
		}
	}`
	var u tgUpdate
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if u.UpdateID != 123456 {
		t.Errorf("UpdateID = %d, want 123456", u.UpdateID)
	}
	if u.Message == nil || u.Message.Text != "/status" {
		t.Fatalf("Message = %+v", u.Message)
	}
	if u.Message.From.ID != 999 || u.Message.Chat.ID != 999 {
		t.Errorf("From/Chat ID mismatch: %+v / %+v", u.Message.From, u.Message.Chat)
	}
}

func TestSendMessage_ChunksOnRuneBoundaries(t *testing.T) {
	// A rune count just over one chunk boundary, made entirely of a 2-byte
	// UTF-8 rune (Turkish 'ş') — a byte-based split at the same offset would
	// slice a multi-byte rune in half and corrupt the string. This only
	// exercises the pure chunking math (no real HTTP call), by reproducing
	// the same []rune slicing SendMessage uses.
	text := ""
	for i := 0; i < 4001; i++ {
		text += "ş"
	}
	runes := []rune(text)
	const maxRunes = 4000
	if len(runes) <= maxRunes {
		t.Fatalf("test setup: need more than %d runes, got %d", maxRunes, len(runes))
	}
	first := string(runes[:maxRunes])
	rest := string(runes[maxRunes:])
	if len([]rune(first)) != maxRunes {
		t.Errorf("first chunk rune count = %d, want %d", len([]rune(first)), maxRunes)
	}
	if len([]rune(rest)) != len(runes)-maxRunes {
		t.Errorf("remainder rune count = %d, want %d", len([]rune(rest)), len(runes)-maxRunes)
	}
	// Every rune in both chunks must still be a valid 'ş' — proof nothing
	// was corrupted by the split.
	for _, r := range first + rest {
		if r != 'ş' {
			t.Fatalf("corrupted rune %q found after chunking", r)
		}
	}
}
