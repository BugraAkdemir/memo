// SPDX-License-Identifier: AGPL-3.0-or-later

package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// withFakeAPI points apiBase at an httptest.Server for the duration of one
// test and restores it afterward — every other test in this package (and
// any future one) keeps hitting a Client with the real default unless it
// opts in here.
func withFakeAPI(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	prev := apiBase
	apiBase = srv.URL + "/bot"
	t.Cleanup(func() {
		srv.Close()
		apiBase = prev
	})
	return srv
}

// methodFromPath extracts the Telegram Bot API method name from a request
// path shaped .../bot<token>/<method>.
func methodFromPath(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return ""
	}
	return path[i+1:]
}

func TestGetMe_Success(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if methodFromPath(r.URL.Path) != "getMe" {
			t.Errorf("unexpected method %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":     true,
			"result": BotInfo{ID: 42, Username: "memo_bot"},
		})
	})
	c := NewClient("faketoken")
	info, err := c.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if info.ID != 42 || info.Username != "memo_bot" {
		t.Errorf("got %+v, want {42 memo_bot}", info)
	}
}

func TestGetMe_APIErrorSurfacesDescription(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":          false,
			"description": "Unauthorized",
		})
	})
	c := NewClient("badtoken")
	_, err := c.GetMe(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("got err=%v, want it to mention the API's own description", err)
	}
}

func TestSendMessage_RealHTTPRoundTrip(t *testing.T) {
	var gotChatID float64
	var gotText string
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ChatID float64 `json:"chat_id"`
			Text   string  `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotChatID = body.ChatID
		gotText = body.Text
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	c := NewClient("faketoken")
	if err := c.SendMessage(context.Background(), 555, "merhaba"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if gotChatID != 555 || gotText != "merhaba" {
		t.Errorf("server received chat_id=%v text=%q, want 555 %q", gotChatID, gotText, "merhaba")
	}
}

func TestSendMessage_ChunksAcrossMultipleRealCalls(t *testing.T) {
	var calls int32
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	c := NewClient("faketoken")
	longText := strings.Repeat("a", 4500) // > the 4000-rune chunk size
	if err := c.SendMessage(context.Background(), 1, longText); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("got %d HTTP calls, want 2 (one per chunk)", got)
	}
}

func TestSendMessage_APIErrorPropagates(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":          false,
			"description": "chat not found",
		})
	})
	c := NewClient("faketoken")
	err := c.SendMessage(context.Background(), 1, "hi")
	if err == nil || !strings.Contains(err.Error(), "chat not found") {
		t.Fatalf("got err=%v, want it to mention the API's own description", err)
	}
}

func TestSendDocument_RealMultipartRoundTrip(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "report-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.WriteString("hello from memo"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()

	var gotFilename string
	var gotContent string
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if methodFromPath(r.URL.Path) != "sendDocument" {
			t.Errorf("unexpected method %q", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		part, header, err := r.FormFile("document")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer part.Close()
		gotFilename = header.Filename
		buf := make([]byte, 64)
		n, _ := part.Read(buf)
		gotContent = string(buf[:n])
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})

	c := NewClient("faketoken")
	if err := c.SendDocument(context.Background(), 1, f.Name(), "renamed.txt"); err != nil {
		t.Fatalf("SendDocument: %v", err)
	}
	if gotFilename != "renamed.txt" {
		t.Errorf("got filename %q, want %q (the caller-supplied name, not the temp file's own)", gotFilename, "renamed.txt")
	}
	if gotContent != "hello from memo" {
		t.Errorf("got content %q, want the real file bytes", gotContent)
	}
}

func TestSendDocument_MissingFileFails(t *testing.T) {
	c := NewClient("faketoken")
	err := c.SendDocument(context.Background(), 1, "/no/such/file", "x.txt")
	if err == nil {
		t.Fatal("expected an error for a nonexistent file, got nil")
	}
}

func TestSetTyping_Success(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Action string `json:"action"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Action != "typing" {
			t.Errorf("got action %q, want %q", body.Action, "typing")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	c := NewClient("faketoken")
	if err := c.SetTyping(context.Background(), 1); err != nil {
		t.Fatalf("SetTyping: %v", err)
	}
}

// TestStartStop_DeliversAMessageThenStopsCleanly exercises the full
// lifecycle: Start validates the token via getMe, the poll loop's getUpdates
// call delivers one message onto MessageChannel(), and Stop() halts the
// loop — IsRunning() must go back to false without the test needing a
// fixed sleep to "probably" be done.
func TestStartStop_DeliversAMessageThenStopsCleanly(t *testing.T) {
	var updatesCalls int32
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch methodFromPath(r.URL.Path) {
		case "getMe":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true, "result": BotInfo{ID: 1, Username: "b"},
			})
		case "getUpdates":
			n := atomic.AddInt32(&updatesCalls, 1)
			if n == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok": true,
					"result": []tgUpdate{{
						UpdateID: 1,
						Message: &tgMessage{
							MessageID: 1,
							From:      &tgUser{ID: 9, FirstName: "Bugra"},
							Chat:      tgChat{ID: 9},
							Date:      time.Now().Unix(),
							Text:      "merhaba",
						},
					}},
				})
				return
			}
			// Every subsequent poll: no new updates, just idle until Stop().
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []tgUpdate{}})
		default:
			t.Errorf("unexpected method %q", r.URL.Path)
		}
	})

	c := NewClient("faketoken")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !c.IsRunning() {
		t.Fatal("IsRunning() = false right after Start")
	}

	select {
	case msg := <-c.MessageChannel():
		if msg.Text != "merhaba" || msg.FromName != "Bugra" || msg.ChatID != 9 {
			t.Errorf("got %+v, want text=merhaba from=Bugra chat=9", msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the poll loop to deliver a message")
	}

	c.Stop()
	deadline := time.Now().Add(5 * time.Second)
	for c.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if c.IsRunning() {
		t.Fatal("IsRunning() still true well after Stop()")
	}
}

// TestPollLoop_ErrorSetsLastErrorAndReconnecting guards the client's own
// health-reporting surface (LastError/IsReconnecting/ErrorChannel), which
// internal/app's Settings status and reconnect-notice logic depend on —
// none of it was under test before this file.
func TestPollLoop_ErrorSetsLastErrorAndReconnecting(t *testing.T) {
	withFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch methodFromPath(r.URL.Path) {
		case "getMe":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true, "result": BotInfo{ID: 1, Username: "b"},
			})
		case "getUpdates":
			// Malformed JSON body -> call()'s json.Unmarshal fails -> the poll
			// loop's error path fires (LastError/IsReconnecting/errCh).
			w.Write([]byte("{not json"))
		default:
			t.Errorf("unexpected method %q", r.URL.Path)
		}
	})

	c := NewClient("faketoken")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	select {
	case err := <-c.ErrorChannel():
		if err == nil {
			t.Fatal("received a nil error on ErrorChannel")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the poll loop to report an error")
	}

	deadline := time.Now().Add(5 * time.Second)
	for c.LastError() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if c.LastError() == "" {
		t.Error("LastError() is still empty after an error was reported")
	}
	if !c.IsReconnecting() {
		t.Error("IsReconnecting() = false, want true while backing off after an error")
	}
}
