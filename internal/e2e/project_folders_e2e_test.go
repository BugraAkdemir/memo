// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// requestLog records every chat request the fake provider receives.
type requestLog struct {
	mu   sync.Mutex
	raws []string
}

func (l *requestLog) add(raw []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.raws = append(l.raws, string(raw))
}

func (l *requestLog) all() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.raws, "\n")
}

// mustHaveAsked fails unless a turn really reached the provider, so a negative
// assertion ("the instruction is absent") cannot pass vacuously.
func (l *requestLog) mustHaveAsked(t *testing.T) string {
	t.Helper()
	got := l.all()
	if !strings.Contains(got, "hello") {
		t.Fatalf("the turn never reached the provider; request log:\n%.400s", got)
	}
	return got
}

func (l *requestLog) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.raws = nil
}

// projectHarness boots a harness whose "home" is a throw-away folder, so a bug
// in the home guard can never write into the developer's real home.
func projectHarness(t *testing.T) (h *Harness, home string, log *requestLog) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("HOME-based fake home is a unix setup")
	}
	var err error
	home, err = filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{".memo/bin", ".memo/data"} { // a Memo install in "home"
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)

	h = NewHarness(t)
	log = &requestLog{}
	h.Fake.Script = func(_ int, req FakeChatRequest) FakeChatResponse {
		log.add(req.Raw)
		return FakeChatResponse{Text: "ok"}
	}
	return h, home, log
}

type projectStatus struct {
	Root      string `json:"root"`
	ID        string `json:"id"`
	SetUp     bool   `json:"set_up"`
	Declined  bool   `json:"declined"`
	Allowed   bool   `json:"allowed"`
	Reason    string `json:"reason"`
	ShouldAsk bool   `json:"should_ask"`
}

func (h *Harness) projectStatus(t *testing.T, path string) projectStatus {
	t.Helper()
	var st projectStatus
	resp := h.postJSON("/api/projects/status", map[string]string{"path": path})
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("status: %d", resp.StatusCode)
	}
	decodeInto(t, resp, &st)
	return st
}

func (h *Harness) postStatus(path string, body any) int {
	resp := h.postJSON(path, body)
	resp.Body.Close()
	return resp.StatusCode
}

func TestProjectFolders_SetupFlowAndTheSessionStartInstruction(t *testing.T) {
	h, home, log := projectHarness(t)
	proj := filepath.Join(home, "work", "shop")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Run("the home folder can never be a project and is left untouched", func(t *testing.T) {
		st := h.projectStatus(t, home)
		if st.Allowed || st.ShouldAsk || st.Reason == "" {
			t.Fatalf("home status = %+v", st)
		}
		if code := h.postStatus("/api/projects/setup", map[string]string{"path": home}); code != http.StatusForbidden {
			t.Fatalf("setup on home: status %d, want 403", code)
		}
		for _, f := range []string{"AGENTS.md", "handoff.md", ".memo/project.json", ".memo/AGENTS.md"} {
			if _, err := os.Stat(filepath.Join(home, f)); !os.IsNotExist(err) {
				t.Fatalf("%s appeared in the home folder", f)
			}
		}
	})

	var id string
	t.Run("a fresh folder is offered, set up, then no longer offered", func(t *testing.T) {
		st := h.projectStatus(t, proj)
		if !st.Allowed || st.SetUp || !st.ShouldAsk || st.ID == "" {
			t.Fatalf("fresh status = %+v", st)
		}
		id = st.ID
		if code := h.postStatus("/api/projects/setup", map[string]string{"path": proj}); code != http.StatusOK {
			t.Fatalf("setup: status %d", code)
		}
		for _, f := range []string{".memo/project.json", ".memo/AGENTS.md", ".memo/notes.md", "AGENTS.md", "handoff.md"} {
			if _, err := os.Stat(filepath.Join(proj, f)); err != nil {
				t.Errorf("missing %s", f)
			}
		}
		after := h.projectStatus(t, proj)
		if !after.SetUp || after.ShouldAsk || after.ID != id {
			t.Fatalf("after setup = %+v (id before: %s)", after, id)
		}
	})

	t.Run("an agent chat in the set-up folder is told to read the rules first", func(t *testing.T) {
		log.reset()
		chat := h.NewAgentChat(proj)
		h.SendMessageStream(chat, "hello")
		if got := log.all(); !strings.Contains(got, ".memo/AGENTS.md") {
			t.Fatalf("the model's request has no instruction to read .memo/AGENTS.md:\n%.600s", got)
		}
	})

	t.Run("a plain chat is not told anything", func(t *testing.T) {
		log.reset()
		h.SendMessageStream(h.NewChat(), "hello")
		if got := log.mustHaveAsked(t); strings.Contains(got, ".memo/AGENTS.md") {
			t.Fatal("a plain chat received the project instruction")
		}
	})

	t.Run("Minimal Mode adds nothing at all", func(t *testing.T) {
		if err := h.App.SetMinimalMode(true); err != nil {
			t.Fatal(err)
		}
		defer h.App.SetMinimalMode(false)
		log.reset()
		h.SendMessageStream(h.NewAgentChat(proj), "hello")
		if got := log.mustHaveAsked(t); strings.Contains(got, ".memo/AGENTS.md") {
			t.Fatal("Minimal Mode must be a pure wrapper, but the project instruction was sent")
		}
		if st := h.projectStatus(t, filepath.Join(home, "work", "other")); st.ShouldAsk {
			t.Fatal("Minimal Mode must not trigger the setup prompt")
		}
	})

	t.Run("a folder that is not set up gets no instruction", func(t *testing.T) {
		bare := filepath.Join(home, "work", "bare")
		if err := os.MkdirAll(bare, 0o755); err != nil {
			t.Fatal(err)
		}
		log.reset()
		h.SendMessageStream(h.NewAgentChat(bare), "hello")
		if got := log.mustHaveAsked(t); strings.Contains(got, ".memo/AGENTS.md") {
			t.Fatal("an un-set-up folder received the project instruction")
		}
	})
}

func TestProjectFolders_DecliningIsRemembered(t *testing.T) {
	h, home, _ := projectHarness(t)
	proj := filepath.Join(home, "w", "p")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if st := h.projectStatus(t, proj); !st.ShouldAsk {
		t.Fatalf("precondition: %+v", st)
	}
	if code := h.postStatus("/api/projects/decline", map[string]any{"path": proj}); code != http.StatusOK {
		t.Fatalf("decline: %d", code)
	}
	if st := h.projectStatus(t, proj); st.ShouldAsk || !st.Declined {
		t.Fatalf("after decline = %+v", st)
	}
	if _, err := os.Stat(filepath.Join(proj, ".memo")); !os.IsNotExist(err) {
		t.Fatal("declining must not create anything")
	}
	if code := h.postStatus("/api/projects/decline", map[string]any{"path": proj, "declined": false}); code != http.StatusOK {
		t.Fatal("undo decline failed")
	}
	if st := h.projectStatus(t, proj); !st.ShouldAsk {
		t.Fatalf("after undo = %+v", st)
	}
}
