package app

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"memo/internal/imgvault"
	"memo/internal/provider"
)

// A tiny but real PNG signature plus filler, so mime sniffing says image/png.
func fakePNG(marker string) []byte {
	return append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte(marker), 64)...)
}

func TestImageStore_SentPictureIsCiphertextOnDiskAndServedIntact(t *testing.T) {
	png := fakePNG("PIXELS-SENT-")
	stored := persistChatImage("upload.png", png)
	if filepath.Dir(stored) != filepath.Clean(imageStoreDirs()[0]) {
		t.Fatalf("stored at %s, want it under %s", stored, imageStoreDirs()[0])
	}
	raw, err := os.ReadFile(stored)
	if err != nil {
		t.Fatal(err)
	}
	if !imgvault.IsSealed(raw) || bytes.Contains(raw, []byte("PIXELS-SENT-")) || bytes.Contains(raw, []byte("PNG")) {
		t.Fatal("the picture sits on disk in the clear")
	}
	want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	if got := (&App{}).GetImageBase64(stored); got != want {
		t.Fatalf("served picture differs from what was sent (len %d vs %d)", len(got), len(want))
	}
}

func TestImageStore_GeneratedPictureIsCiphertextOnDisk(t *testing.T) {
	png := fakePNG("PIXELS-DRAWN-")
	path, err := saveGeneratedImage(provider.GeneratedImage{B64JSON: base64.StdEncoding.EncodeToString(png), MediaType: "image/png"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !imgvault.IsSealed(raw) || bytes.Contains(raw, []byte("PIXELS-DRAWN-")) {
		t.Fatal("the generated picture sits on disk in the clear")
	}
	got, err := readImageFile(path)
	if err != nil || !bytes.Equal(got, png) {
		t.Fatalf("read back: err=%v equal=%v", err, bytes.Equal(got, png))
	}
}

func TestImageStore_OlderPlaintextPicturesAreSealedAtStartupAndStayReadable(t *testing.T) {
	dir := imageStoreDirs()[0]
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "memo-legacy.png")
	png := fakePNG("PIXELS-LEGACY-")
	if err := os.WriteFile(old, png, 0o644); err != nil {
		t.Fatal(err)
	}
	// Readable before the migration (passes through) ...
	if got, err := readImageFile(old); err != nil || !bytes.Equal(got, png) {
		t.Fatalf("legacy read before sealing: %v", err)
	}
	(&App{}).sealStoredImages()
	raw, _ := os.ReadFile(old)
	if !imgvault.IsSealed(raw) || bytes.Contains(raw, []byte("PIXELS-LEGACY-")) {
		t.Fatal("the older picture was not sealed")
	}
	// ... and after.
	if got, err := readImageFile(old); err != nil || !bytes.Equal(got, png) {
		t.Fatalf("legacy read after sealing: %v", err)
	}
}
