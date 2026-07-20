package replcli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"golang.org/x/term"

	"memo/internal/api"
)

// Run starts the interactive terminal chat loop against the Memo backend at
// baseURL. Every run starts a brand-new agent-mode chat rooted at
// projectPath — it never auto-resumes an old one, so the terminal and its
// context always start clean. /session lists every chat from every client
// (CLI and GUI alike) and lets the user explicitly resume any of them. It
// then reads lines from in and writes all output to out until EOF or the
// user types /exit. /clear and /session let the user reset or switch chats
// mid-run. ownBackend tells the welcome panel whether
// this process just started the backend itself (main.go) — only then is an
// embedding-model auto-start race actually possible, so only then is it
// worth briefly retrying the memory status before reporting it as off.
//
// When in is a real terminal the whole session runs in raw mode with a
// dedicated line editor: a live slash-command dropdown (type "/" and
// navigate with the arrows immediately, Claude Code style), input history on
// Up/Down, cursor editing, and Esc/Ctrl+C to cancel a streaming reply. Piped
// input (tests, scripts) keeps the plain line-scanner behavior.
// onClientRegistered, if given, is invoked with the client ID right after a
// successful RegisterClient — main.go uses it to unregister directly from
// its external-signal branch, since that branch can't wait for this
// function's own deferred unregister (the goroutine running Run is left
// blocked on stdin and abandoned when the process exits on signal).
func Run(baseURL, projectPath string, in io.Reader, out io.Writer, ownBackend bool, onClientRegistered ...func(clientID string)) error {
	var keys *keySource
	var ed *editor
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fd := int(f.Fd())
		if oldState, err := term.MakeRaw(fd); err == nil {
			defer term.Restore(fd, oldState)
			out = crlfWriter{out}
			// Bracketed paste (xterm's ESC[200~/ESC[201~ wrapping around
			// pasted text) lets keys.go's readBracketedPaste tell a paste
			// apart from real keystrokes — without it every embedded
			// newline in a multi-line paste reads as a real Enter press.
			// Must be disabled again before the terminal leaves raw mode.
			fmt.Fprint(out, "\033[?2004h")
			defer fmt.Fprint(out, "\033[?2004l")
			keys = newKeySource(f)
			ed = &editor{
				out:         out,
				keys:        keys,
				projectPath: projectPath,
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

	// Attach to the backend's client registry (internal/app/clients.go) so
	// a backend spawned on demand for this session knows to stay up while
	// this CLI is running, and can shut itself down once every attached
	// client (this one, and any GUI opened via /gui) is gone — instead of
	// dying the moment whichever process happened to start it exits.
	// Best-effort: an older/incompatible backend that doesn't know this
	// route just means no heartbeat loop runs, nothing else changes.
	if clientID, err := client.RegisterClient(ctx); err == nil && clientID != "" {
		for _, cb := range onClientRegistered {
			cb(clientID)
		}
		hbCtx, hbCancel := context.WithCancel(ctx)
		defer hbCancel()
		go heartbeatLoop(hbCtx, client, clientID)
		defer func() {
			unregCtx, unregCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer unregCancel()
			_ = client.UnregisterClient(unregCtx, clientID)
		}()
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	s := &session{client: client, ctx: ctx, out: out, scanner: scanner, ownBackend: ownBackend, keys: keys, ed: ed, projectPath: projectPath}

	if err := s.startFreshChat(); err != nil {
		return err
	}
	s.printWelcome()

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

	// projectPath is this REPL run's project root, tagged onto every chat it
	// creates (NewAgentChat) — purely metadata now, since startup no longer
	// filters or resumes by it.
	// chatID is the backend chat ID currently active in this session,
	// updated by startFreshChat and every /clear or /session switch.
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
var promptStyle = bold(gold("❯ "))

// allChats returns every known chat, newest first — CLI-created (agent,
// tagged with a project path) and GUI-created (plain) alike, exactly the
// same set the Flutter GUI's chat list shows. This is the set /session
// lists and picks from, so any chat started in the GUI can be resumed from
// the CLI and vice versa.
func (s *session) allChats() ([]SessionInfo, error) {
	return s.client.ListChats(s.ctx)
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

	var lastSeqBefore uint64
	if memoryLikely {
		if events, err := s.client.Events(s.ctx); err == nil && len(events) > 0 {
			lastSeqBefore = events[len(events)-1].Seq
		}
	}

	// Esc or Ctrl+C during the stream cancels this turn's request instead of
	// killing the whole app.
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	s.interruptCancel = cancel
	s.startInterruptWatch()

	sp := newSpinner(s.out)
	onChunk := func(chunk api.StreamChunk) error {
		sp.Stop()
		return s.handleChunk(chunk)
	}
	err := s.client.SendStream(ctx, line, onChunk)
	sp.Stop()

	// Stop watching for Esc/Ctrl+C as soon as the stream itself ends, not
	// deferred to when sendMessage returns. reportMemorySaved below can
	// block for up to ~2.4s after this point (whenever memory/embedding is
	// enabled) — watchInterrupt's goroutine (keys.go) reads every key from
	// the shared byte channel and silently discards anything that isn't
	// Esc/Ctrl+C, so leaving it attached during that window ate every
	// keystroke (or a whole pasted block) the user typed the moment the
	// reply looked finished, with nothing to show for it — the most likely
	// cause of the CLI "randomly" losing input right after a reply.
	s.stopInterruptWatch()
	s.interruptCancel = nil

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
		s.reportMemorySaved(lastSeqBefore)
	}
}

// reportMemorySaved briefly polls /api/events for a memory:saved event that
// wasn't already there before this turn started. Memory is saved on a
// background worker well after the reply is sent, so this can only ever
// report a save that actually happened — not something inferred from just
// having sent a message. Silently gives up after ~2.4s so a slow/disabled
// save never blocks the prompt from coming back.
func (s *session) reportMemorySaved(afterSeq uint64) {
	const attempts = 6
	const interval = 400 * time.Millisecond
	for range attempts {
		time.Sleep(interval)
		events, err := s.client.Events(s.ctx)
		if err != nil || len(events) == 0 {
			continue
		}
		if memorySavedSince(events, afterSeq) {
			fmt.Fprintln(s.out, dim("✓ hafıza kaydedildi"))
			return
		}
		// A save was attempted and actually failed (backend already emits
		// memory:error for this — autoStartEmbeddingModel/startupEmbeddingModel/
		// saveMemorySync, internal/app/*.go) — surface it instead of silently
		// giving up after this loop, which used to look exactly like nothing
		// happened even though the backend knew perfectly well why.
		if msg, ok := eventDataSince(events, afterSeq, "memory:error"); ok {
			fmt.Fprintln(s.out, yellow("⚠ "+msg))
			return
		}
	}
}

// eventDataSince returns the Data of the most recent event named `name`
// with Seq > afterSeq (afterSeq 0 means "search all of events" — real Seq
// values start at 1, so nothing can be lost that way).
func eventDataSince(events []Event, afterSeq uint64, name string) (string, bool) {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Name == name && events[i].Seq > afterSeq {
			return events[i].Data, true
		}
	}
	return "", false
}

// memorySavedSince reports whether a memory:saved event with Seq > afterSeq
// occurs in events. Keying off Seq rather than the event's Name+Data (or
// identity with a snapshotted "before" event) is deliberate: every
// memory:saved event carries the exact same empty Data, so on two
// back-to-back turns that each produce a save — the ordinary case for a
// real conversation — a value-equality check for "the snapshotted before
// event, recurring" cannot tell the *previous* turn's save apart from the
// *new* one, since they're structurally identical. That collapsed the new
// save into "not new," which is exactly what a live multi-turn CLI probe
// (Fatih persona, 2026-07-15) hit: every turn after the first showed
// [memory:none-detected] despite the backend log showing a real, fast
// (<300ms) SaveInteraction completing each time. Seq is assigned once, at
// push time (internal/app/app.go's eventRing), and never repeats.
//
// MemorySavedSince/EventDataSince below are the exported forms, for
// callers outside this package (main.go's `-p` print mode) that need the
// same "did a save happen since this snapshot" check the interactive REPL
// uses.
func memorySavedSince(events []Event, afterSeq uint64) bool {
	return slices.ContainsFunc(events, func(e Event) bool { return e.Name == "memory:saved" && e.Seq > afterSeq })
}

func MemorySavedSince(events []Event, afterSeq uint64) bool {
	return memorySavedSince(events, afterSeq)
}

func EventDataSince(events []Event, afterSeq uint64, name string) (string, bool) {
	return eventDataSince(events, afterSeq, name)
}

func (s *session) printWelcome() {
	memory, active := s.memorySummary()
	// Best-effort: an older/incompatible backend or a transient error just
	// means the title line omits the version suffix, nothing else degrades.
	version, _ := s.client.Version(s.ctx)
	fmt.Fprintln(s.out, welcomePanel(version, s.projectPath, s.modelSummary(), memory, active))
	// Embedding auto-start (autoStartEmbeddingModel/startupEmbeddingModel,
	// internal/app/llama.go+embedding.go) can fail for reasons the banner
	// alone doesn't explain — no embedding model file found, the model
	// download failed, or Start itself failed (e.g. its port is already
	// occupied by something else). Any of those already emits a
	// memory:error event with the real reason; only silence in this REPL
	// ever kept it from the user. Gated on !active so a stale error from
	// earlier in the ring isn't shown once embedding is actually up.
	if !active {
		if events, err := s.client.Events(s.ctx); err == nil {
			if msg, ok := eventDataSince(events, 0, "memory:error"); ok {
				fmt.Fprintln(s.out, yellow("⚠ "+msg))
			}
		}
	}
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

// askPermission prompts for a single tool call. Mirrors the Flutter GUI's
// PermissionDialog (frontend/lib/widgets/agent/permission_dialog.dart) on
// two points it previously didn't: showing the tool's DangerLevel with
// proportional visual weight instead of identical wording for every tool
// regardless of risk, and offering "allow for this session" (backend
// already understands AllowSession, internal/agent/permissions.go) for
// non-dangerous tools, instead of re-prompting for the identical call every
// single time it recurs in one session.
func (s *session) askPermission(ev AgentEvent) error {
	// The interrupt watcher and the permission prompt would otherwise race
	// for the same key stream — pause it for the duration of the question.
	s.stopInterruptWatch()
	defer s.startInterruptWatch()

	warnPrefix := "⚠"
	if ev.DangerLevel == "dangerous" {
		warnPrefix = "🛑 TEHLİKELİ"
	}
	fmt.Fprintln(s.out, "\n"+yellow(fmt.Sprintf("%s %s bu işlemi yapmak istiyor: %s", warnPrefix, ev.Tool, describeToolCall(ev))))

	allowSession := ev.DangerLevel != "dangerous"
	prompt := "İzin ver mi? [y/n] "
	if allowSession {
		prompt = "İzin ver mi? [y = bir kere, a = bu oturum boyunca, n = hayır] "
	}

	policy := "deny_once"
	if answer, ok := s.promptLine(bold(prompt)); ok {
		answer = strings.ToLower(strings.TrimSpace(answer))
		switch {
		case allowSession && (answer == "a" || answer == "always" || answer == "oturum"):
			policy = "allow_session"
		case answer == "y" || answer == "yes" || answer == "e" || answer == "evet":
			policy = "allow_once"
		}
	}
	return s.client.SendPermission(s.ctx, ev.RequestID, policy)
}

// describeToolCall prefers the backend's own curated, human-readable
// preview (ev.Preview — populated server-side for tools with a PreviewFn,
// e.g. edit_file/insert_line/delete_lines, internal/agent/pipeline.go) over
// a blind truncation of the raw tool-call JSON args. The raw JSON is
// whatever the model itself emitted, in whatever key order it chose — for
// a long "content" field ahead of "path", a flat character truncation can
// end before the target path ever appears, so the user approves a write
// without ever seeing which file it targets.
func describeToolCall(ev AgentEvent) string {
	if ev.Preview != "" {
		return ev.Preview
	}
	return previewArgs(ev.Args)
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
