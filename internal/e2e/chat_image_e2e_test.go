// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
)

// onePixelPNG is a valid 1x1 PNG.
var onePixelPNG, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")

// TestChat_UploadedImageCanBeShownAfterTheTurn was found live. A web or
// mobile client sends an image through the multipart upload endpoint; the
// handler wrote it to a temp file, deleted that file as soon as the turn was
// set up, and the chat message stored the deleted temp path — and
// /api/image, the only way a client not on the backend's machine can load a
// message image, refused every absolute path anyway. So the image was never
// displayable again, on any client.
func TestChat_UploadedImageCanBeShownAfterTheTurn(t *testing.T) {
	h := NewHarness(t)
	h.SetWebSearchEnabled(false)
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		return FakeChatResponse{Text: "nice picture"}
	}
	chatID := h.NewChat()
	h.postJSON("/api/chats/switch", map[string]string{"id": chatID}).Body.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("message", "what is this?")
	fw, _ := mw.CreateFormFile("file", "photo.png")
	fw.Write(onePixelPNG)
	mw.Close()
	resp, err := http.Post(h.BaseURL+"/api/send_file/stream", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	io.ReadAll(resp.Body) // drain the SSE stream to the end of the turn
	resp.Body.Close()

	var msgs []struct {
		Role      string `json:"role"`
		ImagePath string `json:"image_path"`
	}
	decodeInto(t, h.getJSON("/api/messages?chat_id="+chatID), &msgs)
	var path string
	for _, m := range msgs {
		if m.Role == "user" && m.ImagePath != "" {
			path = m.ImagePath
		}
	}
	if path == "" {
		t.Fatalf("no user message recorded an image path: %+v", msgs)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stored image path %q does not exist after the turn: %v", path, err)
	}

	imgResp := h.getJSON("/api/image?path=" + url.QueryEscape(path))
	if imgResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(imgResp.Body)
		imgResp.Body.Close()
		t.Fatalf("GET /api/image for the stored path: status %d (%s)", imgResp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out struct {
		Data string `json:"data"`
	}
	json.NewDecoder(imgResp.Body).Decode(&out)
	imgResp.Body.Close()
	want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(onePixelPNG)
	if out.Data != want {
		t.Errorf("GET /api/image returned %q…, want the uploaded PNG", out.Data[:min(len(out.Data), 40)])
	}
}

// TestImageEndpoint_RefusesFilesOutsideTheImageDirectories keeps the
// endpoint from becoming a general file reader now that it accepts absolute
// paths: config.yaml, the providers file (API keys) and anything outside the
// data dir must stay unreachable.
func TestImageEndpoint_RefusesFilesOutsideTheImageDirectories(t *testing.T) {
	h := NewHarness(t)
	outside := t.TempDir() + "/secret.png"
	os.WriteFile(outside, onePixelPNG, 0o644)
	for _, p := range []string{outside, "/etc/hostname", "data/providers.json"} {
		r := h.getJSON("/api/image?path=" + url.QueryEscape(p))
		r.Body.Close()
		if r.StatusCode == http.StatusOK {
			t.Errorf("GET /api/image?path=%s = 200, want it refused", p)
		}
	}
}

// A picture a person uploads must never sit in the OS temp directory in the clear:
// the handler used to copy it there and keep it for the whole turn. It is read into
// memory and only the sealed copy is ever written.
func TestChat_UploadedPictureNeverTouchesTheTempDirInTheClear(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	h := NewHarness(t)
	h.SetWebSearchEnabled(false)

	var mu sync.Mutex
	var duringTurn []string
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		entries, _ := os.ReadDir(tmp)
		mu.Lock()
		for _, e := range entries {
			duringTurn = append(duringTurn, e.Name())
		}
		mu.Unlock()
		return FakeChatResponse{Text: "nice picture"}
	}
	chatID := h.NewChat()
	h.postJSON("/api/chats/switch", map[string]string{"id": chatID}).Body.Close()

	for _, endpoint := range []string{"/api/send_file/stream"} {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		mw.WriteField("message", "what is this?")
		fw, _ := mw.CreateFormFile("file", "photo.png")
		fw.Write(onePixelPNG)
		mw.Close()
		resp, err := http.Post(h.BaseURL+endpoint, mw.FormDataContentType(), &body)
		if err != nil {
			t.Fatal(err)
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
	}

	mu.Lock()
	defer mu.Unlock()
	for _, name := range duringTurn {
		if strings.HasPrefix(name, "memo_web_") {
			t.Fatalf("the uploaded picture was in the temp directory during the turn: %s", name)
		}
	}
	// And the model did get the picture.
	reqs := h.Fake.Requests()
	if len(reqs) == 0 || !strings.Contains(string(reqs[len(reqs)-1].Raw), "image_url") {
		t.Fatal("the picture did not reach the model")
	}
}
