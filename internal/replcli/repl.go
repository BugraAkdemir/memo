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
	"memo/internal/logx"
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
					// GetSize can return (0, 0, nil) — no error, but no
					// winsize either — for a pty whose window size was
					// never reported (some pty setups, a resize event lost
					// mid-flight). render()'s maxVis clamps to a minimum of
					// 8 either way, but falling all the way to that instead
					// of the normal 80-column assumption made the composer
					// wrap after a handful of characters for no visible
					// reason. Treat non-positive the same as an error.
					if w, _, err := term.GetSize(fd); err == nil && w > 0 {
						return w
					}
					return 80
				},
			}
		}
	}

	clearScreen(out)

	ctx := context.Background()
	client := NewClient(baseURL)

	// Follow the GUI's display language, if it's ever been set — best-effort,
	// an unreachable/older backend or an unset value both just mean
	// SetLanguage's own default (Turkish) applies, same as before this
	// existed.
	if lang, err := client.GetUILanguage(ctx); err == nil {
		SetLanguage(lang)
	}

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
		go logx.GoRecover("replcli.heartbeatLoop", func() { heartbeatLoop(hbCtx, client, clientID) })
		defer func() {
			unregCtx, unregCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer unregCancel()
			_ = client.UnregisterClient(unregCtx, clientID)
		}()
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	s := &session{client: client, ctx: ctx, out: out, scanner: scanner, ownBackend: ownBackend, keys: keys, ed: ed, projectPath: projectPath}

	if ed != nil {
		// Best-effort: an unreachable/older backend just means the status
		// bar starts in its default (off) state, same as GetUILanguage above.
		if enabled, err := client.GetAgentAutoPermission(ctx); err == nil {
			ed.autoPermission = enabled
		}
		ed.onToggleAutoPermission = s.toggleAutoPermission
	}

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

	// sp is the in-flight turn's status-line spinner, non-nil only between
	// sendMessage starting and finishing a turn. handleAgentEvent reuses it
	// to show live tool-call activity instead of printing a permanent line
	// per tool call; askPermission stops and restarts it around its own
	// question. nil outside of sendMessage — every user is a nil-guarded
	// no-op, so calling one from outside a turn is harmless.
	sp *spinner
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
		return fmt.Errorf(t("chat_create_failed"), err)
	}
	return s.activateChat(id)
}

// activateChat switches the backend's active chat to id, re-enables agent
// mode (switching chats resets it backend-side) and records id as this
// session's current chat.
func (s *session) activateChat(id string) error {
	if err := s.client.SwitchChat(s.ctx, id); err != nil {
		return fmt.Errorf(t("chat_switch_failed"), err)
	}
	if err := s.client.SetAgentEnabled(s.ctx, true); err != nil {
		return fmt.Errorf(t("agent_enable_failed"), err)
	}
	// Web search is a global toggle, exactly like agent mode above — but
	// unlike agent mode, nothing in the CLI ever turned it on, so it just
	// sat at whatever it defaulted to (off) no matter what. Best-effort
	// (unlike agent mode's hard error above): an older backend without this
	// route, or a config write failure, just means web search stays off for
	// this chat — not something worth failing chat creation over, since the
	// REPL isn't built around it the way it is around agent mode.
	_ = s.client.SetWebSearchEnabled(s.ctx, true)
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
	fmt.Fprintln(s.out, dim(t("history_divider_start")))
	for _, m := range msgs {
		switch m.Role {
		case "user":
			fmt.Fprintln(s.out, promptStyle+m.Content)
		case "assistant":
			fmt.Fprintln(s.out, bold(brightMagenta("● "))+m.Content)
		}
	}
	fmt.Fprintln(s.out, dim(t("history_divider_end")))
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

	s.sp = newSpinner(s.out)
	onChunk := func(chunk api.StreamChunk) error {
		return s.handleChunk(chunk)
	}
	err := s.client.SendStream(ctx, line, onChunk)
	s.stopSpinner()
	s.sp = nil

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
			fmt.Fprintln(s.out, dim(t("cancelled_stream")))
			return
		}
		fmt.Fprintln(s.out, errorf(t("error_prefix"), friendlyError(err.Error())))
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
			fmt.Fprintln(s.out, dim(t("memory_saved")))
			return
		}
		// A save was attempted and actually failed (backend already emits
		// memory:error for this — autoStartEmbeddingModel/startupEmbeddingModel/
		// saveMemorySync, internal/app/*.go) — surface it instead of silently
		// giving up after this loop, which used to look exactly like nothing
		// happened even though the backend knew perfectly well why.
		if msg, ok := eventDataSince(events, afterSeq, "memory:error"); ok {
			// No forced "⚠ " prefix: several memory:error messages
			// (llama.go's embedding-not-found one, in particular — visible
			// in the very screenshot that prompted this fix) already embed
			// their own "⚠️", and yellow() alone already reads as a warning
			// everywhere else in this file.
			fmt.Fprintln(s.out, yellow(msg))
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
	active := s.memoryActive()
	// Best-effort: an older/incompatible backend or a transient error just
	// means the title line omits the version suffix, nothing else degrades.
	version, _ := s.client.Version(s.ctx)

	// Short deadline, deliberately shorter than the client's own default
	// 10s: CheckVersionUpdate's backend handler makes an external network
	// call, and this is a startup nicety, not something worth stalling the
	// welcome panel over on a slow or absent connection — it just silently
	// falls back to a filler tip instead of an update notice.
	updateNotice := ""
	checkCtx, cancel := context.WithTimeout(s.ctx, 2500*time.Millisecond)
	if latest, err := s.client.CheckVersionUpdate(checkCtx); err == nil && latest != "" {
		updateNotice = fmt.Sprintf(t("update_available"), strings.TrimLeft(latest, "Vv"))
	}
	cancel()

	termWidth := 0
	if s.ed != nil && s.ed.width != nil {
		termWidth = s.ed.width()
	}
	fmt.Fprintln(s.out, welcomePanel(version, s.projectPath, s.modelSummary(), updateNotice, termWidth))

	// The panel itself shows no memory/embedding status row — the backend's
	// status is not trustworthy enough to print as fact (see leftColumn's
	// note: a bare port ping can report "running" for a session whose memory
	// isn't working). What replaces it is this one actionable line, shown
	// only while embedding looks down, telling the user the exact command
	// that fixes it. Running /embedding successfully makes it go away on the
	// next panel draw (/clear, /session, or the next launch).
	if !active {
		fmt.Fprintln(s.out, yellow("⚠ "+t("memory_off_hint")))
		// Embedding auto-start (autoStartEmbeddingModel/startupEmbeddingModel,
		// internal/app/llama.go+embedding.go) can fail for reasons the hint
		// above doesn't cover — no model file found, a failed download, or a
		// port already occupied by something else. The backend already emits
		// a memory:error event carrying the real reason; print it underneath
		// as the detail line, dim so the actionable hint stays the headline.
		if events, err := s.client.Events(s.ctx); err == nil {
			if msg, ok := eventDataSince(events, 0, "memory:error"); ok {
				fmt.Fprintln(s.out, dim("  "+msg))
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
		return t("local_model")
	}
	return t("model_not_loaded_hint")
}

// memoryActive reports whether the embedding model (and therefore RAG
// memory) looks like it's running. When this process just started the
// backend itself (ownBackend), an embedding model can still be mid-load in
// the background — briefly retry before declaring memory off, so the
// welcome panel doesn't flash a "run /embedding" warning at someone whose
// embedding finishes loading half a second later. Attaching to an
// already-running backend has no such race — its embedding status is
// already settled, so a single check is both correct and instant.
//
// This only ever gates a hint, never a status claim: the underlying backend
// check can report "running" off a bare port ping, so it is not reliable
// enough to print as fact (see leftColumn, color.go).
func (s *session) memoryActive() bool {
	attempts := 1
	if s.ownBackend {
		attempts = 6
	}
	const interval = 400 * time.Millisecond
	for i := range attempts {
		if status, err := s.client.EmbeddingStatus(s.ctx); err == nil && status.Running {
			return true
		}
		if i < attempts-1 {
			time.Sleep(interval)
		}
	}
	return false
}

func (s *session) handleChunk(chunk api.StreamChunk) error {
	if chunk.Error != "" {
		s.stopSpinner()
		fmt.Fprintln(s.out, errorf("%s", friendlyError(chunk.Error)))
		return nil
	}

	switch chunk.FinishReason {
	case "status":
		s.stopSpinner()
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
			s.stopSpinner()
			s.aiTurnStarted = true
			fmt.Fprint(s.out, bold(brightMagenta("● ")))
		}
		typewriter(s.out, chunk.Content)
	}
	return nil
}

// stopSpinner halts the current turn's status-line spinner, if one is
// active — used right before printing something that needs the line to
// itself (an error, a status chunk, the reply's first token, a permission
// question). A no-op outside a turn (s.sp nil) or once already stopped
// (spinner.Stop is idempotent).
func (s *session) stopSpinner() {
	if s.sp != nil {
		s.sp.Stop()
	}
}

// resumeSpinner restarts the status-line spinner after stopSpinner — used
// by askPermission once its question is answered, so any further tool
// calls later in the same turn still get a live status line instead of the
// terminal looking frozen. Only does anything if a turn is actually in
// progress (s.sp was set by sendMessage, not nil'd out yet).
func (s *session) resumeSpinner() {
	if s.sp != nil {
		s.sp = newSpinner(s.out)
	}
}

// handleAgentEvent reflects one agent tool-call event as the current turn's
// status-line text rather than a permanent printed line — see spinner.go's
// SetLabel doc comment for why. permission_request is the one event type
// that still needs its own dedicated line (askPermission), since it's an
// actual question needing an answer, not just a status update.
func (s *session) handleAgentEvent(ev AgentEvent) error {
	if s.sp == nil {
		return nil // no turn in flight to attach a status update to
	}
	switch ev.Type {
	case "tool_executing":
		s.sp.SetLabel(dim(fmt.Sprintf(t("tool_running"), ev.Tool)))
	case "tool_result":
		s.sp.SetLabel(green(fmt.Sprintf(t("tool_done"), ev.Tool)))
	case "tool_error":
		s.sp.SetLabel(errorf(t("tool_error"), ev.Tool, ev.Error))
	case "permission_denied":
		s.sp.SetLabel(yellow(fmt.Sprintf(t("tool_denied"), ev.Tool)))
	case "permission_request":
		return s.askPermission(ev)
	}
	return nil
}

// toggleAutoPermission flips the backend's Shift+Tab auto-permission mode
// (PUT /api/agent/auto-permission — internal/agent/pipeline.go's
// autoPermission, the same flag the Flutter GUI's Shift+Tab shortcut
// drives) and returns the resulting state for the editor's status bar. Once
// on, permission_request events stop arriving entirely — every tool call is
// approved before the backend ever emits one — so askPermission below simply
// sees fewer calls rather than needing its own auto-approve branch.
// Best-effort: a transport error leaves the mode unchanged, matching how
// other backend-dependent REPL actions in this file degrade.
func (s *session) toggleAutoPermission(current bool) bool {
	next := !current
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	if err := s.client.SetAgentAutoPermission(ctx, next); err != nil {
		return current
	}
	return next
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

	// The status-line spinner would otherwise keep overwriting the same
	// line the question and its y/n prompt need — give it the line back
	// once answered, so any further tool calls later in this turn still
	// get a live status instead of the terminal looking frozen.
	s.stopSpinner()
	defer s.resumeSpinner()

	warnPrefix := "⚠"
	if ev.DangerLevel == "dangerous" {
		warnPrefix = t("danger_prefix")
	}
	fmt.Fprintln(s.out, "\n"+yellow(fmt.Sprintf(t("permission_ask"), warnPrefix, ev.Tool, describeToolCall(ev))))

	allowSession := ev.DangerLevel != "dangerous"
	prompt := t("permission_prompt_simple")
	if allowSession {
		prompt = t("permission_prompt_session")
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
		return t("friendly_no_model_error")
	}
	return raw
}
