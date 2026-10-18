package replcli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// skillsBackend serves the real skill endpoints' JSON shapes: the list marshals
// skill.SkillDefinition as is (manifest under "Manifest"), and the active set is
// kept PER CHAT, like sessions.SetActiveSkills.
func skillsBackend(t *testing.T) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	active := map[string][]string{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/skills/list", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"Manifest": map[string]any{"name": "docx", "description": "Word belgeleri"}, "Instructions": "x"},
			{"Manifest": map[string]any{"name": "pdf", "description": "PDF işleme"}},
		})
	})
	mux.HandleFunc("/api/skills/active-list", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		json.NewEncoder(w).Encode(map[string][]string{"names": active[r.URL.Query().Get("chat_id")]})
	})
	mux.HandleFunc("/api/skills/active", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ChatID string   `json:"chat_id"`
			Names  []string `json:"names"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		active[body.ChatID] = body.Names
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	return httptest.NewServer(mux)
}

func TestSkill_ListShowsInstalledSkillsAndTheirStateInThisChat(t *testing.T) {
	srv := skillsBackend(t)
	defer srv.Close()
	s, out := newTestSession(t, srv)
	s.chatID = "chat-1"

	s.handleCommand("/skill")
	got := out.String()
	if !strings.Contains(got, "docx") || !strings.Contains(got, "Word belgeleri") || !strings.Contains(got, "pdf") {
		t.Fatalf("skill list:\n%s", got)
	}
	if lineWith(got, "docx") != "" && strings.Contains(lineWith(got, "docx"), "✓") {
		t.Errorf("nothing is active yet:\n%s", got)
	}

	out.Reset()
	s.handleCommand("/skill on docx")
	if !strings.Contains(out.String(), "docx") {
		t.Errorf("no confirmation:\n%s", out.String())
	}
	out.Reset()
	s.handleCommand("/skill")
	if !strings.Contains(lineWith(out.String(), "docx"), "✓") {
		t.Errorf("docx should be marked active in this chat:\n%s", out.String())
	}
}

// Skills belong to a chat: activating one in a REPL session must not turn it on
// in another chat (the desktop app's, or another terminal's).
func TestSkill_ActivationIsScopedToThisSessionsChat(t *testing.T) {
	srv := skillsBackend(t)
	defer srv.Close()
	a, _ := newTestSession(t, srv)
	b, outB := newTestSession(t, srv)
	a.chatID, b.chatID = "chat-a", "chat-b"

	a.handleCommand("/skill on pdf")
	b.handleCommand("/skill")

	if strings.Contains(lineWith(outB.String(), "pdf"), "✓") {
		t.Errorf("a skill turned on in chat A is active in chat B:\n%s", outB.String())
	}
	names, _ := b.client.ChatSkills(b.ctx, "chat-a")
	if len(names) != 1 || names[0] != "pdf" {
		t.Errorf("chat A's skills = %v", names)
	}
}

func TestSkill_OffAndUnknownAndUsage(t *testing.T) {
	srv := skillsBackend(t)
	defer srv.Close()
	s, out := newTestSession(t, srv)
	s.chatID = "c"

	s.handleCommand("/skill on DOCX") // case-insensitive
	names, _ := s.client.ChatSkills(s.ctx, "c")
	if len(names) != 1 || names[0] != "docx" {
		t.Fatalf("after on: %v", names)
	}
	s.handleCommand("/skill off docx")
	if names, _ = s.client.ChatSkills(s.ctx, "c"); len(names) != 0 {
		t.Fatalf("after off: %v", names)
	}

	out.Reset()
	s.handleCommand("/skill on nope")
	if !strings.Contains(out.String(), "nope") {
		t.Errorf("unknown skill must be reported:\n%s", out.String())
	}
	out.Reset()
	s.handleCommand("/skill maybe")
	if !strings.Contains(out.String(), "/skill") {
		t.Errorf("usage expected:\n%s", out.String())
	}
}

// The "/" menu's skill entries toggle in this session's chat.
func TestSkill_MenuToggleFlipsTheState(t *testing.T) {
	srv := skillsBackend(t)
	defer srv.Close()
	s, _ := newTestSession(t, srv)
	s.chatID = "c"

	s.toggleSkill("pdf")
	if names, _ := s.client.ChatSkills(s.ctx, "c"); len(names) != 1 {
		t.Fatalf("toggle on: %v", names)
	}
	s.toggleSkill("pdf")
	if names, _ := s.client.ChatSkills(s.ctx, "c"); len(names) != 0 {
		t.Fatalf("toggle off: %v", names)
	}
	rows := s.skillRows()
	if len(rows) != 2 || !strings.Contains(rows[0].hint(), "Word belgeleri") {
		t.Errorf("rows = %+v", rows)
	}
}

func TestSlashCommands_ListsSkill(t *testing.T) {
	found := false
	for _, c := range slashCommands() {
		if c.label == "/skill" {
			found = true
		}
	}
	if !found {
		t.Error("/skill is missing from the slash-command list")
	}
}

// lineWith returns the first output line that contains word.
func lineWith(out, word string) string {
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, word) {
			return l
		}
	}
	return ""
}
