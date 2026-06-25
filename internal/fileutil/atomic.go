package fileutil

import (
	"io"
	"io/fs"
	"log"
	"os"
)

// AtomicWrite writes data to path using a sibling .tmp file followed by an
// os.Rename. Because Rename is atomic on the same filesystem, a crash between
// the write and the rename leaves the original file intact.
//
// The .tmp file is always in the same directory as path, so the rename never
// crosses a filesystem boundary.
//
// On Windows, os.Rename fails when the destination file is held open by
// another reader (ERROR_ACCESS_DENIED). We fall back to a copy-then-delete
// in that case so session saves and config writes don't fail on Windows.
func AtomicWrite(path string, data []byte, perm fs.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		if copyErr := copyFile(tmp, path); copyErr != nil {
			os.Remove(tmp)
			return err // return the original rename error
		}
		os.Remove(tmp)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil {
			log.Printf("fileutil: close dst %s: %v", dst, cerr)
		}
	}()

	_, err = io.Copy(out, in)
	return err
}
