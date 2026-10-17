// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"memo/internal/components"
)

// ComponentInstall describes one engine folder to replace with a verified
// release. The folder lives in the OVERRIDE tree (<data>/components/binaries/
// <os>/<folder>), never in the install: Windows keeps the install in Program
// Files, which a normal user cannot write, and a rollback is then just deleting
// a directory.
type ComponentInstall struct {
	OverrideRoot string // components.OverrideRoot(dataDir)
	OS           string // "linux", "darwin", "windows"
	Folder       string // "cpu", "amd", "cliproxy", …
	Component    components.Name
	Version      string
	File         File
	HTTP         *http.Client
	OnProgress   func(Progress)

	// Quiesce stops whatever is running out of the folder (llama-server,
	// whisper-server, the sidecar) and returns a function that starts it again.
	// The swap happens between the two. May be nil when nothing runs from it.
	Quiesce func() (resume func() error, err error)
	// Verify runs after resume; an error rolls the folder back. May be nil.
	Verify func() error
}

// Dest is the override folder that will hold the release.
func (c ComponentInstall) Dest() string {
	return filepath.Join(c.OverrideRoot, "binaries", c.OS, c.Folder)
}

func (c ComponentInstall) work() string { return filepath.Join(c.OverrideRoot, ".work") }

// expectedBinaries are file-name prefixes of which at least one must exist in
// the unpacked folder: a release archive that carries only a marker (or the
// wrong thing) must never replace a working engine.
func expectedBinaries(n components.Name) []string {
	switch n {
	case components.Llama:
		return []string{"llama-server"}
	case components.Whisper:
		return []string{"whisper-server"}
	case components.CLIProxy:
		return []string{"cli-proxy-api"}
	}
	return nil
}

// InstallComponent downloads, verifies and swaps in one engine folder. On any
// failure after the swap it restores the previous state — the earlier override
// if there was one, otherwise the bundled copy again — and restarts what it
// stopped.
func InstallComponent(ctx context.Context, c ComponentInstall) (err error) {
	if c.OverrideRoot == "" || c.OS == "" || c.Folder == "" {
		return errors.New("updater: component install needs an override root, os and folder")
	}
	if components.MarkerFile(c.Component) == "" || !folderOK(c.Folder) {
		return fmt.Errorf("updater: bad component %q folder %q", c.Component, c.Folder)
	}
	work := c.work()
	if err := os.MkdirAll(work, 0o700); err != nil {
		return err
	}
	tag := fmt.Sprintf("%s-%s-%s-%s", c.Component, c.Version, c.OS, c.Folder)
	archive := filepath.Join(work, tag+".archive")
	stage := filepath.Join(work, tag+".stage")
	defer func() { _ = os.Remove(archive); _ = os.RemoveAll(stage) }()

	// Room for the archive, its unpacked copy and the previous folder.
	if err := EnsureSpace(work, c.File.Size*3); err != nil {
		return err
	}
	if err := Download(ctx, DownloadOptions{
		URL: c.File.URL, Dest: archive, SHA256: c.File.SHA256, Size: c.File.Size,
		HTTP: c.HTTP, OnProgress: c.OnProgress,
	}); err != nil {
		return err
	}

	_ = os.RemoveAll(stage)
	if err := os.MkdirAll(stage, 0o700); err != nil {
		return err
	}
	if err := Extract(archive, c.File.Format, stage, ExtractLimits{}); err != nil {
		return err
	}
	if err := checkStaged(stage, c.Component, c.Version); err != nil {
		return err
	}
	if err := fixEnginePermissions(stage); err != nil {
		return err
	}

	// Stop what runs from the folder, swap, start it again.
	var resume func() error
	if c.Quiesce != nil {
		if resume, err = c.Quiesce(); err != nil {
			return fmt.Errorf("updater: could not stop the running engine: %w", err)
		}
	}
	resumeNow := func() error {
		if resume == nil {
			return nil
		}
		return resume()
	}

	dest := c.Dest()
	old := dest + ".old"
	_ = os.RemoveAll(old)
	hadOld := false
	if _, statErr := os.Lstat(dest); statErr == nil {
		if err := os.Rename(dest, old); err != nil {
			_ = resumeNow()
			return fmt.Errorf("updater: setting the current engine aside: %w", err)
		}
		hadOld = true
	}
	restore := func() error {
		if rmErr := os.RemoveAll(dest); rmErr != nil {
			return rmErr
		}
		if hadOld {
			return os.Rename(old, dest)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		_ = restore()
		_ = resumeNow()
		return err
	}
	if err := os.Rename(stage, dest); err != nil {
		_ = restore()
		_ = resumeNow()
		return fmt.Errorf("updater: installing the new engine: %w", err)
	}

	// The new files are in place. Does what used them come back up?
	startErr := resumeNow()
	if startErr == nil && c.Verify != nil {
		startErr = c.Verify()
	}
	if startErr != nil {
		// Whatever half-started from the new files must let go of them first.
		if c.Quiesce != nil {
			if again, qerr := c.Quiesce(); qerr == nil {
				resume = again
			}
		}
		if rerr := restore(); rerr != nil {
			return fmt.Errorf("updater: new engine failed (%v) and restoring the old one also failed: %w", startErr, rerr)
		}
		_ = resumeNow()
		return fmt.Errorf("updater: the new %s %s did not work and was removed: %w", c.Component, c.Version, startErr)
	}
	_ = os.RemoveAll(old)
	return nil
}

func folderOK(f string) bool {
	return f != "" && !strings.ContainsAny(f, `/\`+"\x00") && f != "." && f != ".."
}

// checkStaged makes sure an unpacked folder is what the manifest says it is.
func checkStaged(stage string, n components.Name, version string) error {
	got, err := components.ReadMarker(stage, n)
	if err != nil {
		return fmt.Errorf("updater: the release has an unusable %s: %w", components.MarkerFile(n), err)
	}
	if got != version {
		return fmt.Errorf("updater: the release says it is %q but the manifest promised %q", got, version)
	}
	entries, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	for _, e := range entries {
		for _, prefix := range expectedBinaries(n) {
			if strings.HasPrefix(e.Name(), prefix) {
				return nil
			}
		}
	}
	return fmt.Errorf("updater: the %s release contains no %s", n, strings.Join(expectedBinaries(n), "/"))
}

// fixEnginePermissions restores executable bits an archive may have dropped.
func fixEnginePermissions(dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, "llama-server") || strings.HasPrefix(name, "whisper-server") ||
			strings.HasPrefix(name, "memo-lora-train") || strings.HasPrefix(name, "cli-proxy-api") ||
			strings.Contains(name, ".so") || strings.HasSuffix(name, ".dylib") {
			st, err := os.Stat(path)
			if err != nil {
				return err
			}
			return os.Chmod(path, st.Mode().Perm()|0o755)
		}
		return nil
	})
}

// PruneOverrides deletes override folders that the install has caught up with
// or passed — after an app update brought a newer bundled engine, the older
// downloaded one must not keep shadowing it.
//
// A folder holds several components (linux/cpu carries llama.cpp AND
// whisper.cpp built against the same libraries), so it goes only when EVERY
// component it declares is no newer than the bundled copy of the same folder.
// Anything that cannot be compared (no marker on the bundled side, an
// unparseable version) keeps its override: an unreadable state is not a reason
// to delete. Returns the folders it removed.
func PruneOverrides(overrideRoot, goos string, bundledRoots []string) []string {
	all := []components.Name{components.Llama, components.Whisper, components.CLIProxy}
	base := filepath.Join(overrideRoot, "binaries", goos)
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var removed []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasSuffix(e.Name(), ".old") {
			continue
		}
		covered, declared := true, 0
		var dir string
		for _, n := range all {
			over := components.Detect(n, goos, e.Name(), overrideRoot, nil)
			if !over.Override || over.Version == "" {
				continue // this component is not in the override folder
			}
			declared++
			dir = over.Dir
			bundled := components.Detect(n, goos, e.Name(), "", bundledRoots)
			if !bundled.Found() || bundled.Version == "" {
				covered = false
				break
			}
			cmp, err := components.Compare(n, bundled.Version, over.Version)
			if err != nil || cmp < 0 {
				covered = false
				break
			}
		}
		if declared == 0 || !covered {
			continue
		}
		if err := os.RemoveAll(dir); err == nil {
			removed = append(removed, dir)
		}
	}
	return removed
}
