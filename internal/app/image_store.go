package app

import (
	"fmt"
	"os"
	"path/filepath"

	"memo/internal/config"
	"memo/internal/imgvault"
	"memo/internal/logx"
)

// Pictures at rest.
//
// Every image Memo keeps on disk — what a person sent (data/images) and what a
// model drew (data/generated-images) — is written through internal/imgvault, so
// the file is ciphertext and the picture exists only in memory while it is
// served. Reads go through the same door; a file written before sealing existed
// is still readable (it passes through) until sealStoredImages has sealed it.
//
// Writing never falls back to plaintext: if the key cannot be loaded the image is
// not stored (the turn itself still goes ahead), which is the safe failure for a
// feature whose promise is "the files are unreadable".

// writeImageFile stores a picture sealed.
func writeImageFile(path string, data []byte) error {
	v, err := imgvault.Default()
	if err != nil {
		return fmt.Errorf("image key unavailable: %w", err)
	}
	return v.WriteFile(path, data)
}

// readImageFile returns a stored picture's bytes, decrypting when needed.
func readImageFile(path string) ([]byte, error) {
	v, err := imgvault.Default()
	if err != nil {
		// No key: a legacy plaintext picture is still readable, a sealed one is not.
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil, rerr
		}
		if imgvault.IsSealed(raw) {
			return nil, fmt.Errorf("image key unavailable: %w", err)
		}
		return raw, nil
	}
	return v.ReadFile(path)
}

// imageStoreDirs are the directories whose files are pictures to seal.
func imageStoreDirs() []string {
	return []string{
		config.DataPath("images"),
		config.DataPath(generatedImagesSub),
	}
}

// sealStoredImages encrypts, once, every picture stored before sealing existed.
// Cheap and idempotent: a sealed file is skipped by its header. Runs in the
// background at startup; a failure is logged and never blocks the app.
func (a *App) sealStoredImages() {
	v, err := imgvault.Default()
	if err != nil {
		logx.Printf("images: cannot seal stored pictures: %v", err)
		return
	}
	total := 0
	for _, dir := range imageStoreDirs() {
		n, err := v.MigrateDir(dir)
		if err != nil {
			logx.Printf("images: sealing %s: %v", filepath.Base(dir), err)
		}
		total += n
	}
	if total > 0 {
		logx.Printf("images: sealed %d stored picture(s)", total)
	}
}
