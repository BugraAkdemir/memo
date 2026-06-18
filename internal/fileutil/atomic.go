package fileutil

import (
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
func AtomicWrite(path string, data []byte, perm fs.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		if err2 := os.Remove(tmp); err2 != nil {
			log.Printf("fileutil: remove orphan tmp %s: %v", tmp, err2)
		}
		return err
	}
	return nil
}
