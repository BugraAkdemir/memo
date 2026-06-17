package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type SelfCloneArgs struct {
	Dest string `json:"dest"`
}

// SelfClone projenin tamamını local'de başka bir dizine kopyalar.
// Binary + çalışma dizini içeriğini hedef path'e yazar.
func SelfClone(ctx context.Context, argsJSON json.RawMessage, basePath string, _ func(string) error) (string, error) {
	var args SelfCloneArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Dest == "" {
		return "", fmt.Errorf("dest path is required")
	}

	dest := filepath.Clean(args.Dest)

	// Kendine kopyalamayı engelle
	absSrc, _ := filepath.Abs(basePath)
	absDest, _ := filepath.Abs(dest)
	if absDest == absSrc || strings.HasPrefix(absDest, absSrc+string(filepath.Separator)) {
		return "", fmt.Errorf("destination cannot be inside source directory")
	}

	if err := os.MkdirAll(dest, 0755); err != nil {
		return "", fmt.Errorf("cannot create dest dir: %w", err)
	}

	copied := 0
	err := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// .git ve node_modules gibi dizinleri atla
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
		}

		rel, _ := filepath.Rel(basePath, path)
		target := filepath.Join(dest, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		if err := copyFileTo(path, target); err != nil {
			return err
		}
		copied++
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("clone failed: %w", err)
	}

	// Binary'yi de kopyala
	if binary, err := os.Executable(); err == nil {
		binDest := filepath.Join(dest, filepath.Base(binary))
		_ = copyFileTo(binary, binDest)
	}

	return fmt.Sprintf("Cloned %d files to %s", copied, dest), nil
}

func copyFileTo(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
