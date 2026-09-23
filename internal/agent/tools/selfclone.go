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

// selfCloneProtectedPaths mirrors defaultProtectedPaths()'s OS-critical
// entries but excludes Linux's "/home/" and "/tmp/": unlike validatePath's
// callers (write_file/edit_file/etc., which must ALWAYS stay inside
// basePath, protected-or-not), SelfClone's entire purpose is writing the
// project to a destination OUTSIDE basePath — and for an ordinary non-root
// desktop user, /home/<user>/... and /tmp/ are the only realistic writable
// locations at all. Blocking them wholesale wouldn't harden anything, it
// would just disable the tool. selfCloneHasDotComponent below is what
// actually keeps a destination under /home/ from reaching a credential or
// persistence location.
func selfCloneProtectedPaths() []string {
	all := defaultProtectedPaths()
	blocked := make([]string, 0, len(all))
	for _, p := range all {
		if p == "/home/" || p == "/tmp/" {
			continue
		}
		blocked = append(blocked, p)
	}
	return blocked
}

// selfCloneHasDotComponent reports whether any path segment of realDest
// starts with "." (and isn't "." or ".." itself). Found in a 2026-09-23
// security audit: SelfClone had no protected-path check of ANY kind before
// this, so an unattended (bypass-permissions) Self-Driving task could
// write_file a malicious file inside the sandbox, then call self_clone with
// dest=~/.ssh to silently overwrite the real ~/.ssh/authorized_keys with
// it — zero permission prompt, zero user visibility, full sandbox escape to
// persistent host compromise. Rather than enumerate every dangerous name
// (the exact mistake the old run_command blacklist made — see
// BUG_REPORT.md's P0-4/P2-4), this blocks by SHAPE: virtually every
// credential/persistence target on a real system lives inside a dotfile or
// dot-directory (~/.ssh, ~/.gnupg, ~/.aws, ~/.kube, ~/.config/systemd,
// ~/.bashrc, ~/.profile, ~/.local/share/...), so refusing any destination
// with a dot-prefixed path component closes that whole class at once
// instead of chasing individual names one incident at a time.
func selfCloneHasDotComponent(realDest string) bool {
	for _, part := range strings.Split(realDest, string(filepath.Separator)) {
		if part != "." && part != ".." && strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
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

	// Resolve symlinks (a not-yet-existing dest resolves as far as its real
	// ancestors go, same BUG-C1 fallback validatePath uses) before deciding
	// whether it's safe to write to — checking the raw, unresolved
	// absDest would let a symlink at any ancestor silently redirect the
	// whole clone into a protected/dotfile location undetected.
	realDest, err := resolveRealPath(absDest)
	if err != nil {
		return "", fmt.Errorf("cannot resolve destination: %w", err)
	}
	if protected, ok := isUnderProtectedSystemPath(realDest, selfCloneProtectedPaths()); ok {
		return "", fmt.Errorf("access denied: %q is within a protected system directory (%s) — self_clone refuses to write there", args.Dest, protected)
	}
	if selfCloneHasDotComponent(realDest) {
		return "", fmt.Errorf("access denied: %q resolves to %q, a hidden/dotfile path — self_clone refuses to write into a dotfile or dotfile-named directory, since that is where credentials and persistence configs (~/.ssh, ~/.config, ~/.bashrc, ...) live", args.Dest, realDest)
	}

	if err := os.MkdirAll(dest, 0755); err != nil {
		return "", fmt.Errorf("cannot create dest dir: %w", err)
	}

	copied := 0
	err = filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
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
		if err := copyFileTo(binary, binDest); err != nil {
			return "", fmt.Errorf("clone files ok but binary copy failed: %w", err)
		}
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
