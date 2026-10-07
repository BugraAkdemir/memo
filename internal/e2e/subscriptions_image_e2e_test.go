// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memo/internal/config"
)

func generatedImage(events []SSEEvent) (string, bool) {
	for _, ev := range events {
		if ev.FinishReason == "generated_image" {
			return ev.Content, true
		}
	}
	return "", false
}

// imageLogLines is what the fake sidecar recorded: one "model path source-images"
// line per real image request.
func imageLogLines(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(config.DataPath("cliproxy"), "image_requests.log"))
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

// A person chatting on a subscription asks for a picture: the picture is drawn
// by THAT account's image model (the vendor of the model in use first), never by
// the default image model set for API providers, and the very next message is
// answered by the chat model again.
func TestSubscriptions_APictureRequestGoesToTheAccountsOwnImageModel(t *testing.T) {
	t.Setenv("FAKECPA_IMAGE_MODELS", "1")
	h := NewHarness(t)
	h.SetAgentEnabled(false)
	h.SetWebSearchEnabled(false)
	bundleSidecar(t)

	for _, vendor := range []string{"antigravity", "codex"} {
		h.postJSON("/api/subscriptions", map[string]string{"action": "login", "provider": vendor}).Body.Close()
		v := vendor
		waitFor(t, v+" sign-in", 60*time.Second, func() bool {
			st := h.subscriptions(t)
			for _, a := range st.Accounts {
				if a.Provider == v {
					for _, m := range st.Models {
						if m.ID == v+"-image" {
							return true
						}
					}
				}
			}
			return false
		})
	}

	// A default image model exists for API providers; a subscription must not use it.
	h.Fake.SetImageModels("fake-image")
	if err := h.App.SetImageConfig(config.ImageConfig{AutoRoute: true, DefaultProvider: "e2e-fake", DefaultModel: "fake-image"}); err != nil {
		t.Fatal(err)
	}

	chatID := h.NewChat()
	h.postJSON("/api/chats/switch", map[string]string{"id": chatID}).Body.Close()

	// Chatting with a Codex model.
	if err := h.App.SetProviderModel("Subscriptions", "codex-model-a"); err != nil {
		t.Fatal(err)
	}
	h.App.SetActiveProvider("Subscriptions")
	events := h.SendMessageStream(chatID, "bana bir kedi resmi çiz")
	if path, ok := generatedImage(events); !ok || path == "" {
		t.Fatalf("no picture came back: %+v", events)
	}
	lines := imageLogLines(t)
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "codex-image /v1/images/generations 0") {
		t.Fatalf("sidecar image requests = %q, want one text-to-image call to codex-image", lines)
	}
	if got := len(h.Fake.ImageRequests()); got != 0 {
		t.Fatalf("the default image model was used %d time(s) on a subscription", got)
	}

	// The next message is plain chat again, still on the same Codex model.
	if got := FinalContent(h.SendMessageStream(chatID, "teşekkürler")); got != "pong from codex-model-a" {
		t.Fatalf("follow-up reply = %q, want the chat model's answer", got)
	}
	if h.App.GetActiveProvider() != "Subscriptions" {
		t.Fatalf("active provider changed to %q", h.App.GetActiveProvider())
	}

	// Switch to an Antigravity model: its own image model draws.
	if err := h.App.SetProviderModel("Subscriptions", "antigravity-model-a"); err != nil {
		t.Fatal(err)
	}
	if _, ok := generatedImage(h.SendMessageStream(chatID, "draw me a dragon")); !ok {
		t.Fatal("no picture for the Antigravity chat")
	}
	lines = imageLogLines(t)
	if len(lines) != 2 || !strings.HasPrefix(lines[1], "antigravity-image /v1/images/generations 0") {
		t.Fatalf("sidecar image requests = %q, want the second to go to antigravity-image", lines)
	}

	// A photo with an edit request: image-to-image on the same account's model.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("message", "make it black and white")
	fw, _ := mw.CreateFormFile("file", "photo.png")
	fw.Write(onePixelPNG)
	mw.Close()
	resp, err := http.Post(h.BaseURL+"/api/send_file/stream", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()
	lines = imageLogLines(t)
	if len(lines) != 3 || !strings.HasPrefix(lines[2], "antigravity-image /v1/images/edits 1") {
		t.Fatalf("sidecar image requests = %q, want an image-to-image call carrying the photo", lines)
	}
}

// No image model on the subscription and none set as the default: a picture
// request is not hijacked — the chat model answers it as it always did — but the
// explicit /image form (here: the same wording through a forced turn) is told why
// nothing is drawn. (The explicit form is covered by the Telegram test.)
func TestSubscriptions_WithoutAnyImageModelThePromptStaysWithTheChatModel(t *testing.T) {
	h := NewHarness(t)
	h.SetWebSearchEnabled(false)
	chatID := h.NewChat()
	events := h.SendMessageStream(chatID, "bana bir kedi resmi çiz")
	if _, ok := generatedImage(events); ok {
		t.Fatal("a picture came out of nowhere")
	}
	if got := FinalContent(events); got != "ok" {
		t.Fatalf("reply = %q, want the chat model's own answer", got)
	}
}
