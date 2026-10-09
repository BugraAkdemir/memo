// fakecpa stands in for the CLIProxyAPI binary in Memo's tests. It speaks just
// enough of the real one's surface to exercise the lifecycle code:
//
//	fakecpa -config cfg.yaml                 serve /v1/models + /v1/chat/completions (Bearer-key gated)
//	fakecpa -config cfg.yaml -<p>-login      print an authorize URL, write a credential, exit
//
// Knobs (env): FAKECPA_LOGIN_DELAY_MS, FAKECPA_LOGIN_FAIL=1, FAKECPA_CRASH_ONCE_MS,
// FAKECPA_MODELS_DELAY_MS (the real binary lists no models for ~30s after start),
// FAKECPA_IMAGE_MODELS=1 (one "<vendor>-image" model each, plus /v1/images/*),
// FAKECPA_PASTE_CALLBACK=1 (ask for the callback URL on stdin, like the real
// binary does when no browser turns up locally; the line it is given is
// written to $FAKECPA_AUTH_TRACE/pasted.txt).
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func main() {
	cfgPath := flag.String("config", "", "config file")
	ag := flag.Bool("antigravity-login", false, "")
	cl := flag.Bool("claude-login", false, "")
	cx := flag.Bool("codex-login", false, "")
	flag.Parse()

	raw, err := os.ReadFile(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no config:", err)
		os.Exit(2)
	}
	cfg := string(raw)
	port := first(cfg, `port:\s*(\d+)`)
	key, _ := strconv.Unquote(first(cfg, `-\s*("[^"]*")`))
	authDir, _ := strconv.Unquote(first(cfg, `auth-dir:\s*("[^"]*")`))

	switch {
	case *ag:
		login("antigravity", authDir)
	case *cl:
		login("claude", authDir)
	case *cx:
		login("codex", authDir)
	}

	if ms, _ := strconv.Atoi(os.Getenv("FAKECPA_CRASH_ONCE_MS")); ms > 0 {
		marker := filepath.Join(filepath.Dir(authDir), "crashed")
		if _, err := os.Stat(marker); err != nil {
			_ = os.WriteFile(marker, []byte("x"), 0o600)
			go func() { time.Sleep(time.Duration(ms) * time.Millisecond); os.Exit(3) }()
		}
	}

	// FAKECPA_IMAGE_MODELS=1 adds one image model per signed-in vendor
	// ("<vendor>-image") and serves /v1/images/{generations,edits} for exactly
	// those, the way the real sidecar does: a request with no prompt is the cheap
	// probe ("prompt is required" for a model it serves, "is not supported" for the
	// rest), and every real request is appended to <data>/cliproxy/image_requests.log
	// as "model path source-images" so a test can see where a picture went.
	imageModels := os.Getenv("FAKECPA_IMAGE_MODELS") == "1"
	images := func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+key {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		var req struct {
			Model  string            `json:"model"`
			Prompt string            `json:"prompt"`
			Images []json.RawMessage `json:"images"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if !imageModels || !strings.HasSuffix(req.Model, "-image") {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":{"message":"model %s is not supported"}}`, req.Model)
			return
		}
		if strings.TrimSpace(req.Prompt) == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"prompt is required"}}`))
			return
		}
		if f, err := os.OpenFile(filepath.Join(filepath.Dir(authDir), "image_requests.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			fmt.Fprintf(f, "%s %s %d\n", req.Model, r.URL.Path, len(req.Images))
			f.Close()
		}
		fmt.Fprintf(w, `{"data":[{"b64_json":%q}],"output_format":"png"}`, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")
	}
	http.HandleFunc("/v1/images/generations", images)
	http.HandleFunc("/v1/images/edits", images)

	started := time.Now()
	modelsDelay, _ := strconv.Atoi(os.Getenv("FAKECPA_MODELS_DELAY_MS"))
	http.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+key {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		type model struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
		}
		out := []model{}
		ents, _ := os.ReadDir(authDir)
		if time.Since(started) < time.Duration(modelsDelay)*time.Millisecond {
			ents = nil
		}
		for _, e := range ents {
			b, _ := os.ReadFile(filepath.Join(authDir, e.Name()))
			var c struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(b, &c) != nil || c.Type == "" {
				continue
			}
			out = append(out, model{c.Type + "-model-a", "model", c.Type}, model{c.Type + "-model-b", "model", c.Type})
			if imageModels {
				out = append(out, model{c.Type + "-image", "model", c.Type})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": out})
	})
	// Just enough of chat/completions to prove a client sends the right model and
	// key: the reply names the model it was asked for.
	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+key {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		var req struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		text := "pong from " + req.Model
		if !req.Stream {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "x", "object": "chat.completion", "model": req.Model,
				"choices": []map[string]any{{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": text}}},
				"usage":   map[string]any{"prompt_tokens": 7, "completion_tokens": 3, "total_tokens": 10},
			})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		chunk := func(delta map[string]any, finish any, usage any) {
			c := map[string]any{"id": "x", "object": "chat.completion.chunk", "model": req.Model,
				"choices": []map[string]any{{"index": 0, "delta": delta, "finish_reason": finish}}}
			if usage != nil {
				c["usage"] = usage
			}
			b, _ := json.Marshal(c)
			fmt.Fprintf(w, "data: %s\n\n", b)
			fl.Flush()
		}
		chunk(map[string]any{"role": "assistant", "content": text}, nil, nil)
		chunk(map[string]any{}, "stop", map[string]any{"prompt_tokens": 7, "completion_tokens": 3, "total_tokens": 10})
		fmt.Fprint(w, "data: [DONE]\n\n")
		fl.Flush()
	})
	fmt.Println("API server started successfully on: 127.0.0.1:" + port)
	if err := http.ListenAndServe("127.0.0.1:"+port, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func login(provider, authDir string) {
	// The real sidecar probes for a browser by running `xdg-open about:blank`
	// and WAITING for it before it prints any URL (see internal/cliproxy/browser.go).
	if os.Getenv("FAKECPA_PROBE_BROWSER") == "1" {
		fmt.Println("Opening browser for " + provider + " authentication")
		_ = exec.Command("xdg-open", "about:blank").Run()
	}
	// The real binary prints other https links too; only the OAuth one counts.
	fmt.Println("See https://example.test/docs for help")
	fmt.Println("Attempting to open URL in browser: https://example.test/oauth/authorize?client_id=fake&state=SECRETSTATE&code_challenge=SECRETCHALLENGE")
	ms, _ := strconv.Atoi(os.Getenv("FAKECPA_LOGIN_DELAY_MS"))
	if ms == 0 {
		ms = 250
	}
	time.Sleep(time.Duration(ms) * time.Millisecond)
	if os.Getenv("FAKECPA_PASTE_CALLBACK") == "1" {
		// What the real sidecar does once its loopback listener has waited
		// without a browser: ask for the callback URL on stdin (Codex, Claude
		// and Antigravity all print "Paste the … callback URL"). It is the
		// only way in from a machine that is not the browser's.
		fmt.Println("To authenticate from a remote machine, an SSH tunnel may be required.")
		fmt.Print("Paste the " + provider + " callback URL (or press Enter to keep waiting): ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && strings.TrimSpace(line) == "" {
			fmt.Println("authentication failed: no callback URL was provided")
			os.Exit(1)
		}
		if !strings.Contains(line, "code=") {
			fmt.Println("authentication failed: the pasted line carries no authorization code")
			os.Exit(1)
		}
		if err := os.WriteFile(filepath.Join(os.Getenv("FAKECPA_AUTH_TRACE"), "pasted.txt"), []byte(line), 0o600); err != nil {
			fmt.Println("could not record the pasted URL: " + err.Error())
			os.Exit(1)
		}
	}
	if os.Getenv("FAKECPA_LOGIN_FAIL") == "1" {
		fmt.Println("authentication failed: access_denied for https://example.test/oauth/token?code=SECRETCODE")
		os.Exit(1)
	}
	_ = os.MkdirAll(authDir, 0o700)
	body, _ := json.Marshal(map[string]any{
		"type": provider, "email": "fake@example.com", "project_id": "fake-project",
		"access_token": "SECRET-ACCESS-TOKEN", "refresh_token": "SECRET-REFRESH-TOKEN",
	})
	if err := os.WriteFile(filepath.Join(authDir, provider+"-fake@example.com.json"), body, 0o600); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("Authentication successful")
	fmt.Fprintln(os.Stderr, "EXITING NOW")
	os.Exit(0)
}

func first(s, pat string) string {
	m := regexp.MustCompile(pat).FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}
