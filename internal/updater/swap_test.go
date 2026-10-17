// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func put(t *testing.T, root, rel, content string, mode os.FileMode) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// release lays out a Memo release (or install) tagged with ver.
func release(t *testing.T, root, ver string, mode os.FileMode) {
	t.Helper()
	put(t, root, "memo-backend", "backend-"+ver, mode)
	put(t, root, "memo_flutter", "flutter-"+ver, mode)
	put(t, root, "memo-updater", "updater-"+ver, mode)
	put(t, root, "run_memo.sh", "#!/bin/sh\n# "+ver, mode)
	put(t, root, "lib/libapp.so", "libapp-"+ver, mode)
	put(t, root, "binaries/linux/cpu/llama-server", "llama-"+ver, mode)
	put(t, root, "binaries/linux/cpu/libggml.so.0", "ggml-"+ver, mode)
	put(t, root, "binaries/linux/cpu/LLAMA_VERSION", "b"+ver+"\n", 0o644)
	put(t, root, "data/icudtl.dat", "icu-"+ver, 0o644)
	put(t, root, "data/flutter_assets/asset.bin", "asset-"+ver, 0o644)
}

// userData is what no update may ever touch.
func userData(t *testing.T, root string) {
	t.Helper()
	put(t, root, "config/config.yaml", "llama:\n  mode: cpu\n", 0o600)
	put(t, root, ".env", "SECRET=1\n", 0o600)
	put(t, root, "data/providers.json", `{"keys":"encrypted"}`, 0o600)
	put(t, root, "data/models/model.gguf", "GGUF", 0o644)
	put(t, root, "data/memory/mem.db", "SQLITE", 0o600)
	put(t, root, "data/sessions/chat1.json", "{}", 0o600)
	put(t, root, "data/permissions.json", "[]", 0o600)
	put(t, root, "data/components/binaries/linux/cpu/LLAMA_VERSION", "b11456\n", 0o644)
	put(t, root, "bin/memo", "backend-OLD", 0o755)
}

// snapshot captures every file, symlink and directory under root, except the
// updater's own scratch directory.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "." || rel == ".update" || strings.HasPrefix(rel, ".update"+string(filepath.Separator)) {
			return nil
		}
		switch {
		case d.IsDir():
			out[rel] = "<dir>"
		case d.Type()&fs.ModeSymlink != 0:
			l, _ := os.Readlink(p)
			out[rel] = "<link " + l + ">"
		default:
			b, _ := os.ReadFile(p)
			st, _ := d.Info()
			out[rel] = fmt.Sprintf("%s|%o", b, st.Mode().Perm())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func diff(a, b map[string]string) []string {
	var d []string
	for k, v := range a {
		if w, ok := b[k]; !ok {
			d = append(d, "missing after: "+k)
		} else if v != w {
			d = append(d, fmt.Sprintf("changed: %s: %q -> %q", k, v, w))
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			d = append(d, "new after: "+k)
		}
	}
	return d
}

type dirs struct{ install, staged, backup string }

func setup(t *testing.T) dirs {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("script-install swap is unix only")
	}
	install := t.TempDir()
	release(t, install, "OLD", 0o755)
	userData(t, install)
	staged := filepath.Join(install, ".update", "staging", "Memo")
	release(t, staged, "NEW", 0o644) // archives don't reliably keep exec bits
	return dirs{install, staged, filepath.Join(install, ".update", "backup")}
}

func TestPlaceNew_ReplacesTheAppAndNothingElse(t *testing.T) {
	d := setup(t)
	before := snapshot(t, d.install)

	sw, err := PlaceNew(d.install, d.staged, d.backup)
	if err != nil {
		t.Fatal(err)
	}
	after := snapshot(t, d.install)

	// Every user file is byte-identical.
	for rel, v := range before {
		isApp := false
		for _, item := range append(append([]string{}, SwapItems...), "bin/memo") {
			if rel == filepath.FromSlash(item) || strings.HasPrefix(rel, filepath.FromSlash(item)+string(filepath.Separator)) {
				isApp = true
			}
		}
		if !isApp && after[rel] != v {
			t.Errorf("user file %s changed: %q -> %q", rel, v, after[rel])
		}
	}
	// The app is the new one.
	for rel, want := range map[string]string{
		"memo-backend":                     "backend-NEW",
		"memo_flutter":                     "flutter-NEW",
		"memo-updater":                     "updater-NEW",
		"lib/libapp.so":                    "libapp-NEW",
		"binaries/linux/cpu/llama-server":  "llama-NEW",
		"binaries/linux/cpu/LLAMA_VERSION": "bNEW\n",
		"data/icudtl.dat":                  "icu-NEW",
		"data/flutter_assets/asset.bin":    "asset-NEW",
		"bin/memo":                         "backend-NEW", // the CLI is refreshed from memo-backend
	} {
		b, err := os.ReadFile(filepath.Join(d.install, filepath.FromSlash(rel)))
		if err != nil || string(b) != want {
			t.Errorf("%s = %q, %v; want %q", rel, b, err, want)
		}
	}
	// Executable bits were restored although the staged files were 0644.
	for _, rel := range []string{"memo-backend", "memo_flutter", "memo-updater", "run_memo.sh", "bin/memo", "binaries/linux/cpu/llama-server", "binaries/linux/cpu/libggml.so.0"} {
		st, err := os.Stat(filepath.Join(d.install, filepath.FromSlash(rel)))
		if err != nil || st.Mode()&0o111 == 0 {
			t.Errorf("%s is not executable: %v %v", rel, st, err)
		}
	}
	// The old app is kept until Commit.
	if b, _ := os.ReadFile(filepath.Join(d.backup, "memo-backend")); string(b) != "backend-OLD" {
		t.Errorf("backup memo-backend = %q", b)
	}
	if err := sw.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(d.backup); err == nil {
		t.Error("Commit left the backup behind")
	}
}

func TestSwap_RollbackRestoresTheExactPreviousTree(t *testing.T) {
	d := setup(t)
	before := snapshot(t, d.install)
	sw, err := PlaceNew(d.install, d.staged, d.backup)
	if err != nil {
		t.Fatal(err)
	}
	if err := sw.Rollback(); err != nil {
		t.Fatal(err)
	}
	if diffs := diff(before, snapshot(t, d.install)); len(diffs) > 0 {
		t.Fatalf("rollback did not restore the tree:\n%s", strings.Join(diffs, "\n"))
	}
}

// The strongest claim the swap makes: a failure at ANY point leaves the old
// install exactly as it was. Fail the Nth rename for every N.
func TestPlaceNew_FailureAtAnyStepRestoresTheOriginal(t *testing.T) {
	// First, count the renames a clean run performs.
	d := setup(t)
	var count int32
	orig := renameFn
	renameFn = func(a, b string) error { atomic.AddInt32(&count, 1); return orig(a, b) }
	if _, err := PlaceNew(d.install, d.staged, d.backup); err != nil {
		renameFn = orig
		t.Fatal(err)
	}
	renameFn = orig
	total := int(count)
	if total < 15 {
		t.Fatalf("only %d renames counted; the test would prove little", total)
	}

	for failAt := 1; failAt <= total; failAt++ {
		d := setup(t)
		before := snapshot(t, d.install)
		var n int32
		renameFn = func(a, b string) error {
			if int(atomic.AddInt32(&n, 1)) == failAt {
				return errors.New("injected failure")
			}
			return orig(a, b)
		}
		sw, err := PlaceNew(d.install, d.staged, d.backup)
		renameFn = orig
		if err == nil {
			t.Fatalf("failAt=%d: PlaceNew succeeded despite the injected failure (sw=%v)", failAt, sw)
		}
		if diffs := diff(before, snapshot(t, d.install)); len(diffs) > 0 {
			t.Fatalf("failAt=%d (of %d): the install was left damaged:\n%s", failAt, total, strings.Join(diffs, "\n"))
		}
	}
}

func TestPlaceNew_RefusesAReleaseWithoutABackend(t *testing.T) {
	d := setup(t)
	if err := os.Remove(filepath.Join(d.staged, "memo-backend")); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, d.install)
	if _, err := PlaceNew(d.install, d.staged, d.backup); err == nil {
		t.Fatal("a release with no memo-backend was installed")
	}
	if diffs := diff(before, snapshot(t, d.install)); len(diffs) > 0 {
		t.Fatalf("install changed:\n%s", strings.Join(diffs, "\n"))
	}
}

func TestPlaceNew_ItemsTheNewReleaseDoesNotShipAreLeftAlone(t *testing.T) {
	d := setup(t)
	put(t, d.install, "run_memo.command", "mac launcher", 0o755) // old install has it, new release doesn't
	if _, err := PlaceNew(d.install, d.staged, d.backup); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(d.install, "run_memo.command")); string(b) != "mac launcher" {
		t.Errorf("an item absent from the new release was removed: %q", b)
	}
}

func TestPlaceNew_FirstRunWithNoCLIYet(t *testing.T) {
	d := setup(t)
	if err := os.Remove(filepath.Join(d.install, "bin", "memo")); err != nil {
		t.Fatal(err)
	}
	if _, err := PlaceNew(d.install, d.staged, d.backup); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(d.install, "bin", "memo")); string(b) != "backend-NEW" {
		t.Errorf("bin/memo = %q", b)
	}
}

// ── Apply (the whole sequence, with a fake Memo) ────────────────────────────

type fakeMemo struct {
	t        *testing.T
	install  string
	launches []string // memo-backend content seen at each relaunch
	stops    int
	health   func(n int) (string, error)
	failNth  int // fail the Nth relaunch (1-based); 0 = never
	// exitsImmediately makes the launched process look dead at once.
	exitsImmediately bool
	healthN          int32
}

func (f *fakeMemo) relaunch() (*Launched, error) {
	b, _ := os.ReadFile(filepath.Join(f.install, "memo-backend"))
	f.launches = append(f.launches, string(b))
	if f.failNth > 0 && len(f.launches) == f.failNth {
		return nil, errors.New("exec failed")
	}
	l := &Launched{Stop: func() { f.stops++ }}
	if f.exitsImmediately {
		ch := make(chan struct{})
		close(ch)
		l.Exited = ch
	}
	return l, nil
}

func (f *fakeMemo) deps(alive func(int) bool) ApplyDeps {
	return ApplyDeps{
		Relaunch: f.relaunch,
		Health: func(ctx context.Context) (string, error) {
			return f.health(int(atomic.AddInt32(&f.healthN, 1)))
		},
		AliveFn: alive,
	}
}

func fastOpts(d dirs, expect string) ApplyOptions {
	return ApplyOptions{
		Install: d.install, Staged: d.staged, Backup: d.backup, WaitPIDs: []int{4242}, Expect: expect,
		WaitTimeout: 300 * time.Millisecond, HealthTimeout: 400 * time.Millisecond, PollEvery: 5 * time.Millisecond,
	}
}

func TestApply_HappyPath(t *testing.T) {
	d := setup(t)
	f := &fakeMemo{t: t, install: d.install, health: func(n int) (string, error) {
		if n < 3 {
			return "", errors.New("connection refused") // still starting
		}
		return "V4.7.0", nil
	}}
	var polls int32
	alive := func(pid int) bool { return atomic.AddInt32(&polls, 1) < 4 } // exits after a few polls

	if err := Apply(context.Background(), fastOpts(d, "V4.7.0"), f.deps(alive)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.launches, []string{"backend-NEW"}) {
		t.Errorf("relaunches saw %v; want exactly one launch of the NEW backend", f.launches)
	}
	if atomic.LoadInt32(&polls) < 4 {
		t.Error("did not wait for the old process to exit")
	}
	if _, err := os.Stat(d.backup); err == nil {
		t.Error("the backup was kept after a healthy start")
	}
	if b, _ := os.ReadFile(filepath.Join(d.install, "config", "config.yaml")); string(b) != "llama:\n  mode: cpu\n" {
		t.Error("user config touched")
	}
}

func TestApply_NeverTouchesFilesWhileTheOldMemoStillRuns(t *testing.T) {
	d := setup(t)
	before := snapshot(t, d.install)
	f := &fakeMemo{t: t, install: d.install, health: func(int) (string, error) { return "V4.7.0", nil }}
	err := Apply(context.Background(), fastOpts(d, "V4.7.0"), f.deps(func(int) bool { return true })) // never exits
	if err == nil {
		t.Fatal("Apply proceeded with the old Memo still running")
	}
	if len(f.launches) != 0 {
		t.Errorf("relaunched %v although nothing was swapped", f.launches)
	}
	if diffs := diff(before, snapshot(t, d.install)); len(diffs) > 0 {
		t.Fatalf("files changed while Memo was still running:\n%s", strings.Join(diffs, "\n"))
	}
}

func TestApply_UnhealthyNewVersionIsRolledBackAndTheOldOneRelaunched(t *testing.T) {
	d := setup(t)
	before := snapshot(t, d.install)
	f := &fakeMemo{t: t, install: d.install, health: func(int) (string, error) { return "", errors.New("connection refused") }}
	err := Apply(context.Background(), fastOpts(d, "V4.7.0"), f.deps(func(int) bool { return false }))
	if !errors.Is(err, ErrRolledBack) {
		t.Fatalf("err = %v, want ErrRolledBack", err)
	}
	if !reflect.DeepEqual(f.launches, []string{"backend-NEW", "backend-OLD"}) {
		t.Errorf("launches = %v; want the new one tried, then the old one restored", f.launches)
	}
	if f.stops != 1 {
		t.Errorf("the failed new process was stopped %d times, want 1", f.stops)
	}
	if diffs := diff(before, snapshot(t, d.install)); len(diffs) > 0 {
		t.Fatalf("rollback left a different install:\n%s", strings.Join(diffs, "\n"))
	}
}

func TestApply_WrongVersionAnsweringCountsAsUnhealthy(t *testing.T) {
	// The relaunch "worked" but an OLD backend is what answers (e.g. it never
	// actually stopped): not a success.
	d := setup(t)
	f := &fakeMemo{t: t, install: d.install, health: func(int) (string, error) { return "V4.6.0", nil }}
	err := Apply(context.Background(), fastOpts(d, "V4.7.0"), f.deps(func(int) bool { return false }))
	if !errors.Is(err, ErrRolledBack) {
		t.Fatalf("err = %v, want ErrRolledBack", err)
	}
}

func TestApply_RelaunchFailureRollsBack(t *testing.T) {
	d := setup(t)
	before := snapshot(t, d.install)
	f := &fakeMemo{t: t, install: d.install, failNth: 1, health: func(int) (string, error) { return "V4.7.0", nil }}
	err := Apply(context.Background(), fastOpts(d, "V4.7.0"), f.deps(func(int) bool { return false }))
	if !errors.Is(err, ErrRolledBack) {
		t.Fatalf("err = %v, want ErrRolledBack", err)
	}
	if diffs := diff(before, snapshot(t, d.install)); len(diffs) > 0 {
		t.Fatalf("install not restored:\n%s", strings.Join(diffs, "\n"))
	}
	if len(f.launches) != 2 || f.launches[1] != "backend-OLD" {
		t.Errorf("launches = %v; want a second launch of the OLD backend", f.launches)
	}
}

func TestApply_SwapFailureRelaunchesTheOldVersionAndChangesNothing(t *testing.T) {
	d := setup(t)
	before := snapshot(t, d.install)
	orig := renameFn
	var n int32
	renameFn = func(a, b string) error {
		if atomic.AddInt32(&n, 1) == 5 {
			return errors.New("disk on fire")
		}
		return orig(a, b)
	}
	defer func() { renameFn = orig }()

	f := &fakeMemo{t: t, install: d.install, health: func(int) (string, error) { return "V4.7.0", nil }}
	err := Apply(context.Background(), fastOpts(d, "V4.7.0"), f.deps(func(int) bool { return false }))
	if err == nil {
		t.Fatal("swap failure reported as success")
	}
	if !reflect.DeepEqual(f.launches, []string{"backend-OLD"}) {
		t.Errorf("launches = %v; the user must be left with their old Memo running", f.launches)
	}
	if diffs := diff(before, snapshot(t, d.install)); len(diffs) > 0 {
		t.Fatalf("install changed:\n%s", strings.Join(diffs, "\n"))
	}
}

func TestApply_CancelledContextStopsWaiting(t *testing.T) {
	d := setup(t)
	f := &fakeMemo{t: t, install: d.install, health: func(int) (string, error) { return "V4.7.0", nil }}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	o := fastOpts(d, "V4.7.0")
	o.WaitTimeout = time.Minute
	if err := Apply(ctx, o, f.deps(func(int) bool { return true })); err == nil {
		t.Fatal("a cancelled context was ignored")
	}
}

func TestPidAlive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	if !pidAlive(os.Getpid()) {
		t.Error("own pid reported dead")
	}
	if pidAlive(0) || pidAlive(-5) {
		t.Error("non-positive pid reported alive")
	}
	if pidAlive(2147483646) {
		t.Error("a pid that cannot exist reported alive")
	}
}

func TestApply_ALauncherThatDiesEarlyFailsFastInsteadOfWaitingOutTheTimeout(t *testing.T) {
	d := setup(t)
	f := &fakeMemo{t: t, install: d.install, exitsImmediately: true,
		health: func(int) (string, error) { return "", errors.New("connection refused") }}
	o := fastOpts(d, "V4.7.0")
	o.HealthTimeout = time.Minute // would hang the test if the early exit were ignored
	start := time.Now()
	err := Apply(context.Background(), o, f.deps(func(int) bool { return false }))
	if !errors.Is(err, ErrRolledBack) {
		t.Fatalf("err = %v, want ErrRolledBack", err)
	}
	if took := time.Since(start); took > 10*time.Second {
		t.Fatalf("took %v: the early exit was not noticed", took)
	}
}
