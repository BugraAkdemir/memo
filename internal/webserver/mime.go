package webserver

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// detectIsImageFile reports whether the file at path should be routed into
// the vision pipeline. It sniffs the actual file content first
// (http.DetectContentType) rather than trusting a client-supplied
// Content-Type header — which lets any file be labelled "image/png"
// regardless of what it actually contains.
//
// The extension of originalFilename is used as a fallback only when
// sniffing is inconclusive (file unreadable, or content.DetectContentType
// falls back to its generic "application/octet-stream" default) — never
// when sniffing positively identified a *different*, non-image type. That
// distinction matters: a text file renamed "evil.png" sniffs as
// "text/html"/"text/plain", a confident non-image result, so the extension
// must not override it — only a genuinely ambiguous file (e.g. a raw image
// format DetectContentType has no signature for) should fall through to the
// filename.
func detectIsImageFile(path, originalFilename string) bool {
	var head []byte
	if f, err := os.Open(path); err == nil {
		buf := make([]byte, 512)
		if n, _ := f.Read(buf); n > 0 {
			head = buf[:n]
		}
		f.Close()
	}
	return detectIsImageBytes(head, originalFilename)
}

// detectIsImageBytes is detectIsImageFile for content already in memory (only
// the first 512 bytes are looked at) — the upload handlers read a picture
// straight into memory so it never sits in the OS temp directory in the clear.
func detectIsImageBytes(content []byte, originalFilename string) bool {
	sniffed := ""
	if len(content) > 512 {
		content = content[:512]
	}
	if len(content) > 0 {
		sniffed = http.DetectContentType(content)
	}

	if strings.HasPrefix(sniffed, "image") {
		return true
	}
	if sniffed != "" && sniffed != "application/octet-stream" {
		return false
	}

	if originalFilename == "" {
		return false
	}
	switch strings.ToLower(filepath.Ext(originalFilename)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		return true
	default:
		return false
	}
}
