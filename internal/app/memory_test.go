package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/memory"
	"memo/internal/provider"
)

func TestIsLLMErrorReply(t *testing.T) {
	cases := []struct {
		reply string
		want  bool
	}{
		{"⚠️ Yerel model yüklenmemiş. Lütfen bir model başlatın veya API sağlayıcı seçin.", true},
		{"⚠️ Empty response", true},
		{"⚠️ Cannot read image: open x: no such file", true},
		{"Merhaba, nasıl yardımcı olabilirim?", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isLLMErrorReply(c.reply); got != c.want {
			t.Errorf("isLLMErrorReply(%q) = %v, want %v", c.reply, got, c.want)
		}
	}
}

// TestSaveMemoryAsyncSkipsErrorReplies is a regression test for BUG-H7:
// callLLM's synthetic "⚠️ ..." error strings (model not loaded, provider
// error, unreadable attachment, etc.) used to be saved into RAG memory like
// any genuine reply, polluting future retrieval with error noise. They must
// never reach memorySaveCh.
func TestSaveMemoryAsyncSkipsErrorReplies(t *testing.T) {
	a := &App{
		cfg:          &config.AppConfig{Memory: config.MemoryConfig{MemoryEnabled: true}},
		memorySaveCh: make(chan saveTask, 1),
	}

	a.saveMemoryAsync("kullanıcı mesajı", "⚠️ Yerel model yüklenmemiş. Lütfen bir model başlatın veya API sağlayıcı seçin.")

	select {
	case task := <-a.memorySaveCh:
		t.Fatalf("error reply must not be queued for memory save, got %+v", task)
	case <-time.After(50 * time.Millisecond):
		// expected: nothing queued
	}

	a.saveMemoryAsync("kullanıcı mesajı", "gerçek bir yanıt")

	select {
	case task := <-a.memorySaveCh:
		if task.reply != "gerçek bir yanıt" {
			t.Errorf("reply = %q, want %q", task.reply, "gerçek bir yanıt")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("genuine reply should have been queued for memory save")
	}
}

// TestSaveMemoryAsync_ClosedChannelDoesNotPanic is the regression test for
// BUG_REPORT.md's RC-7: Shutdown() closes memorySaveCh once webSrv.Stop()
// returns, but that only waits on each HTTP handler's own call stack — a
// streaming reply's own detached goroutine can still be mid-finishStream
// when that happens, and finishStream calls saveMemoryAsync synchronously.
// A send on the now-closed channel used to panic with no recover of its
// own; whichever unrelated streaming goroutine happened to be running the
// call would catch it (via that goroutine's own recoverStreamPanic) and
// misreport it as a generic "internal error" for that turn, with the
// memory save itself silently lost either way. saveMemoryAsync must
// recover its own send instead of panicking through to its caller.
func TestSaveMemoryAsync_ClosedChannelDoesNotPanic(t *testing.T) {
	a := &App{
		cfg:          &config.AppConfig{Memory: config.MemoryConfig{MemoryEnabled: true}},
		memorySaveCh: make(chan saveTask, 1),
	}
	close(a.memorySaveCh)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("saveMemoryAsync panicked instead of recovering its own closed-channel send: %v", r)
		}
	}()
	a.saveMemoryAsync("kullanıcı mesajı", "gerçek bir yanıt")
}

func TestParseExtractedFacts(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"none, exact", "NONE", nil},
		{"none, case-insensitive with whitespace", "  none  \n", nil},
		{"single fact", "User's name is Ahmet", []string{"User's name is Ahmet"}},
		{
			"multiple facts, one per line",
			"User's dog is named Zeytin\nUser's favorite color is red",
			[]string{"User's dog is named Zeytin", "User's favorite color is red"},
		},
		{
			"dash-prefixed lines are cleaned",
			"- User's dog is named Zeytin\n- User's cat is named Pasha",
			[]string{"User's dog is named Zeytin", "User's cat is named Pasha"},
		},
		{
			"numbered lines are cleaned despite the 'no numbering' instruction",
			"1. User's dog is named Zeytin\n2. User's cat is named Pasha",
			[]string{"User's dog is named Zeytin", "User's cat is named Pasha"},
		},
		{
			"parenthesis-numbered lines are cleaned",
			"1) User's name is Ahmet",
			[]string{"User's name is Ahmet"},
		},
		{
			"asterisk/bullet-prefixed lines are cleaned",
			"* User's name is Ahmet\n• User's job is engineer",
			[]string{"User's name is Ahmet", "User's job is engineer"},
		},
		{
			"blank lines are skipped",
			"User's name is Ahmet\n\n\nUser's job is engineer",
			[]string{"User's name is Ahmet", "User's job is engineer"},
		},
		{"empty response", "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseExtractedFacts(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("parseExtractedFacts(%q) = %v, want %v", tc.raw, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("fact[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestParseExtractedFacts_CapsCountAndLength is a regression guard against a
// hallucinating or misbehaving model flooding the pinned-facts list: even if
// the model ignores the "one fact per line" / length guidance entirely, the
// parser must still bound both how many facts come out and how long each one is.
func TestParseExtractedFacts_CapsCountAndLength(t *testing.T) {
	var lines []string
	for i := range maxExtractedFactsPerTurn + 10 {
		lines = append(lines, fmt.Sprintf("fact number %d", i))
	}
	got := parseExtractedFacts(strings.Join(lines, "\n"))
	if len(got) != maxExtractedFactsPerTurn {
		t.Fatalf("len(facts) = %d, want %d (cap must hold)", len(got), maxExtractedFactsPerTurn)
	}

	longLine := strings.Repeat("x", maxExtractedFactLength+100)
	got = parseExtractedFacts(longLine)
	if len(got) != 1 || len(got[0]) != maxExtractedFactLength {
		t.Fatalf("long fact not truncated to %d chars, got len=%d", maxExtractedFactLength, len(got[0]))
	}
}

// TestExtractAndPinFacts_DoesNotSendAssistantReply is the regression test for
// BUG-H4: the extraction call used to send "User: ...\nAssistant: ..." with
// the full assistant reply included, so a reply narrating third-party
// information the assistant read via a tool call (e.g. WhatsApp group
// contents) got pinned as if it were a durable fact about the user. This
// asserts the outbound extraction request only ever contains the user's own
// message, never the assistant's reply text.
func TestExtractAndPinFacts_DoesNotSendAssistantReply(t *testing.T) {
	store := newExtractionTestStore(t)

	const thirdPartyReply = "TEKNOFEST-MEBROBOT MSE grubu → Sunum bitmiş, proje tasarımına başlanacak"
	const userMsg = "whatsapp sohbet listeme bak"

	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"NONE"}}]}`)
	}))
	defer srv.Close()

	router := provider.NewRouter([]provider.ProviderConfig{{
		Type: provider.ProviderCustom, Name: "test", BaseURL: srv.URL, Model: "test-model", Enabled: true,
	}})

	a := &App{
		store:              store,
		providerRouter:     router,
		activeProviderName: "test",
		cfg:                &config.AppConfig{Memory: config.MemoryConfig{AutoFactExtraction: true}},
	}

	a.extractAndPinFacts(context.Background(), userMsg)

	if !strings.Contains(capturedBody, userMsg) {
		t.Fatalf("extraction request should contain the user's own message; body=%s", capturedBody)
	}
	if strings.Contains(capturedBody, thirdPartyReply) {
		t.Fatalf("extraction request must NOT contain the assistant's reply text (third-party leak risk); body=%s", capturedBody)
	}
}

// TestExtractAndPinFacts_SkipsAlreadyPinnedDuplicate is the regression test
// for BUG-M6: the same durable fact ("User's name is Ece.") got re-extracted
// and re-pinned on every turn it was still relevant to, live-verified to
// happen 4 times identically in a single persona test. Since pinned facts
// bypass RAG ranking and have a fixed cap (pinnedFactsLimit), unchecked
// duplicates crowd out real one-off facts.
func TestExtractAndPinFacts_SkipsAlreadyPinnedDuplicate(t *testing.T) {
	store := newExtractionTestStore(t)

	// Pre-pin the fact as if it were extracted on an earlier turn.
	if err := store.SaveExplicit(context.Background(), "User's name is Ece.", "auto-extracted"); err != nil {
		t.Fatalf("SaveExplicit() error = %v", err)
	}

	// This turn's extraction re-derives the exact same fact (same wording,
	// as observed live) plus one genuinely new fact.
	router := newExtractionTestRouter(t, "User's name is Ece.\nUser's favorite color is orange")

	a := &App{
		store:              store,
		providerRouter:     router,
		activeProviderName: "test",
		cfg:                &config.AppConfig{Memory: config.MemoryConfig{AutoFactExtraction: true}},
	}

	a.extractAndPinFacts(context.Background(), "adım Ece, en sevdiğim renk turuncu")

	pinned, err := store.GetPinnedFacts(context.Background())
	if err != nil {
		t.Fatalf("GetPinnedFacts() error = %v", err)
	}
	if len(pinned) != 2 {
		t.Fatalf("len(pinned) = %d, want 2 (1 pre-existing + 1 new, duplicate skipped): %+v", len(pinned), pinned)
	}
	nameCount := 0
	for _, p := range pinned {
		if p.Content == "User's name is Ece." {
			nameCount++
		}
	}
	if nameCount != 1 {
		t.Errorf("\"User's name is Ece.\" pinned %d times, want exactly 1 (duplicate must be skipped)", nameCount)
	}
}

// newExtractionTestRouter builds a real *provider.Router pointed at an
// httptest server that returns responseContent as the chat completion's
// message content — the same pattern TestCallLLMStream_ExternalProvider_*
// (llm_test.go) uses to test provider-router-calling App methods without a
// live external service.
func newExtractionTestRouter(t *testing.T, responseContent string) *provider.Router {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, responseContent)
	}))
	t.Cleanup(srv.Close)

	return provider.NewRouter([]provider.ProviderConfig{{
		Type:    provider.ProviderCustom,
		Name:    "test",
		BaseURL: srv.URL,
		Model:   "test-model",
		Enabled: true,
	}})
}

func newExtractionTestStore(t *testing.T) *memory.Store {
	t.Helper()
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
	t.Cleanup(func() { store.Close() })
	return store
}

func TestExtractAndPinFacts_SavesEachExtractedFact(t *testing.T) {
	store := newExtractionTestStore(t)
	router := newExtractionTestRouter(t, "User's dog is named Zeytin\nUser's favorite color is red")

	a := &App{
		store:              store,
		providerRouter:     router,
		activeProviderName: "test",
		cfg:                &config.AppConfig{Memory: config.MemoryConfig{AutoFactExtraction: true}},
	}

	a.extractAndPinFacts(context.Background(), "kopeğimin adı Zeytin, en sevdiğim renk kırmızı")

	pinned, err := store.GetPinnedFacts(context.Background())
	if err != nil {
		t.Fatalf("GetPinnedFacts() error = %v", err)
	}
	if len(pinned) != 2 {
		t.Fatalf("len(pinned) = %d, want 2: %+v", len(pinned), pinned)
	}
	for _, p := range pinned {
		if p.Source != "explicit" {
			t.Errorf("Source = %q, want explicit", p.Source)
		}
	}
}

func TestExtractAndPinFacts_NoneResponsePinsNothing(t *testing.T) {
	store := newExtractionTestStore(t)
	router := newExtractionTestRouter(t, "NONE")

	a := &App{
		store:              store,
		providerRouter:     router,
		activeProviderName: "test",
		cfg:                &config.AppConfig{Memory: config.MemoryConfig{AutoFactExtraction: true}},
	}

	a.extractAndPinFacts(context.Background(), "selam")

	pinned, err := store.GetPinnedFacts(context.Background())
	if err != nil {
		t.Fatalf("GetPinnedFacts() error = %v", err)
	}
	if len(pinned) != 0 {
		t.Fatalf("len(pinned) = %d, want 0 for a NONE response: %+v", len(pinned), pinned)
	}
}

// TestExtractAndPinFacts_DisabledConfigNeverCallsProvider is a regression
// guard for the config kill-switch: when AutoFactExtraction is false,
// extraction must not run at all — not even a provider call — since every
// call has a real cost (API billing, local inference cycles) that must stop
// completely when the user opts out.
func TestExtractAndPinFacts_DisabledConfigNeverCallsProvider(t *testing.T) {
	store := newExtractionTestStore(t)
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		fmt.Fprint(w, `{"choices":[{"message":{"content":"User's name is Ahmet"}}]}`)
	}))
	defer srv.Close()
	router := provider.NewRouter([]provider.ProviderConfig{{
		Type: provider.ProviderCustom, Name: "test", BaseURL: srv.URL, Model: "test-model", Enabled: true,
	}})

	a := &App{
		store:              store,
		providerRouter:     router,
		activeProviderName: "test",
		cfg:                &config.AppConfig{Memory: config.MemoryConfig{AutoFactExtraction: false}},
	}

	a.extractAndPinFacts(context.Background(), "adım Ahmet")

	if hit {
		t.Fatal("provider must not be called when AutoFactExtraction is disabled")
	}
	pinned, _ := store.GetPinnedFacts(context.Background())
	if len(pinned) != 0 {
		t.Fatalf("len(pinned) = %d, want 0", len(pinned))
	}
}

// TestExtractAndPinFacts_NoModelConfigured_SkipsGracefully guards against a
// completely unconfigured App (no provider router, no local client — e.g.
// first run before any model is set up) causing a panic in a background
// worker. It also exercises the real routing path: extractAndPinFacts now
// goes through callLLM like every other LLM call in this package, so with
// nothing configured it should get callLLM's own local model's
// "not loaded" error reply and skip cleanly, not a bespoke nil check.
func TestExtractAndPinFacts_NoModelConfigured_SkipsGracefully(t *testing.T) {
	store := newExtractionTestStore(t)
	a := &App{
		store: store,
		cfg:   &config.AppConfig{Memory: config.MemoryConfig{AutoFactExtraction: true}},
	}

	a.extractAndPinFacts(context.Background(), "adım Ahmet")

	pinned, _ := store.GetPinnedFacts(context.Background())
	if len(pinned) != 0 {
		t.Fatalf("len(pinned) = %d, want 0", len(pinned))
	}
}

// TestExtractAndPinFacts_LocalOnlySetup_ActuallyRuns is the direct
// regression test for the bug this routing fix closes: extraction used to
// call a.providerRouter.ChatCompletion directly, so a local-only setup (no
// external provider configured — a.providerRouter nil, only the local
// llama.cpp client set, exactly this app's core "local-first" use case) made
// extraction a silent, permanent no-op. Now that it routes through callLLM,
// it must reach callLLM's local-model branch and actually extract/pin a fact
// using only a.client — the same field every other local-model chat
// completion in this app goes through.
func TestExtractAndPinFacts_LocalOnlySetup_ActuallyRuns(t *testing.T) {
	store := newExtractionTestStore(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"content":"User's name is Ahmet"}}]}`)
	}))
	defer srv.Close()

	a := &App{
		store:  store,
		client: api.NewClient(srv.URL, 30),
		cfg:    &config.AppConfig{Memory: config.MemoryConfig{AutoFactExtraction: true}},
	}

	a.extractAndPinFacts(context.Background(), "adım Ahmet")

	pinned, err := store.GetPinnedFacts(context.Background())
	if err != nil {
		t.Fatalf("GetPinnedFacts() error = %v", err)
	}
	if len(pinned) != 1 {
		t.Fatalf("len(pinned) = %d, want 1 — extraction must reach the local-model branch when providerRouter is nil", len(pinned))
	}
}

// TestDebugMemorySearch_IncludesPinnedFacts is a regression test: the
// Settings > Bellek Ara debug panel used to call Store.DebugSearch directly,
// which never surfaces pinned (source='explicit') facts distinctly — a
// pinned fact would only show up if it also happened to score well on the
// underlying hybrid vector/FTS search, mislabeled as a plain "vector"/"fts"
// match with no indication it's actually guaranteed to be injected into
// every real chat prompt. DebugMemorySearch must merge in GetPinnedFacts the
// same way retrieveMemory does for the real chat path.
func TestDebugMemorySearch_IncludesPinnedFacts(t *testing.T) {
	store := newExtractionTestStore(t)
	if err := store.SaveExplicit(context.Background(), "kullanicinin adi Ahmet", "profile"); err != nil {
		t.Fatalf("SaveExplicit() error = %v", err)
	}

	a := &App{store: store}

	results := a.DebugMemorySearch("tamamen alakasiz bir sorgu")
	found := false
	for _, r := range results {
		if r.MatchType == "pinned" && strings.Contains(r.Content, "Ahmet") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the pinned fact labeled MatchType=pinned in debug search results, got %+v", results)
	}
}
