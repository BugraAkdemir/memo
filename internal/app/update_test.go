// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"memo/internal/components"
	"memo/internal/config"
	"memo/internal/llama"
	"memo/internal/shutdown"
	"memo/internal/updater"
)

// ── classifyInstall ─────────────────────────────────────────────────────────

func facts(goos, goarch, exe string, env map[string]string, files ...string) installFacts {
	have := map[string]bool{}
	for _, f := range files {
		have[f] = true
	}
	return installFacts{
		GOOS: goos, GOARCH: goarch, Exe: exe,
		Getenv:   func(k string) string { return env[k] },
		Exists:   func(p string) bool { return have[p] },
		Writable: func(string) bool { return !have["READONLY"] },
	}
}

func TestClassifyInstall(t *testing.T) {
	home := "/home/u/.memo"
	cases := []struct {
		name     string
		f        installFacts
		mode     string
		dir      string
		launcher string
	}{
		{"script install, backend binary", facts("linux", "amd64", home+"/memo-backend", nil, home+"/memo-backend", home+"/run_memo.sh"),
			updater.ModeAuto, home, home + "/run_memo.sh"},
		{"script install, CLI in bin/", facts("linux", "amd64", home+"/bin/memo", nil, home+"/memo-backend", home+"/run_memo.sh"),
			updater.ModeAuto, home, home + "/run_memo.sh"},
		{"macOS launcher", facts("darwin", "arm64", home+"/memo-backend", nil, home+"/memo-backend", home+"/run_memo.command"),
			updater.ModeAuto, home, home + "/run_memo.command"},
		{"AppImage env", facts("linux", "amd64", home+"/memo-backend", map[string]string{"APPIMAGE": "/x/Memo.AppImage"}, home+"/memo-backend", home+"/run_memo.sh"),
			updater.ModeManual, "", ""},
		{"AppImage mount", facts("linux", "amd64", "/tmp/.mount_MemoAB/usr/bin/memo-backend", nil, "/tmp/.mount_MemoAB/usr/bin/memo-backend"),
			updater.ModeManual, "", ""},
		{"docker", facts("linux", "amd64", "/app/memo-backend", nil, "/.dockerenv", "/app/memo-backend", "/app/run_memo.sh"),
			updater.ModeManual, "", ""},
		{"source checkout (no memo-backend)", facts("linux", "amd64", "/src/memo/memo", nil, "/src/memo/main.go"),
			updater.ModeManual, "", ""},
		{"no launcher script", facts("linux", "amd64", home+"/memo-backend", nil, home+"/memo-backend"),
			updater.ModeManual, "", ""},
		{"install dir not writable", facts("linux", "amd64", home+"/memo-backend", nil, home+"/memo-backend", home+"/run_memo.sh", "READONLY"),
			updater.ModeManual, "", ""},
		{"linux arm64 has no published build", facts("linux", "arm64", home+"/memo-backend", nil, home+"/memo-backend", home+"/run_memo.sh"),
			updater.ModeManual, "", ""},
		{"windows setup install", facts("windows", "amd64", `C:\Program Files\Memo\memo-backend.exe`, nil, `C:\Program Files\Memo\unins000.exe`),
			updater.ModeInstaller, `C:\Program Files\Memo`, ""},
		{"windows portable zip", facts("windows", "amd64", `D:\memo\memo-backend.exe`, nil),
			updater.ModeManual, "", ""},
		{"windows arm64", facts("windows", "arm64", `C:\Program Files\Memo\memo-backend.exe`, nil, `C:\Program Files\Memo\unins000.exe`),
			updater.ModeManual, "", ""},
		{"unknown exe", facts("linux", "amd64", "", nil), updater.ModeManual, "", ""},
	}
	for _, c := range cases {
		mode, dir, launcher := classifyInstall(c.f)
		if mode != c.mode || dir != c.dir || launcher != c.launcher {
			t.Errorf("%s: got (%s, %q, %q), want (%s, %q, %q)", c.name, mode, dir, launcher, c.mode, c.dir, c.launcher)
		}
	}
}

// ── a fake update site ──────────────────────────────────────────────────────

type site struct {
	t    *testing.T
	pub  ed25519.PublicKey
	priv ed25519.PrivateKey
	mux  *http.ServeMux
	srv  *httptest.Server
	hits atomic.Int64
}

func newSite(t *testing.T) *site {
	t.Helper()
	pub, priv, err := updater.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	s := &site{t: t, pub: pub, priv: priv, mux: http.NewServeMux()}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		s.mux.ServeHTTP(w, r)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *site) put(path string, body []byte) {
	s.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) })
}

func (s *site) putSigned(path string, body []byte) {
	sig, err := updater.Sign(s.priv, body)
	if err != nil {
		s.t.Fatal(err)
	}
	s.put(path, body)
	s.put(path+".sig", []byte(sig))
}

func (s *site) beacon(v string, extra map[string]any) {
	m := map[string]any{"version": v, "mini_version_note_tr": "kısa not", "mini_version_note_en": "short note"}
	for k, x := range extra {
		m[k] = x
	}
	b, _ := json.Marshal(m)
	s.put("/version.json", b)
}

func (s *site) client() *updater.Client {
	c := updater.NewClient(s.pub)
	c.BeaconURL = s.srv.URL + "/version.json"
	c.BaseURL = s.srv.URL + "/update"
	return c
}

// serveFile publishes a local file and returns its URL.
func (s *site) serveFile(path, local string) string {
	s.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, local) })
	return s.srv.URL + path
}

func (s *site) appManifest(version, platform string, mut func(map[string]any)) {
	m := map[string]any{
		"schema": 1, "type": "app", "version": version, "platform": platform, "kind": "archive", "format": "tar.gz",
		"url": s.srv.URL + "/memo.tar.gz", "sha256": strings.Repeat("a", 64), "size": 1234, "root": "Memo",
	}
	if mut != nil {
		mut(m)
	}
	b, _ := json.Marshal(m)
	v, _ := updater.ParseVersion(version)
	s.putSigned("/update/app/"+v.String()+"/"+platform+".json", b)
}

func (s *site) componentManifest(name, version, requiresApp string, files map[string]updater.File) {
	m := updater.ComponentManifest{Schema: 1, Type: "component", Component: name, Version: version, RequiresApp: requiresApp, Files: files}
	b, _ := json.Marshal(m)
	s.putSigned("/update/components/"+name+"/"+version+"/manifest.json", b)
}

func updApp(t *testing.T, s *site, version string, f *installFacts) *App {
	t.Helper()
	a := &App{version: version, cfg: config.Default()}
	a.update.client = s.client()
	if f == nil {
		ff := facts("linux", "amd64", "/home/u/.memo/memo-backend", nil, "/home/u/.memo/memo-backend", "/home/u/.memo/run_memo.sh")
		f = &ff
	}
	a.update.facts = f
	return a
}

// ── CheckForUpdate: the application ─────────────────────────────────────────

func TestCheckForUpdate_AnAppUpdate(t *testing.T) {
	s := newSite(t)
	s.beacon("V4.7.0", map[string]any{"required_update": true})
	s.appManifest("V4.7.0", "linux-amd64", func(m map[string]any) { m["size"] = 777 })
	a := updApp(t, s, "V4.6.0", nil)

	info, err := a.CheckForUpdate(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.Latest != "V4.7.0" || info.Current != "V4.6.0" {
		t.Fatalf("info = %+v", info)
	}
	if info.Mode != updater.ModeAuto || info.SizeBytes != 777 || !info.Required {
		t.Errorf("mode/size/required = %s %d %v", info.Mode, info.SizeBytes, info.Required)
	}
	if info.NoteTR != "kısa not" || info.NoteEN != "short note" {
		t.Errorf("notes = %q / %q", info.NoteTR, info.NoteEN)
	}
	if len(info.Components) != 0 {
		t.Errorf("component updates offered next to an app update: %+v", info.Components)
	}
}

func TestCheckForUpdate_NothingNewOrOlder(t *testing.T) {
	for _, tc := range []struct{ cur, beacon string }{
		{"V4.7.0", "V4.7.0"},
		{"V4.7.0", "V4.6.0"}, // a downgrade is not an update
		{"V4.10.0", "V4.9.0"},
	} {
		s := newSite(t)
		s.beacon(tc.beacon, map[string]any{"required_update": true})
		info, _ := updApp(t, s, tc.cur, nil).CheckForUpdate(context.Background(), true)
		if info.Available || info.Required {
			t.Errorf("current %s, beacon %s: info = %+v", tc.cur, tc.beacon, info)
		}
	}
}

func TestCheckForUpdate_DevBuildIsNeverOffered(t *testing.T) {
	s := newSite(t)
	s.beacon("V9.9.9", nil)
	info, _ := updApp(t, s, "dev-build", nil).CheckForUpdate(context.Background(), true)
	if info.Available || info.Error == "" {
		t.Fatalf("a build that can't read its own version was offered an update: %+v", info)
	}
}

func TestCheckForUpdate_NoManifestFallsBackToManual(t *testing.T) {
	s := newSite(t)
	s.beacon("V4.7.0", nil) // announced, but no manifest published for this platform
	info, _ := updApp(t, s, "V4.6.0", nil).CheckForUpdate(context.Background(), true)
	if !info.Available || info.Mode != updater.ModeManual || info.DownloadURL == "" || info.Error == "" {
		t.Fatalf("info = %+v; want available, manual, with the download page and the reason", info)
	}
}

func TestCheckForUpdate_ForgedManifestNeverOffersOneClick(t *testing.T) {
	s := newSite(t)
	s.beacon("V4.7.0", nil)
	_, evil, _ := updater.GenerateKey()
	body, _ := json.Marshal(map[string]any{"schema": 1, "type": "app", "version": "V4.7.0", "platform": "linux-amd64",
		"kind": "archive", "format": "tar.gz", "url": "https://evil.example/x", "sha256": strings.Repeat("b", 64), "size": 10})
	sig, _ := updater.Sign(evil, body)
	s.put("/update/app/4.7.0/linux-amd64.json", body)
	s.put("/update/app/4.7.0/linux-amd64.json.sig", []byte(sig))

	info, _ := updApp(t, s, "V4.6.0", nil).CheckForUpdate(context.Background(), true)
	if info.Mode == updater.ModeAuto {
		t.Fatalf("a manifest signed by the wrong key enabled one-click update: %+v", info)
	}
}

func TestCheckForUpdate_ManualInstallsStillHearAboutIt(t *testing.T) {
	s := newSite(t)
	s.beacon("V4.7.0", nil)
	f := facts("linux", "amd64", "/tmp/.mount_MemoX/usr/bin/memo-backend", nil)
	info, _ := updApp(t, s, "V4.6.0", &f).CheckForUpdate(context.Background(), true)
	if !info.Available || info.Mode != updater.ModeManual {
		t.Fatalf("info = %+v", info)
	}
}

func TestCheckForUpdate_BeaconDownIsAnErrorFieldNotAFailure(t *testing.T) {
	s := newSite(t) // no /version.json -> 404
	info, err := updApp(t, s, "V4.6.0", nil).CheckForUpdate(context.Background(), true)
	if err != nil || info.Available || info.Error == "" {
		t.Fatalf("err=%v info=%+v", err, info)
	}
}

func TestCheckForUpdate_CachesButForceBypasses(t *testing.T) {
	s := newSite(t)
	s.beacon("V4.6.0", nil)
	a := updApp(t, s, "V4.6.0", nil)
	_, _ = a.CheckForUpdate(context.Background(), false)
	first := s.hits.Load()
	_, _ = a.CheckForUpdate(context.Background(), false)
	if s.hits.Load() != first {
		t.Error("a second check inside the TTL hit the network")
	}
	_, _ = a.CheckForUpdate(context.Background(), true)
	if s.hits.Load() == first {
		t.Error("force did not bypass the cache")
	}
}

func TestCheckLatestVersion_IsSemverAware(t *testing.T) {
	for _, tc := range []struct{ cur, beacon, want string }{
		{"V4.6.0", "V4.7.0", "V4.7.0"},
		{"V4.7.0", "V4.6.0", ""}, // the old `!=` check called this an update
		{"V4.7.0", "V4.7.0", ""},
		{"dev", "V4.7.0", ""},
	} {
		s := newSite(t)
		s.beacon(tc.beacon, nil)
		got, err := updApp(t, s, tc.cur, nil).CheckLatestVersion()
		if err != nil || got != tc.want {
			t.Errorf("cur %s beacon %s: got %q, %v; want %q", tc.cur, tc.beacon, got, err, tc.want)
		}
	}
}

// ── CheckForUpdate: engine components ───────────────────────────────────────

// engineTree lays a bundled cpu engine (with a marker) in a temp cwd, so the
// resolvers treat it as the installed one.
func engineTree(t *testing.T, llamaMarker string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "binaries", runtime.GOOS, "cpu")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, executable("llama-server")), []byte("#!/bin/sh\necho 'version: 9441 (abc)'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if llamaMarker != "" {
		if err := os.WriteFile(filepath.Join(dir, "LLAMA_VERSION"), []byte(llamaMarker+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	return dir
}

func cleanOverride(t *testing.T) {
	t.Helper()
	root := components.OverrideRoot(config.DataDir())
	_ = os.RemoveAll(root)
	t.Cleanup(func() { _ = os.RemoveAll(root) })
}

func compApp(t *testing.T, s *site, appVersion string) *App {
	a := updApp(t, s, appVersion, nil)
	a.cfg.Llama.EngineMode = "cpu"
	a.llamaServer = llama.NewServer(48123, 2048)
	a.llamaEmbedServer = llama.NewServer(48124, 512)
	return a
}

func llamaFile(url string) map[string]updater.File {
	return map[string]updater.File{runtime.GOOS + "/cpu": {URL: url, SHA256: strings.Repeat("c", 64), Size: 4096, Format: "tar.gz"}}
}

func findComp(info *updater.Info, name string) *updater.ComponentInfo {
	for i := range info.Components {
		if info.Components[i].Name == name {
			return &info.Components[i]
		}
	}
	return nil
}

func TestCheckComponents(t *testing.T) {
	cleanOverride(t)
	engineTree(t, "b9441")
	s := newSite(t)
	s.beacon("V4.6.0", map[string]any{"components": map[string]string{"llama": "b11456"}})
	s.componentManifest("llama", "b11456", "4.6.0", llamaFile("https://data.memocpp.com/x.tar.gz"))
	a := compApp(t, s, "V4.6.0")

	info, _ := a.CheckForUpdate(context.Background(), true)
	c := findComp(info, "llama")
	if c == nil || !c.Available || c.Installed != "b9441" || c.Latest != "b11456" || c.Folder != "cpu" || c.SizeBytes != 4096 {
		t.Fatalf("llama = %+v", c)
	}
	if info.Available {
		t.Error("a component release must not look like an app update")
	}
}

func TestCheckComponents_Reasons(t *testing.T) {
	type tc struct {
		name       string
		marker     string // installed marker ("" = unmarked folder)
		latest     string
		requires   string
		files      map[string]updater.File
		noManifest bool
		appVer     string
		wantReason string
		wantAvail  bool
	}
	good := llamaFile("https://data.memocpp.com/x.tar.gz")
	for _, c := range []tc{
		{name: "update available", marker: "b9441", latest: "b11456", files: good, appVer: "V4.6.0", wantAvail: true},
		{name: "already current", marker: "b11456", latest: "b11456", files: good, appVer: "V4.6.0", wantReason: "up_to_date"},
		{name: "installed is newer than announced", marker: "b12000", latest: "b11456", files: good, appVer: "V4.6.0", wantReason: "up_to_date"},
		{name: "installed version unknown", marker: "", latest: "b11456", files: good, appVer: "V4.6.0", wantReason: "unknown_installed"},
		{name: "needs a newer app", marker: "b9441", latest: "b11456", requires: "4.9.0", files: good, appVer: "V4.6.0", wantReason: "needs_newer_app"},
		{name: "app is new enough", marker: "b9441", latest: "b11456", requires: "4.6.0", files: good, appVer: "V4.6.0", wantAvail: true},
		{name: "no file for this folder", marker: "b9441", latest: "b11456",
			files: map[string]updater.File{runtime.GOOS + "/nvidia": good[runtime.GOOS+"/cpu"]}, appVer: "V4.6.0", wantReason: "not_for_this_machine"},
		{name: "manifest missing", marker: "b9441", latest: "b11456", noManifest: true, appVer: "V4.6.0", wantReason: "check_failed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cleanOverride(t)
			engineTree(t, c.marker)
			s := newSite(t)
			s.beacon(c.appVer, map[string]any{"components": map[string]string{"llama": c.latest}})
			if !c.noManifest {
				s.componentManifest("llama", c.latest, c.requires, c.files)
			}
			info, _ := compApp(t, s, c.appVer).CheckForUpdate(context.Background(), true)
			ci := findComp(info, "llama")
			if ci == nil {
				t.Fatalf("llama missing from %+v", info.Components)
			}
			if ci.Available != c.wantAvail || ci.Reason != c.wantReason {
				t.Errorf("got available=%v reason=%q; want %v %q (%+v)", ci.Available, ci.Reason, c.wantAvail, c.wantReason, ci)
			}
		})
	}
}

func TestCheckComponents_EngineFromElsewhereIsNotOurs(t *testing.T) {
	cleanOverride(t)
	t.Chdir(t.TempDir()) // nothing bundled under the working directory
	s := newSite(t)
	s.beacon("V4.6.0", map[string]any{"components": map[string]string{"llama": "b11456", "whisper": "1.9.0", "cliproxy": "v8.0.26"}})
	a := compApp(t, s, "V4.6.0")
	a.cfg.Llama.BinaryPath = "/usr/local/bin/llama-server" // the person's own build
	info, _ := a.CheckForUpdate(context.Background(), true)
	for _, c := range info.Components {
		if c.Available || (c.Reason != "not_ours" && c.Reason != "unknown_installed") {
			t.Errorf("%s offered for an engine Memo does not own: %+v", c.Name, c)
		}
	}
}

// ── ApplyUpdate gating ──────────────────────────────────────────────────────

func TestApplyUpdate_RejectsBadRequestsAndRunsOneAtATime(t *testing.T) {
	a := &App{version: "V4.6.0", cfg: config.Default()}
	for _, bad := range [][2]string{{"", ""}, {"firmware", ""}, {"component", ""}, {"component", "sqlite"}, {"component", "../llama"}} {
		if err := a.ApplyUpdate(bad[0], bad[1]); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
	a.update.running = true
	if err := a.ApplyUpdate("app", ""); !errors.Is(err, updater.ErrBusy) {
		t.Errorf("err = %v, want ErrBusy while one is running", err)
	}
}

func TestUpdateStatus_DefaultsToIdle(t *testing.T) {
	a := &App{}
	if st := a.UpdateStatus(); st.State != updater.StateIdle {
		t.Errorf("state = %q", st.State)
	}
}

// ── component update, end to end through the App ────────────────────────────

func tarGz(t *testing.T, files map[string]struct {
	body string
	mode int64
}) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.tar.gz")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, c := range files {
		_ = tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: c.mode, Size: int64(len(c.body))})
		_, _ = tw.Write([]byte(c.body))
	}
	_ = tw.Close()
	_ = gz.Close()
	_ = f.Close()
	return p
}

type fileSpec = struct {
	body string
	mode int64
}

func engineArchive(t *testing.T, reportedBuild, marker string) string {
	return tarGz(t, map[string]fileSpec{
		"llama-server":  {"#!/bin/sh\necho 'version: " + reportedBuild + " (deadbeef)'\n", 0o755},
		"LLAMA_VERSION": {marker + "\n", 0o644},
	})
}

func sumAndSize(t *testing.T, p string) (string, int64) {
	sum, err := updater.SHA256File(p)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(p)
	return sum, st.Size()
}

func publishLlama(t *testing.T, s *site, archive, version string) {
	sum, size := sumAndSize(t, archive)
	url := s.serveFile("/engine-"+version+".tar.gz", archive)
	s.beacon("V4.6.0", map[string]any{"components": map[string]string{"llama": version}})
	s.componentManifest("llama", version, "", map[string]updater.File{
		runtime.GOOS + "/cpu": {URL: url, SHA256: sum, Size: size, Format: "tar.gz"},
	})
}

func TestRunComponentUpdate_InstallsVerifiesAndTheCheckThenSaysUpToDate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as the engine")
	}
	cleanOverride(t)
	engineTree(t, "b9441")
	s := newSite(t)
	publishLlama(t, s, engineArchive(t, "11456", "b11456"), "b11456")
	a := compApp(t, s, "V4.6.0")

	if err := a.runComponentUpdate(context.Background(), components.Llama); err != nil {
		t.Fatal(err)
	}
	inst := components.Detect(components.Llama, runtime.GOOS, "cpu", components.OverrideRoot(config.DataDir()), components.BundledRoots())
	if !inst.Override || inst.Version != "b11456" {
		t.Fatalf("installed = %+v; want the downloaded b11456", inst)
	}
	if st := a.UpdateStatus(); st.State != updater.StateDone {
		t.Errorf("status = %+v", st)
	}
	// And llama-server now resolves to the downloaded copy.
	if f := llama.EngineFlavor("", "cpu"); f != "cpu" {
		t.Errorf("EngineFlavor = %q", f)
	}
	info, _ := a.CheckForUpdate(context.Background(), true)
	if c := findComp(info, "llama"); c == nil || c.Available || c.Reason != "up_to_date" || c.Installed != "b11456" {
		t.Errorf("after the update: %+v", c)
	}
}

func TestRunComponentUpdate_BinaryThatIsNotWhatItClaimsIsRolledBack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as the engine")
	}
	cleanOverride(t)
	engineTree(t, "b9441")
	s := newSite(t)
	// The marker says b11456 but the binary reports another build.
	publishLlama(t, s, engineArchive(t, "10000", "b11456"), "b11456")
	a := compApp(t, s, "V4.6.0")

	err := a.runComponentUpdate(context.Background(), components.Llama)
	if err == nil {
		t.Fatal("an engine whose binary disagrees with its marker was accepted")
	}
	if _, statErr := os.Stat(filepath.Join(components.OverrideRoot(config.DataDir()), "binaries", runtime.GOOS, "cpu")); statErr == nil {
		t.Error("the rejected engine was left in the override tree")
	}
	inst := components.Detect(components.Llama, runtime.GOOS, "cpu", components.OverrideRoot(config.DataDir()), components.BundledRoots())
	if inst.Override || inst.Version != "b9441" {
		t.Errorf("after a rejected update the bundled engine must be in use: %+v", inst)
	}
}

func TestRunComponentUpdate_TamperedDownloadChangesNothing(t *testing.T) {
	cleanOverride(t)
	engineTree(t, "b9441")
	s := newSite(t)
	good := engineArchive(t, "11456", "b11456")
	publishLlama(t, s, good, "b11456")
	// The bytes served are not the bytes the (signed) manifest describes.
	evil := engineArchive(t, "11456", "b11456")
	b, _ := os.ReadFile(evil)
	b[len(b)/2] ^= 0xff
	_ = os.WriteFile(good, b, 0o644)

	a := compApp(t, s, "V4.6.0")
	err := a.runComponentUpdate(context.Background(), components.Llama)
	if !errors.Is(err, updater.ErrChecksum) {
		t.Fatalf("err = %v, want ErrChecksum", err)
	}
	if _, statErr := os.Stat(components.OverrideRoot(config.DataDir()) + "/binaries"); statErr == nil {
		t.Error("something was installed from a download that failed verification")
	}
}

// ── application update: everything up to the hand-over ──────────────────────

func releaseArchive(t *testing.T, withUpdater bool, updaterScript string) string {
	files := map[string]fileSpec{
		"Memo/memo-backend": {"backend", 0o755},
		"Memo/run_memo.sh":  {"#!/bin/sh\n", 0o755},
		"Memo/lib/x.so":     {"so", 0o644},
	}
	if withUpdater {
		files["Memo/memo-updater"] = fileSpec{updaterScript, 0o755}
	}
	return tarGz(t, files)
}

func autoApp(t *testing.T, s *site, install string) *App {
	t.Helper()
	f := facts(runtime.GOOS, runtime.GOARCH, filepath.Join(install, "memo-backend"), nil,
		filepath.Join(install, "memo-backend"), filepath.Join(install, "run_memo.sh"))
	a := updApp(t, s, "V4.6.0", &f)
	return a
}

func publishApp(t *testing.T, s *site, archive string) {
	sum, size := sumAndSize(t, archive)
	url := s.serveFile("/memo.tar.gz", archive)
	s.beacon("V4.7.0", nil)
	s.appManifest("V4.7.0", runtime.GOOS+"-"+runtime.GOARCH, func(m map[string]any) {
		m["url"], m["sha256"], m["size"] = url, sum, size
	})
}

func installDir(t *testing.T) string {
	d := t.TempDir()
	for _, f := range []string{"memo-backend", "run_memo.sh"} {
		if err := os.WriteFile(filepath.Join(d, f), []byte("old"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func drainShutdown() {
	select {
	case <-shutdown.Requested():
	default:
	}
}

func TestRunArchiveUpdate_StagesTheReleaseAndHandsOverToItsUpdater(t *testing.T) {
	if runtime.GOOS == "windows" || (runtime.GOOS+"-"+runtime.GOARCH != "linux-amd64" && runtime.GOOS+"-"+runtime.GOARCH != "darwin-arm64" && runtime.GOOS+"-"+runtime.GOARCH != "darwin-amd64") {
		t.Skip("script-install flow")
	}
	drainShutdown()
	install := installDir(t)
	argsFile := filepath.Join(t.TempDir(), "args.txt")
	// The "new release's updater" just records how it was called.
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsFile + "\n"
	s := newSite(t)
	publishApp(t, s, releaseArchive(t, true, script))
	a := autoApp(t, s, install)

	info, _ := a.CheckForUpdate(context.Background(), true)
	if err := a.runArchiveUpdate(context.Background(), info); err != nil {
		t.Fatal(err)
	}

	// The updater was started with everything it needs.
	var got []string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(argsFile); err == nil && len(b) > 0 {
			got = strings.Split(strings.TrimSpace(string(b)), "\n")
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(got) == 0 {
		t.Fatal("the new release's updater was never started")
	}
	flag := map[string]string{}
	for i := 1; i+1 < len(got); i += 2 {
		flag[got[i]] = got[i+1]
	}
	if got[0] != "apply" || flag["--install"] != install || flag["--expect"] != "V4.7.0" ||
		flag["--launch"] != filepath.Join(install, "run_memo.sh") || flag["--wait-pid"] != itoa(os.Getpid()) ||
		!strings.HasPrefix(flag["--health"], "http://127.0.0.1:") || !strings.HasSuffix(flag["--health"], "/api/version") ||
		flag["--work"] != filepath.Join(install, ".update") ||
		flag["--staged"] != filepath.Join(install, ".update", "staging", "Memo") ||
		!strings.HasPrefix(flag["--backup"], filepath.Join(install, ".update")) {
		t.Fatalf("updater args = %v", got)
	}
	// The release really is unpacked where --staged says, and the downloaded archive is gone.
	if b, _ := os.ReadFile(filepath.Join(flag["--staged"], "lib", "x.so")); string(b) != "so" {
		t.Error("the release was not unpacked")
	}
	if entries, _ := os.ReadDir(filepath.Join(install, ".update", "download")); len(entries) != 0 {
		t.Errorf("the downloaded archive was kept (%d files)", len(entries))
	}
	// The installed app is untouched until the updater runs.
	if b, _ := os.ReadFile(filepath.Join(install, "memo-backend")); string(b) != "old" {
		t.Error("the install was modified before the hand-over")
	}
	if st := a.UpdateStatus(); st.State != updater.StateRestarting {
		t.Errorf("status = %+v", st)
	}
	// And the backend asks to exit shortly after.
	select {
	case <-shutdown.Requested():
	case <-time.After(restartGrace + 3*time.Second):
		t.Error("the backend never asked to shut down for the update")
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestRunArchiveUpdate_RefusalsLeaveTheInstallAndCleanUp(t *testing.T) {
	plat := runtime.GOOS + "-" + runtime.GOARCH
	if plat != "linux-amd64" && plat != "darwin-arm64" && plat != "darwin-amd64" {
		t.Skip("script-install flow")
	}
	drainShutdown()

	t.Run("release without an updater", func(t *testing.T) {
		install := installDir(t)
		s := newSite(t)
		publishApp(t, s, releaseArchive(t, false, ""))
		a := autoApp(t, s, install)
		info, _ := a.CheckForUpdate(context.Background(), true)
		if err := a.runArchiveUpdate(context.Background(), info); err == nil || !strings.Contains(err.Error(), "incomplete") {
			t.Fatalf("err = %v", err)
		}
		if _, err := os.Stat(filepath.Join(install, ".update")); err == nil {
			t.Error("the scratch directory was left behind")
		}
	})

	t.Run("download that fails verification", func(t *testing.T) {
		install := installDir(t)
		s := newSite(t)
		archive := releaseArchive(t, true, "#!/bin/sh\n")
		publishApp(t, s, archive)
		b, _ := os.ReadFile(archive)
		b[len(b)/2] ^= 0xff
		_ = os.WriteFile(archive, b, 0o644)
		a := autoApp(t, s, install)
		info, _ := a.CheckForUpdate(context.Background(), true)
		if err := a.runArchiveUpdate(context.Background(), info); !errors.Is(err, updater.ErrChecksum) {
			t.Fatalf("err = %v, want ErrChecksum", err)
		}
		if b, _ := os.ReadFile(filepath.Join(install, "memo-backend")); string(b) != "old" {
			t.Error("install modified")
		}
		if _, err := os.Stat(filepath.Join(install, ".update")); err == nil {
			t.Error("the scratch directory was left behind")
		}
	})

	t.Run("a previous failed update's backup is never deleted", func(t *testing.T) {
		install := installDir(t)
		backup := filepath.Join(install, ".update", "backup")
		if err := os.MkdirAll(backup, 0o755); err != nil {
			t.Fatal(err)
		}
		precious := filepath.Join(backup, "memo-backend")
		_ = os.WriteFile(precious, []byte("the only good copy"), 0o755)
		s := newSite(t)
		publishApp(t, s, releaseArchive(t, true, "#!/bin/sh\n"))
		a := autoApp(t, s, install)
		info, _ := a.CheckForUpdate(context.Background(), true)
		if err := a.runArchiveUpdate(context.Background(), info); err == nil {
			t.Fatal("proceeded although an unfinished update's backup exists")
		}
		if b, _ := os.ReadFile(precious); string(b) != "the only good copy" {
			t.Fatal("the backup of a previous update was destroyed")
		}
	})
}

func TestRunAppUpdate_ManualInstallCannotUpdateItself(t *testing.T) {
	s := newSite(t)
	s.beacon("V4.7.0", nil)
	f := facts("linux", "amd64", "/tmp/.mount_MemoX/usr/bin/memo-backend", nil)
	a := updApp(t, s, "V4.6.0", &f)
	if err := a.runAppUpdate(context.Background()); err == nil || !strings.Contains(err.Error(), "memocpp.com") {
		t.Fatalf("err = %v; want a pointer to the download page", err)
	}
}

func TestRunAppUpdate_NothingToDo(t *testing.T) {
	s := newSite(t)
	s.beacon("V4.6.0", nil)
	if err := updApp(t, s, "V4.6.0", nil).runAppUpdate(context.Background()); err == nil {
		t.Fatal("reported success with nothing to install")
	}
}

func TestWithLibraryPath(t *testing.T) {
	env := withLibraryPath([]string{"A=1", "LD_LIBRARY_PATH=/old", "PATH=/bin"}, "/engine")
	joined := strings.Join(env, "\n")
	switch runtime.GOOS {
	case "linux":
		if !strings.Contains(joined, "LD_LIBRARY_PATH=/engine:/old") {
			t.Errorf("env = %v", env)
		}
	}
	if len(withLibraryPath(nil, "/engine")) != 1 {
		t.Error("an empty environment must still get the library path")
	}
}
