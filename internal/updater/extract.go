// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractLimits bound what an archive may unpack to. The archive is verified
// against a signed digest first, so these are a second wall, not the first.
type ExtractLimits struct {
	MaxFiles int
	MaxBytes int64
}

// DefaultExtractLimits fits the largest real archive (~1.8 GB unpacked) with
// room to grow.
var DefaultExtractLimits = ExtractLimits{MaxFiles: 200_000, MaxBytes: 8 << 30}

// Extract unpacks src (format "tar.gz" or "zip") into dst, which must exist.
// It refuses absolute paths, "..", links that point outside dst, hard links and
// device nodes — the classic ways an archive escapes its directory.
func Extract(src, format, dst string, lim ExtractLimits) error {
	if lim.MaxFiles == 0 {
		lim = DefaultExtractLimits
	}
	root, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	switch format {
	case "tar.gz":
		return extractTarGz(src, root, realRoot, lim)
	case "zip":
		return extractZip(src, root, realRoot, lim)
	}
	return fmt.Errorf("updater: cannot unpack format %q", format)
}

// target resolves an archive member name to a path under root, or errors.
func target(root, name string) (string, error) {
	n := strings.ReplaceAll(name, `\`, "/")
	if n == "" || strings.HasPrefix(n, "/") || strings.ContainsRune(n, 0) {
		return "", fmt.Errorf("updater: unsafe path in archive: %q", name)
	}
	clean := filepath.Clean(filepath.FromSlash(n))
	if clean == "." {
		return root, nil
	}
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.VolumeName(clean) != "" {
		return "", fmt.Errorf("updater: unsafe path in archive: %q", name)
	}
	return filepath.Join(root, clean), nil
}

// inside reports whether the real (symlink-resolved) location of path's
// existing ancestors is within realRoot. A lexical check alone is not enough: an
// earlier member can plant a link ("x/a -> ..") that makes a later, lexically
// harmless link ("x/a/l -> ../../y") or file land outside the destination.
func inside(realRoot, path string) error {
	dir := filepath.Dir(path)
	for {
		if _, err := os.Lstat(dir); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(realRoot, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("updater: archive member would be written outside the destination: %s", path)
	}
	return nil
}

// linkOK reports whether a symlink at path with the given target stays inside
// root once resolved (relative targets only).
func linkOK(root, path, linkname string) bool {
	if filepath.IsAbs(linkname) || strings.HasPrefix(linkname, "/") || strings.ContainsRune(linkname, 0) {
		return false
	}
	resolved := filepath.Join(filepath.Dir(path), filepath.FromSlash(linkname))
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func extractTarGz(src, root, realRoot string, lim ExtractLimits) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	var files int
	var total int64
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		files++
		if files > lim.MaxFiles {
			return errors.New("updater: archive has too many entries")
		}
		path, err := target(root, h.Name)
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeXGlobalHeader && path != root {
			if err := inside(realRoot, path); err != nil {
				return err
			}
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			total += h.Size
			if total > lim.MaxBytes || h.Size < 0 {
				return errors.New("updater: archive unpacks to more than the allowed size")
			}
			if err := writeFile(path, tr, os.FileMode(h.Mode)&0o777, h.Size); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if !linkOK(root, path, h.Linkname) {
				return fmt.Errorf("updater: link %q points outside the archive", h.Name)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			_ = os.Remove(path)
			if err := os.Symlink(h.Linkname, path); err != nil {
				return err
			}
		case tar.TypeXGlobalHeader:
			// pax global header: metadata only.
		default:
			return fmt.Errorf("updater: archive member %q has unsupported type %q", h.Name, string(h.Typeflag))
		}
	}
}

func extractZip(src, root, realRoot string, lim ExtractLimits) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	if len(zr.File) > lim.MaxFiles {
		return errors.New("updater: archive has too many entries")
	}
	var total int64
	for _, zf := range zr.File {
		path, err := target(root, zf.Name)
		if err != nil {
			return err
		}
		if path != root {
			if err := inside(realRoot, path); err != nil {
				return err
			}
		}
		mode := zf.Mode()
		switch {
		case mode.IsDir() || strings.HasSuffix(zf.Name, "/"):
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
		case mode&os.ModeSymlink != 0:
			rc, err := zf.Open()
			if err != nil {
				return err
			}
			b, err := io.ReadAll(io.LimitReader(rc, 4096))
			rc.Close()
			if err != nil {
				return err
			}
			if !linkOK(root, path, string(b)) {
				return fmt.Errorf("updater: link %q points outside the archive", zf.Name)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			_ = os.Remove(path)
			if err := os.Symlink(string(b), path); err != nil {
				return err
			}
		case mode.IsRegular():
			total += int64(zf.UncompressedSize64)
			if total > lim.MaxBytes || zf.UncompressedSize64 > uint64(lim.MaxBytes) {
				return errors.New("updater: archive unpacks to more than the allowed size")
			}
			rc, err := zf.Open()
			if err != nil {
				return err
			}
			err = writeFile(path, rc, mode.Perm(), int64(zf.UncompressedSize64))
			rc.Close()
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("updater: archive member %q has an unsupported type", zf.Name)
		}
	}
	return nil
}

// writeFile copies exactly size bytes from r into a new file at path. A member
// that turns out longer or shorter than its header claimed is an error.
func writeFile(path string, r io.Reader, perm os.FileMode, size int64) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// A later member must not be written THROUGH a symlink an earlier member
	// planted: remove whatever is there first (a plain Remove never follows).
	_ = os.Remove(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm|0o600)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(r, size+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("updater: %s: member is %d bytes, its header says %d", filepath.Base(path), n, size)
	}
	return nil
}
