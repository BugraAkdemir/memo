// SPDX-License-Identifier: AGPL-3.0-or-later

package replcli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// providerModelServer fakes the three endpoints /model touches when an external
// provider is active: which one is active, its live model list, and the model
// switch. switches records every PUT body.
type providerModelServer struct {
	srv      *httptest.Server
	mu       sync.Mutex
	switches []map[string]any
	active   string
	models   []ProviderModel
	current  string
	listFail bool
}

func newProviderModelServer(t *testing.T, active string, models []ProviderModel, current string) *providerModelServer {
	t.Helper()
	p := &providerModelServer{active: active, models: models, current: current}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/providers/active", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"provider": p.active})
	})
	mux.HandleFunc("/api/providers/model", func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			if p.listFail {
				http.Error(w, "boom", http.StatusInternalServerError)
				return
			}
			if r.URL.Query().Get("name") != p.active {
				http.Error(w, "wrong provider asked", http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(ProviderModelList{Models: p.models, Current: p.current})
		case http.MethodPut:
			body := map[string]any{}
			json.NewDecoder(r.Body).Decode(&body)
			p.switches = append(p.switches, body)
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		}
	})
	// The local-model fallback path.
	mux.HandleFunc("/api/models/local", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]LocalModel{{Filename: "llama-chat.gguf", Path: "/m/llama-chat.gguf"}})
	})
	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	return p
}

func (p *providerModelServer) sess(keys string) (*session, *bytes.Buffer) {
	var out bytes.Buffer
	s := &session{
		client:  NewClient(p.srv.URL),
		ctx:     context.Background(),
		out:     &out,
		scanner: bufio.NewScanner(strings.NewReader("")),
	}
	if keys != "" {
		s.keys = newKeySource(strings.NewReader(keys))
	}
	return s, &out
}

func (p *providerModelServer) lastSwitch() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.switches) == 0 {
		return nil
	}
	return p.switches[len(p.switches)-1]
}

var subsTestModels = []ProviderModel{
	{ID: "claude-sonnet-4-6", OwnedBy: "antigravity"},
	{ID: "claude-opus-4-6-thinking", OwnedBy: "antigravity"},
	{ID: "gemini-3-flash", OwnedBy: "antigravity"},
}

func TestModelCommand_TypedNameSwitchesTheActiveProvidersModel(tt *testing.T) {
	p := newProviderModelServer(tt, "Subscriptions", subsTestModels, "claude-sonnet-4-6")
	s, out := p.sess("")

	s.handleCommand("/model gemini") // unique substring

	sw := p.lastSwitch()
	if sw == nil || sw["name"] != "Subscriptions" || sw["model"] != "gemini-3-flash" {
		tt.Fatalf("switch = %v, output:\n%s", sw, out.String())
	}
	if want := fmt.Sprintf(t("provider_model_switched"), "gemini-3-flash", "Subscriptions"); !strings.Contains(out.String(), want) {
		tt.Errorf("no confirmation %q in:\n%s", want, out.String())
	}
}

func TestModelCommand_ExactIdBeatsSubstring(tt *testing.T) {
	models := []ProviderModel{{ID: "gemini-3"}, {ID: "gemini-3-flash"}}
	p := newProviderModelServer(tt, "Subscriptions", models, "")
	s, _ := p.sess("")

	s.handleCommand("/model gemini-3")

	if sw := p.lastSwitch(); sw == nil || sw["model"] != "gemini-3" {
		tt.Errorf("switch = %v, want the exact id gemini-3 (not the longer match)", sw)
	}
}

func TestModelCommand_AmbiguousNameIsNotGuessed(tt *testing.T) {
	p := newProviderModelServer(tt, "Subscriptions", subsTestModels, "")
	s, out := p.sess("")

	s.handleCommand("/model claude") // matches two models

	if sw := p.lastSwitch(); sw != nil {
		tt.Errorf("guessed between two models: %v", sw)
	}
	// Falls through to the local-model lookup, which says it found nothing.
	if !strings.Contains(out.String(), "bulunamadı") {
		tt.Errorf("expected the local not-found message, got:\n%s", out.String())
	}
}

func TestModelCommand_MenuPicksWithArrowKeys(tt *testing.T) {
	p := newProviderModelServer(tt, "Subscriptions", subsTestModels, "claude-sonnet-4-6")
	s, out := p.sess("\x1b[B\r") // Down, Enter -> the second entry

	s.handleCommand("/model")

	sw := p.lastSwitch()
	if sw == nil || sw["model"] != "claude-opus-4-6-thinking" {
		tt.Fatalf("switch = %v, output:\n%s", sw, out.String())
	}
	if !strings.Contains(out.String(), "claude-sonnet-4-6") || !strings.Contains(out.String(), t("current_marker")) {
		tt.Errorf("the menu should list every model and mark the current one:\n%s", out.String())
	}
}

func TestModelCommand_MenuEscapeSwitchesNothing(tt *testing.T) {
	p := newProviderModelServer(tt, "Subscriptions", subsTestModels, "")
	s, _ := p.sess("\x1b")

	s.handleCommand("/model")

	if sw := p.lastSwitch(); sw != nil {
		tt.Errorf("Esc still switched: %v", sw)
	}
}

func TestModelCommand_WithoutAnExternalProviderStaysLocal(tt *testing.T) {
	p := newProviderModelServer(tt, "", subsTestModels, "")
	s, out := p.sess("")

	s.handleCommand("/model nonexistent")

	if p.lastSwitch() != nil {
		tt.Error("touched an external provider with none active")
	}
	if !strings.Contains(out.String(), "bulunamadı") {
		tt.Errorf("expected the local lookup's message, got:\n%s", out.String())
	}
}

func TestModelCommand_ListFailureFallsBackToLocal(tt *testing.T) {
	p := newProviderModelServer(tt, "Subscriptions", subsTestModels, "")
	p.listFail = true
	s, out := p.sess("")

	s.handleCommand("/model nonexistent")

	if p.lastSwitch() != nil {
		tt.Error("switched with no list")
	}
	if !strings.Contains(out.String(), "bulunamadı") {
		tt.Errorf("expected the local lookup's message, got:\n%s", out.String())
	}
}

func TestModelCommand_BareModelWithoutATerminalPrintsUsage(tt *testing.T) {
	p := newProviderModelServer(tt, "Subscriptions", subsTestModels, "")
	s, out := p.sess("") // no key source: piped input

	s.handleCommand("/model")

	if !strings.Contains(out.String(), t("model_usage")) {
		tt.Errorf("want the usage line when there is no terminal for a menu, got:\n%s", out.String())
	}
}
