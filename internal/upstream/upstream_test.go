//go:build upstream

// SPDX-License-Identifier: AGPL-3.0-or-later

package upstream

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// The real probe table. Run with `go test -tags upstream ./internal/upstream/`;
// the scheduled upstream workflow does, and turns a DRIFT into an issue.
//
// Every probe is unauthenticated: the EXPECTED answer is a refusal, which still
// proves the endpoint lives at the same address, speaks the same error shape and
// is not blocking us. Expectations were recorded from real answers on 2026-10-07;
// when one moves legitimately, update the probe in the same change that adapts
// Memo to it.
//
// UPSTREAM_REPORT, when set, is a file the results are appended to (markdown)
// for the workflow to put in the issue.

var jsonHdr = map[string]string{"Content-Type": "application/json"}

func probes() []Probe {
	anth := map[string]string{"anthropic-version": "2023-06-01", "Content-Type": "application/json"}
	return []Probe{
		// ── Vendor APIs ────────────────────────────────────────────────────
		{Name: "Anthropic Messages API (no key)", Method: "POST", URL: "https://api.anthropic.com/v1/messages", Header: anth, Body: `{}`, Want: JSONError(401)},
		{Name: "Anthropic models list (no key)", URL: "https://api.anthropic.com/v1/models", Header: anth, Want: JSONError(401)},
		{Name: "OpenAI models list (no key)", URL: "https://api.openai.com/v1/models", Want: JSONError(401)},
		{Name: "Google Generative Language models (no key)", URL: "https://generativelanguage.googleapis.com/v1beta/models", Want: JSONError(400, 401, 403)},
		{Name: "Google Cloud Code Assist loadCodeAssist (no auth)", Method: "POST", URL: "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist", Header: jsonHdr, Body: `{}`, Want: JSONError(401)},

		// ── OAuth endpoints the bundled sidecar signs in through ───────────
		{Name: "Google OAuth token endpoint", Method: "POST", URL: "https://oauth2.googleapis.com/token", Header: map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, Body: "grant_type=refresh_token&refresh_token=invalid", Want: JSONError(400, 401)},
		{Name: "Google OAuth authorize endpoint", URL: "https://accounts.google.com/o/oauth2/v2/auth", Want: Status(200, 302, 400)},
		{Name: "Anthropic OAuth token endpoint", Method: "POST", URL: "https://platform.claude.com/v1/oauth/token", Header: jsonHdr, Body: `{"grant_type":"refresh_token","refresh_token":"invalid"}`, Want: Status(400, 401, 405)},
		{Name: "OpenAI auth (Codex sign-in) host", URL: "https://auth.openai.com/", Want: Status(200, 301, 302, 303, 307, 308, 401, 403, 404)},

		// ── Antigravity ────────────────────────────────────────────────────
		// The update manifest CLIProxyAPI reads to learn the current client
		// version it presents to Google; if it moves, the sidecar's fingerprint
		// goes stale and Google starts refusing it.
		{Name: "Antigravity update manifest", URL: "https://antigravity-hub-auto-updater-974169037036.us-central1.run.app/manifest/latest-arm64-mac.yml", Want: BodyContains("version:")},

		// ── Memo's own download host and site ──────────────────────────────
		{Name: "memocpp.com guide", URL: "https://memocpp.com/guide", Want: Status(200)},
		{Name: "install script get-memo.sh", URL: "https://data.memocpp.com/get-memo.sh", Want: scriptWant("#!/")},
		{Name: "install script get-memo.ps1", URL: "https://data.memocpp.com/get-memo.ps1", Want: scriptWant("")},
		{Name: "update script update.sh", URL: "https://data.memocpp.com/update.sh", Want: scriptWant("#!/")},
		{Name: "uninstall script uninstall.sh", URL: "https://data.memocpp.com/uninstall.sh", Want: scriptWant("#!/")},
		{Name: "stable archive memo.tar.gz (HEAD)", Method: "HEAD", URL: "https://data.memocpp.com/memo.tar.gz", Want: Status(200)},
		{Name: "stable archive memo.exe (HEAD)", Method: "HEAD", URL: "https://data.memocpp.com/memo.exe", Want: Status(200)},
		{Name: "stable archive memo-mac.zip (HEAD)", Method: "HEAD", URL: "https://data.memocpp.com/memo-mac.zip", Want: Status(200)},
		{Name: "update beacon version.json", URL: "https://version-zeta.vercel.app/version.json", Want: BodyContains(`"version"`)},

		// ── Model downloads ────────────────────────────────────────────────
		{Name: "HuggingFace model search API", URL: "https://huggingface.co/api/models?search=llama&filter=gguf&limit=1", Want: BodyContains(`"id"`)},
	}
}

// scriptWant: a served install script must be 200, must not point at the sold
// domain, and (for shell scripts) must start with a shebang — a CDN error page or
// a hijacked host serving something else fails this.
func scriptWant(prefix string) func(int, []byte, http.Header) string {
	return func(status int, body []byte, _ http.Header) string {
		if status != http.StatusOK {
			return "want status 200"
		}
		if strings.Contains(strings.ToLower(string(body)), "bugradev.com") {
			return "script still references the sold domain bugradev.com"
		}
		if prefix != "" && !strings.HasPrefix(string(body), prefix) {
			return fmt.Sprintf("script does not start with %q — not the script we published", prefix)
		}
		if len(body) < 200 {
			return "script is suspiciously short"
		}
		return ""
	}
}

var reportMu sync.Mutex

func report(t *testing.T, line string) {
	t.Helper()
	path := os.Getenv("UPSTREAM_REPORT")
	if path == "" {
		return
	}
	reportMu.Lock()
	defer reportMu.Unlock()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Logf("cannot write report: %v", err)
		return
	}
	defer f.Close()
	fmt.Fprintln(f, line)
}

func TestUpstreamProbes(t *testing.T) {
	// t.Cleanup, not defer: the parallel subtests below run AFTER this function
	// body returns, and a deferred cancel() would kill every request first (the
	// first version of this test reported every probe "inconclusive").
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)
	client := &http.Client{Timeout: 30 * time.Second}

	var mu sync.Mutex
	counts := map[Verdict]int{}

	t.Run("probes", func(t *testing.T) {
		for _, p := range probes() {
			p := p
			t.Run(p.Name, func(t *testing.T) {
				t.Parallel()
				r := Run(ctx, client, p)
				mu.Lock()
				counts[r.Verdict]++
				mu.Unlock()
				line := fmt.Sprintf("- **%s** — %s: %s", r.Verdict, p.Name, r.Detail)
				t.Log(line)
				switch r.Verdict {
				case Drift:
					report(t, line+"  \n  `"+p.URL+"`")
					t.Errorf("DRIFT in %q: %s", p.Name, r.Detail)
				case Inconclusive:
					report(t, line)
				}
			})
		}
	})

	// A watch that could not conclude about most of what it watches is itself
	// broken (runner without network, a blanket block) — say so rather than
	// reporting a quiet green.
	total := counts[OK] + counts[Drift] + counts[Inconclusive]
	if total > 0 && counts[Inconclusive]*2 > total {
		line := fmt.Sprintf("- **watch degraded** — %d of %d probes were inconclusive; this run proves little", counts[Inconclusive], total)
		report(t, line)
		t.Logf("%s", line)
	}
	t.Logf("summary: ok=%d drift=%d inconclusive=%d", counts[OK], counts[Drift], counts[Inconclusive])
}
