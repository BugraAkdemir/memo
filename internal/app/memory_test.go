package app

import (
	"testing"
	"time"

	"memo/internal/config"
)

func TestIsLLMErrorReply(t *testing.T) {
	cases := []struct {
		reply string
		want  bool
	}{
		{"⚠️ Yerel model yüklenmemiş. Lütfen bir model başlatın veya API sağlayıcı seçin.", true},
		{"⚠️ Empty response", true},
		{"⚠️ Cannot read image: open x: no such file", true},
		{"Merhaba, nasıl yardımcı olabilirim?", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isLLMErrorReply(c.reply); got != c.want {
			t.Errorf("isLLMErrorReply(%q) = %v, want %v", c.reply, got, c.want)
		}
	}
}

// TestSaveMemoryAsyncSkipsErrorReplies is a regression test for BUG-H7:
// callLLM's synthetic "⚠️ ..." error strings (model not loaded, provider
// error, unreadable attachment, etc.) used to be saved into RAG memory like
// any genuine reply, polluting future retrieval with error noise. They must
// never reach memorySaveCh.
func TestSaveMemoryAsyncSkipsErrorReplies(t *testing.T) {
	a := &App{
		cfg:          &config.AppConfig{Memory: config.MemoryConfig{MemoryEnabled: true}},
		memorySaveCh: make(chan saveTask, 1),
	}

	a.saveMemoryAsync("kullanıcı mesajı", "⚠️ Yerel model yüklenmemiş. Lütfen bir model başlatın veya API sağlayıcı seçin.")

	select {
	case task := <-a.memorySaveCh:
		t.Fatalf("error reply must not be queued for memory save, got %+v", task)
	case <-time.After(50 * time.Millisecond):
		// expected: nothing queued
	}

	a.saveMemoryAsync("kullanıcı mesajı", "gerçek bir yanıt")

	select {
	case task := <-a.memorySaveCh:
		if task.reply != "gerçek bir yanıt" {
			t.Errorf("reply = %q, want %q", task.reply, "gerçek bir yanıt")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("genuine reply should have been queued for memory save")
	}
}
