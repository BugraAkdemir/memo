// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"memo/internal/telegram"
)

// FakeTelegram is a Telegram Bot API server just big enough to drive the real
// bridge: getMe, getUpdates (long poll over a queue), sendMessage, sendPhoto,
// sendDocument, sendChatAction, getFile and the file download. Creating one
// points every telegram.Client created afterwards at it (telegram.EnvAPIBase),
// so the app's own StartTelegram talks to it without any test-only seam in the
// app itself.
type FakeTelegram struct {
	Srv *httptest.Server

	mu      sync.Mutex
	updates []map[string]any
	nextID  int
	texts   []TGText
	uploads []TGUpload
	files   map[string][]byte
}

// TGText is a text message the bot sent.
type TGText struct {
	ChatID int64
	Text   string
}

// TGUpload is a photo or document the bot sent.
type TGUpload struct {
	Method  string // "sendPhoto" or "sendDocument"
	ChatID  int64
	Name    string
	Caption string
	Bytes   []byte
}

// NewFakeTelegram starts the server and aims the telegram client at it for the
// rest of the test.
func NewFakeTelegram(t *testing.T) *FakeTelegram {
	t.Helper()
	f := &FakeTelegram{nextID: 1, files: map[string][]byte{}}
	f.Srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.Srv.Close)
	t.Setenv(telegram.EnvAPIBase, f.Srv.URL)
	return f
}

// QueueText delivers a text message from chatID to the bot.
func (f *FakeTelegram) QueueText(chatID int64, text string) {
	f.queue(chatID, map[string]any{"text": text})
}

// QueuePhoto delivers a photo (with an optional caption) from chatID.
func (f *FakeTelegram) QueuePhoto(chatID int64, caption string, data []byte) {
	f.mu.Lock()
	id := fmt.Sprintf("file-%d", len(f.files)+1)
	f.files[id] = data
	f.mu.Unlock()
	m := map[string]any{"photo": []any{
		map[string]any{"file_id": id + "-thumb", "width": 90, "height": 90},
		map[string]any{"file_id": id, "width": 800, "height": 600},
	}}
	if caption != "" {
		m["caption"] = caption
	}
	f.queue(chatID, m)
}

func (f *FakeTelegram) queue(chatID int64, fields map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	msg := map[string]any{
		"message_id": f.nextID,
		"from":       map[string]any{"id": chatID, "first_name": "Owner"},
		"chat":       map[string]any{"id": chatID},
		"date":       time.Now().Unix(),
	}
	for k, v := range fields {
		msg[k] = v
	}
	f.updates = append(f.updates, map[string]any{"update_id": f.nextID, "message": msg})
	f.nextID++
}

// WaitTexts blocks until the bot has sent at least n text messages and returns
// all of them.
func (f *FakeTelegram) WaitTexts(t *testing.T, n int, timeout time.Duration) []TGText {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		f.mu.Lock()
		got := append([]TGText(nil), f.texts...)
		f.mu.Unlock()
		if len(got) >= n {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("bot sent %d text message(s) in %v, want %d: %+v", len(got), timeout, n, got)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// WaitUploads blocks until the bot has sent at least n photos/documents.
func (f *FakeTelegram) WaitUploads(t *testing.T, n int, timeout time.Duration) []TGUpload {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		f.mu.Lock()
		got := append([]TGUpload(nil), f.uploads...)
		f.mu.Unlock()
		if len(got) >= n {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("bot sent %d upload(s) in %v, want %d", len(got), timeout, n)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Texts returns the text messages sent so far.
func (f *FakeTelegram) Texts() []TGText {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]TGText(nil), f.texts...)
}

func (f *FakeTelegram) handle(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if i := strings.Index(path, "/file/bot"); i >= 0 {
		// /file/bot<token>/<file_path>
		rest := path[i+len("/file/bot"):]
		_, fp, _ := strings.Cut(rest, "/")
		f.mu.Lock()
		data, ok := f.files[strings.TrimPrefix(fp, "files/")]
		f.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
		return
	}
	method := path[strings.LastIndex(path, "/")+1:]
	w.Header().Set("Content-Type", "application/json")
	switch method {
	case "getMe":
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"username":"memo_e2e_bot"}}`))
	case "getUpdates":
		var p struct {
			Offset int `json:"offset"`
		}
		_ = json.NewDecoder(r.Body).Decode(&p)
		deadline := time.Now().Add(1500 * time.Millisecond)
		for {
			f.mu.Lock()
			var out []map[string]any
			for _, u := range f.updates {
				if u["update_id"].(int) >= p.Offset {
					out = append(out, u)
				}
			}
			f.mu.Unlock()
			if len(out) > 0 || time.Now().After(deadline) || r.Context().Err() != nil {
				if out == nil {
					out = []map[string]any{}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": out})
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	case "sendMessage":
		var p struct {
			ChatID int64  `json:"chat_id"`
			Text   string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&p)
		f.mu.Lock()
		f.texts = append(f.texts, TGText{ChatID: p.ChatID, Text: p.Text})
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	case "sendChatAction":
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	case "getFile":
		var p struct {
			FileID string `json:"file_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&p)
		f.mu.Lock()
		data, ok := f.files[p.FileID]
		f.mu.Unlock()
		if !ok {
			_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: wrong file_id"}`))
			return
		}
		_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"file_path":"files/%s","file_size":%d}}`, p.FileID, len(data))
	case "sendPhoto", "sendDocument":
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: not multipart"}`))
			return
		}
		chat, _ := strconv.ParseInt(r.FormValue("chat_id"), 10, 64)
		field := "photo"
		if method == "sendDocument" {
			field = "document"
		}
		file, hdr, err := r.FormFile(field)
		if err != nil {
			_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: no file"}`))
			return
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		f.mu.Lock()
		f.uploads = append(f.uploads, TGUpload{Method: method, ChatID: chat, Name: hdr.Filename, Caption: r.FormValue("caption"), Bytes: data})
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	default:
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}
}
