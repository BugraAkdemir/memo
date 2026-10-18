package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"memo/internal/provider"
)

// streamingScripted answers each model call from a script of streams; it also
// implements ChatCompletion so a fallback can be observed.
type streamingScripted struct {
	mu         sync.Mutex
	streams    [][]provider.StreamChunk
	streamErr  error // returned by ChatCompletionStream instead of a stream
	blocking   []provider.ChatResponse
	streamed   int
	blockCalls int
	sawStream  []bool
}

func (p *streamingScripted) ChatCompletionStream(_ context.Context, req provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sawStream = append(p.sawStream, req.Stream)
	if p.streamErr != nil {
		return nil, p.streamErr
	}
	chunks := p.streams[p.streamed]
	p.streamed++
	ch := make(chan provider.StreamChunk, len(chunks)+1)
	for _, c := range chunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

func (p *streamingScripted) ChatCompletion(_ context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sawStream = append(p.sawStream, req.Stream)
	resp := p.blocking[p.blockCalls]
	p.blockCalls++
	return &resp, nil
}

func newStreamTestPipeline(t *testing.T, prov AgentProvider) (*Pipeline, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	pl := NewPipeline(NewRegistry(), NewPermissionManager(t.TempDir()), NewSandbox(DefaultSandboxConfig(dir)), prov, nil)
	pl.autoPermission = true
	return pl, dir
}

// run drains a turn and returns its events (in order), content and thinking.
func runStreamTurn(t *testing.T, ctx context.Context, pl *Pipeline) (events []AgentEvent, content, thinking string, last provider.StreamChunk) {
	t.Helper()
	var mu sync.Mutex
	ch, err := pl.RunStream(ctx, nil, "m", func(ev AgentEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var cb, kb strings.Builder
	for c := range ch {
		cb.WriteString(c.Content)
		kb.WriteString(c.Thinking)
		last = c
	}
	return events, cb.String(), kb.String(), last
}

// The text a step streams while it also calls a tool is NARRATION: it is shown
// live (deltas), then kept in order with the tool events (one narration event
// before tool_executing). The next step's text is the final answer and travels
// as the ordinary Content chunk.
func TestRunStream_StreamsNarrationThenToolThenFinalAnswer(t *testing.T) {
	prov := &streamingScripted{streams: [][]provider.StreamChunk{
		{
			{Thinking: "önce dosyaya "},
			{Thinking: "bakmalıyım"},
			{Content: "Dosyayı "},
			{Content: "okuyorum."},
			{Done: true, FinishReason: "tool_calls", ToolCalls: []provider.ToolCall{mustToolCall(t, "c1", "read_file", map[string]string{"path": "a.txt"})}, Usage: &provider.Usage{PromptTokens: 100, CompletionTokens: 10, TotalTokens: 110}},
		},
		{
			{Content: "İçerik: hi"},
			{Done: true, FinishReason: "stop", Usage: &provider.Usage{PromptTokens: 150, CompletionTokens: 5, TotalTokens: 155}},
		},
	}}
	pl, _ := newStreamTestPipeline(t, prov)
	events, content, thinking, last := runStreamTurn(t, WithLiveNarration(context.Background()), pl)

	if thinking != "önce dosyaya bakmalıyım" {
		t.Errorf("thinking = %q (must be forwarded exactly once, as it streamed)", thinking)
	}
	if content != "İçerik: hi" {
		t.Errorf("content = %q, want only the final answer (narration travels as events)", content)
	}

	var seq []string
	var deltas strings.Builder
	var narration string
	for _, ev := range events {
		switch ev.Type {
		case EventNarrationDelta:
			deltas.WriteString(ev.Content)
			seq = append(seq, "delta")
		case EventNarration:
			narration = ev.Content
			seq = append(seq, "narration")
		case EventToolExecuting:
			seq = append(seq, "tool")
		case EventToolResult:
			seq = append(seq, "result")
		}
	}
	if deltas.String() != "Dosyayı okuyorum.İçerik: hi" {
		t.Errorf("deltas = %q", deltas.String())
	}
	if narration != "Dosyayı okuyorum." {
		t.Errorf("narration = %q", narration)
	}
	// Order matters: the narration is recorded BEFORE the tool it introduces.
	joined := strings.Join(seq, ",")
	if !strings.Contains(joined, "narration,tool,result") {
		t.Errorf("event order = %s, want narration before tool_executing/tool_result", joined)
	}
	if last.Usage == nil || last.Usage.PromptTokens != 250 || last.Usage.CompletionTokens != 15 {
		t.Errorf("usage = %+v, want 250/15 summed over both streamed steps", last.Usage)
	}
	for i, s := range prov.sawStream {
		if !s {
			t.Errorf("model call %d was not a streaming request", i)
		}
	}
}

// Without the live flag (WhatsApp, routines, sub-agents, the task loop) the
// pipeline still streams from the provider but emits no per-token events —
// only one narration event per tool step.
func TestRunStream_NoDeltaEventsUnlessLiveNarrationAsked(t *testing.T) {
	prov := &streamingScripted{streams: [][]provider.StreamChunk{
		{{Content: "bakıyorum "}, {Done: true, ToolCalls: []provider.ToolCall{mustToolCall(t, "c1", "read_file", map[string]string{"path": "a.txt"})}}},
		{{Content: "bitti"}, {Done: true}},
	}}
	pl, _ := newStreamTestPipeline(t, prov)
	events, content, _, _ := runStreamTurn(t, context.Background(), pl)
	for _, ev := range events {
		if ev.Type == EventNarrationDelta {
			t.Fatalf("got a narration delta without WithLiveNarration: %+v", ev)
		}
	}
	narrations := 0
	for _, ev := range events {
		if ev.Type == EventNarration {
			narrations++
		}
	}
	if narrations != 1 || content != "bitti" {
		t.Errorf("narrations=%d content=%q, want 1 and the final answer", narrations, content)
	}
}

// A model that returns text with its tool call in a BLOCKING call (a provider
// without streaming) still gets its narration shown.
func TestRunStream_BlockingProviderStillSurfacesNarration(t *testing.T) {
	prov := &scriptedProvider{responses: []provider.ChatResponse{
		{Content: "Önce bakıyorum.", ToolCalls: []provider.ToolCall{mustToolCall(t, "c1", "read_file", map[string]string{"path": "a.txt"})}},
		{Content: "bitti"},
	}}
	pl, _ := newStreamTestPipeline(t, prov)
	events, content, _, _ := runStreamTurn(t, context.Background(), pl)
	var narration string
	for _, ev := range events {
		if ev.Type == EventNarration {
			narration = ev.Content
		}
	}
	if narration != "Önce bakıyorum." || content != "bitti" {
		t.Errorf("narration=%q content=%q", narration, content)
	}
}

// An endpoint that refuses streaming with tools (400) is retried once the
// blocking way, and the turn stays on blocking calls.
func TestRunStream_FallsBackToBlockingWhenStreamIsRejected(t *testing.T) {
	prov := &streamingScripted{
		streamErr: errors.New("all providers failed: [custom] status 400: stream with tools is not supported"),
		blocking: []provider.ChatResponse{
			{ToolCalls: []provider.ToolCall{mustToolCall(t, "c1", "read_file", map[string]string{"path": "a.txt"})}},
			{Content: "bitti"},
		},
	}
	pl, _ := newStreamTestPipeline(t, prov)
	_, content, _, last := runStreamTurn(t, context.Background(), pl)
	if content != "bitti" || last.Error != "" {
		t.Fatalf("content=%q last=%+v", content, last)
	}
	if prov.blockCalls != 2 {
		t.Errorf("blocking calls = %d, want 2", prov.blockCalls)
	}
	// One failed stream attempt, then blocking for the rest: the second model
	// call must NOT try streaming again.
	if got := len(prov.sawStream); got != 3 { // 1 rejected stream + 2 blocking
		t.Errorf("model calls seen = %d, want 3", got)
	}
}

// A 400 that is really "the prompt is too big" must NOT be retried as a
// blocking call (it would fail identically); it is reported as is.
func TestRunStream_ContextOverflowIsNotMistakenForUnsupportedStreaming(t *testing.T) {
	prov := &streamingScripted{
		streamErr: errors.New("all providers failed: [custom] status 400: The input token count exceeds the maximum number of tokens allowed (1048576)."),
	}
	pl, _ := newStreamTestPipeline(t, prov)
	_, _, _, last := runStreamTurn(t, context.Background(), pl)
	if !strings.Contains(last.Error, "exceeds the maximum") {
		t.Fatalf("last = %+v, want the overflow error", last)
	}
	if prov.blockCalls != 0 {
		t.Errorf("blocking retry made for a context overflow")
	}
}

// 429/401/5xx say nothing about streaming support and must never be retried.
func TestRunStream_RateLimitIsNotRetriedBlocking(t *testing.T) {
	prov := &streamingScripted{streamErr: errors.New("[custom] rate limited: status 429: slow down")}
	pl, _ := newStreamTestPipeline(t, prov)
	_, _, _, last := runStreamTurn(t, context.Background(), pl)
	if last.Error == "" || prov.blockCalls != 0 {
		t.Fatalf("last=%+v blockCalls=%d", last, prov.blockCalls)
	}
}

// A stream that yields nothing at all (an endpoint that ignored stream=true
// and sent plain JSON) is retried once, blocking.
func TestRunStream_EmptyStreamFallsBackToBlocking(t *testing.T) {
	prov := &streamingScripted{
		streams:  [][]provider.StreamChunk{{}},
		blocking: []provider.ChatResponse{{Content: "merhaba"}},
	}
	pl, _ := newStreamTestPipeline(t, prov)
	_, content, _, last := runStreamTurn(t, context.Background(), pl)
	if content != "merhaba" || last.Error != "" {
		t.Fatalf("content=%q last=%+v", content, last)
	}
}

// A stream that fails midway reports the error; nothing is silently dropped.
func TestRunStream_MidStreamErrorIsReported(t *testing.T) {
	prov := &streamingScripted{streams: [][]provider.StreamChunk{
		{{Content: "yarım "}, {Error: "connection reset", Done: true}},
	}}
	pl, _ := newStreamTestPipeline(t, prov)
	_, _, _, last := runStreamTurn(t, context.Background(), pl)
	if !strings.Contains(last.Error, "connection reset") {
		t.Fatalf("last = %+v", last)
	}
}
