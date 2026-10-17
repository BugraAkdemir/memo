// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"memo/internal/components"
	"memo/internal/config"
	"memo/internal/llama"
	"memo/internal/logx"
	"memo/internal/shutdown"
	"memo/internal/updater"
	"memo/internal/whisper"
)

// In-app update (PLAN_auto_update.md). Two kinds of update share this file:
//
//   - the APPLICATION: a new release of Memo itself. On Linux/macOS installs made
//     by scripts/get-memo.sh the backend downloads and verifies the release,
//     unpacks it, starts the memo-updater found inside it and exits; on Windows
//     it downloads Setup.exe and hands over to it; everywhere else the person is
//     told where to download.
//   - an engine COMPONENT (llama.cpp, whisper.cpp, the Subscriptions sidecar):
//     new binaries fetched into <data>/components while Memo keeps running.
//
// Everything that decides WHAT to download comes from a manifest signed with the
// key compiled into the binary; the beacon (version.json) only says which
// version to ask for.

const (
	manualDownloadURL = "https://memocpp.com"
	updateCheckTTL    = 2 * time.Minute
	// restartGrace gives the UI one more status poll to show "restarting" before
	// the backend goes away.
	restartGrace = 1500 * time.Millisecond
)

// updateRuntime is the App's update state. The zero value is ready to use.
type updateRuntime struct {
	mu      sync.Mutex
	status  updater.Status
	running bool
	info    *updater.Info
	infoAt  time.Time

	// Test seams.
	client *updater.Client
	facts  *installFacts
}

// installFacts are the observable properties that decide how an install can be
// updated. Injected so the decision is testable on any machine.
type installFacts struct {
	GOOS, GOARCH string
	Exe          string
	Getenv       func(string) string
	Exists       func(string) bool
	Writable     func(string) bool
}

func realFacts() installFacts {
	exe, _ := os.Executable()
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return installFacts{
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Exe: exe,
		Getenv: os.Getenv,
		Exists: func(p string) bool { _, err := os.Stat(p); return err == nil },
		Writable: func(dir string) bool {
			f, err := os.CreateTemp(dir, ".memo-write-test-*")
			if err != nil {
				return false
			}
			name := f.Name()
			f.Close()
			os.Remove(name)
			return true
		},
	}
}

// updatePlatforms are the platforms CI publishes a signed manifest for.
var updatePlatforms = map[string]bool{
	"linux-amd64": true, "darwin-arm64": true, "darwin-amd64": true, "windows-amd64": true,
}

// classifyInstall decides how this install can be updated and where it lives.
func classifyInstall(f installFacts) (mode, dir, launcher string) {
	platform := f.GOOS + "-" + f.GOARCH
	if !updatePlatforms[platform] || f.Exe == "" {
		return updater.ModeManual, "", ""
	}
	if f.GOOS == "windows" {
		// Only an install made by the Setup program (it leaves its uninstaller
		// beside the program) is updated by running a newer Setup. Split the path
		// by hand: this must answer the same way when tested on another OS.
		i := strings.LastIndexAny(f.Exe, `\/`)
		if i < 0 {
			return updater.ModeManual, "", ""
		}
		exeDir := f.Exe[:i]
		if f.Exists(exeDir + `\unins000.exe`) {
			return updater.ModeInstaller, exeDir, ""
		}
		return updater.ModeManual, "", ""
	}
	exeDir := filepath.Dir(f.Exe)
	// An AppImage is a read-only mount; a container image is replaced, not
	// patched; neither can swap files under itself.
	if f.Getenv("APPIMAGE") != "" || strings.Contains(f.Exe, "/.mount_") || f.Exists("/.dockerenv") {
		return updater.ModeManual, "", ""
	}
	d := exeDir
	if filepath.Base(d) == "bin" { // the installed CLI: ~/.memo/bin/memo
		d = filepath.Dir(d)
	}
	if !f.Exists(filepath.Join(d, "memo-backend")) {
		return updater.ModeManual, "", "" // a source checkout, a distro package…
	}
	for _, name := range []string{"run_memo.sh", "run_memo.command"} {
		if f.Exists(filepath.Join(d, name)) {
			if !f.Writable(d) {
				return updater.ModeManual, "", ""
			}
			return updater.ModeAuto, d, filepath.Join(d, name)
		}
	}
	return updater.ModeManual, "", ""
}

func (a *App) installFacts() installFacts {
	if a.update.facts != nil {
		return *a.update.facts
	}
	return realFacts()
}

// updateClient builds the client for the update endpoints. The environment
// variables are the test/dev seam (like MEMO_TELEGRAM_API_BASE); a replacement
// signing key is honoured ONLY together with a loopback base URL, so pointing
// real traffic somewhere else can never also swap the key that vouches for it.
func (a *App) updateClient() *updater.Client {
	if a.update.client != nil {
		return a.update.client
	}
	c := updater.NewClient(updater.PublicKey())
	if v := strings.TrimSpace(os.Getenv("MEMO_UPDATE_BEACON_URL")); v != "" {
		c.BeaconURL = v
	}
	if v := strings.TrimSpace(os.Getenv("MEMO_UPDATE_BASE_URL")); v != "" {
		c.BaseURL = v
		if u, err := url.Parse(v); err == nil && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1") {
			if k := strings.TrimSpace(os.Getenv("MEMO_UPDATE_PUBKEY")); k != "" {
				if pub, err := updater.DecodePublic(k); err == nil {
					c.Pub = pub
				}
			}
		}
	}
	return c
}

// ── check ───────────────────────────────────────────────────────────────────

// CheckLatestVersion is the original single-string check, kept for the REPL
// and the old /api/version/check callers. It is now semver-aware: a beacon that
// names an OLDER version is not an update.
func (a *App) CheckLatestVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	bc, err := a.updateClient().FetchBeacon(ctx)
	if err != nil {
		return "", fmt.Errorf("version check: %w", err)
	}
	newer, err := updater.Newer(a.GetVersion(), bc.Version)
	if err != nil || !newer {
		return "", nil
	}
	return bc.Version, nil
}

// CheckForUpdate asks the beacon and the signed manifests what is new. A fresh
// answer is reused for a couple of minutes; force bypasses that.
func (a *App) CheckForUpdate(ctx context.Context, force bool) (*updater.Info, error) {
	a.update.mu.Lock()
	if !force && a.update.info != nil && time.Since(a.update.infoAt) < updateCheckTTL {
		cp := *a.update.info
		a.update.mu.Unlock()
		return &cp, nil
	}
	a.update.mu.Unlock()

	info := a.buildUpdateInfo(ctx)

	a.update.mu.Lock()
	a.update.info, a.update.infoAt = info, time.Now()
	cp := *info
	a.update.mu.Unlock()
	return &cp, nil
}

func (a *App) invalidateUpdateInfo() {
	a.update.mu.Lock()
	a.update.info = nil
	a.update.mu.Unlock()
}

func (a *App) buildUpdateInfo(ctx context.Context) *updater.Info {
	facts := a.installFacts()
	mode, installDir, _ := classifyInstall(facts)
	info := &updater.Info{
		Current:     a.GetVersion(),
		Mode:        mode,
		Platform:    facts.GOOS + "-" + facts.GOARCH,
		DownloadURL: manualDownloadURL,
		Components:  []updater.ComponentInfo{},
		CheckedAt:   time.Now().Unix(),
	}
	if installDir != "" {
		info.LastFailure = updater.ReadFailure(filepath.Join(installDir, updater.FailureFile))
	}
	client := a.updateClient()
	bc, err := client.FetchBeacon(ctx)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	info.Latest, info.NoteTR, info.NoteEN = bc.Version, bc.NoteTR, bc.NoteEN

	newer, perr := updater.Newer(info.Current, bc.Version)
	if perr != nil {
		// A build that cannot read its own version (a dev build) is never offered
		// anything.
		info.Error = perr.Error()
		return info
	}
	if newer {
		info.Available = true
		info.Required = bc.Required
		if mode != updater.ModeManual {
			v, _ := updater.ParseVersion(bc.Version)
			m, err := client.AppManifest(ctx, v, info.Platform)
			if err != nil {
				// No (valid) manifest for this platform: the release may still be
				// publishing, or something is wrong with it. Either way do not offer
				// a one-click update; point the person at the site instead.
				logx.Printf("update: no usable manifest for %s %s: %v", bc.Version, info.Platform, err)
				info.Mode = updater.ModeManual
				info.Error = "update manifest: " + err.Error()
			} else {
				info.SizeBytes = m.Size
			}
		}
		// An application update replaces the bundled engines too; component
		// updates would be redundant (and older than what is coming).
		return info
	}

	info.Components = a.checkComponents(ctx, client, bc)
	return info
}

// componentOrder keeps the list stable.
var componentOrder = []components.Name{components.Llama, components.Whisper, components.CLIProxy}

func (a *App) checkComponents(ctx context.Context, client *updater.Client, bc *updater.Beacon) []updater.ComponentInfo {
	out := []updater.ComponentInfo{}
	cur, _ := updater.ParseVersion(a.GetVersion())
	for _, name := range componentOrder {
		latest, ok := bc.Components[string(name)]
		if !ok {
			continue
		}
		ci := updater.ComponentInfo{Name: string(name), Latest: latest}
		folder := a.engineFolder(name)
		if folder == "" {
			ci.Reason = "not_ours"
			out = append(out, ci)
			continue
		}
		ci.Folder = folder
		inst := components.Detect(name, runtime.GOOS, folder, components.OverrideRoot(config.DataDir()), components.BundledRoots())
		ci.Installed = inst.Version
		switch {
		case !inst.Found():
			ci.Reason = "not_ours"
		case !inst.Known():
			// Installed before version markers existed: nothing to compare. Never
			// guess that it is outdated.
			ci.Reason = "unknown_installed"
		default:
			c, err := components.Compare(name, inst.Version, latest)
			switch {
			case err != nil:
				ci.Reason = "check_failed"
				logx.Printf("update: comparing %s %q with %q: %v", name, inst.Version, latest, err)
			case c >= 0:
				ci.Reason = "up_to_date"
			default:
				a.fillComponentFromManifest(ctx, client, &ci, name, latest, folder, cur)
			}
		}
		out = append(out, ci)
	}
	return out
}

func (a *App) fillComponentFromManifest(ctx context.Context, client *updater.Client, ci *updater.ComponentInfo, name components.Name, latest, folder string, cur updater.Version) {
	m, err := client.ComponentManifest(ctx, string(name), latest)
	if err != nil {
		ci.Reason = "check_failed"
		logx.Printf("update: component manifest %s %s: %v", name, latest, err)
		return
	}
	f, ok := m.Files[runtime.GOOS+"/"+folder]
	if !ok {
		ci.Reason = "not_for_this_machine"
		return
	}
	if m.RequiresApp != "" {
		need, err := updater.ParseVersion(m.RequiresApp)
		if err == nil && cur.Compare(need) < 0 {
			ci.Reason, ci.RequiresApp = "needs_newer_app", m.RequiresApp
			return
		}
	}
	ci.Available, ci.SizeBytes = true, f.Size
}

// engineFolder returns the engine folder in use for a component, or "" when the
// engine comes from somewhere Memo does not own.
func (a *App) engineFolder(name components.Name) string {
	a.cfgMu.RLock()
	llamaBin, mode, whisperBin := a.cfg.Llama.BinaryPath, a.cfg.Llama.EngineMode, a.cfg.Whisper.BinaryPath
	a.cfgMu.RUnlock()
	switch name {
	case components.Llama:
		return llama.EngineFlavor(llamaBin, mode)
	case components.Whisper:
		return whisper.EngineFlavor(whisperBin)
	case components.CLIProxy:
		if _, _, err := a.subsManager().Binary(); err == nil {
			return "cliproxy"
		}
	}
	return ""
}

// ── status ──────────────────────────────────────────────────────────────────

// UpdateStatus reports the progress of a running update.
func (a *App) UpdateStatus() updater.Status {
	a.update.mu.Lock()
	defer a.update.mu.Unlock()
	st := a.update.status
	if st.State == "" {
		st.State = updater.StateIdle
	}
	return st
}

func (a *App) setUpdateStatus(f func(*updater.Status)) {
	a.update.mu.Lock()
	f(&a.update.status)
	a.update.mu.Unlock()
}

func (a *App) updateProgress(p updater.Progress) {
	a.setUpdateStatus(func(s *updater.Status) { s.Done, s.Total = p.Done, p.Total })
}

// ── apply ───────────────────────────────────────────────────────────────────

// ApplyUpdate starts an update in the background and returns at once; progress
// is read with UpdateStatus. kind is "app" or "component" (then component names
// llama, whisper or cliproxy). Only one update runs at a time.
func (a *App) ApplyUpdate(kind, component string) error {
	switch kind {
	case "app":
	case "component":
		if !validComponent(component) {
			return fmt.Errorf("unknown component %q", component)
		}
	default:
		return fmt.Errorf("unknown update kind %q", kind)
	}
	a.update.mu.Lock()
	if a.update.running {
		a.update.mu.Unlock()
		return updater.ErrBusy
	}
	a.update.running = true
	a.update.status = updater.Status{State: updater.StateVerifying, Kind: kind, Component: component}
	a.update.mu.Unlock()

	goRecover("update", func() {
		var err error
		if kind == "app" {
			err = a.runAppUpdate(a.lifecycle())
		} else {
			err = a.runComponentUpdate(a.lifecycle(), components.Name(component))
		}
		a.update.mu.Lock()
		a.update.running = false
		if err != nil {
			logx.Printf("update: %s %s failed: %v", kind, component, err)
			a.update.status.State = updater.StateError
			a.update.status.Error = err.Error()
		}
		a.update.mu.Unlock()
		a.invalidateUpdateInfo()
	})
	return nil
}

func validComponent(s string) bool {
	for _, n := range componentOrder {
		if string(n) == s {
			return true
		}
	}
	return false
}

func (a *App) runAppUpdate(ctx context.Context) error {
	info, err := a.CheckForUpdate(ctx, true)
	if err != nil {
		return err
	}
	if !info.Available {
		return errors.New("there is no update to install")
	}
	switch info.Mode {
	case updater.ModeInstaller:
		return a.runInstallerUpdate(ctx, info)
	case updater.ModeAuto:
		return a.runArchiveUpdate(ctx, info)
	}
	return errors.New("this installation cannot update itself; download the new version from " + manualDownloadURL)
}

func (a *App) runInstallerUpdate(ctx context.Context, info *updater.Info) error {
	client := a.updateClient()
	v, err := updater.ParseVersion(info.Latest)
	if err != nil {
		return err
	}
	m, err := client.AppManifest(ctx, v, info.Platform)
	if err != nil {
		return err
	}
	if m.Kind != updater.KindInstaller {
		return fmt.Errorf("the %s release is not an installer", info.Platform)
	}
	dir := config.DataPath("updates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := updater.EnsureSpace(dir, m.Size*2); err != nil {
		return err
	}
	dest := filepath.Join(dir, "Memo-Setup-v"+v.String()+".exe")
	a.setUpdateStatus(func(s *updater.Status) { s.State, s.Done, s.Total = updater.StateDownloading, 0, m.Size })
	if err := updater.Download(ctx, updater.DownloadOptions{
		URL: m.URL, Dest: dest, SHA256: m.SHA256, Size: m.Size, HTTP: client.HTTP, OnProgress: a.updateProgress,
	}); err != nil {
		return err
	}
	a.setUpdateStatus(func(s *updater.Status) { s.State = updater.StateInstalling })
	if err := updater.RunInstaller(dest); err != nil {
		return err
	}
	// The installer takes over: leave so its file replacement is not blocked.
	a.setUpdateStatus(func(s *updater.Status) { s.State = updater.StateRestarting })
	a.requestShutdownSoon()
	return nil
}

func (a *App) runArchiveUpdate(ctx context.Context, info *updater.Info) error {
	client := a.updateClient()
	facts := a.installFacts()
	mode, install, launcher := classifyInstall(facts)
	if mode != updater.ModeAuto {
		return errors.New("this installation cannot update itself")
	}
	v, err := updater.ParseVersion(info.Latest)
	if err != nil {
		return err
	}
	m, err := client.AppManifest(ctx, v, info.Platform)
	if err != nil {
		return err
	}
	if m.Kind != updater.KindArchive {
		return fmt.Errorf("the %s release is not an archive", info.Platform)
	}

	work := filepath.Join(install, ".update")
	_ = os.Remove(filepath.Join(install, updater.FailureFile)) // a new attempt supersedes the old note
	if st, err := os.Stat(filepath.Join(work, "backup")); err == nil && st.IsDir() {
		// A previous update left its backup because even the rollback failed. That
		// directory may hold the only intact copy of the app: never delete it.
		return fmt.Errorf("a previous update did not finish cleanly; its backup is still in %s (see update.log there)", work)
	}
	_ = os.RemoveAll(work)
	if err := os.MkdirAll(filepath.Join(work, "download"), 0o700); err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(work)
		}
	}()

	// Archive + its unpacked copy; the backup is a rename, it costs nothing.
	if err := updater.EnsureSpace(install, m.Size*3); err != nil {
		return err
	}
	archive := filepath.Join(work, "download", "release."+m.Format)
	a.setUpdateStatus(func(s *updater.Status) { s.State, s.Done, s.Total = updater.StateDownloading, 0, m.Size })
	if err := updater.Download(ctx, updater.DownloadOptions{
		URL: m.URL, Dest: archive, SHA256: m.SHA256, Size: m.Size, HTTP: client.HTTP, OnProgress: a.updateProgress,
	}); err != nil {
		return err
	}

	a.setUpdateStatus(func(s *updater.Status) { s.State = updater.StateInstalling })
	staging := filepath.Join(work, "staging")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return err
	}
	if err := updater.Extract(archive, m.Format, staging, updater.ExtractLimits{}); err != nil {
		return err
	}
	_ = os.Remove(archive)
	root := m.Root
	if root == "" {
		root = "Memo"
	}
	staged := filepath.Join(staging, root)
	newUpdater := filepath.Join(staged, "memo-updater")
	for _, need := range []string{filepath.Join(staged, "memo-backend"), newUpdater} {
		if _, err := os.Stat(need); err != nil {
			return fmt.Errorf("the downloaded release is incomplete (%s missing)", filepath.Base(need))
		}
	}
	if err := os.Chmod(newUpdater, 0o755); err != nil {
		return err
	}

	port := 8090
	if ws := a.getWebServer(); ws != nil && ws.GetPort() > 0 {
		port = ws.GetPort()
	}
	// The updater writes update.log itself (--log); its own stdout/stderr stay
	// unattached so nothing is logged twice.
	logPath := filepath.Join(install, "update.log")
	// The NEW release's updater does the swap, so the swap logic always matches
	// the layout it is installing. It waits for this process to exit first.
	_, err = updater.StartDetachedArgs(newUpdater, []string{
		"apply",
		"--install", install,
		"--work", work,
		"--staged", staged,
		"--backup", filepath.Join(work, "backup"),
		"--wait-pid", strconv.Itoa(os.Getpid()),
		"--expect", m.Version,
		"--health", fmt.Sprintf("http://127.0.0.1:%d/api/version", port),
		"--launch", launcher,
		"--log", logPath,
	}, install, os.Environ(), nil)
	if err != nil {
		return err
	}
	cleanup = false // the updater owns the scratch directory from here on
	a.setUpdateStatus(func(s *updater.Status) { s.State = updater.StateRestarting })
	a.requestShutdownSoon()
	return nil
}

// requestShutdownSoon ends this process shortly, after the UI has had a moment
// to read "restarting".
func (a *App) requestShutdownSoon() {
	time.AfterFunc(restartGrace, shutdown.Request)
}

// ── component updates ───────────────────────────────────────────────────────

func (a *App) runComponentUpdate(ctx context.Context, name components.Name) error {
	info, err := a.CheckForUpdate(ctx, true)
	if err != nil {
		return err
	}
	var ci *updater.ComponentInfo
	for i := range info.Components {
		if info.Components[i].Name == string(name) {
			ci = &info.Components[i]
		}
	}
	if ci == nil || !ci.Available {
		return fmt.Errorf("there is no %s update to install", name)
	}
	client := a.updateClient()
	m, err := client.ComponentManifest(ctx, string(name), ci.Latest)
	if err != nil {
		return err
	}
	f, ok := m.Files[runtime.GOOS+"/"+ci.Folder]
	if !ok {
		return fmt.Errorf("the %s %s release has nothing for %s/%s", name, ci.Latest, runtime.GOOS, ci.Folder)
	}
	a.setUpdateStatus(func(s *updater.Status) { s.State, s.Done, s.Total = updater.StateDownloading, 0, f.Size })

	overrideRoot := components.OverrideRoot(config.DataDir())
	inst := updater.ComponentInstall{
		OverrideRoot: overrideRoot,
		OS:           runtime.GOOS,
		Folder:       ci.Folder,
		Component:    name,
		Version:      ci.Latest,
		File:         f,
		HTTP:         client.HTTP,
		OnProgress:   a.updateProgress,
		Quiesce:      func() (func() error, error) { return a.quiesceEngine(name) },
	}
	inst.Verify = func() error { return a.verifyEngine(name, inst.Dest(), ci.Latest) }
	a.setUpdateStatus(func(s *updater.Status) { s.State = updater.StateInstalling })
	if err := updater.InstallComponent(ctx, inst); err != nil {
		return err
	}
	a.setUpdateStatus(func(s *updater.Status) { s.State = updater.StateDone })
	return nil
}

// quiesceEngine stops whatever runs out of the engine folder being replaced and
// returns the function that starts it again.
func (a *App) quiesceEngine(name components.Name) (func() error, error) {
	switch name {
	case components.Llama:
		return a.quiesceLlama()
	case components.Whisper:
		return a.quiesceWhisper()
	case components.CLIProxy:
		return a.quiesceSidecar()
	}
	return nil, fmt.Errorf("unknown component %q", name)
}

func (a *App) quiesceLlama() (func() error, error) {
	a.swarmMu.Lock()
	swarm := a.swarmServer
	a.swarmMu.Unlock()
	if swarm != nil && swarm.IsRunning() {
		return nil, errors.New("Memo Swarm is using the engine; stop it first")
	}
	chat := a.llamaServer.GetStatus()
	emb := a.llamaEmbedServer.GetStatus()
	// A llama-server somebody else started (no tracked process) is not ours to stop.
	if (chat.Running && chat.PID == 0) || (emb.Running && emb.PID == 0) {
		return nil, errors.New("a llama-server that Memo did not start is running; stop it first")
	}
	if chat.Running {
		if err := a.StopLocalModel(); err != nil {
			return nil, err
		}
	}
	if emb.Running {
		if err := a.StopEmbeddingModel(); err != nil {
			return nil, err
		}
	}
	a.cfgMu.RLock()
	embLayers := a.cfg.Llama.EmbeddingGPULayers
	a.cfgMu.RUnlock()
	return func() error {
		if emb.Running {
			if err := a.StartEmbeddingModel(emb.ModelPath, embLayers); err != nil {
				return fmt.Errorf("embedding model: %w", err)
			}
		}
		if chat.Running {
			if err := a.StartLocalModel(chat.ModelPath, chat.CtxSize, chat.Port, -1); err != nil {
				a.llamaServer.Stop()
				return fmt.Errorf("chat model: %w", err)
			}
		}
		return nil
	}, nil
}

func (a *App) quiesceWhisper() (func() error, error) {
	a.whisperMu.Lock()
	ws := a.whisperServer
	a.whisperServer = nil
	a.whisperMu.Unlock()
	wasRunning := ws != nil && ws.IsRunning()
	if ws != nil {
		if err := ws.Stop(); err != nil {
			a.whisperMu.Lock()
			a.whisperServer = ws
			a.whisperMu.Unlock()
			return nil, err
		}
	}
	return func() error {
		if !wasRunning {
			return nil
		}
		a.startSTTServer()
		a.whisperMu.RLock()
		ok := a.whisperServer != nil && a.whisperServer.IsRunning()
		a.whisperMu.RUnlock()
		if !ok {
			return errors.New("speech-to-text did not start with the new engine")
		}
		return nil
	}, nil
}

func (a *App) quiesceSidecar() (func() error, error) {
	m := a.subsManager()
	wasRunning := m.Running()
	m.Stop()
	return func() error {
		// The new binary must pass the sidecar's own checksum gate.
		if _, _, err := m.Binary(); err != nil {
			return err
		}
		if wasRunning {
			goRecover("update.restartSidecar", a.startSubscriptions)
		}
		return nil
	}, nil
}

// verifyEngine runs the freshly installed binary far enough to prove it loads
// and is the version it claims. It works even when nothing was running before,
// which is when the restart step proves nothing.
func (a *App) verifyEngine(name components.Name, dir, version string) error {
	var bin string
	var args []string
	switch name {
	case components.Llama:
		bin, args = filepath.Join(dir, executable("llama-server")), []string{"--version"}
	case components.Whisper:
		bin, args = filepath.Join(dir, executable("whisper-server")), []string{"--help"}
	case components.CLIProxy:
		_, _, err := a.subsManager().Binary()
		return err
	}
	if _, err := os.Stat(bin); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = withLibraryPath(os.Environ(), dir)
	out, err := cmd.CombinedOutput()
	text := string(out)
	if name == components.Llama {
		if err != nil {
			return fmt.Errorf("llama-server --version: %w: %s", err, firstLine(text))
		}
		want := strings.TrimPrefix(strings.TrimPrefix(version, "b"), "B")
		if !strings.Contains(text, "version: "+want) {
			return fmt.Errorf("llama-server reports %q, expected build %s", firstLine(text), want)
		}
		return nil
	}
	// whisper-server's --help exits non-zero on some builds; what matters is
	// that it started and printed its usage rather than failing to load a library.
	if !strings.Contains(strings.ToLower(text), "usage") {
		return fmt.Errorf("whisper-server did not start: %s", firstLine(text))
	}
	return nil
}

func executable(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// withLibraryPath prepends dir to the loader's search path so an engine finds
// its own libraries.
func withLibraryPath(env []string, dir string) []string {
	key := "LD_LIBRARY_PATH"
	switch runtime.GOOS {
	case "darwin":
		key = "DYLD_LIBRARY_PATH"
	case "windows":
		key = "PATH"
	}
	out := make([]string, 0, len(env)+1)
	set := false
	for _, e := range env {
		if k, v, ok := strings.Cut(e, "="); ok && (k == key || (runtime.GOOS == "windows" && strings.EqualFold(k, key))) {
			out = append(out, k+"="+dir+string(os.PathListSeparator)+v)
			set = true
			continue
		}
		out = append(out, e)
	}
	if !set {
		out = append(out, key+"="+dir)
	}
	return out
}

// pruneDownloadedEngines removes downloaded engine folders the install has
// caught up with. Called once at startup, after an app update may have bundled
// newer engines.
func (a *App) pruneDownloadedEngines() {
	removed := updater.PruneOverrides(components.OverrideRoot(config.DataDir()), runtime.GOOS, components.BundledRoots())
	for _, r := range removed {
		logx.Printf("update: removed downloaded engine %s (the install now bundles the same or a newer one)", r)
	}
}
