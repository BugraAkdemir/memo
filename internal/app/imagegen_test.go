// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/provider"
	"memo/internal/sessions"
)

// fakeImageGenerator stands in for an OpenRouter image model so these tests
// never touch the network (or the user's credits).
type fakeImageGenerator struct {
	imageOnly  bool
	gotPrompt  string
	gotModel   string
	resp       *provider.ImageResponse
	err        error
	generateHz int
}

func (f *fakeImageGenerator) IsImageOnlyModel(ctx context.Context, model string) bool {
	return f.imageOnly
}

func (f *fakeImageGenerator) GenerateImage(ctx context.Context, req provider.ImageRequest) (*provider.ImageResponse, error) {
	f.generateHz++
	f.gotPrompt = req.Prompt
	f.gotModel = req.Model
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func newTestAppForImageGen(t *testing.T) (*App, *sessions.Manager) {
	t.Helper()
	// saveGeneratedImage writes under config.DataDir() — keep it in the
	// test's own temp dir instead of the developer's real data directory.
	t.Setenv("MEMO_DATA_DIR", t.TempDir())
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)

	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: false},
			Llama:  config.LlamaConfig{CtxSize: 4096},
		},
		identity:       identity.New("Test", "Memo", "casual", "", false),
		sessions:       sm,
		providerCfgMgr: provider.NewConfigManager(t.TempDir()+"/providers.json", nil),
	}
	return a, sm
}

func TestImageExtension(t *testing.T) {
	cases := map[string]string{
		"image/png":     ".png",
		"image/jpeg":    ".jpg",
		"image/JPG":     ".jpg",
		"image/webp":    ".webp",
		"image/gif":     ".gif",
		"image/svg+xml": ".svg",
		"":              ".png",
		"nonsense":      ".png",
	}
	for mediaType, want := range cases {
		if got := imageExtension(mediaType); got != want {
			t.Errorf("imageExtension(%q) = %q, want %q", mediaType, got, want)
		}
	}
}

func TestSaveGeneratedImage_WritesDecodedBytes(t *testing.T) {
	t.Setenv("MEMO_DATA_DIR", t.TempDir())
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)

	want := []byte{0x89, 'P', 'N', 'G', 9, 9, 9}
	path, err := saveGeneratedImage(provider.GeneratedImage{
		B64JSON:   base64.StdEncoding.EncodeToString(want),
		MediaType: "image/png",
	})
	if err != nil {
		t.Fatalf("saveGeneratedImage() error = %v", err)
	}
	if filepath.Ext(path) != ".png" {
		t.Errorf("saved path %q has extension %q, want .png", path, filepath.Ext(path))
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved image: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("saved bytes = %v, want %v", got, want)
	}
}

func TestSaveGeneratedImage_RejectsUndecodableBase64(t *testing.T) {
	t.Setenv("MEMO_DATA_DIR", t.TempDir())
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)

	if _, err := saveGeneratedImage(provider.GeneratedImage{B64JSON: "!!!not base64!!!"}); err == nil {
		t.Fatal("saveGeneratedImage() error = nil for undecodable base64, want an error")
	}
}

// The turn's shape end to end: a generated_image marker carrying the saved
// path, a terminal Done, and the path persisted on the assistant message so
// reloading the chat shows the same bubble.
func TestStreamImageGeneration_EmitsMarkerAndPersistsImagePath(t *testing.T) {
	a, sm := newTestAppForImageGen(t)
	chatID := sm.NewChat()

	gen := &fakeImageGenerator{
		imageOnly: true,
		resp: &provider.ImageResponse{
			Images: []provider.GeneratedImage{{
				B64JSON:   base64.StdEncoding.EncodeToString([]byte("fake-png-bytes")),
				MediaType: "image/png",
			}},
			Usage: &provider.Usage{PromptTokens: 7, CompletionTokens: 4175},
		},
	}

	out := make(chan api.StreamChunk, 8)
	a.streamImageGeneration(context.Background(), gen, "inclusionai/ming-image-0.1-design",
		"uzayda uçan kedi resmi çiz", "uzayda uçan kedi resmi çiz", chatID, out)
	close(out)

	var markerPath string
	var sawDone bool
	for chunk := range out {
		if chunk.Error != "" {
			t.Fatalf("unexpected error chunk: %q", chunk.Error)
		}
		if chunk.FinishReason == imageGenerationMarker {
			markerPath = chunk.Content
		}
		if chunk.Done {
			sawDone = true
		}
	}
	if markerPath == "" {
		t.Fatal("no generated_image marker chunk — the UI would never learn where the image landed")
	}
	if !sawDone {
		t.Error("stream never sent a Done chunk — the caller would hang")
	}
	if _, err := os.Stat(markerPath); err != nil {
		t.Errorf("marker path %q is not a readable file: %v", markerPath, err)
	}

	if gen.gotPrompt != "uzayda uçan kedi resmi çiz" {
		t.Errorf("prompt sent = %q, want the user's message", gen.gotPrompt)
	}
	if gen.gotModel != "inclusionai/ming-image-0.1-design" {
		t.Errorf("model sent = %q, want the configured image model", gen.gotModel)
	}

	msgs := sm.GetActiveMessagesForSession(chatID)
	if len(msgs) != 1 {
		t.Fatalf("persisted %d messages, want 1 (the assistant's image turn)", len(msgs))
	}
	if msgs[0].Role != "assistant" {
		t.Errorf("persisted role = %q, want assistant", msgs[0].Role)
	}
	if msgs[0].ImagePath != markerPath {
		t.Errorf("persisted ImagePath = %q, want the marker's path %q", msgs[0].ImagePath, markerPath)
	}
}

func TestStreamImageGeneration_BlankPromptDoesNotCallTheProvider(t *testing.T) {
	a, sm := newTestAppForImageGen(t)
	chatID := sm.NewChat()

	gen := &fakeImageGenerator{imageOnly: true}
	out := make(chan api.StreamChunk, 8)
	a.streamImageGeneration(context.Background(), gen, "some/image-model", "   ", "   ", chatID, out)
	close(out)

	var sawError bool
	for chunk := range out {
		if chunk.Error != "" {
			sawError = true
		}
	}
	if !sawError {
		t.Error("blank prompt produced no error chunk")
	}
	if gen.generateHz != 0 {
		t.Errorf("GenerateImage called %d times for a blank prompt, want 0 — that's a paid call", gen.generateHz)
	}
}
