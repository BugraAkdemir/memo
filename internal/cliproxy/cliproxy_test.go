// SPDX-License-Identifier: AGPL-3.0-or-later

package cliproxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var fakeBin string // path of the compiled fakecpa

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "fakecpa")
	if err != nil {
		panic(err)
	}
	fakeBin = filepath.Join(dir, binaryFile(runtime.GOOS, runtime.GOARCH))
	build := exec.Command("go", "build", "-o", fakeBin, "./testdata/fakecpa")
	if out, err := build.CombinedOutput(); err != nil {
		os.Stderr.WriteString("building fakecpa: " + err.Error() + "\n" + string(out))
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// bundle lays a binaries/<os>/cliproxy tree under a fresh root, with a correct
// SHA256 file, and returns the root.
func bundle(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "binaries", runtime.GOOS, "cliproxy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := binaryFile(runtime.GOOS, runtime.GOARCH)
	b, err := os.ReadFile(fakeBin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	os.WriteFile(filepath.Join(dir, "SHA256"), []byte(hex.EncodeToString(sum[:])+"  "+name+"\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "VERSION"), []byte("v0.0.0-test\n"), 0o644)
	return root
}

// newMgr returns a Manager that finds the fake in a fresh bundle and keeps its
// state under a temp dir. It is stopped when the test ends.
func newMgr(t *testing.T) *Manager {
	t.Helper()
	noBrowser(t)
	m := New(filepath.Join(t.TempDir(), "cliproxy"))
	m.roots = []string{bundle(t)}
	t.Cleanup(m.Stop)
	return m
}

var (
	openedMu   sync.Mutex
	openedList []string
)

// noBrowser keeps login tests from launching the developer's real browser and
// records what would have been opened (see openedURLs). Safe to call repeatedly.
func noBrowser(t *testing.T) {
	t.Helper()
	openedMu.Lock()
	openedList = nil
	openedMu.Unlock()
	prev := openURL
	openURL = func(u string) error {
		openedMu.Lock()
		openedList = append(openedList, u)
		openedMu.Unlock()
		return nil
	}
	t.Cleanup(func() { openURL = prev })
}

func openedURLs() []string {
	openedMu.Lock()
	defer openedMu.Unlock()
	return append([]string(nil), openedList...)
}

func fast(t *testing.T) {
	t.Helper()
	noBrowser(t)
	pb, pr, ph := restartBackoff, healthTimeout, loginURLTimeout
	restartBackoff, healthTimeout, loginURLTimeout = 150*time.Millisecond, 15*time.Second, 8*time.Second
	t.Cleanup(func() { restartBackoff, healthTimeout, loginURLTimeout = pb, pr, ph })
}

func eventually(t *testing.T, what string, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(40 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// ── locating and verifying the bundled binary ───────────────────────────────

func TestBinary_VerifiedAndFound(t *testing.T) {
	m := New(t.TempDir())
	m.roots = []string{bundle(t)}
	p, ver, err := m.Binary()
	if err != nil {
		t.Fatalf("Binary: %v", err)
	}
	if !strings.HasSuffix(p, binaryFile(runtime.GOOS, runtime.GOARCH)) || ver != "v0.0.0-test" {
		t.Errorf("got %q %q", p, ver)
	}
}

func TestBinary_NotBundled(t *testing.T) {
	m := New(t.TempDir())
	m.roots = []string{t.TempDir()}
	if _, _, err := m.Binary(); !errors.Is(err, ErrNotBundled) {
		t.Errorf("err = %v, want ErrNotBundled", err)
	}
}

func TestBinary_RefusesTamperedAndUnchecked(t *testing.T) {
	m := New(t.TempDir())
	root := bundle(t)
	m.roots = []string{root}
	dir := filepath.Join(root, "binaries", runtime.GOOS, "cliproxy")
	name := binaryFile(runtime.GOOS, runtime.GOARCH)

	// Tampered: bytes no longer match the shipped checksum.
	f, _ := os.OpenFile(filepath.Join(dir, name), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("evil")
	f.Close()
	if _, _, err := m.Binary(); !errors.Is(err, ErrUnverified) {
		t.Errorf("tampered binary: err = %v, want ErrUnverified", err)
	}

	// No checksum file at all.
	os.Remove(filepath.Join(dir, "SHA256"))
	m2 := New(t.TempDir())
	m2.roots = []string{root}
	if _, _, err := m2.Binary(); !errors.Is(err, ErrUnverified) {
		t.Errorf("no SHA256: err = %v, want ErrUnverified", err)
	}
}

func TestBinary_FindsTreeInParentOfExeDir(t *testing.T) {
	// ~/.memo/bin/memo lives one level below the tree the installer copied.
	root := bundle(t)
	m := New(t.TempDir())
	m.roots = []string{filepath.Join(root, "bin"), root}
	if _, _, err := m.Binary(); err != nil {
		t.Errorf("parent root not searched: %v", err)
	}
}

func TestBinaryFile_PerPlatform(t *testing.T) {
	cases := []struct{ goos, arch, want string }{
		{"linux", "amd64", "cli-proxy-api"},
		{"linux", "arm64", "cli-proxy-api"},
		{"windows", "amd64", "cli-proxy-api.exe"},
		{"darwin", "arm64", "cli-proxy-api-arm64"},
		{"darwin", "amd64", "cli-proxy-api-x64"},
	}
	for _, c := range cases {
		if got := binaryFile(c.goos, c.arch); got != c.want {
			t.Errorf("%s/%s = %q, want %q", c.goos, c.arch, got, c.want)
		}
	}
}

func TestPinnedFileMatchesTheLayoutWeShip(t *testing.T) {
	// PINNED.txt is the single source of truth the vendor script and CI read;
	// every row must name a dest/file this package knows how to locate.
	b, err := os.ReadFile("PINNED.txt")
	if err != nil {
		t.Fatal(err)
	}
	rows := 0
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) != 6 || f[0] != "asset" {
			continue
		}
		rows++
		if len(f[2]) != 64 || len(f[5]) != 64 {
			t.Errorf("%s: archive sha %q / binary sha %q are not 64 hex chars", f[1], f[2], f[5])
		}
		osName, sub, _ := strings.Cut(f[3], "/")
		if !strings.HasPrefix(sub, "cliproxy") {
			t.Errorf("%s: dest %q is not under <os>/cliproxy*", f[1], f[3])
		}
		arch := "amd64"
		if strings.Contains(f[1], "aarch64") || strings.Contains(f[4], "arm64") {
			arch = "arm64"
		}
		if got := binaryFile(osName, arch); got != f[4] {
			t.Errorf("%s: file %q but Binary() would look for %q", f[1], f[4], got)
		}
	}
	if rows != 5 {
		t.Errorf("PINNED.txt has %d asset rows, want 5 (linux x2, windows, darwin x2)", rows)
	}
}

// ── running it ──────────────────────────────────────────────────────────────

func httpGet(t *testing.T, url, key string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var b strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		b.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return resp.StatusCode, b.String()
}

func TestStart_LoopbackKeyGatedAndStops(t *testing.T) {
	fast(t)
	m := newMgr(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !m.Running() {
		t.Fatal("not running after Start")
	}
	base, key := m.BaseURL(), m.APIKey()
	if !strings.HasPrefix(base, "http://127.0.0.1:") || !strings.HasSuffix(base, "/v1") || key == "" {
		t.Fatalf("base=%q key=%q", base, key)
	}
	if code, _ := httpGet(t, base+"/models", ""); code != http.StatusUnauthorized {
		t.Errorf("no key: status %d, want 401", code)
	}
	if code, _ := httpGet(t, base+"/models", key); code != http.StatusOK {
		t.Errorf("with key: status %d, want 200", code)
	}

	// The config must be loopback-only, key-gated, management off.
	cfg, _ := os.ReadFile(m.configPath())
	for _, want := range []string{`host: "127.0.0.1"`, `secret-key: ""`, "disable-control-panel: true", "api-keys:"} {
		if !strings.Contains(string(cfg), want) {
			t.Errorf("config lacks %q:\n%s", want, cfg)
		}
	}
	if st, err := os.Stat(m.configPath()); err == nil && runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Errorf("config mode %v, want 0600 (it holds the client key)", st.Mode().Perm())
	}

	// Start is idempotent.
	if err := m.Start(context.Background()); err != nil {
		t.Errorf("second Start: %v", err)
	}

	m.Stop()
	if m.Running() {
		t.Error("still running after Stop")
	}
	eventually(t, "port released", 5*time.Second, func() bool { return portFree(m.st.Port) })
}

func TestStart_KeyAndPortSurviveARestartOfMemo(t *testing.T) {
	fast(t)
	dir := filepath.Join(t.TempDir(), "cliproxy")
	root := bundle(t)

	a := New(dir)
	a.roots = []string{root}
	if err := a.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	key, base := a.APIKey(), a.BaseURL()
	a.Stop()
	eventually(t, "port released", 5*time.Second, func() bool { return portFree(a.st.Port) })

	b := New(dir)
	b.roots = []string{root}
	t.Cleanup(b.Stop)
	if err := b.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if b.APIKey() != key || b.BaseURL() != base {
		t.Errorf("identity changed across restart: %q/%q -> %q/%q", base, key, b.BaseURL(), b.APIKey())
	}
}

func TestStart_RemembersAPortThatSomethingElseTook(t *testing.T) {
	fast(t)
	dir := filepath.Join(t.TempDir(), "cliproxy")
	root := bundle(t)

	a := New(dir)
	a.roots = []string{root}
	a.mu.Lock()
	a.ensureIdentityLocked()
	old := a.st.Port
	a.mu.Unlock()

	l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(old)) // squat on it
	if err != nil {
		t.Skip("could not squat on the port: ", err)
	}
	defer l.Close()

	b := New(dir)
	b.roots = []string{root}
	t.Cleanup(b.Stop)
	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start with a taken saved port: %v", err)
	}
	if b.st.Port == old {
		t.Error("kept a port that was already taken")
	}
}

func TestStart_NotBundledIsAClearError(t *testing.T) {
	m := New(t.TempDir())
	m.roots = []string{t.TempDir()}
	if err := m.Start(context.Background()); !errors.Is(err, ErrNotBundled) {
		t.Errorf("err = %v, want ErrNotBundled", err)
	}
}

func TestMonitor_RestartsAfterACrash(t *testing.T) {
	fast(t)
	t.Setenv("FAKECPA_CRASH_ONCE_MS", "500")
	m := newMgr(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	first := m.proc
	eventually(t, "the first process to die", 5*time.Second, func() bool { return !first.alive() })
	eventually(t, "a replacement to answer", 10*time.Second, func() bool {
		return m.Running() && m.ping(context.Background()) == nil
	})
}

func TestStop_IsNotFollowedByARestart(t *testing.T) {
	fast(t)
	m := newMgr(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	m.Stop()
	time.Sleep(3 * restartBackoff)
	if m.Running() {
		t.Error("monitor resurrected an intentionally stopped sidecar")
	}
}

// ── login, accounts, models ─────────────────────────────────────────────────

func TestLogin_FlowAccountsModelsLogout(t *testing.T) {
	fast(t)
	m := newMgr(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, _ := m.Models(context.Background()); len(got) != 0 {
		t.Fatalf("models before login = %v, want none", got)
	}

	u, err := m.StartLogin(context.Background(), ProviderAntigravity)
	if err != nil {
		t.Fatalf("StartLogin: %v", err)
	}
	if !strings.HasPrefix(u, "https://example.test/oauth/authorize?") {
		t.Errorf("url = %q (must be the OAuth link, not the docs link)", u)
	}
	eventually(t, "login to finish", 8*time.Second, func() bool { return m.LoginStatus().Done })
	if st := m.LoginStatus(); st.Error != "" {
		t.Fatalf("login error: %s", st.Error)
	}

	accts := m.Accounts()
	if len(accts) != 1 || accts[0].Provider != ProviderAntigravity || accts[0].Email != "fake@example.com" || accts[0].Project != "fake-project" {
		t.Fatalf("accounts = %+v", accts)
	}
	// Identity only — a token must never leave this package.
	b, _ := json.Marshal(accts)
	if strings.Contains(string(b), "SECRET") {
		t.Errorf("account JSON leaks a token: %s", b)
	}

	// The running sidecar serves the new account's models.
	modelsTTL = 0
	t.Cleanup(func() { modelsTTL = 30 * time.Second })
	got, err := m.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].OwnedBy != "antigravity" || got[0].ID != "antigravity-model-b" {
		t.Errorf("models = %+v (want 2, antigravity-owned, descending by id)", got)
	}
	if len(m.CachedModels()) != 2 {
		t.Error("CachedModels empty after a fetch")
	}

	if err := m.Logout(ProviderAntigravity); err != nil {
		t.Fatal(err)
	}
	if len(m.Accounts()) != 0 || len(m.CachedModels()) != 0 {
		t.Error("account or cached models survived Logout")
	}
}

func TestLogin_FailureCarriesAMessageWithoutOAuthSecrets(t *testing.T) {
	fast(t)
	t.Setenv("FAKECPA_LOGIN_FAIL", "1")
	m := newMgr(t)
	if _, err := m.StartLogin(context.Background(), ProviderClaude); err != nil {
		t.Fatal(err)
	}
	eventually(t, "login to fail", 8*time.Second, func() bool { return m.LoginStatus().Done })
	st := m.LoginStatus()
	if st.Error == "" {
		t.Fatal("a failed login reported no error")
	}
	if strings.Contains(st.Error, "SECRET") {
		t.Errorf("error leaks OAuth query values: %s", st.Error)
	}
	if len(m.Accounts()) != 0 {
		t.Error("a failed login left an account")
	}
}

func TestLogin_ErrorsForUnknownProviderAndNoBinary(t *testing.T) {
	m := newMgr(t)
	if _, err := m.StartLogin(context.Background(), "nope"); !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("err = %v, want ErrUnknownProvider", err)
	}
	if err := m.Logout("nope"); !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("Logout err = %v, want ErrUnknownProvider", err)
	}
	empty := New(t.TempDir())
	empty.roots = []string{t.TempDir()}
	if _, err := empty.StartLogin(context.Background(), ProviderCodex); !errors.Is(err, ErrNotBundled) {
		t.Errorf("err = %v, want ErrNotBundled", err)
	}
}

func TestLogin_AllThreeProvidersUseTheirOwnFlag(t *testing.T) {
	fast(t)
	for _, p := range Providers {
		m := newMgr(t)
		if _, err := m.StartLogin(context.Background(), p); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		eventually(t, p+" login", 8*time.Second, func() bool { return m.LoginStatus().Done })
		a := m.Accounts()
		if len(a) != 1 || a[0].Provider != p {
			t.Errorf("%s: accounts = %+v", p, a)
		}
	}
}

func TestLoginURL_PicksTheOAuthLink(t *testing.T) {
	cases := map[string]string{
		"Visit: https://example.test/docs and then":                           "",
		"Attempting to open URL in browser: https://a.test/o?client_id=1&x=2": "https://a.test/o?client_id=1&x=2",
		"https://claude.ai/oauth/authorize?code=true":                         "https://claude.ai/oauth/authorize?code=true",
		"no url here": "",
	}
	for in, want := range cases {
		if got := loginURL(in); got != want {
			t.Errorf("loginURL(%q) = %q, want %q", in, got, want)
		}
	}
	if got := redactLine("see https://a.test/o?state=SECRET&c=1 ok"); strings.Contains(got, "SECRET") {
		t.Errorf("redactLine kept the query: %q", got)
	}
}

// R2 and zip extraction drop the executable bit; Binary() must make a
// VERIFIED binary runnable, and must not touch one that failed its checksum.
func TestBinary_MakesAVerifiedBinaryExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no executable bit on Windows")
	}
	root := bundle(t)
	dir := filepath.Join(root, "binaries", runtime.GOOS, "cliproxy")
	p := filepath.Join(dir, binaryFile(runtime.GOOS, runtime.GOARCH))
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(t.TempDir())
	m.roots = []string{root}
	if _, _, err := m.Binary(); err != nil {
		t.Fatalf("Binary: %v", err)
	}
	if st, _ := os.Stat(p); st.Mode().Perm()&0o111 != 0o111 {
		t.Errorf("mode %v after Binary(), want it made executable", st.Mode().Perm())
	}

	// A tampered one is refused and left exactly as it was.
	os.Chmod(p, 0o644)
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("evil")
	f.Close()
	m2 := New(t.TempDir())
	m2.roots = []string{root}
	if _, _, err := m2.Binary(); !errors.Is(err, ErrUnverified) {
		t.Fatalf("tampered: %v", err)
	}
	if st, _ := os.Stat(p); st.Mode().Perm()&0o111 != 0 {
		t.Errorf("a binary that FAILED its checksum was made executable (%v)", st.Mode().Perm())
	}
}

// A search root of "." (running from a checkout) must still yield a path that
// works after the child's working directory changes: Start runs the sidecar
// with cmd.Dir set to the data dir, so a relative path would fail to exec.
func TestStart_WorksWhenTheBundleIsFoundThroughARelativeRoot(t *testing.T) {
	fast(t)
	root := bundle(t)
	t.Chdir(root)

	m := New(filepath.Join(t.TempDir(), "cliproxy"))
	m.roots = []string{"."}
	t.Cleanup(m.Stop)

	p, _, err := m.Binary()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(p) {
		t.Fatalf("Binary() = %q, want an absolute path", p)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start through a relative root: %v", err)
	}
}

// A sidecar that has just started lists no models for a while. That empty answer
// must not be cached, or the real list would stay hidden for a full TTL.
func TestModels_AnEmptyAnswerIsNotCached(t *testing.T) {
	fast(t)
	t.Setenv("FAKECPA_MODELS_DELAY_MS", "700")
	m := newMgr(t)
	// A credential already on disk, as after a restart of Memo.
	if err := os.MkdirAll(m.authDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(m.authDir(), "antigravity-x.json"), []byte(`{"type":"antigravity","email":"x@example.com"}`), 0o600)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	first, err := m.Models(context.Background())
	if err != nil || len(first) != 0 {
		t.Fatalf("early Models = %v, %v; want an empty list (the sidecar is still loading)", first, err)
	}
	eventually(t, "the real list to show through (despite the default 30s TTL)", 5*time.Second, func() bool {
		l, err := m.Models(context.Background())
		return err == nil && len(l) == 2
	})
}

// Memo's default data dir is the RELATIVE "data". The sidecar runs with its
// working directory set to the manager's dir, so a relative dir made the
// config path resolve twice ("data/cliproxy/data/cliproxy/config.yaml"): the
// login died with "failed to read config file" before printing any URL, and
// the UI just kept saying "waiting for the browser". Only the real app (not a
// test with an absolute TempDir) ever ran with a relative dir.
func TestLogin_WorksWithARelativeDataDir(t *testing.T) {
	fast(t)
	t.Chdir(t.TempDir())
	m := New(filepath.Join("data", "cliproxy"))
	m.roots = []string{bundle(t)}
	t.Cleanup(m.Stop)

	if !filepath.IsAbs(m.dir) {
		t.Fatalf("manager dir %q must be absolute", m.dir)
	}
	u, err := m.StartLogin(context.Background(), ProviderAntigravity)
	if err != nil {
		t.Fatalf("StartLogin with a relative data dir: %v", err)
	}
	if !strings.HasPrefix(u, "https://example.test/oauth/authorize?") {
		t.Errorf("url = %q", u)
	}
	eventually(t, "login to finish", 8*time.Second, func() bool { return m.LoginStatus().Done })
	if len(m.Accounts()) != 1 {
		t.Errorf("accounts = %v, want the one the login wrote", m.Accounts())
	}
}

// On KDE the sidecar's own browser probe (`xdg-open about:blank`) never returns,
// so it never printed the sign-in URL and the UI waited on a page that was never
// opened. The login process must get a PATH whose xdg-open returns at once, and
// Memo must open the printed URL itself.
func TestLogin_ABrowserProbeThatHangsDoesNotBlockTheURL(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the xdg-open shim is Linux-only")
	}
	fast(t)

	hang := t.TempDir()
	if err := os.WriteFile(filepath.Join(hang, "xdg-open"), []byte("#!/bin/sh\nsleep 600\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", hang+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKECPA_PROBE_BROWSER", "1")

	m := newMgr(t)
	u, err := m.StartLogin(context.Background(), ProviderAntigravity)
	if err != nil {
		t.Fatalf("StartLogin with a hanging xdg-open on PATH: %v", err)
	}
	eventually(t, "Memo to open the sign-in page itself", 3*time.Second, func() bool { return len(openedURLs()) == 1 })
	if got := openedURLs()[0]; got != u {
		t.Errorf("opened %q, want the sign-in URL %q", got, u)
	}
}
