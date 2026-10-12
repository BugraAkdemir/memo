package backupdata

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveRejectsFileTooLargeToRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, MaxFileBytes+1); err != nil {
		t.Fatal(err)
	}
	_, _, err := Archive([]Source{{Entry: "data/models/model.gguf", Path: path}})
	if err == nil || !strings.Contains(err.Error(), "restore size limit") {
		t.Fatalf("expected restore size limit, got %v", err)
	}
}

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
