package telegram

import (
	"encoding/json"
	"testing"
)

func TestVoicePayloadPreservesBoundedMediaReference(t *testing.T) {
	for _, kind := range []string{"voice", "audio"} {
		var msg tgMessage
		if err := json.Unmarshal([]byte(`{"`+kind+`":{"file_id":"voice-ref","mime_type":"audio/ogg","file_size":321}}`), &msg); err != nil {
			t.Fatal(err)
		}
		v := voiceOf(&msg)
		if v == nil || v.FileID != "voice-ref" || v.MimeType != "audio/ogg" || v.Size != 321 {
			t.Fatal(v)
		}
	}
	if voiceOf(&tgMessage{}) != nil {
		t.Fatal("invented voice")
	}
}
