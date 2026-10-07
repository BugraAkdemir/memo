//go:build upstream

// SPDX-License-Identifier: AGPL-3.0-or-later

package cliproxy

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Upstream watch for the bundled CLIProxyAPI (run by the scheduled `upstream`
// workflow with `go test -tags upstream`). Credential-free: GitHub's public
// release API and our own public download host. See internal/upstream's package
// doc for the drift/inconclusive distinction.
//
//	UPSTREAM_REPORT  markdown file DRIFT/NOTICE lines are appended to
//	UPSTREAM_DEEP=1  also download the linux binary from R2 and run it
//	GITHUB_TOKEN     optional, only to dodge the unauthenticated API rate limit

const (
	repoAPI = "https://api.github.com/repos/router-for-me/CLIProxyAPI/releases"
	r2Base  = "https://data.memocpp.com/binaries/"
)

type pinRow struct {
	asset, archiveSHA, dest, file, binSHA string
}

func readPin(t *testing.T) (version string, rows []pinRow) {
	t.Helper()
	f, err := os.Open("PINNED.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fl := strings.Fields(sc.Text())
		switch {
		case len(fl) == 2 && fl[0] == "version":
			version = fl[1]
		case len(fl) == 6 && fl[0] == "asset":
			rows = append(rows, pinRow{fl[1], fl[2], fl[3], fl[4], fl[5]})
		}
	}
	if version == "" || len(rows) == 0 {
		t.Fatal("PINNED.txt has no version or no asset rows")
	}
	return
}

func note(t *testing.T, line string) {
	t.Helper()
	t.Log(line)
	if p := os.Getenv("UPSTREAM_REPORT"); p != "" {
		if f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintln(f, line)
			f.Close()
		}
	}
}

func get(t *testing.T, url string) ([]byte, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "memo-upstream-watch")
	req.Header.Set("Accept", "application/vnd.github+json")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" && strings.Contains(url, "api.github.com") {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("inconclusive (network): %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 403 || resp.StatusCode == 429 || resp.StatusCode >= 500 {
		t.Skipf("inconclusive: %s answered %d", url, resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	return b, resp.StatusCode
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name   string `json:"name"`
		Digest string `json:"digest"`
	} `json:"assets"`
}

// The pinned release must still exist and every asset must still carry the digest
// we pinned. A deleted release or a re-uploaded asset would otherwise only show
// up when a build next tried to vendor it.
func TestUpstream_PinnedReleaseIsIntact(t *testing.T) {
	version, rows := readPin(t)
	body, code := get(t, repoAPI+"/tags/"+version)
	if code != http.StatusOK {
		note(t, fmt.Sprintf("- **DRIFT** — CLIProxyAPI release %s: GitHub answered %d (deleted or renamed?)", version, code))
		t.Fatalf("pinned release %s: HTTP %d", version, code)
	}
	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		t.Fatal(err)
	}
	have := map[string]string{}
	for _, a := range rel.Assets {
		have[a.Name] = strings.TrimPrefix(a.Digest, "sha256:")
	}
	for _, r := range rows {
		got, ok := have[r.asset]
		switch {
		case !ok:
			note(t, fmt.Sprintf("- **DRIFT** — CLIProxyAPI %s no longer has asset `%s`", version, r.asset))
			t.Errorf("asset %s missing from release %s", r.asset, version)
		case got != r.archiveSHA:
			note(t, fmt.Sprintf("- **DRIFT** — CLIProxyAPI %s asset `%s` was re-uploaded: digest is now %s, pinned %s", version, r.asset, got, r.archiveSHA))
			t.Errorf("asset %s digest changed", r.asset)
		}
	}
}

// Informational: a newer release exists. Never fails; the workflow turns the
// notice into an issue so someone can test and bump PINNED.txt.
func TestUpstream_NewerReleaseNotice(t *testing.T) {
	version, _ := readPin(t)
	body, code := get(t, repoAPI+"/latest")
	if code != http.StatusOK {
		t.Skipf("latest release lookup answered %d", code)
	}
	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		t.Fatal(err)
	}
	if rel.TagName != "" && rel.TagName != version {
		note(t, fmt.Sprintf("- **NOTICE** — CLIProxyAPI %s is out (Memo bundles %s). Test it and bump `internal/cliproxy/PINNED.txt`.", rel.TagName, version))
	}
}

// The binaries on R2 are what every release build bundles. Their SHA256 files must
// match the pin — a wrong or missing object breaks the next build (verify_cliproxy.sh
// would stop it) and is better caught here, on a schedule.
func TestUpstream_R2CopiesMatchThePin(t *testing.T) {
	_, rows := readPin(t)
	for _, r := range rows {
		body, code := get(t, r2Base+r.dest+"/SHA256")
		if code != http.StatusOK {
			note(t, fmt.Sprintf("- **DRIFT** — R2 object `%s/SHA256` answered %d", r.dest, code))
			t.Errorf("%s/SHA256: HTTP %d", r.dest, code)
			continue
		}
		want := false
		for _, line := range strings.Split(string(body), "\n") {
			if f := strings.Fields(line); len(f) == 2 && f[1] == r.file && f[0] == r.binSHA {
				want = true
			}
		}
		if !want {
			note(t, fmt.Sprintf("- **DRIFT** — R2 `%s/SHA256` does not list `%s` with the pinned digest %s", r.dest, r.file, r.binSHA))
			t.Errorf("%s: SHA256 file does not match the pin for %s", r.dest, r.file)
		}
	}
}

// The deep check: take the real linux binary exactly as users get it (from R2),
// verify it, and run it through the same Manager Memo uses. This is the one place
// that proves the config we generate is still accepted, the key gate still works
// and the login flags still exist in the pinned release.
func TestUpstream_RealBinaryHonoursOurContract(t *testing.T) {
	if os.Getenv("UPSTREAM_DEEP") != "1" {
		t.Skip("set UPSTREAM_DEEP=1 to download and run the real binary")
	}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("the deep check runs the linux/amd64 binary")
	}
	_, rows := readPin(t)
	var row pinRow
	for _, r := range rows {
		if r.dest == "linux/cliproxy" {
			row = r
		}
	}
	if row.file == "" {
		t.Fatal("no linux/cliproxy row in PINNED.txt")
	}

	root := t.TempDir()
	dir := filepath.Join(root, "binaries", "linux", "cliproxy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin, code := get(t, r2Base+row.dest+"/"+row.file)
	if code != http.StatusOK {
		note(t, fmt.Sprintf("- **DRIFT** — R2 binary `%s/%s` answered %d", row.dest, row.file, code))
		t.Fatalf("binary download: HTTP %d", code)
	}
	if sum := sha256.Sum256(bin); hex.EncodeToString(sum[:]) != row.binSHA {
		note(t, fmt.Sprintf("- **DRIFT** — R2 binary `%s/%s` does not match the pinned SHA-256", row.dest, row.file))
		t.Fatal("downloaded binary does not match the pin")
	}
	os.WriteFile(filepath.Join(dir, row.file), bin, 0o755)
	os.WriteFile(filepath.Join(dir, "SHA256"), []byte(row.binSHA+"  "+row.file+"\n"), 0o644)

	// The login flags Memo passes still exist.
	out, _ := exec.Command(filepath.Join(dir, row.file), "-h").CombinedOutput()
	for _, flag := range []string{"-antigravity-login", "-claude-login", "-codex-login", "-config"} {
		if !strings.Contains(string(out), flag) {
			note(t, fmt.Sprintf("- **DRIFT** — the pinned binary no longer has the `%s` flag", flag))
			t.Errorf("flag %s missing from -h", flag)
		}
	}

	// And it runs with the config we generate: listens on our loopback port,
	// refuses a client without the key, answers the one with it.
	m := New(filepath.Join(t.TempDir(), "cliproxy"))
	m.roots = []string{root}
	t.Cleanup(m.Stop)
	if err := m.Start(context.Background()); err != nil {
		note(t, "- **DRIFT** — the pinned binary did not come up with the config Memo generates: "+err.Error())
		t.Fatalf("Start: %v", err)
	}
	req, _ := http.NewRequest(http.MethodGet, m.BaseURL()+"/models", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		note(t, fmt.Sprintf("- **DRIFT** — the sidecar answered %d (not 401) to a request without its client key", resp.StatusCode))
		t.Errorf("unauthenticated /v1/models = %d, want 401", resp.StatusCode)
	}
	if _, err := m.Models(context.Background()); err != nil {
		t.Errorf("authenticated /v1/models failed: %v", err)
	}

	// Memo recognises image models by asking the images endpoint, with no prompt,
	// whether it serves the model (internal/provider/openai_images.go): a model it
	// serves answers "prompt is required", any other "is not supported". That is
	// the sidecar's wording, not an API contract — if it changes, image models
	// silently fall back to the chat path and fail there, so watch it.
	probe := func(model string) (int, string) {
		body := strings.NewReader(`{"model":"` + model + `"}`)
		req, _ := http.NewRequest(http.MethodPost, m.BaseURL()+"/images/generations", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+m.APIKey())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return resp.StatusCode, strings.ToLower(string(b))
	}
	if code, body := probe("gpt-image-2.5"); code != http.StatusBadRequest || !strings.Contains(body, "prompt is required") {
		note(t, fmt.Sprintf("- **DRIFT** — the images endpoint no longer answers a prompt-less image-model request with `prompt is required` (got %d: %.120s); Memo's dynamic image-model detection would stop working", code, body))
		t.Errorf("image-model probe: %d %s", code, body)
	}
	if code, body := probe("claude-sonnet-4-6"); code != http.StatusBadRequest || strings.Contains(body, "prompt is required") {
		note(t, fmt.Sprintf("- **DRIFT** — the images endpoint answered a chat model with %d %.120s; Memo would treat chat models as image models", code, body))
		t.Errorf("chat-model probe: %d %s", code, body)
	}
}
