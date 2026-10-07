// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/provider"
)

const ownerChat = int64(777)

// telegramHarness boots the app with a fake Bot API behind a started bridge and
// links the owner by sending a first message ("hi", answered by the scripted chat
// model), so the tests below start from a bridge that is ready to take commands.
func telegramHarness(t *testing.T) (*Harness, *FakeTelegram) {
	h, ft, _ := telegramHarnessWithReply(t)
	return h, ft
}

// telegramHarnessWithReply also returns a setter for what the scripted chat model
// answers. The script reads it through an atomic, because the provider's handler
// runs on another goroutine than the test.
func telegramHarnessWithReply(t *testing.T) (*Harness, *FakeTelegram, func(string)) {
	t.Helper()
	h := NewHarness(t)
	h.SetWebSearchEnabled(false)
	var reply atomic.Pointer[string]
	ok := "ok"
	reply.Store(&ok)
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		return FakeChatResponse{Text: *reply.Load()}
	}
	ft := NewFakeTelegram(t)
	if err := h.App.StartTelegram(context.Background(), "123:e2e"); err != nil {
		t.Fatalf("StartTelegram: %v", err)
	}
	t.Cleanup(h.App.StopTelegram)
	ft.QueueText(ownerChat, "hi")
	ft.WaitTexts(t, 1, 15*time.Second)
	return h, ft, func(s string) { reply.Store(&s) }
}

func fakePNGBytes() []byte {
	b, _ := base64.StdEncoding.DecodeString(fakePNG)
	return b
}

// lastText waits for the bot's n-th text message and returns it.
func lastText(t *testing.T, ft *FakeTelegram, n int) string {
	t.Helper()
	return ft.WaitTexts(t, n, 15*time.Second)[n-1].Text
}

func TestTelegram_ModelCommandListsAndSwitches(t *testing.T) {
	h, ft := telegramHarness(t)
	if err := h.App.UpdateProvider(provider.ProviderConfig{
		Type: provider.ProviderCustom, Name: "second", BaseURL: h.Fake.Srv.URL, Model: "second-model", APIKey: "k", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	h.App.SetActiveProvider("e2e-fake")

	ft.QueueText(ownerChat, "/model")
	list := lastText(t, ft, 2)
	for _, want := range []string{"fake-model", "second-model", "✅"} {
		if !strings.Contains(list, want) {
			t.Errorf("/model list lacks %q:\n%s", want, list)
		}
	}

	// By number.
	ft.QueueText(ownerChat, "/model 2")
	if got := lastText(t, ft, 3); !strings.Contains(got, "second-model") {
		t.Errorf("/model 2 reply = %q", got)
	}
	if h.App.GetActiveProvider() != "second" {
		t.Fatalf("active provider = %q after /model 2, want second", h.App.GetActiveProvider())
	}

	// By name (a fragment).
	ft.QueueText(ownerChat, "/model fake")
	if got := lastText(t, ft, 4); !strings.Contains(got, "fake-model") {
		t.Errorf("/model fake reply = %q", got)
	}
	if h.App.GetActiveProvider() != "e2e-fake" {
		t.Fatalf("active provider = %q after /model fake, want e2e-fake", h.App.GetActiveProvider())
	}

	// A bad number and an unknown name are said out loud, not guessed at.
	ft.QueueText(ownerChat, "/model 99")
	if got := lastText(t, ft, 5); !strings.Contains(got, "Invalid number") {
		t.Errorf("/model 99 reply = %q", got)
	}
	ft.QueueText(ownerChat, "/model nonexistent-xyz")
	if got := lastText(t, ft, 6); !strings.Contains(got, "No model matches") {
		t.Errorf("/model unknown reply = %q", got)
	}
}

func TestTelegram_AskingForAPictureDrawsItAndTheNextMessageIsTextAgain(t *testing.T) {
	h, ft := telegramHarness(t)
	h.Fake.SetImageModels("fake-image")
	if err := h.App.SetImageConfig(config.ImageConfig{AutoRoute: true, DefaultProvider: "e2e-fake", DefaultModel: "fake-image"}); err != nil {
		t.Fatal(err)
	}
	chatCallsBefore := len(h.Fake.Requests())

	ft.QueueText(ownerChat, "bana bir kedi resmi çiz")
	ups := ft.WaitUploads(t, 1, 20*time.Second)
	if ups[0].Method != "sendPhoto" || !bytes.Equal(ups[0].Bytes, fakePNGBytes()) {
		t.Fatalf("upload = %s, %d bytes; want the drawn PNG as a photo", ups[0].Method, len(ups[0].Bytes))
	}
	reqs := h.Fake.ImageRequests()
	if len(reqs) != 1 || reqs[0].Model != "fake-image" || reqs[0].Prompt != "bana bir kedi resmi çiz" || reqs[0].Images != 0 {
		t.Fatalf("image requests = %+v, want one text-to-image call", reqs)
	}
	if got := len(h.Fake.Requests()); got != chatCallsBefore {
		t.Fatalf("the chat model was called %d time(s) for a picture request; the turn belongs to the image model", got-chatCallsBefore)
	}
	// The active model was never touched.
	if h.App.GetActiveProvider() != "e2e-fake" {
		t.Fatalf("active provider changed to %q", h.App.GetActiveProvider())
	}

	// And the very next message is an ordinary chat turn again.
	ft.QueueText(ownerChat, "teşekkürler")
	if got := lastText(t, ft, 2); got != "ok" {
		t.Fatalf("follow-up reply = %q, want the chat model's answer", got)
	}
	if got := len(h.Fake.Requests()); got != chatCallsBefore+1 {
		t.Errorf("chat model calls = %d, want exactly one for the follow-up", got-chatCallsBefore)
	}
}

func TestTelegram_APhotoWithAnEditRequestIsEditedAndSentBack(t *testing.T) {
	h, ft := telegramHarness(t)
	h.Fake.SetImageModels("fake-image")
	_ = h.App.SetImageConfig(config.ImageConfig{AutoRoute: true, DefaultProvider: "e2e-fake", DefaultModel: "fake-image"})

	ft.QueuePhoto(ownerChat, "make it black and white", fakePNGBytes())
	ft.WaitUploads(t, 1, 20*time.Second)
	reqs := h.Fake.ImageRequests()
	if len(reqs) != 1 || reqs[0].Path != "/images/edits" || reqs[0].Images != 1 {
		t.Fatalf("image requests = %+v, want one image-to-image call carrying the photo", reqs)
	}
}

func TestTelegram_APlainPhotoIsDescribedByTheChatModel(t *testing.T) {
	h, ft, setReply := telegramHarnessWithReply(t)
	h.Fake.SetImageModels("fake-image")
	_ = h.App.SetImageConfig(config.ImageConfig{AutoRoute: true, DefaultProvider: "e2e-fake", DefaultModel: "fake-image"})
	setReply("a tiny pixel")
	before := len(h.Fake.Requests())

	ft.QueuePhoto(ownerChat, "", fakePNGBytes())
	if got := lastText(t, ft, 2); got != "a tiny pixel" {
		t.Fatalf("reply = %q, want the chat model's description", got)
	}
	reqs := h.Fake.Requests()
	if len(reqs) != before+1 {
		t.Fatalf("chat calls = %d, want 1", len(reqs)-before)
	}
	raw := string(reqs[len(reqs)-1].Raw)
	if !strings.Contains(raw, "image_url") || !strings.Contains(raw, "data:image/png;base64,") {
		t.Errorf("the photo did not reach the model as an image part:\n%.400s", raw)
	}
	if len(h.Fake.ImageRequests()) != 0 {
		t.Error("a plain photo must not be sent to the image model")
	}
}

func TestTelegram_ImageCommandForcesAPictureAndExplainsWhenThereIsNoModel(t *testing.T) {
	h, ft := telegramHarness(t)

	ft.QueueText(ownerChat, "/image")
	if got := lastText(t, ft, 2); !strings.Contains(got, "Usage: /image") {
		t.Errorf("bare /image = %q, want the usage", got)
	}

	// No default image model: say so, do not hand a drawing request to the chat model.
	before := len(h.Fake.Requests())
	ft.QueueText(ownerChat, "/image a red balloon")
	if got := lastText(t, ft, 3); !strings.Contains(got, "no model to draw with") && !strings.Contains(got, "There is no model") {
		t.Errorf("reply = %q, want an explanation that no image model is set", got)
	}
	if len(h.Fake.Requests()) != before {
		t.Error("the chat model answered a /image request")
	}

	// With one set, the same wording that no heuristic would catch is drawn.
	h.Fake.SetImageModels("fake-image")
	_ = h.App.SetImageConfig(config.ImageConfig{AutoRoute: false, DefaultProvider: "e2e-fake", DefaultModel: "fake-image"})
	ft.QueueText(ownerChat, "/image a red balloon")
	ft.WaitUploads(t, 1, 20*time.Second)
	if reqs := h.Fake.ImageRequests(); len(reqs) != 1 || reqs[0].Prompt != "a red balloon" {
		t.Fatalf("image requests = %+v, want the prompt without the command", reqs)
	}
}

func TestTelegram_APictureFailureIsExplainedNotDumped(t *testing.T) {
	h, ft := telegramHarness(t)
	h.Fake.SetImageModels("fake-image")
	_ = h.App.SetImageConfig(config.ImageConfig{AutoRoute: true, DefaultProvider: "e2e-fake", DefaultModel: "fake-image"})
	h.Fake.mu.Lock()
	h.Fake.ImageStatus = 500
	h.Fake.ImageErrorBody = `{"error":{"message":"Internal error encountered. request id 9f3a"}}`
	h.Fake.mu.Unlock()

	ft.QueueText(ownerChat, "draw me a cat")
	got := lastText(t, ft, 2)
	if !strings.Contains(got, "(HTTP 500)") || strings.Contains(got, "request id") || strings.Contains(got, "status 500") {
		t.Fatalf("a 500 reached the chat as %q; want a sentence about what happened, tagged with the status", got)
	}
}

func TestTelegram_PicturesAreCiphertextOnDisk(t *testing.T) {
	h, ft := telegramHarness(t)
	h.Fake.SetImageModels("fake-image")
	_ = h.App.SetImageConfig(config.ImageConfig{AutoRoute: true, DefaultProvider: "e2e-fake", DefaultModel: "fake-image"})

	ft.QueuePhoto(ownerChat, "make it a cartoon", fakePNGBytes())
	ft.WaitUploads(t, 1, 20*time.Second)

	for _, sub := range []string{"images", "generated-images"} {
		files, _ := filepath.Glob(filepath.Join(config.DataPath(sub), "*"))
		if len(files) == 0 {
			t.Fatalf("no picture stored under %s", sub)
		}
		for _, f := range files {
			raw, _ := os.ReadFile(f)
			if !bytes.HasPrefix(raw, []byte("MEMOIMG1")) || bytes.Contains(raw, []byte("PNG")) {
				t.Errorf("%s is not ciphertext: starts %q", f, raw[:min(len(raw), 12)])
			}
		}
	}
}
