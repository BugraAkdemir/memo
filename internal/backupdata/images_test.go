package backupdata

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestSessionPicturePathsRemapWindowsAndUnix(t *testing.T) {
	for _, path := range []string{`C:\\Users\\test\\Memo\\data\\images\\one.png`, `/old/data/images/one.png`} {
		blob := []byte(`{"messages":[{"image_path":"` + path + `"}]}`)
		got, err := RestoreContent("sessions/one.json", blob, "/new/data")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(got, []byte(filepath.ToSlash(filepath.Join("/new/data", "images", "one.png")))) {
			t.Fatalf("unmapped: %s", got)
		}
	}
}
