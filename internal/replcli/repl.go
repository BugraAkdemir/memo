package replcli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"memo/internal/api"
)

// Run starts the interactive terminal chat loop against the Memo backend at
// baseURL. It resumes the most recently used agent-mode chat rooted at
// projectPath, if one exists, replaying its history into the terminal —
// otherwise it creates a fresh one. It then reads lines from in and writes
// all output to out until EOF or the user types /exit. /clear and /session
// let the user reset or switch chats mid-run. ownBackend tells the welcome
// panel whether
// this process just started the backend itself (main.go) — only then is an
// embedding-model auto-start race actually possible, so only then is it
// worth briefly retrying the memory status before reporting it as off.
//
// When in is a real terminal the whole session runs in raw mode with a
// dedicated line editor: a live slash-command dropdown (type "/" and
// navigate with the arrows immediately, Claude Code style), input history on
// Up/Down, cursor editing, and Esc/Ctrl+C to cancel a streaming reply. Piped
// input (tests, scripts) keeps the plain line-scanner behavior.
func Run(baseURL, projectPath string, in io.Reader, out io.Writer, ownBackend bool) error {
	var keys *keySource
	var ed *editor
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fd := int(f.Fd())
		if oldState, err := term.MakeRaw(fd); err == nil {
			defer term.Restore(fd, oldState)
			out = crlfWriter{out}
			keys = newKeySource(f)
			ed = &editor{
				out:  out,
				keys: keys,
				width: func() int {
					w, _, err := term.GetSize(fd)
					if err != nil {
						return 80
					}
					return w
				},
			}
		}
	}

	clearScreen(out)

	ctx := context.Background()
	client := NewClient(baseURL)

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	s := &session{client: client, ctx: ctx, out: out, scanner: scanner, ownBackend: ownBackend, keys: keys, ed: ed, projectPath: projectPath}

	resumed, err := s.resumeOrStartChat()
	if err != nil {
		return err
	}
	s.printWelcome()
	if resumed {
		s.replayHistory()
	}

	for {
		// A blank line always precedes the prompt — separates this turn's
		// prompt from whatever was printed above it (previous reply, command
		// output, or the welcome panel), so input and output never look
		// stuck together.
		fmt.Fprintln(out)
		line, ok := s.readInput()
		if !ok {
			fmt.Fprintln(out)
			return nil
		}
		line = strings.TrimSpace(line)
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
// read from the same input source the outer prompt loop uses), and the
// slash-command handlers in commands.go.
type session struct {
	client     *Client
	ctx        context.Context
	out        io.Writer
	scanner    *bufio.Scanner
	ownBackend bool

	// projectPath is this REPL run's project root — used both to create new
	// agent chats and to find an existing one to resume on startup.
	// chatID is the backend chat ID currently active in this session,
	// updated by resumeOrStartChat and every /clear or /session switch.
	projectPath string
	chatID      string

	// keys and ed are non-nil only when stdin is a real terminal. keys is
	// the one shared key stream (editor, menus and the mid-stream interrupt
	// watcher all read from it); ed is the raw-mode line editor.
	keys *keySource
	ed   *editor

	// interruptCancel + watcher wire Esc/Ctrl+C to the in-flight request's
	// context while a reply streams. The watcher must be paused whenever
	// something else needs the keyboard mid-stream (permission prompts).
	interruptCancel func()
	watcher         *interruptWatch

	// aiTurnStarted tracks whether the reply marker has already been printed
	// for the turn in progress — a turn can arrive as several chunks, but
	// the marker belongs only in front of the first one that has content.
	// Reset at the start of every sendMessage call.
	aiTurnStarted bool
}

// promptStyle is the main composer prompt. Kept at display width 2 so the
// editor's column math stays trivial.
var promptStyle = bold(brightCyan("❯ "))

// resumeOrStartChat looks for the most recently used agent chat rooted at
// s.projectPath and switches to it if one exists, so a new `memo` run in the
// same project picks up where the last one left off instead of always
// starting blank. Falls back to a brand-new agent chat — including whenever
// listing chats itself fails, since a chat always has to exist either way.
func (s *session) resumeOrStartChat() (resumed bool, err error) {
	if id, ok := s.findRecentChat(); ok {
		if err := s.client.SwitchChat(s.ctx, id); err != nil {
			return false, fmt.Errorf("sohbete geçilemedi: %w", err)
		}
		s.chatID = id
		resumed = true
	} else {
		id, err := s.client.NewAgentChat(s.ctx, s.projectPath)
		if err != nil {
			return false, fmt.Errorf("agent sohbeti oluşturulamadı: %w", err)
		}
		if err := s.client.SwitchChat(s.ctx, id); err != nil {
			return false, fmt.Errorf("sohbete geçilemedi: %w", err)
		}
		s.chatID = id
	}
	if err := s.client.SetAgentEnabled(s.ctx, true); err != nil {
		return false, fmt.Errorf("agent modu açılamadı: %w", err)
	}
	return resumed, nil
}

// findRecentChat returns the ID of the most recently updated agent chat
// rooted at s.projectPath, if any (chats come back sorted newest-first).
func (s *session) findRecentChat() (string, bool) {
	chats, err := s.client.ListChats(s.ctx)
	if err != nil {
		return "", false
	}
	for _, c := range chats {
		if c.ProjectPath == s.projectPath {
			return c.ID, true
		}
	}
	return "", false
}

// projectChats returns every known chat rooted at s.projectPath, newest
// first — the set /session lists and picks from.
func (s *session) projectChats() ([]SessionInfo, error) {
	chats, err := s.client.ListChats(s.ctx)
	if err != nil {
		return nil, err
	}
	var out []SessionInfo
	for _, c := range chats {
		if c.ProjectPath == s.projectPath {
			out = append(out, c)
		}
	}
	return out, nil
}

// startFreshChat creates a brand-new agent chat rooted at s.projectPath and
// makes it active — the shared core of /clear.
func (s *session) startFreshChat() error {
	id, err := s.client.NewAgentChat(s.ctx, s.projectPath)
	if err != nil {
		return fmt.Errorf("sohbet oluşturulamadı: %w", err)
	}
	return s.activateChat(id)
}

// activateChat switches the backend's active chat to id, re-enables agent
// mode (switching chats resets it backend-side) and records id as this
// session's current chat.
func (s *session) activateChat(id string) error {
	if err := s.client.SwitchChat(s.ctx, id); err != nil {
		return fmt.Errorf("sohbete geçilemedi: %w", err)
	}
	if err := s.client.SetAgentEnabled(s.ctx, true); err != nil {
		return fmt.Errorf("agent modu açılamadı: %w", err)
	}
	s.chatID = id
	return nil
}

// replayHistory prints the active chat's saved messages into the terminal so
// a resumed session reads like an uninterrupted conversation instead of
// silently forgetting everything before this run.
func (s *session) replayHistory() {
	msgs, err := s.client.Messages(s.ctx)
	if err != nil || len(msgs) == 0 {
		return
	}
	fmt.Fprintln(s.out)
	fmt.Fprintln(s.out, dim("── önceki sohbet geçmişi ──"))
	for _, m := range msgs {
		switch m.Role {
		case "user":
			fmt.Fprintln(s.out, promptStyle+m.Content)
		case "assistant":
			fmt.Fprintln(s.out, bold(brightMagenta("● "))+m.Content)
		}
	}
	fmt.Fprintln(s.out, dim("── buradan devam ediyorsun ──"))
}

// readInput reads one top-level line: through the raw-mode editor on a real
// terminal, through the scanner otherwise (tests, piped input).
func (s *session) readInput() (string, bool) {
	if s.ed != nil {
		return s.ed.readLine(promptStyle)
	}
	fmt.Fprint(s.out, userInputStart+"> ")
	ok := s.scanner.Scan()
	// Reset right away, win or lose — the background must never bleed
	// into anything printed after the user's own line (blank line,
	// command output, the reply).
	fmt.Fprint(s.out, colorReset)
	if !ok {
		return "", false
	}
	return s.scanner.Text(), true
}

// promptLine reads a secondary, free-form answer (y/n, a search term) with
// whichever input path is active. No history, no dropdown.
func (s *session) promptLine(prompt string) (string, bool) {
	if s.ed != nil {
		return s.ed.readLinePlain(prompt)
	}
	fmt.Fprint(s.out, prompt)
	if !s.scanner.Scan() {
		return "", false
	}
	return s.scanner.Text(), true
}

// startInterruptWatch resumes the Esc/Ctrl+C watcher for the in-flight turn,
// if one is in flight and the keyboard is ours to watch.
func (s *session) startInterruptWatch() {
	if s.keys != nil && s.interruptCancel != nil && s.watcher == nil {
		s.watcher = s.keys.watchInterrupt(s.interruptCancel)
	}
}

// stopInterruptWatch fully detaches the watcher from the key stream so the
// editor or a menu can read keys again.
func (s *session) stopInterruptWatch() {
	if s.watcher != nil {
		s.watcher.Stop()
		s.watcher = nil
	}
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

	// Esc or Ctrl+C during the stream cancels this turn's request instead of
	// killing the whole app.
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	s.interruptCancel = cancel
	s.startInterruptWatch()
	defer func() {
		s.stopInterruptWatch()
		s.interruptCancel = nil
	}()

	sp := newSpinner(s.out)
	onChunk := func(chunk api.StreamChunk) error {
		sp.Stop()
		return s.handleChunk(chunk)
	}
	err := s.client.SendStream(ctx, line, onChunk)
	sp.Stop()

	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			fmt.Fprintln(s.out, dim("⨯ iptal edildi"))
			return
		}
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
	// The interrupt watcher and the permission prompt would otherwise race
	// for the same key stream — pause it for the duration of the question.
	s.stopInterruptWatch()
	defer s.startInterruptWatch()

	fmt.Fprintln(s.out, "\n"+yellow(fmt.Sprintf("⚠ %s bu işlemi yapmak istiyor: %s", ev.Tool, previewArgs(ev.Args))))

	policy := "deny_once"
	if answer, ok := s.promptLine(bold("İzin ver mi? [y/n] ")); ok {
		answer = strings.ToLower(strings.TrimSpace(answer))
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
