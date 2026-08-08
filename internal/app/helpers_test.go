package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/memory"
	moodpkg "memo/internal/mood"
	"memo/internal/sessions"
	"memo/internal/skill"
)

func TestBuildMessages_MoodDisabled_StripsAssistant(t *testing.T) {
	id := identity.New("Test", "Memo", "casual", "", false)

	t.Run("mood_nil_strips_assistant", func(t *testing.T) {
		a := &App{
			cfg: &config.AppConfig{
				Memory: config.MemoryConfig{MemoryEnabled: false},
				Llama:  config.LlamaConfig{CtxSize: 4096},
			},
			identity: id,
			mood:     nil,
		}

		messages := a.buildMessages(context.Background(), "merhaba", nil)
		if len(messages) == 0 {
			t.Fatal("expected at least one message")
		}
		sys := messages[0]
		content, ok := sys.Content.(string)
		if !ok {
			t.Fatalf("expected string content, got %T", sys.Content)
		}
		if sys.Role != "system" {
			t.Fatalf("expected system role, got %s", sys.Role)
		}
		if !strings.Contains(content, "Memo") {
			t.Error("system prompt should contain assistant name")
		}
		if strings.Contains(content, "Current Emotional State") {
			t.Error("system prompt should NOT contain mood directive when mood is nil")
		}
	})

	t.Run("mood_enabled_false_strips_assistant", func(t *testing.T) {
		moodEngine, err := moodpkg.New(moodpkg.Config{
			Enabled: false,
			DBPath:  t.TempDir() + "/mood.db",
		})
		if err != nil {
			t.Fatalf("create mood engine: %v", err)
		}
		defer moodEngine.Close()

		a := &App{
			cfg: &config.AppConfig{
				Memory: config.MemoryConfig{MemoryEnabled: false},
				Llama:  config.LlamaConfig{CtxSize: 4096},
			},
			identity: id,
			mood:     moodEngine,
		}

		messages := a.buildMessages(context.Background(), "test message", nil)
		content := messages[0].Content.(string)

		// When mood is disabled NOTHING mood-related may be injected — the model
		// is driven solely by the configured system prompt.
		if strings.Contains(content, "nötr moddasın") {
			t.Error("system prompt must NOT contain the neutral mood block when mood is disabled")
		}
		if strings.Contains(content, "Current Emotional State") {
			t.Error("system prompt must NOT contain mood directive when mood is disabled")
		}
		if !strings.Contains(content, "Memo") {
			t.Error("system prompt should still contain the base identity")
		}
	})

	t.Run("mood_disabled_memory_strips_assistant", func(t *testing.T) {
		memContent := "[2026-06-20] User: Test question\nAssistant: Old cold response"
		memories := []memory.MemoryResult{
			{Content: memContent, Similarity: 0.9},
		}

		formatted := memory.FormatMemoriesUserOnly(memories)
		if strings.Contains(formatted, "Old cold response") {
			t.Error("FormatMemoriesUserOnly should strip assistant replies")
		}
		if !strings.Contains(formatted, "Test question") {
			t.Error("FormatMemoriesUserOnly should keep user messages")
		}

		formattedFull := memory.FormatMemoriesForPrompt(memories)
		if !strings.Contains(formattedFull, "Old cold response") {
			t.Error("FormatMemoriesForPrompt should include assistant replies")
		}
	})

	t.Run("mood_enabled_true_includes_directives", func(t *testing.T) {
		moodEngine, err := moodpkg.New(moodpkg.Config{
			Enabled:  true,
			DBPath:   t.TempDir() + "/mood.db",
			Alpha:    0.95,
			Beta:     0.80,
			SigmaMin: 0.0,
			SigmaMax: 0.0,
		})
		if err != nil {
			t.Fatalf("create mood engine: %v", err)
		}
		defer moodEngine.Close()

		// Push score high enough to be non-neutral (> 2.0)
		for i := 0; i < 5; i++ {
			moodEngine.Update(context.Background(), 10.0)
		}

		a := &App{
			cfg: &config.AppConfig{
				Memory: config.MemoryConfig{MemoryEnabled: false},
				Llama:  config.LlamaConfig{CtxSize: 4096},
			},
			identity: id,
			mood:     moodEngine,
		}

		messages := a.buildMessages(context.Background(), "test", nil)
		content := messages[0].Content.(string)

		score := moodEngine.Score()
		t.Logf("mood score: %.2f", score)

		if strings.Contains(content, "nötr moddasın") {
			t.Error("system prompt should NOT contain neutral directive when mood is enabled")
		}
		// With β=0.80 and 5 updates at +10, score should be well above 2.0 (neutral threshold)
		if score <= 2.0 {
			t.Skip("score still neutral, skipping label check")
		}
		if !strings.Contains(content, "Current Emotional State") {
			t.Error("system prompt should contain mood directive when mood score is non-neutral")
		}
	})
}

// TestBuildMessages_MinimalModeReadsIdentityNotStaleConfigCopy is the
// regression guard for BUG-M3: buildMessages used to decide whether to skip
// mood/web-search injection by reading a.cfg.Identity.MinimalMode, a
// separate copy of the same setting kept "in sync" by SetMinimalMode's two
// non-atomic writes (a.cfg.Identity.MinimalMode = enabled; then
// a.identity.SetMinimalMode(enabled)) — while identity.BuildSystemPrompt
// decided whether to skip the identity block by reading a.identity's own
// copy. A toggle landing between those two writes (or two readers simply
// racing on the unsynchronized fields) could apply the two decisions
// inconsistently. Fixed by having both decisions read a.identity's single,
// now lock-protected MinimalMode field. This test deliberately leaves
// a.cfg.Identity.MinimalMode == true (stale/never synced) while
// a.identity's own MinimalMode stays false, and asserts mood injection
// still happens — proving buildMessages no longer consults the config copy.
func TestBuildMessages_MinimalModeReadsIdentityNotStaleConfigCopy(t *testing.T) {
	id := identity.New("Test", "Memo", "casual", "", false) // a.identity.MinimalMode = false

	moodEngine, err := moodpkg.New(moodpkg.Config{
		Enabled:  true,
		DBPath:   t.TempDir() + "/mood.db",
		Alpha:    0.95,
		Beta:     0.80,
		SigmaMin: 0.0,
		SigmaMax: 0.0,
	})
	if err != nil {
		t.Fatalf("create mood engine: %v", err)
	}
	defer moodEngine.Close()

	// Push score non-neutral — BuildDirective returns "" at the neutral
	// label regardless of Enabled, so a neutral score can't distinguish
	// "correctly injected" from "wrongly skipped".
	ctx := context.Background()
	for range 5 {
		moodEngine.Update(ctx, 10.0)
	}
	if score := moodEngine.Score(); score <= 2.0 {
		t.Skip("score still neutral, skipping")
	}

	a := &App{
		cfg: &config.AppConfig{
			Identity: config.IdentityConfig{MinimalMode: true}, // deliberately out of sync
			Memory:   config.MemoryConfig{MemoryEnabled: false},
			Llama:    config.LlamaConfig{CtxSize: 4096},
		},
		identity: id,
		mood:     moodEngine,
	}

	messages := a.buildMessages(ctx, "test", nil)
	content := messages[0].Content.(string)

	if !strings.Contains(content, "Current Emotional State") {
		t.Error("expected mood directive to still be injected: a.identity.MinimalMode is false, " +
			"so buildMessages must not defer to the stale a.cfg.Identity.MinimalMode=true copy")
	}
}

func TestBuildMessages_MemoryEnabledNotCrash(t *testing.T) {
	id := identity.New("Test", "Memo", "casual", "", false)
	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: true},
			Llama:  config.LlamaConfig{CtxSize: 4096},
		},
		identity: id,
	}

	messages := a.buildMessages(context.Background(), "hello", nil)
	if len(messages) == 0 {
		t.Fatal("expected messages even when memory retrieval fails")
	}
	if messages[0].Role != "system" {
		t.Errorf("expected system role, got %s", messages[0].Role)
	}
}

// TestBuildMessages_AgentModeSkipsBlindWebSearchInjection is a regression
// test: buildMessagesForSession used to run websearch.Search on every
// single message whenever the web-search toggle was on, regardless of
// agent mode — reported directly by a user as web search firing
// indiscriminately even for "naber" — even though agent mode already
// registers its own web_search tool (internal/agent/tools.go) whose
// description tells the model to call it only when actually needed. With
// agent mode also on, the blind injection is pure redundant network traffic
// on every turn; it must not run. context.Background() (not a cancelled
// one) is used deliberately — a cancelled context would make
// websearch.Search fail fast for an unrelated reason (ctx error), which
// would let this test pass even if the actual skip-when-agentEnabled logic
// regressed. A short wall-clock budget catches that instead: a live DDG
// HTTP call takes far longer than this to complete or fail.
func TestBuildMessages_AgentModeSkipsBlindWebSearchInjection(t *testing.T) {
	id := identity.New("Test", "Memo", "casual", "", false)
	a := &App{
		cfg: &config.AppConfig{
			Memory:    config.MemoryConfig{MemoryEnabled: false},
			Llama:     config.LlamaConfig{CtxSize: 4096},
			WebSearch: config.WebSearchConfig{Enabled: true},
		},
		identity:     id,
		agentEnabled: true,
	}

	start := time.Now()
	messages := a.buildMessages(context.Background(), "naber", nil)
	elapsed := time.Since(start)

	if elapsed > 200*time.Millisecond {
		t.Fatalf("buildMessages took %v — suggests a live web search was attempted despite agent mode being on", elapsed)
	}
	sys, ok := messages[0].Content.(string)
	if !ok {
		t.Fatal("expected system message content to be a string")
	}
	if strings.Contains(sys, "Web Search Results") {
		t.Error("system prompt contains injected web search results — blind injection must be skipped when agent mode is on")
	}
}

// TestRetrieveMemory_MergesInPinnedFactsRegardlessOfRanking proves pinned
// (explicit) facts are injected unconditionally, not just when they happen
// to win RAG's similarity ranking. TopK is set to 0, which makes
// RetrieveContext return nil immediately (see store.go's `if topK <= 0`
// guard) — so the only way the explicit fact can appear in retrieveMemory's
// result is via the GetPinnedFacts merge step, not RAG.
func TestRetrieveMemory_MergesInPinnedFactsRegardlessOfRanking(t *testing.T) {
	store, err := memory.NewStore(memory.StoreConfig{
		Dir:       t.TempDir(),
		Dimension: 4,
		EmbeddingFunc: func(_ context.Context, _ string) ([]float32, error) {
			return []float32{1, 0, 0, 0}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveExplicit(context.Background(), "kullanicinin adi Ahmet", "profile"); err != nil {
		t.Fatalf("SaveExplicit() error = %v", err)
	}

	a := &App{
		store: store,
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: true, TopK: 0, MinSimilarity: 0},
		},
	}

	results := a.retrieveMemory(context.Background(), "irrelevant query")
	found := false
	for _, r := range results {
		if strings.Contains(r.Content, "Ahmet") {
			found = true
		}
	}
	if !found {
		t.Fatalf("pinned fact missing when RAG (topK=0) excludes everything: %+v", results)
	}
}

// TestRetrieveMemory_PinnedFactsComeFirst is a regression test: pinned facts
// must be placed BEFORE RAG results in the returned slice, not appended
// after. identity.BuildSystemPrompt truncates the formatted memory block
// from the tail once it exceeds its token budget — if pinned facts were
// appended last (as an earlier version of this code did), they'd be the
// first thing dropped for exactly the heavy users (large RAG history) who
// most need them protected. Confirmed by temporarily reverting to
// append-at-end during development: this test failed (pinned fact ended up
// last) before the fix, passes now.
func TestRetrieveMemory_PinnedFactsComeFirst(t *testing.T) {
	store, err := memory.NewStore(memory.StoreConfig{
		Dir:       t.TempDir(),
		Dimension: 4,
		EmbeddingFunc: func(_ context.Context, _ string) ([]float32, error) {
			return []float32{1, 0, 0, 0}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveInteraction(context.Background(), "kanka naber", "iyilik"); err != nil {
		t.Fatalf("SaveInteraction() error = %v", err)
	}
	if err := store.SaveExplicit(context.Background(), "kullanicinin adi Ahmet", "profile"); err != nil {
		t.Fatalf("SaveExplicit() error = %v", err)
	}

	a := &App{
		store: store,
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: true, TopK: 5, MinSimilarity: 0},
		},
	}

	results := a.retrieveMemory(context.Background(), "naber")
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results (routine + pinned), got %d: %+v", len(results), results)
	}
	if results[0].Source != "explicit" {
		t.Fatalf("results[0].Source = %q, want explicit — pinned facts must come first so tail-truncation hits RAG results, not pinned facts: %+v", results[0].Source, results)
	}
}

// TestBuildMessagesForSession_IgnoresConcurrentActiveChatSwitch is a
// regression test for BUG-H1: buildMessages() used to read history from
// whatever chat was globally active *at call time* — if the active chat
// changed between building the prompt and writing the reply (a user
// switching chats mid-stream), a response built from one chat's history
// could be appended to a different, newly-active chat. The streaming entry
// points in chat.go now capture a chatID once, up front, and pass it to
// buildMessagesForSession explicitly — this must stay anchored to that
// chatID no matter what SwitchChat does afterward.
func TestBuildMessagesForSession_IgnoresConcurrentActiveChatSwitch(t *testing.T) {
	id := identity.New("Test", "Memo", "casual", "", false)
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	chatA := sm.GetActiveID()
	sm.AddMessageToSession(chatA, "user", "chat A history", "", "")
	chatB := sm.NewChat()
	sm.AddMessageToSession(chatB, "user", "chat B history", "", "")

	// The exact race: chatID was captured as chatA, but the globally active
	// chat has since moved to chatB — e.g. the user switched chats while
	// this call was building its prompt.
	if err := sm.SwitchChat(chatB); err != nil {
		t.Fatalf("SwitchChat() error = %v", err)
	}

	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: false},
			Llama:  config.LlamaConfig{CtxSize: 4096},
		},
		identity: id,
		sessions: sm,
	}

	messages := a.buildMessagesForSession(context.Background(), chatA, "new message", nil)

	var foundA bool
	for _, m := range messages {
		content, ok := m.Content.(string)
		if !ok {
			continue
		}
		if strings.Contains(content, "chat A history") {
			foundA = true
		}
		if strings.Contains(content, "chat B history") {
			t.Fatalf("buildMessagesForSession(chatA, ...) leaked chat B's history into the prompt: %q", content)
		}
	}
	if !foundA {
		t.Fatal("buildMessagesForSession(chatA, ...) did not include chat A's own history")
	}
}

// TestBuildMessagesForSession_IncludesActiveSkillInstructions is the
// regression test for the bug found while investigating "do activated
// skills actually reach the model": buildActiveSkillPrompt() used to be
// appended by routeStream (chat.go) and callAgentWithOrchestra (llm.go)
// *after* buildMessagesForSession had already built the message list, by
// searching for a `role: "system"` message to append onto. That search
// silently found nothing whenever a.llamaServer was running: the local-
// model branch of buildMessagesForSession never emits a `role: "system"`
// message at all (it merges systemPrompt into a user-role message
// instead) — so an active skill's instructions never reached the local
// model, with no error or indication anywhere that anything was wrong.
//
// Fixed by baking buildActiveSkillPrompt()'s output into systemPrompt
// itself, before either branch below decides how to fold it into the
// outgoing messages — this test covers the a.llamaServer == nil branch
// (real system-role message), which exercises the same systemPrompt
// variable the local-model branch also uses. The local-model branch
// itself needs a real, live *llama.Server (IsRunning() checks an actual
// OS process) and isn't practical to exercise in a plain unit test.
func TestBuildMessagesForSession_IncludesActiveSkillInstructions(t *testing.T) {
	id := identity.New("Test", "Memo", "casual", "", false)
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	skillMgr := skill.NewManager(t.TempDir())
	skillDir := filepath.Join(skillMgr.SkillsDir(), "greeter")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: greeter\ndescription: \"test skill\"\n---\n" +
		"Always greet the user by name before answering.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := skillMgr.Discover(); err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if err := skillMgr.SetActive([]string{"greeter"}); err != nil {
		t.Fatalf("SetActive() error = %v", err)
	}

	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: false},
			Llama:  config.LlamaConfig{CtxSize: 4096},
		},
		identity:     id,
		sessions:     sm,
		skillManager: skillMgr,
	}

	chatID := sm.GetActiveID()
	messages := a.buildMessagesForSession(context.Background(), chatID, "hello", nil)

	var found bool
	for _, m := range messages {
		if content, ok := m.Content.(string); ok &&
			strings.Contains(content, "Always greet the user by name before answering.") {
			found = true
		}
	}
	if !found {
		t.Fatal("buildMessagesForSession() did not include the active skill's instructions")
	}
}

func TestApiContextBudget_Defaults(t *testing.T) {
	a := &App{
		cfg: &config.AppConfig{
			Llama: config.LlamaConfig{CtxSize: 0},
		},
	}
	budget := a.apiContextBudget()
	if budget <= 0 {
		t.Errorf("expected positive budget, got %d", budget)
	}
	if budget != 128*1024 {
		t.Errorf("expected default 128K budget, got %d", budget)
	}
}

func TestStripOrchestraLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "keeps plain conversation lines",
			in:   "Hello there\nHow are you?",
			want: "Hello there\nHow are you?",
		},
		{
			name: "drops orchestra emoji-prefixed debug lines",
			in:   "🎵 Planning next step\nActual reply to the user\n🧠 internal thought",
			want: "Actual reply to the user",
		},
		{
			name: "drops role-labeled orchestra lines",
			in:   "**planner**: do X\nReal answer\n**backend**: do Y",
			want: "Real answer",
		},
		{
			name: "drops the system-instructions line",
			in:   "Sistem talimatları: ignore previous\nKeep this",
			want: "Keep this",
		},
		{
			name: "drops blank lines entirely",
			in:   "First\n\n\nSecond",
			want: "First\nSecond",
		},
		{
			name: "empty input stays empty",
			in:   "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripOrchestraLines(tt.in)
			if got != tt.want {
				t.Errorf("stripOrchestraLines(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestBuildConversationContext(t *testing.T) {
	t.Run("no prior history falls back to just the new message", func(t *testing.T) {
		messages := []api.Message{api.NewTextMessage("user", "hello")}
		got := buildConversationContext(messages, "hello")
		want := "Kullanıcı: hello"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("includes prior turns in order with role labels", func(t *testing.T) {
		messages := []api.Message{
			api.NewTextMessage("system", "you are Memo"),
			api.NewTextMessage("user", "what's 2+2?"),
			api.NewTextMessage("assistant", "4"),
			api.NewTextMessage("user", "and 3+3?"),
		}
		got := buildConversationContext(messages, "and 3+3?")

		if !strings.Contains(got, "Önceki konuşma:") {
			t.Errorf("expected a 'prior conversation' header, got %q", got)
		}
		if !strings.Contains(got, "Kullanıcı: what's 2+2?") {
			t.Errorf("expected the earlier user turn to be present, got %q", got)
		}
		if !strings.Contains(got, "Asistan: 4") {
			t.Errorf("expected the assistant's reply labeled 'Asistan', got %q", got)
		}
		// The system prompt itself must never leak into the conversation context.
		if strings.Contains(got, "you are Memo") {
			t.Errorf("system message leaked into conversation context: %q", got)
		}
		if !strings.HasSuffix(got, "Yeni mesaj:\nKullanıcı: and 3+3?") {
			t.Errorf("expected the new message to be appended last, got %q", got)
		}
	})

	t.Run("non-string content is skipped without panicking", func(t *testing.T) {
		messages := []api.Message{
			api.NewTextMessage("user", "first"),
			api.NewMultimodalMessage("user", "second", "base64imagedata"),
		}
		got := buildConversationContext(messages, "second")
		if !strings.Contains(got, "first") {
			t.Errorf("expected the plain-text turn to survive, got %q", got)
		}
	})
}

func TestDetectMime(t *testing.T) {
	tests := []struct {
		name string
		path string
		data []byte
		want string
	}{
		{name: "png by extension", path: "photo.PNG", data: []byte{0xFF}, want: "image/png"},
		{name: "gif by extension", path: "anim.gif", data: nil, want: "image/gif"},
		{name: "webp by extension", path: "sticker.webp", data: nil, want: "image/webp"},
		{name: "bmp by extension", path: "scan.bmp", data: nil, want: "image/bmp"},
		{
			name: "unknown extension sniffs real jpeg content",
			path: "upload.tmp",
			data: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46},
			want: "image/jpeg",
		},
		{
			name: "unrecognizable binary content falls back to jpeg",
			path: "upload.tmp",
			data: []byte{0x00, 0x01, 0x02, 0x03},
			want: "image/jpeg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectMime(tt.path, tt.data)
			if got != tt.want {
				t.Errorf("detectMime(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
