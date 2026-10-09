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
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

// ── per-model quota ─────────────────────────────────────────────────────────

func quotaFixture(t *testing.T, status int, body string) (*Manager, *int) {
	t.Helper()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("Authorization") != "Bearer SECRET-TOKEN" {
			http.Error(w, "bad token", http.StatusUnauthorized)
			return
		}
		var req map[string]string
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["project"] != "proj-1" {
			http.Error(w, "bad project", http.StatusBadRequest)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	prev := antigravityQuotaURL
	antigravityQuotaURL = srv.URL
	t.Cleanup(func() { antigravityQuotaURL = prev })

	m := newMgr(t)
	if err := os.MkdirAll(m.authDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	cred := `{"type":"antigravity","email":"a@b.c","project_id":"proj-1","access_token":"SECRET-TOKEN"}`
	if err := os.WriteFile(filepath.Join(m.authDir(), "antigravity-a@b.c.json"), []byte(cred), 0o600); err != nil {
		t.Fatal(err)
	}
	return m, &hits
}

func TestQuotas_ReadsRemainingFractionPerModelAndCaches(t *testing.T) {
	m, hits := quotaFixture(t, 200, `{"models":{
		"claude-sonnet-4-6":{"quotaInfo":{"remainingFraction":0.25,"resetTime":"2026-10-13T20:54:16Z"}},
		"gemini-3-flash":{"quotaInfo":{"remainingFraction":1}},
		"used-up":{"quotaInfo":{"resetTime":"2026-10-08T00:00:00Z"}},
		"no-quota-info":{"displayName":"x"}}}`)

	q, err := m.Quotas(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := q["claude-sonnet-4-6"]; got.Remaining != 0.25 || got.ResetAt != "2026-10-13T20:54:16Z" {
		t.Errorf("claude-sonnet-4-6 = %+v", got)
	}
	if got := q["gemini-3-flash"]; got.Remaining != 1 {
		t.Errorf("gemini-3-flash = %+v", got)
	}
	if got, ok := q["used-up"]; !ok || got.Remaining != 0 {
		t.Errorf("a model with only a reset time is used up (0 left), got %+v ok=%v", got, ok)
	}
	if _, ok := q["no-quota-info"]; ok {
		t.Error("a model without quotaInfo must have no entry")
	}
	if _, err := m.Quotas(context.Background()); err != nil || *hits != 1 {
		t.Errorf("second call must come from the cache: err=%v hits=%d", err, *hits)
	}
	m.invalidateModels()
	if _, err := m.Quotas(context.Background()); err != nil || *hits != 2 {
		t.Errorf("a model-cache reset (sign-in/out) must refetch: err=%v hits=%d", err, *hits)
	}
}

func TestQuotas_FailureIsUnavailableNotFatalAndNeverLeaksTheToken(t *testing.T) {
	m, hits := quotaFixture(t, 500, `boom SECRET-TOKEN`)
	_, err := m.Quotas(context.Background())
	if !errors.Is(err, ErrQuotaUnavailable) {
		t.Fatalf("err = %v, want ErrQuotaUnavailable", err)
	}
	if strings.Contains(err.Error(), "SECRET-TOKEN") {
		t.Errorf("error leaks the token: %v", err)
	}
	if _, err := m.Quotas(context.Background()); err == nil || *hits != 1 {
		t.Errorf("a failure must be remembered briefly instead of hammering the vendor: hits=%d", *hits)
	}
}

func TestQuotas_NoAntigravityAccountMeansNoQuota(t *testing.T) {
	m := newMgr(t)
	if _, err := m.Quotas(context.Background()); !errors.Is(err, ErrQuotaUnavailable) {
		t.Fatalf("err = %v, want ErrQuotaUnavailable", err)
	}
}

// ── account-wide quota (Codex, Claude) and the non-blocking snapshot ─────────

func TestParseCodexUsage_PicksTheMostLimitingWindow(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	// Shape measured live on 2026-10-07 (field names only; values are made up).
	body := []byte(`{"plan_type":"plus","rate_limit":{"allowed":true,"limit_reached":false,
		"primary_window":{"limit_window_seconds":18000,"reset_after_seconds":3600,"reset_at":0,"used_percent":40},
		"secondary_window":{"limit_window_seconds":604800,"reset_after_seconds":86400,"reset_at":1792000000,"used_percent":85}}}`)
	q, ok := parseCodexUsage(body, now)
	if !ok {
		t.Fatal("not parsed")
	}
	if q.Remaining < 0.1499 || q.Remaining > 0.1501 {
		t.Errorf("Remaining = %v, want 0.15 (the weekly window is the tighter one)", q.Remaining)
	}
	if q.Window != "7d" {
		t.Errorf("Window = %q, want 7d", q.Window)
	}
	if q.ResetAt != time.Unix(1792000000, 0).UTC().Format(time.RFC3339) {
		t.Errorf("ResetAt = %q, want the window's own reset_at", q.ResetAt)
	}

	// reset_after_seconds alone is enough, and a single window works.
	q, ok = parseCodexUsage([]byte(`{"rate_limit":{"primary_window":{"limit_window_seconds":18000,"reset_after_seconds":600,"used_percent":10}}}`), now)
	if !ok || q.Window != "5h" || q.ResetAt != now.Add(10*time.Minute).Format(time.RFC3339) {
		t.Errorf("single window: %+v ok=%v", q, ok)
	}
	// The websocket spelling (primary/secondary, window_minutes) is accepted too.
	q, ok = parseCodexUsage([]byte(`{"rate_limits":{"primary":{"used_percent":25,"window_minutes":300,"reset_after_seconds":60}}}`), now)
	if !ok || q.Remaining != 0.75 || q.Window != "5h" {
		t.Errorf("websocket spelling: %+v ok=%v", q, ok)
	}
}

func TestParseCodexUsage_RejectsWhatItCannotRead(t *testing.T) {
	for _, body := range []string{``, `not json`, `{}`, `{"rate_limit":{"primary_window":null,"secondary_window":null}}`,
		`{"rate_limit":{"primary_window":{"used_percent":140}}}`, `{"rate_limit":{"primary_window":{"used_percent":"lots"}}}`} {
		if q, ok := parseCodexUsage([]byte(body), time.Now()); ok {
			t.Errorf("parseCodexUsage(%q) = %+v, want not ok", body, q)
		}
	}
}

func TestParseClaudeUsage(t *testing.T) {
	q, ok := parseClaudeUsage([]byte(`{"five_hour":{"utilization":30.5,"resets_at":"2026-10-07T15:00:00Z"},
		"seven_day":{"utilization":60,"resets_at":"2026-10-12T00:00:00Z"},"seven_day_opus":null}`), time.Now())
	if !ok || q.Window != "7d" || q.Remaining != 0.4 || q.ResetAt != "2026-10-12T00:00:00Z" {
		t.Errorf("got %+v ok=%v, want the 7d window at 40%% left", q, ok)
	}
	if _, ok := parseClaudeUsage([]byte(`{"five_hour":null}`), time.Now()); ok {
		t.Error("a response with no window must not parse")
	}
	if _, ok := parseClaudeUsage([]byte(`nope`), time.Now()); ok {
		t.Error("garbage must not parse")
	}
}

func TestQuotaSet_ForPrefersTheModelsOwnFigureThenItsVendors(t *testing.T) {
	s := QuotaSet{
		Models:  map[string]Quota{"gemini-3-flash": {Remaining: 0.9}},
		Vendors: map[string]Quota{"openai": {Remaining: 0.2}},
	}
	if q, ok := s.For("gemini-3-flash", "antigravity"); !ok || q.Remaining != 0.9 {
		t.Errorf("per-model: %+v %v", q, ok)
	}
	if q, ok := s.For("gpt-5.5", "openai"); !ok || q.Remaining != 0.2 {
		t.Errorf("vendor-wide: %+v %v", q, ok)
	}
	if _, ok := s.For("claude-sonnet-4-6", "antigravity"); ok {
		t.Error("a model with neither figure must have none")
	}
}

// vendorQuotaFixture signs in a Codex and a Claude account against local servers
// and counts how often each vendor is asked.
func vendorQuotaFixture(t *testing.T, delay time.Duration) (m *Manager, codexHits, claudeHits *atomic.Int32) {
	t.Helper()
	codexHits, claudeHits = new(atomic.Int32), new(atomic.Int32)
	codex := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		codexHits.Add(1)
		time.Sleep(delay)
		if r.Header.Get("Authorization") != "Bearer CODEX-TOKEN" || r.Header.Get("chatgpt-account-id") != "acct-1" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"rate_limit":{"primary_window":{"limit_window_seconds":18000,"reset_after_seconds":120,"used_percent":50}}}`))
	}))
	claude := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claudeHits.Add(1)
		if r.Header.Get("Authorization") != "Bearer CLAUDE-TOKEN" || r.Header.Get("anthropic-beta") != "oauth-2025-04-20" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":10,"resets_at":"2026-10-07T15:00:00Z"}}`))
	}))
	t.Cleanup(codex.Close)
	t.Cleanup(claude.Close)
	pc, pl := codexUsageURL, claudeUsageURL
	codexUsageURL, claudeUsageURL = codex.URL, claude.URL
	t.Cleanup(func() { codexUsageURL, claudeUsageURL = pc, pl })

	m = newMgr(t)
	// Cleanups run last-in-first-out: this one runs BEFORE the URLs are restored,
	// so a background refresh still in flight never reads them mid-restore.
	t.Cleanup(func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			m.snap.mu.Lock()
			idle := m.snap.busy == nil
			m.snap.mu.Unlock()
			if idle {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
	if err := os.MkdirAll(m.authDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(m.authDir(), name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("codex-a@b.c.json", `{"type":"codex","email":"a@b.c","access_token":"CODEX-TOKEN","account_id":"acct-1"}`)
	write("claude-a@b.c.json", `{"type":"claude","email":"a@b.c","access_token":"CLAUDE-TOKEN"}`)
	return m, codexHits, claudeHits
}

func TestQuotaSnapshot_ReadsCodexAndClaudeAccountsAndCaches(t *testing.T) {
	m, codexHits, claudeHits := vendorQuotaFixture(t, 0)
	set := m.QuotaSnapshot(context.Background(), 3*time.Second)
	if q := set.Vendors["openai"]; q.Remaining != 0.5 || q.Window != "5h" || q.ResetAt == "" {
		t.Errorf("openai = %+v", q)
	}
	if q := set.Vendors["anthropic"]; q.Remaining != 0.9 || q.Window != "5h" {
		t.Errorf("anthropic = %+v", q)
	}
	for i := 0; i < 5; i++ {
		m.QuotaSnapshot(context.Background(), 0)
	}
	if codexHits.Load() != 1 || claudeHits.Load() != 1 {
		t.Errorf("vendors were asked %d/%d times for 6 snapshots, want 1/1 (cached)", codexHits.Load(), claudeHits.Load())
	}
}

// The reason this exists: opening the model picker used to wait on the vendor.
func TestQuotaSnapshot_NeverHoldsTheCallerWhenAskedNotTo(t *testing.T) {
	m, _, _ := vendorQuotaFixture(t, 400*time.Millisecond)
	start := time.Now()
	set := m.QuotaSnapshot(context.Background(), 0)
	if took := time.Since(start); took > 150*time.Millisecond {
		t.Errorf("a snapshot with maxWait 0 took %v — it waited on the vendor", took)
	}
	if len(set.Vendors) != 0 {
		t.Errorf("nothing is cached yet, got %+v", set)
	}
	// The refresh it started lands in the background; the next call has the figures.
	eventually(t, "the background refresh to land", 3*time.Second, func() bool {
		return len(m.QuotaSnapshot(context.Background(), 0).Vendors) > 0
	})
}

func TestQuotaSnapshot_WaitsOnlyAsLongAsAskedForAFreshFigure(t *testing.T) {
	m, _, _ := vendorQuotaFixture(t, 1500*time.Millisecond)
	start := time.Now()
	m.QuotaSnapshot(context.Background(), 200*time.Millisecond)
	if took := time.Since(start); took < 150*time.Millisecond || took > 900*time.Millisecond {
		t.Errorf("waited %v, want about the 200ms asked for", took)
	}
}

func TestQuotaSnapshot_KeepsTheLastFiguresWhenARefreshFails(t *testing.T) {
	m, codexHits, _ := vendorQuotaFixture(t, 0)
	old := quotaTTL
	quotaTTL = 50 * time.Millisecond
	t.Cleanup(func() { quotaTTL = old })

	if q := m.QuotaSnapshot(context.Background(), 2*time.Second).Vendors["openai"]; q.Remaining != 0.5 {
		t.Fatalf("first read: %+v", q)
	}
	// Codex now refuses (an expired token) while Claude keeps answering: the
	// Codex figure from a moment ago is better than nothing, and Claude's is
	// still refreshed.
	codexUsageURL = "http://127.0.0.1:1/" // nothing listens
	time.Sleep(80 * time.Millisecond)
	set := m.QuotaSnapshot(context.Background(), 2*time.Second)
	if q := set.Vendors["openai"]; q.Remaining != 0.5 {
		t.Errorf("after a failed refresh the previous Codex figure must stay, got %+v", q)
	}
	if q := set.Vendors["anthropic"]; q.Remaining != 0.9 {
		t.Errorf("one vendor failing must not drop the other: %+v", set.Vendors)
	}
	_ = codexHits
}

func TestQuotaSnapshot_NoVendorAccountMeansAnEmptyAnswerNotAnError(t *testing.T) {
	m := newMgr(t)
	set := m.QuotaSnapshot(context.Background(), time.Second)
	if len(set.Vendors) != 0 || len(set.Models) != 0 {
		t.Errorf("got %+v", set)
	}
}

// The context popover shows the 5-hour and the weekly meter side by side, so a
// Quota must keep every window, shortest first — not just the tightest one.
func TestParseCodexUsage_KeepsEveryWindowShortestFirst(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	body := []byte(`{"rate_limit":{
		"secondary_window":{"limit_window_seconds":604800,"reset_after_seconds":86400,"used_percent":85},
		"primary_window":{"limit_window_seconds":18000,"reset_after_seconds":3600,"used_percent":40}}}`)
	q, ok := parseCodexUsage(body, now)
	if !ok || len(q.Windows) != 2 {
		t.Fatalf("got %+v ok=%v, want two windows", q, ok)
	}
	if q.Windows[0].Label != "5h" || q.Windows[1].Label != "7d" {
		t.Errorf("order = %q,%q, want 5h then 7d regardless of map order", q.Windows[0].Label, q.Windows[1].Label)
	}
	if got := q.Windows[0].Remaining; got < 0.5999 || got > 0.6001 {
		t.Errorf("5h remaining = %v, want 0.6", got)
	}
	if q.Windows[0].ResetAt != now.Add(time.Hour).Format(time.RFC3339) {
		t.Errorf("5h reset = %q", q.Windows[0].ResetAt)
	}
	// The headline figure is unchanged: still the most limiting window.
	if q.Window != "7d" {
		t.Errorf("headline window = %q, want 7d", q.Window)
	}
}

func TestParseClaudeUsage_OneFigurePerWindowLength(t *testing.T) {
	q, ok := parseClaudeUsage([]byte(`{"five_hour":{"utilization":10,"resets_at":"2026-10-07T15:00:00Z"},
		"seven_day":{"utilization":20,"resets_at":"2026-10-12T00:00:00Z"},
		"seven_day_opus":{"utilization":70,"resets_at":"2026-10-12T00:00:00Z"}}`), time.Now())
	if !ok || len(q.Windows) != 2 {
		t.Fatalf("got %+v ok=%v, want 5h and 7d only", q, ok)
	}
	if q.Windows[1].Label != "7d" || q.Windows[1].Remaining < 0.2999 || q.Windows[1].Remaining > 0.3001 {
		t.Errorf("7d = %+v, want the tighter opus figure (30%% left)", q.Windows[1])
	}
}

// ── signing in from a machine that is not the browser's ─────────────────────

// The reported bug: Memo was installed on a Raspberry Pi / VDS and reached from
// a laptop. The sign-in link opened, the user signed in, and the vendor sent
// the browser to `http://localhost:<port>/…` — the laptop's own loopback, where
// no sidecar is listening — so the login silently never completed.
//
// Every vendor checks redirect_uri against its own registered OAuth client, so
// rewriting it is not an option: Google's authorize endpoint answers
// `redirect_uri_mismatch` for a public host and
// "device_id and device_name are required for private IP" for a LAN one
// (verified live). What the sidecar does instead — it says so itself, in
// "To authenticate from a remote machine, an SSH tunnel may be required" — is
// ask for the callback URL on stdin once no browser turns up locally. This
// covers that hand-off end to end: the login reaches the ask, reports it, takes
// the pasted URL, and finishes.
func TestLogin_APastedCallbackURLFinishesARemoteSignIn(t *testing.T) {
	fast(t)
	t.Setenv("FAKECPA_PASTE_CALLBACK", "1")
	trace := t.TempDir()
	t.Setenv("FAKECPA_AUTH_TRACE", trace)

	m := newMgr(t)
	if _, err := m.StartLogin(context.Background(), ProviderCodex); err != nil {
		t.Fatalf("StartLogin: %v", err)
	}

	// The sidecar asks for the URL only after waiting for a browser that never
	// comes; the UI must be able to see that it is waiting.
	eventually(t, "the sidecar to ask for the callback URL", 10*time.Second, func() bool {
		return m.LoginStatus().NeedsPaste
	})

	// What the browser's address bar holds after the vendor redirects to the
	// (empty) localhost page: the whole URL, code and state included.
	const pasted = "http://localhost:1455/auth/callback?code=REALCODE&state=REALSTATE"
	if err := m.SubmitCallbackURL(pasted); err != nil {
		t.Fatalf("SubmitCallbackURL: %v", err)
	}

	eventually(t, "the login to finish", 10*time.Second, func() bool { return m.LoginStatus().Done })
	if st := m.LoginStatus(); st.Error != "" {
		t.Fatalf("login error: %s", st.Error)
	}

	// The sidecar really received it, byte for byte — a mangled query would
	// exchange nothing.
	b, err := os.ReadFile(filepath.Join(trace, "pasted.txt"))
	if err != nil {
		t.Fatalf("the sidecar never recorded the pasted URL: %v (tail: %v)", err, m.LoginStatus())
	}
	if strings.TrimSpace(string(b)) != pasted {
		t.Errorf("sidecar read %q, want %q", strings.TrimSpace(string(b)), pasted)
	}
	if len(m.Accounts()) != 1 {
		t.Errorf("accounts = %v, want the one the completed login wrote", m.Accounts())
	}
}

// The state the UI polls must not keep saying "paste this" once the login is
// over, or the Settings tab would show the box forever.
func TestLogin_NeedsPasteIsFalseOnceTheLoginIsDone(t *testing.T) {
	fast(t)
	t.Setenv("FAKECPA_PASTE_CALLBACK", "1")
	t.Setenv("FAKECPA_AUTH_TRACE", t.TempDir())

	m := newMgr(t)
	if _, err := m.StartLogin(context.Background(), ProviderClaude); err != nil {
		t.Fatalf("StartLogin: %v", err)
	}
	eventually(t, "the paste prompt", 10*time.Second, func() bool { return m.LoginStatus().NeedsPaste })
	if err := m.SubmitCallbackURL("http://localhost:54545/callback?code=X&state=Y"); err != nil {
		t.Fatalf("SubmitCallbackURL: %v", err)
	}
	eventually(t, "the login to finish", 10*time.Second, func() bool { return m.LoginStatus().Done })
	if st := m.LoginStatus(); st.NeedsPaste {
		t.Error("NeedsPaste is still set after the login finished")
	}
}

// A paste with no login behind it must say so rather than write into the void,
// and an empty paste is never a callback URL.
func TestSubmitCallbackURL_ErrorsWithoutALiveLogin(t *testing.T) {
	fast(t)
	m := newMgr(t)

	if err := m.SubmitCallbackURL("http://localhost:1455/auth/callback?code=X"); err == nil {
		t.Error("a callback URL with no login in flight was accepted")
	}
	if err := m.SubmitCallbackURL("   "); err == nil {
		t.Error("a blank callback URL was accepted")
	}

	if _, err := m.StartLogin(context.Background(), ProviderCodex); err != nil {
		t.Fatalf("StartLogin: %v", err)
	}
	eventually(t, "the login to finish", 10*time.Second, func() bool { return m.LoginStatus().Done })
	if err := m.SubmitCallbackURL("http://localhost:1455/auth/callback?code=X"); err == nil {
		t.Error("a callback URL was accepted after the login had already finished")
	}
}

// The three vendors print three different prompts ("Paste the Codex callback
// URL…", "…Claude…", "…antigravity…"). Every one of them must be recognised, or
// a vendor Memo already works with on a desktop would silently never offer the
// paste box on a remote install.
func TestWantsPaste_MatchesEveryVendorsPrompt(t *testing.T) {
	for _, line := range []string{
		"Paste the Codex callback URL (or press Enter to keep waiting): ",
		"Paste the Claude callback URL (or press Enter to keep waiting): ",
		"Paste the antigravity callback URL (or press Enter to keep waiting): ",
	} {
		if !wantsPaste(line) {
			t.Errorf("wantsPaste(%q) = false, want true", line)
		}
	}
	for _, line := range []string{
		"Waiting for Codex authentication callback...",
		"Attempting to open URL in browser: https://example.test/oauth/authorize?client_id=fake",
		"To authenticate from a remote machine, an SSH tunnel may be required.",
	} {
		if wantsPaste(line) {
			t.Errorf("wantsPaste(%q) = true, want false", line)
		}
	}
}
