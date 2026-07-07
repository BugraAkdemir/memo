package webserver

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDetectIsImageFile is a regression test for BUG-QH2: handleSendFileStream
// used to trust the client-supplied multipart Content-Type header, letting
// any file be labelled "image/png" and routed into the vision pipeline
// regardless of its actual content. detectIsImageFile must decide purely
// from file content (falling back to extension only when sniffing is
// inconclusive), the same way the non-streaming handleSendFile always did.
func TestDetectIsImageFile(t *testing.T) {
	dir := t.TempDir()

	pngMagic := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}
	pngPath := filepath.Join(dir, "photo.png")
	if err := os.WriteFile(pngPath, pngMagic, 0644); err != nil {
		t.Fatal(err)
	}

	textPath := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(textPath, []byte("just some plain text content"), 0644); err != nil {
		t.Fatal(err)
	}

	// The actual bug: a text file whose *name* claims to be a .png. Content
	// sniffing must win — this must NOT be treated as an image.
	spoofedPath := filepath.Join(dir, "spoofed.png")
	if err := os.WriteFile(spoofedPath, []byte("<script>alert(1)</script>"), 0644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		path     string
		filename string
		want     bool
	}{
		{"real PNG content + png name", pngPath, "photo.png", true},
		{"plain text + txt name", textPath, "notes.txt", false},
		{"text content spoofed as .png by filename", spoofedPath, "spoofed.png", false},
		{"missing file falls back to extension", filepath.Join(dir, "does-not-exist.jpg"), "does-not-exist.jpg", true},
		{"missing file, non-image extension", filepath.Join(dir, "does-not-exist.txt"), "does-not-exist.txt", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := detectIsImageFile(c.path, c.filename); got != c.want {
				t.Errorf("detectIsImageFile(%q, %q) = %v, want %v", c.path, c.filename, got, c.want)
			}
		})
	}
}
