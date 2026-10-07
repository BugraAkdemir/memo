package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestImageOf_PicksTheLargestPhotoAndImageDocuments(t *testing.T) {
	m := &tgMessage{Photo: []tgPhotoSize{
		{FileID: "small", Width: 90, Height: 90},
		{FileID: "big", Width: 1280, Height: 960},
		{FileID: "mid", Width: 320, Height: 240},
	}}
	if got := imageOf(m); got == nil || got.FileID != "big" || got.MimeType != "image/jpeg" {
		t.Fatalf("photo: got %+v, want the largest size", got)
	}
	if got := imageOf(&tgMessage{Document: &tgDocument{FileID: "d1", MimeType: "image/PNG"}}); got == nil || got.FileID != "d1" || got.MimeType != "image/png" {
		t.Fatalf("image document: got %+v", got)
	}
	if got := imageOf(&tgMessage{Document: &tgDocument{FileID: "d2", MimeType: "application/pdf"}}); got != nil {
		t.Fatalf("a pdf is not an image: %+v", got)
	}
	if got := imageOf(&tgMessage{Text: "hello"}); got != nil {
		t.Fatalf("plain text has no image: %+v", got)
	}
}

func TestPoll_DeliversAPhotoWithItsCaptionAndAPhotoWithoutOne(t *testing.T) {
	var served int32
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch methodFromPath(r.URL.Path) {
		case "getMe":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": BotInfo{ID: 1, Username: "b"}})
		case "getUpdates":
			if atomic.AddInt32(&served, 1) > 1 {
				time.Sleep(50 * time.Millisecond)
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []any{}})
				return
			}
			from := map[string]any{"id": 7, "first_name": "Ada"}
			chat := map[string]any{"id": 7}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []any{
				map[string]any{"update_id": 1, "message": map[string]any{
					"message_id": 10, "from": from, "chat": chat, "date": 1,
					"caption": "make it anime",
					"photo":   []any{map[string]any{"file_id": "p1", "width": 800, "height": 600}},
				}},
				map[string]any{"update_id": 2, "message": map[string]any{
					"message_id": 11, "from": from, "chat": chat, "date": 2,
					"photo": []any{map[string]any{"file_id": "p2", "width": 800, "height": 600}},
				}},
				map[string]any{"update_id": 3, "message": map[string]any{
					"message_id": 12, "from": from, "chat": chat, "date": 3, // a sticker-ish update: no text, no image
				}},
			}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		}
	})
	c := NewClient("tok")
	if err := c.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	recv := func() Message {
		select {
		case m := <-c.MessageChannel():
			return m
		case <-time.After(3 * time.Second):
			t.Fatal("no message delivered")
			return Message{}
		}
	}
	a := recv()
	if a.Text != "make it anime" || a.Image == nil || a.Image.FileID != "p1" {
		t.Fatalf("captioned photo: %+v", a)
	}
	b := recv()
	if b.Text != "" || b.Image == nil || b.Image.FileID != "p2" {
		t.Fatalf("caption-less photo: %+v", b)
	}
	select {
	case m := <-c.MessageChannel():
		t.Fatalf("an update with neither text nor image was delivered: %+v", m)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestDownloadFile_RoundTripAndLimits(t *testing.T) {
	payload := []byte("\x89PNG-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/getFile"):
			var body struct {
				FileID string `json:"file_id"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.FileID == "huge" {
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"file_path": "photos/h.jpg", "file_size": MaxDownloadBytes + 1}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"file_path": "photos/a.jpg", "file_size": len(payload)}})
		case strings.HasSuffix(r.URL.Path, "/photos/a.jpg"):
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	prevAPI, prevFile := apiBase, fileBase
	apiBase, fileBase = srv.URL+"/bot", srv.URL+"/file/bot"
	defer func() { apiBase, fileBase = prevAPI, prevFile }()

	c := NewClient("tok")
	got, err := c.DownloadFile(context.Background(), "abc")
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("download: %q %v", got, err)
	}
	if _, err := c.DownloadFile(context.Background(), "huge"); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("an oversized file must be refused before downloading: %v", err)
	}
}

func TestSendPhoto_UploadsMultipartWithCaption(t *testing.T) {
	var gotMethod, gotCaption, gotName string
	var gotChat string
	var gotBytes []byte
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = methodFromPath(r.URL.Path)
		mt, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || !strings.HasPrefix(mt, "multipart/") {
			t.Errorf("content type %q", r.Header.Get("Content-Type"))
			return
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err != nil {
				break
			}
			b, _ := io.ReadAll(p)
			switch p.FormName() {
			case "chat_id":
				gotChat = string(b)
			case "caption":
				gotCaption = string(b)
			case "photo":
				gotName, gotBytes = p.FileName(), b
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	c := NewClient("tok")
	if err := c.SendPhoto(context.Background(), 99, []byte("IMG"), "memo.png", "ready"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "sendPhoto" || gotChat != "99" || gotCaption != "ready" || gotName != "memo.png" || string(gotBytes) != "IMG" {
		t.Fatalf("method=%q chat=%q caption=%q name=%q bytes=%q", gotMethod, gotChat, gotCaption, gotName, gotBytes)
	}
}

func TestSendPhoto_APIRefusalIsAnError(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "Bad Request: PHOTO_INVALID_DIMENSIONS"})
	})
	err := NewClient("tok").SendPhoto(context.Background(), 1, []byte("x"), "a.png", "")
	if err == nil || !strings.Contains(err.Error(), "PHOTO_INVALID_DIMENSIONS") {
		t.Fatalf("got %v, want the API's own reason", err)
	}
}
