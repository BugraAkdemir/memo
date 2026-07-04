package replcli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"memo/internal/api"
)

// Run starts the interactive terminal chat loop against the Memo backend at
// baseURL. It creates one fresh agent-mode chat rooted at projectPath, then
// reads lines from in and writes all output to out until EOF or the user
// types /exit. Every run is a brand-new chat — there is no session history
// or switching inside the REPL. ownBackend tells the welcome panel whether
// this process just started the backend itself (main.go) — only then is an
// embedding-model auto-start race actually possible, so only then is it
// worth briefly retrying the memory status before reporting it as off.
func Run(baseURL, projectPath string, in io.Reader, out io.Writer, ownBackend bool) error {
	clearScreen(out)

	ctx := context.Background()
	client := NewClient(baseURL)

	id, err := client.NewAgentChat(ctx, projectPath)
	if err != nil {
		return fmt.Errorf("agent sohbeti oluşturulamadı: %w", err)
	}
	if err := client.SwitchChat(ctx, id); err != nil {
		return fmt.Errorf("sohbete geçilemedi: %w", err)
	}
	if err := client.SetAgentEnabled(ctx, true); err != nil {
		return fmt.Errorf("agent modu açılamadı: %w", err)
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	s := &session{client: client, ctx: ctx, out: out, scanner: scanner, ownBackend: ownBackend}
	s.printWelcome()

	for {
		// A blank line always precedes the prompt — separates this turn's
		// prompt from whatever was printed above it (previous reply, command
		// output, or the welcome panel), so input and output never look
		// stuck together.
		fmt.Fprintln(out)
		fmt.Fprint(out, userInputStart+"> ")
		ok := scanner.Scan()
		// Reset right away, win or lose — the background must never bleed
		// into anything printed after the user's own line (blank line,
		// command output, the reply).
		fmt.Fprint(out, colorReset)
		if !ok {
			fmt.Fprintln(out)
			return nil
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/exit" {
			return nil
		}
		if strings.HasPrefix(line, "/") {
			if s.handleCommand(line) {
				return nil
			}
			continue
		}

		fmt.Fprintln(out)
		s.sendMessage(line)
	}
}

// session carries the state a single streamed turn needs to react to
// mid-stream agent events (in particular, blocking for a permission answer
// read from the same stdin scanner the outer prompt loop uses), and the
// slash-command handlers in commands.go.
type session struct {
	client     *Client
	ctx        context.Context
	out        io.Writer
	scanner    *bufio.Scanner
	ownBackend bool

	// aiTurnStarted tracks whether the reply marker has already been printed
	// for the turn in progress — a turn can arrive as several chunks, but
	// the marker belongs only in front of the first one that has content.
	// Reset at the start of every sendMessage call.
	aiTurnStarted bool
}

// sendMessage sends one line to the backend and streams the reply. The
// spinner is stopped unconditionally after SendStream returns — not just
// from inside the chunk callback — because a turn that ends with zero
// chunks (an empty response body, a request that errors before emitting
// anything) would otherwise leave the spinner's goroutine running forever,
// racing the next prompt for the same stdout and garbling both.
func (s *session) sendMessage(line string) {
	s.aiTurnStarted = false

	// Only worth checking for a save confirmation if embedding is actually
	// running — otherwise every message would pay the polling delay below
	// for a save that was never going to happen.
	memoryLikely := false
	if status, err := s.client.EmbeddingStatus(s.ctx); err == nil && status.Running {
		memoryLikely = true
	}

	var lastEventBefore Event
	var hadEventBefore bool
	if memoryLikely {
		if events, err := s.client.Events(s.ctx); err == nil && len(events) > 0 {
			lastEventBefore = events[len(events)-1]
			hadEventBefore = true
		}
	}

	sp := newSpinner(s.out)
	onChunk := func(chunk api.StreamChunk) error {
		sp.Stop()
		return s.handleChunk(chunk)
	}
	err := s.client.SendStream(s.ctx, line, onChunk)
	sp.Stop()

	if err != nil {
		fmt.Fprintln(s.out, errorf("Hata: %s", friendlyError(err.Error())))
		return
	}
	// The reply is raw model text, typewriter-revealed exactly as received —
	// it does not reliably end with its own newline. Without this, whatever
	// gets printed next (the memory-saved confirmation, or just the blank
	// line the main loop prints before the following prompt) could glue
	// directly onto the tail of the reply instead of starting fresh.
	fmt.Fprintln(s.out)
	if memoryLikely {
		s.reportMemorySaved(lastEventBefore, hadEventBefore)
	}
}

// reportMemorySaved briefly polls /api/events for a memory:saved event that
// wasn't already there before this turn started. Memory is saved on a
// background worker well after the reply is sent, so this can only ever
// report a save that actually happened — not something inferred from just
// having sent a message. Silently gives up after ~2.4s so a slow/disabled
// save never blocks the prompt from coming back.
func (s *session) reportMemorySaved(before Event, hadBefore bool) {
	const attempts = 6
	const interval = 400 * time.Millisecond
	for range attempts {
		time.Sleep(interval)
		events, err := s.client.Events(s.ctx)
		if err != nil || len(events) == 0 {
			continue
		}
		last := events[len(events)-1]
		if last.Name != "memory:saved" {
			continue
		}
		if hadBefore && last == before {
			continue // same event that was already there before this turn
		}
		fmt.Fprintln(s.out, dim("✓ hafıza kaydedildi"))
		return
	}
}

func (s *session) printWelcome() {
	memory, active := s.memorySummary()
	fmt.Fprintln(s.out, welcomePanel(s.modelSummary(), memory, active))
}

// modelSummary describes which model/provider is actually going to answer —
// an external provider if one is active, otherwise the local chat model if
// one is running, otherwise a clear "nothing loaded yet" hint.
func (s *session) modelSummary() string {
	if name, err := s.client.ActiveProviderName(s.ctx); err == nil && name != "" {
		if providers, err := s.client.ListProviders(s.ctx); err == nil {
			for _, p := range providers {
				if p.Name == name {
					if p.Model != "" {
						return fmt.Sprintf("%s (%s)", name, p.Model)
					}
					return name
				}
			}
		}
		return name
	}
	if status, err := s.client.ModelStatus(s.ctx); err == nil && status.Running {
		if status.ModelName != "" {
			return status.ModelName
		}
		return "yerel model"
	}
	return "yüklü değil — /model ya da /connect"
}

// memorySummary reports whether the embedding model (and therefore RAG
// memory) is actually running. When this process just started the backend
// itself (ownBackend), an embedding model can still be mid-load in the
// background — briefly retry before declaring memory off, so the welcome
// panel doesn't flash a stale warning for something that finishes loading
// half a second later. Attaching to an already-running backend has no such
// race — its embedding status is already settled, so a single check is both
// correct and instant.
func (s *session) memorySummary() (text string, active bool) {
	attempts := 1
	if s.ownBackend {
		attempts = 6
	}
	const interval = 400 * time.Millisecond
	for i := range attempts {
		status, err := s.client.EmbeddingStatus(s.ctx)
		if err == nil && status.Running {
			if status.ModelName != "" {
				return status.ModelName, true
			}
			return "açık", true
		}
		if i < attempts-1 {
			time.Sleep(interval)
		}
	}
	return "kapalı", false
}

func (s *session) handleChunk(chunk api.StreamChunk) error {
	if chunk.Error != "" {
		fmt.Fprintln(s.out, errorf("%s", friendlyError(chunk.Error)))
		return nil
	}

	switch chunk.FinishReason {
	case "status":
		fmt.Fprintln(s.out, dim(fmt.Sprintf("[%s]", chunk.Content)))
	case "agent_event":
		var ev AgentEvent
		if err := json.Unmarshal([]byte(chunk.Content), &ev); err != nil {
			return nil // malformed event payload — skip, matches SSE tolerance
		}
		return s.handleAgentEvent(ev)
	case "usage", "activity":
		// structured payloads not needed by the terminal REPL — ignored.
	default:
		// "" (plain streamed token) or "stop" (may carry trailing text).
		// Agent mode isn't truly token-streamed backend-side (the whole
		// reply arrives as one chunk), so reveal it with a typewriter
		// effect instead of dumping it on screen all at once.
		if !s.aiTurnStarted && chunk.Content != "" {
			s.aiTurnStarted = true
			fmt.Fprint(s.out, bold(brightMagenta("● ")))
		}
		typewriter(s.out, chunk.Content)
	}
	return nil
}

func (s *session) handleAgentEvent(ev AgentEvent) error {
	switch ev.Type {
	case "tool_executing":
		fmt.Fprintln(s.out, "\n"+dim(fmt.Sprintf("⚙ %s çalışıyor...", ev.Tool)))
	case "tool_result":
		fmt.Fprintln(s.out, green(fmt.Sprintf("✓ %s tamamlandı", ev.Tool)))
	case "tool_error":
		fmt.Fprintln(s.out, errorf("✗ %s hata: %s", ev.Tool, ev.Error))
	case "permission_denied":
		fmt.Fprintln(s.out, yellow(fmt.Sprintf("✗ %s reddedildi", ev.Tool)))
	case "permission_request":
		return s.askPermission(ev)
	}
	return nil
}

func (s *session) askPermission(ev AgentEvent) error {
	fmt.Fprintln(s.out, "\n"+yellow(fmt.Sprintf("⚠ %s bu işlemi yapmak istiyor: %s", ev.Tool, previewArgs(ev.Args))))
	fmt.Fprint(s.out, bold("İzin ver mi? [y/n] "))

	policy := "deny_once"
	if s.scanner.Scan() {
		answer := strings.ToLower(strings.TrimSpace(s.scanner.Text()))
		if answer == "y" || answer == "yes" || answer == "e" || answer == "evet" {
			policy = "allow_once"
		}
	}
	return s.client.SendPermission(s.ctx, ev.RequestID, policy)
}

func previewArgs(args json.RawMessage) string {
	if len(args) == 0 {
		return ""
	}
	const max = 120
	s := string(args)
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

// friendlyError rewrites the raw dial/transport errors a missing model
// produces into a plain-language hint, instead of dumping a Go error string
// straight into the chat.
func friendlyError(raw string) string {
	if strings.Contains(raw, "connection refused") {
		return "Önce bir model başlatmalısın: /model <isim> ile yerel bir model, ya da /connect ile harici bir sağlayıcı bağla. Yüklü modelleri görmek için /models yaz."
	}
	return raw
}
