// SPDX-License-Identifier: AGPL-3.0-or-later

// Command memo-updater swaps a downloaded Memo release into a script-installed
// Memo (Linux/macOS: ~/.memo) after the running Memo has exited, relaunches it,
// and rolls back if the new version does not come up.
//
// The backend downloads and verifies the release, unpacks it into
// <install>/.update/staging, starts the memo-updater found INSIDE that new
// release (so the swap logic always matches the layout it is installing), and
// shuts itself down. See PLAN_auto_update.md and internal/updater.
//
//	memo-updater apply --install DIR --work DIR --staged DIR --backup DIR \
//	    --wait-pid N --expect V4.7.0 --health http://127.0.0.1:8090/api/version \
//	    --launch DIR/run_memo.sh --log FILE
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"memo/internal/updater"
)

type intList []int

func (l *intList) String() string { return fmt.Sprint(*l) }
func (l *intList) Set(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fmt.Errorf("bad pid %q", s)
	}
	*l = append(*l, n)
	return nil
}

func main() { os.Exit(run(os.Args[1:], os.Stderr)) }

func run(args []string, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "apply" {
		fmt.Fprintln(stderr, "usage: memo-updater apply --install DIR --work DIR --staged DIR --backup DIR --wait-pid N --expect V1.2.3 --health URL --launch SCRIPT --log FILE")
		return 2
	}
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		install   = fs.String("install", "", "the Memo install directory")
		work      = fs.String("work", "", "scratch directory inside the install (<install>/.update)")
		staged    = fs.String("staged", "", "the unpacked new release")
		backup    = fs.String("backup", "", "where replaced files are kept until the new version proves itself")
		expect    = fs.String("expect", "", "version the new backend must report")
		healthURL = fs.String("health", "", "the backend's /api/version URL")
		launch    = fs.String("launch", "", "the launcher script that starts Memo")
		logPath   = fs.String("log", "", "append progress here")
		pids      intList
	)
	fs.Var(&pids, "wait-pid", "a process that must exit first (repeatable)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	if err := validate(*install, *work, *staged, *backup, *launch, *expect, *healthURL); err != nil {
		fmt.Fprintln(stderr, "memo-updater:", err)
		return 2
	}

	// This process outlives the backend that started it; a hangup when that
	// backend's terminal or session goes away must not kill it mid-swap.
	signal.Ignore(syscall.SIGHUP, syscall.SIGPIPE)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var logw io.Writer = stderr
	if *logPath != "" {
		if f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
			defer f.Close()
			logw = io.MultiWriter(stderr, f)
		}
	}
	logf := func(format string, a ...any) { fmt.Fprintf(logw, "memo-updater: "+format+"\n", a...) }

	deps := updater.ApplyDeps{
		Relaunch: func() (*updater.Launched, error) {
			return updater.StartDetached(*launch, *install, os.Environ(), logw)
		},
		Health: updater.HTTPHealth(*healthURL),
	}
	err := updater.Apply(ctx, updater.ApplyOptions{
		Install: *install, Staged: *staged, Backup: *backup,
		WaitPIDs: pids, Expect: *expect, Log: logf,
	}, deps)

	// The scratch directory holds the backup. Delete it only when nothing in
	// it can still be needed: success, or a rollback that put everything back.
	if err == nil || errors.Is(err, updater.ErrRolledBack) {
		if rerr := os.RemoveAll(*work); rerr != nil {
			logf("could not remove %s: %v", *work, rerr)
		}
	} else {
		logf("keeping %s (it may hold the previous version)", *work)
	}
	if err != nil {
		logf("FAILED: %v", err)
		return 1
	}
	logf("done")
	return 0
}

// validate keeps a mistyped (or malicious local) invocation from pointing the
// swap at paths it was never meant to touch: everything must live under the
// install directory's own scratch space.
func validate(install, work, staged, backup, launch, expect, healthURL string) error {
	for name, p := range map[string]string{"install": install, "work": work, "staged": staged, "backup": backup, "launch": launch} {
		if p == "" || !filepath.IsAbs(p) {
			return fmt.Errorf("--%s must be an absolute path", name)
		}
	}
	if _, err := updater.ParseVersion(expect); err != nil {
		return fmt.Errorf("--expect: %w", err)
	}
	if !strings.HasPrefix(healthURL, "http://127.0.0.1:") && !strings.HasPrefix(healthURL, "http://localhost:") {
		return errors.New("--health must be a loopback URL")
	}
	in := filepath.Clean(install)
	if filepath.Clean(work) != filepath.Join(in, ".update") {
		return errors.New("--work must be <install>/.update")
	}
	for name, p := range map[string]string{"staged": staged, "backup": backup} {
		if rel, err := filepath.Rel(filepath.Clean(work), filepath.Clean(p)); err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("--%s must be inside --work", name)
		}
	}
	if rel, err := filepath.Rel(in, filepath.Clean(launch)); err != nil || strings.HasPrefix(rel, "..") {
		return errors.New("--launch must be inside the install directory")
	}
	return nil
}
