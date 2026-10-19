// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/api"
	"memo/internal/beta"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/memory"
	"memo/internal/project"
	"memo/internal/sessions"
)

// projectScopeApp is an App wired with a project manager whose guard treats
// <tmp>/home as the user's home, so nothing touches the real one.
func projectScopeApp(t *testing.T) (a *App, sm *sessions.Manager, home string) {
	t.Helper()
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home = filepath.Join(tmp, "home")
	if err := os.MkdirAll(filepath.Join(home, ".memo", "bin"), 0o755); err != nil { // Memo's own install
		t.Fatal(err)
	}
	sm, err = sessions.NewManager(filepath.Join(tmp, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	a = &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: true, TopK: 5, PinnedFactsPerTurn: 5},
			Llama:  config.LlamaConfig{CtxSize: 8192},
		},
		identity: identity.New("Tester", "Memo", "casual", "", false),
		sessions: sm,
		projectMgr: project.NewManager(
			project.Guard{Home: home, Protected: []string{filepath.Join(home, ".memo")}},
			filepath.Join(tmp, "projects.json"),
		),
	}
	return a, sm, home
}

func mkProject(t *testing.T, home, name string) string {
	t.Helper()
	p := filepath.Join(home, "work", name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMemoryScopeForChat(t *testing.T) {
	a, sm, home := projectScopeApp(t)

	t.Run("a plain chat reads everything", func(t *testing.T) {
		sc, ok := a.memoryScopeForChat(sm.NewChat())
		if !ok || sc != memory.AllProjects {
			t.Fatalf("plain chat scope = %+v, %v", sc, ok)
		}
	})

	t.Run("the Live Mode chat is a general chat even though its folder is home", func(t *testing.T) {
		id := sm.NewBackgroundChat("Live Mode")
		if err := sm.SetProjectPath(id, home); err != nil {
			t.Fatal(err)
		}
		a.liveModeChatID = id
		defer func() { a.liveModeChatID = "" }()
		if sc, ok := a.memoryScopeForChat(id); !ok || sc != memory.AllProjects {
			t.Fatalf("live mode scope = %+v, %v", sc, ok)
		}
	})

	t.Run("project chats are confined to their own project", func(t *testing.T) {
		pa, pb := mkProject(t, home, "a"), mkProject(t, home, "b")
		c1, c2, c3 := sm.NewAgentChat(pa), sm.NewAgentChat(pa), sm.NewAgentChat(pb)
		s1, ok1 := a.memoryScopeForChat(c1)
		s2, ok2 := a.memoryScopeForChat(c2)
		s3, ok3 := a.memoryScopeForChat(c3)
		if !ok1 || !ok2 || !ok3 || !s1.InProject() || !s3.InProject() {
			t.Fatalf("scopes = %+v %+v %+v", s1, s2, s3)
		}
		if s1 != s2 {
			t.Error("two chats in the same folder must share one project pool")
		}
		if s1 == s3 {
			t.Error("two different folders must not share a pool")
		}
		if sm.GetProjectID(c1) != s1.WriteProjectID() {
			t.Error("the project id was not recorded on the chat")
		}
	})

	t.Run("change_directory moves the chat to the other project's pool", func(t *testing.T) {
		pa, pb := mkProject(t, home, "ca"), mkProject(t, home, "cb")
		c := sm.NewAgentChat(pa)
		before, _ := a.memoryScopeForChat(c)
		if err := sm.SetProjectPath(c, pb); err != nil {
			t.Fatal(err)
		}
		after, ok := a.memoryScopeForChat(c)
		if !ok || after == before || !after.InProject() {
			t.Fatalf("scope did not follow the folder: before %+v after %+v", before, after)
		}
	})

	t.Run("a deleted folder keeps the chat in its project", func(t *testing.T) {
		p := mkProject(t, home, "gone")
		c := sm.NewAgentChat(p)
		before, _ := a.memoryScopeForChat(c)
		if err := os.RemoveAll(p); err != nil {
			t.Fatal(err)
		}
		after, ok := a.memoryScopeForChat(c)
		if !ok || after != before {
			t.Fatalf("scope after the folder vanished = %+v, %v; want %+v", after, ok, before)
		}
	})

	t.Run("a project chat that cannot be identified gets no memory, not the general pool", func(t *testing.T) {
		c := sm.NewAgentChat(home) // the home folder can never be a project
		if sc, ok := a.memoryScopeForChat(c); ok {
			t.Fatalf("home-rooted agent chat got scope %+v; it must get none", sc)
		}
		c2 := sm.NewAgentChat(filepath.Join(home, "never-existed"))
		if _, ok := a.memoryScopeForChat(c2); ok {
			t.Fatal("a chat in a folder that never existed got a scope")
		}
	})
}

func TestMemoryAllowedFor(t *testing.T) {
	a := &App{}
	proj, plain := memory.ProjectScope("p_x"), memory.AllProjects
	defer beta.SetGate(nil)

	beta.SetGate(func() bool { return false })
	if a.memoryAllowedFor(proj, true) {
		t.Error("project chat in Code Mode must have no memory while Beta is off")
	}
	if !a.memoryAllowedFor(proj, false) {
		t.Error("a project chat with Code Mode off keeps its (project) memory")
	}
	if a.memoryAllowedFor(plain, true) || !a.memoryAllowedFor(plain, false) {
		t.Error("plain chats are unchanged: memory unless Code Mode")
	}

	beta.SetGate(func() bool { return true })
	if !a.memoryAllowedFor(proj, true) {
		t.Error("with Beta on a project chat in Code Mode gets its project memory")
	}
	if a.memoryAllowedFor(plain, true) {
		t.Error("Code Mode on a plain chat still has no memory")
	}
}

// TestBuildMessages_ProjectChatNeverSeesPersonalMemoryOrIdentity is the end to
// end privacy check: a project chat's prompt must carry the project's own
// memory, and nothing of the person's general memory, name or style.
func TestBuildMessages_ProjectChatNeverSeesPersonalMemoryOrIdentity(t *testing.T) {
	a, sm, home := projectScopeApp(t)
	defer beta.SetGate(nil)
	beta.SetGate(func() bool { return true })

	// Every text embeds to the same vector, so similarity can never be what
	// keeps a row out of a result — only the scope can.
	store, err := memory.NewStore(memory.StoreConfig{
		Dir: t.TempDir(), Dimension: 3,
		EmbeddingFunc: func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a.store = store
	a.identity.SetLearnedStyleNotes("PIRATE-STYLE-NOTE")

	pa := mkProject(t, home, "alpha")
	projChat := sm.NewAgentChat(pa)
	res, err := a.projectManager().Open(pa)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	mustSave := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	mustSave(store.SaveExplicit(ctx, memory.AllProjects, "The person's name is Tester-Secret", "profile"))
	mustSave(store.SaveInteraction(ctx, memory.AllProjects, "general-secret: my holiday in Rome", "nice"))
	mustSave(store.SaveInteraction(ctx, memory.ProjectScope("p_other"), "other-project-secret: client Acme", "ok"))
	mustSave(store.SaveInteraction(ctx, memory.ProjectScope(res.ID), "project-note: this repo indents with tabs", "ok"))

	prompt := func(chat string, codeOff bool) string {
		t.Helper()
		if codeOff {
			off := false
			if err := sm.SetCodeMode(chat, &off); err != nil {
				t.Fatal(err)
			}
		}
		ctx := context.Background()
		if !codeOff {
			ctx = withCodeMode(ctx)
		}
		msgs := a.buildMessagesForSession(ctx, chat, "what does this repo use for indentation?", nil, nil)
		var sb strings.Builder
		for _, m := range msgs {
			if s, ok := m.Content.(string); ok {
				sb.WriteString(s)
				sb.WriteString("\n")
			}
		}
		return sb.String()
	}
	_ = api.Message{}

	for _, tc := range []struct {
		name    string
		codeOff bool
	}{{"Code Mode on", false}, {"Code Mode off", true}} {
		t.Run(tc.name, func(t *testing.T) {
			got := prompt(projChat, tc.codeOff)
			if !strings.Contains(got, "project-note") {
				t.Errorf("the project's own memory is missing:\n%s", got)
			}
			for _, leak := range []string{"Tester-Secret", "general-secret", "other-project-secret", "PIRATE-STYLE-NOTE", "Tester"} {
				if strings.Contains(got, leak) {
					t.Errorf("LEAK %q in a project chat prompt:\n%s", leak, got)
				}
			}
		})
	}

	t.Run("a plain chat still sees every pool and the person's name", func(t *testing.T) {
		plain := sm.NewChat()
		got := prompt(plain, true)
		for _, want := range []string{"Tester-Secret", "general-secret", "project-note", "other-project-secret"} {
			if !strings.Contains(got, want) {
				t.Errorf("plain chat lost %q:\n%s", want, got)
			}
		}
	})

	t.Run("Beta off: a project chat in Code Mode has no memory at all", func(t *testing.T) {
		beta.SetGate(func() bool { return false })
		got := prompt(projChat, false)
		for _, none := range []string{"project-note", "Tester-Secret", "general-secret"} {
			if strings.Contains(got, none) {
				t.Errorf("Code Mode project chat with Beta off carried %q", none)
			}
		}
	})
}

func TestSaveMemorySync_ProjectTurnLandsInItsOwnPool(t *testing.T) {
	a, _, _ := projectScopeApp(t)
	store, err := memory.NewStore(memory.StoreConfig{
		Dir: t.TempDir(), Dimension: 3,
		EmbeddingFunc: func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a.store = store

	msg := "please remember that the deployment pipeline for this service runs on every merge to main"
	a.saveMemorySync(context.Background(), memory.ProjectScope("p_proj"), msg, "Understood, noted for this service.")

	ctx := context.Background()
	if got, _ := store.RetrieveContext(ctx, memory.ProjectScope("p_proj"), "pipeline", 5, 0); len(got) != 1 {
		t.Fatalf("project pool has %d rows, want 1", len(got))
	}
	if got, _ := store.RetrieveContext(ctx, memory.ProjectScope(""), "pipeline", 5, 0); len(got) != 0 {
		t.Fatalf("the project turn leaked into the general pool: %d rows", len(got))
	}
}
