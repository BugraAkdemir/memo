package replcli

import (
	"fmt"
	"io"
	"strings"
)

// commandSpec is one entry of the live slash-command dropdown.
type commandSpec struct {
	label string
	hint  string
}

// slashCommands is the single source of truth for the dropdown, the bare "/"
// menu and tab completion. A function, not a package-level literal, because
// its hints must reflect activeLang — which SetLanguage only sets after this
// package's vars would already have finished initializing.
func slashCommands() []commandSpec {
	return []commandSpec{
		{"/help", t("cmd_help_hint")},
		{"/models", t("cmd_models_hint")},
		{"/model", t("cmd_model_hint")},
		{"/embedding", t("cmd_embedding_hint")},
		{"/model-download", t("cmd_model_download_hint")},
		{"/connect", t("cmd_connect_hint")},
		{"/disconnect", t("cmd_disconnect_hint")},
		{"/gui", t("cmd_gui_hint")},
		{"/clear", t("cmd_clear_hint")},
		{"/session", t("cmd_session_hint")},
		{"/tasklist", t("cmd_tasklist_hint")},
		{"/remote", t("cmd_remote_hint")},
		{"/update", t("cmd_update_hint")},
		{"/theme", t("cmd_theme_hint")},
		{"/exit", t("cmd_exit_hint")},
	}
}

// statusBarText is the composer's persistent bottom hint — the terminal
// equivalent of Claude Code's "manual mode on · ? for shortcuts" bar.
func statusBarText() string { return t("status_bar_text") }

// statusBarLine renders the composer's bottom line. themeDefault replaces
// the claude-code theme's static hint bar with live data — model, memory,
// and auto-permission state all folded into one line — instead of swapping
// to a separate "auto-accept on" line the way claude-code does; auto-permission's
// segment is computed fresh every render (e.autoPermission, a plain field
// Shift+Tab already updates instantly) since it can change on any
// keystroke, while liveStatusPrefix (model/memory) is a cheaper snapshot
// the session refreshes at natural checkpoints (welcome, /model, a
// finished reply) rather than on every keystroke — recomputing model/memory
// status per keypress would mean a backend round trip per character typed.
func (e *editor) statusBarLine() string {
	if e.theme == themeDefault {
		auto := dim(t("live_status_auto_off"))
		if e.autoPermission {
			auto = yellow(t("live_status_auto_on"))
		}
		prefix := e.liveStatusPrefix
		if prefix == "" {
			prefix = dim("memo")
		}
		return prefix + dim("  ·  ") + auto + dim("  ·  "+t("live_status_esc_hint"))
	}
	if e.autoPermission {
		return yellow(t("auto_permission_status_on"))
	}
	return dim(statusBarText())
}

// editor is a raw-mode line editor: cursor movement, in-line editing,
// history, and a live slash-command dropdown that opens the moment "/" is
// typed — no Enter needed — and is navigated with the arrow keys, the way
// Claude Code's own composer works. It is only constructed when stdin is a
// real terminal; tests and piped input keep the plain bufio.Scanner path.
type editor struct {
	out   io.Writer
	keys  *keySource
	width func() int // terminal columns; consulted on every render

	// projectPath roots the "@" file-mention dropdown's directory walk
	// (filematch.go). Empty is valid — it just means the dropdown never
	// finds anything to show.
	projectPath string

	history []string

	buf     []rune
	cursor  int
	histIdx int    // == len(history) while editing the live draft
	draft   []rune // stashed draft while browsing history

	menuSel       int
	menuSuppress  bool   // true after Esc closed the dropdown, until the buffer changes
	notice        string // one-shot dim message drawn under the input line
	rowsBelow     int    // rows currently drawn under the input line
	pendingCtrlC  bool   // first Ctrl+C on an empty line arms quit-on-repeat
	historyEnable bool
	menuEnable    bool

	// autoPermission mirrors the backend's Shift+Tab auto-permission flag
	// (internal/agent/pipeline.go's autoPermission — every tool call
	// auto-approved, no permission_request event ever emitted) so the
	// status bar can show its current state live, the same as the Flutter
	// GUI's chat_screen.dart indicator. onToggleAutoPermission, if set, is
	// invoked on a Shift+Tab keypress (from either the main composer or a
	// secondary y/n prompt) with the current state and must apply the
	// flip server-side and return the resulting state (unchanged on
	// failure); nil leaves Shift+Tab a no-op.
	autoPermission         bool
	onToggleAutoPermission func(current bool) bool

	// theme selects the status-bar style — see statusBarLine. liveStatusPrefix
	// is themeDefault's cached "<model> · hafıza ●" segment, already ANSI-colored;
	// set by the session (refreshLiveStatus, repl.go), read verbatim here.
	theme            replTheme
	liveStatusPrefix string
}

// readLine edits one full line with the slash dropdown and history enabled.
// ok=false means the user asked to leave (Ctrl+D on empty line, double
// Ctrl+C, or EOF).
func (e *editor) readLine(prompt string) (line string, ok bool) {
	e.menuEnable = true
	e.historyEnable = true
	return e.edit(prompt)
}

// readLinePlain edits a secondary prompt (y/n answers, search terms): full
// in-line editing but no dropdown and no history.
func (e *editor) readLinePlain(prompt string) (line string, ok bool) {
	e.menuEnable = false
	e.historyEnable = false
	return e.edit(prompt)
}

func (e *editor) edit(prompt string) (string, bool) {
	e.buf = nil
	e.cursor = 0
	e.histIdx = len(e.history)
	e.draft = nil
	e.menuSel = 0
	e.menuSuppress = false
	e.notice = ""
	e.rowsBelow = 0

	e.render(prompt)
	for {
		k := e.keys.readKey()
		if k.kind != keyCtrlC {
			e.pendingCtrlC = false
		}
		if k.kind != keyNone {
			e.notice = ""
		}

		switch k.kind {
		case keyEOF:
			e.finish(prompt)
			return "", false

		case keyCtrlD:
			if len(e.buf) == 0 {
				e.finish(prompt)
				return "", false
			}

		case keyCtrlC:
			if len(e.buf) > 0 {
				e.buf = nil
				e.cursor = 0
				e.menuSel = 0
			} else if e.pendingCtrlC {
				e.finish(prompt)
				return "", false
			} else {
				e.pendingCtrlC = true
				e.notice = t("ctrl_c_again_to_exit")
			}

		case keyEnter:
			mode, atStart, _ := e.currentMenuMode()
			m := e.matches()
			if mode == menuAt && len(m) > 0 {
				// A file mention is inserted, not submitted — the user
				// keeps composing the rest of the message.
				e.applySelection(mode, atStart, m[e.menuSel])
				break
			}
			if mode == menuSlash && len(m) > 0 {
				// Enter runs the highlighted command, even if only a prefix
				// was typed ("/mo" + Enter → the selected entry).
				e.applySelection(mode, atStart, m[e.menuSel])
			}
			line := string(e.buf)
			e.finish(prompt)
			if e.historyEnable && strings.TrimSpace(line) != "" {
				if n := len(e.history); n == 0 || e.history[n-1] != line {
					e.history = append(e.history, line)
				}
			}
			return line, true

		case keyTab:
			if mode, atStart, _ := e.currentMenuMode(); mode != menuNone {
				if m := e.matches(); len(m) > 0 {
					e.applySelection(mode, atStart, m[e.menuSel])
				}
			}

		case keyShiftTab:
			if e.onToggleAutoPermission != nil {
				e.autoPermission = e.onToggleAutoPermission(e.autoPermission)
			}

		case keyEsc:
			if e.menuOpen() {
				e.menuSuppress = true
			}

		case keyUp:
			if m := e.matches(); e.menuOpen() && len(m) > 0 {
				e.menuSel = (e.menuSel - 1 + len(m)) % len(m)
			} else {
				e.historyUp()
			}

		case keyDown:
			if m := e.matches(); e.menuOpen() && len(m) > 0 {
				e.menuSel = (e.menuSel + 1) % len(m)
			} else {
				e.historyDown()
			}

		case keyLeft:
			if e.cursor > 0 {
				e.cursor--
			}
		case keyRight:
			if e.cursor < len(e.buf) {
				e.cursor++
			}
		case keyHome:
			e.cursor = 0
		case keyEnd:
			e.cursor = len(e.buf)

		case keyBackspace:
			if e.cursor > 0 {
				e.buf = append(e.buf[:e.cursor-1], e.buf[e.cursor:]...)
				e.cursor--
				e.edited()
			}
		case keyDelete:
			if e.cursor < len(e.buf) {
				e.buf = append(e.buf[:e.cursor], e.buf[e.cursor+1:]...)
				e.edited()
			}
		case keyCtrlU:
			if e.cursor > 0 {
				e.buf = append([]rune{}, e.buf[e.cursor:]...)
				e.cursor = 0
				e.edited()
			}
		case keyCtrlK:
			if e.cursor < len(e.buf) {
				e.buf = e.buf[:e.cursor]
				e.edited()
			}
		case keyCtrlW:
			e.deleteWordBack()
		case keyCtrlL:
			clearScreen(e.out)

		case keyRune:
			e.buf = append(e.buf[:e.cursor], append([]rune{k.r}, e.buf[e.cursor:]...)...)
			e.cursor++
			e.edited()

		case keyPaste:
			pasted := []rune(k.text)
			e.buf = append(e.buf[:e.cursor], append(pasted, e.buf[e.cursor:]...)...)
			e.cursor += len(pasted)
			e.edited()
		}

		e.render(prompt)
	}
}

// edited runs after every buffer mutation: re-arms the dropdown after an Esc
// and clamps the selection to the new match list.
func (e *editor) edited() {
	e.menuSuppress = false
	if m := e.matches(); e.menuSel >= len(m) {
		e.menuSel = 0
	}
}

func (e *editor) deleteWordBack() {
	if e.cursor == 0 {
		return
	}
	i := e.cursor
	for i > 0 && e.buf[i-1] == ' ' {
		i--
	}
	for i > 0 && e.buf[i-1] != ' ' {
		i--
	}
	e.buf = append(e.buf[:i], e.buf[e.cursor:]...)
	e.cursor = i
	e.edited()
}

func (e *editor) historyUp() {
	if !e.historyEnable || e.histIdx == 0 || len(e.history) == 0 {
		return
	}
	if e.histIdx == len(e.history) {
		e.draft = append([]rune{}, e.buf...)
	}
	e.histIdx--
	e.buf = []rune(e.history[e.histIdx])
	e.cursor = len(e.buf)
	e.menuSuppress = true // recalled lines shouldn't pop the dropdown open
}

func (e *editor) historyDown() {
	if !e.historyEnable || e.histIdx >= len(e.history) {
		return
	}
	e.histIdx++
	if e.histIdx == len(e.history) {
		e.buf = append([]rune{}, e.draft...)
	} else {
		e.buf = []rune(e.history[e.histIdx])
	}
	e.cursor = len(e.buf)
	e.menuSuppress = true
}

// menuMode identifies which live dropdown (if any) applies to the current
// buffer/cursor position: the whole-line "/" command palette (unchanged from
// before), or an anywhere-in-message "@" file-reference picker.
type menuMode int

const (
	menuNone menuMode = iota
	menuSlash
	menuAt
)

// currentMenuMode inspects the buffer and cursor and reports which dropdown
// (if any) should be showing. For menuAt it also returns atStart — the
// buffer index the "@" itself sits at, needed to splice just that token
// (not the whole line) once a file is picked — and query, the text already
// typed after "@" up to the cursor.
func (e *editor) currentMenuMode() (mode menuMode, atStart int, query string) {
	if !e.menuEnable || e.menuSuppress || len(e.buf) == 0 {
		return menuNone, 0, ""
	}
	if e.buf[0] == '/' && !strings.ContainsRune(string(e.buf), ' ') {
		return menuSlash, 0, ""
	}
	// Walk back from the cursor to the start of the word it's currently in
	// (or right after, if it sits at a word boundary with nothing typed
	// yet). i == e.cursor means nothing was scanned — the cursor is right
	// after a space (or at column 0), not inside/after an "@" token, so
	// that case is deliberately excluded below.
	i := e.cursor
	for i > 0 && e.buf[i-1] != ' ' {
		i--
	}
	if i < e.cursor && i < len(e.buf) && e.buf[i] == '@' {
		return menuAt, i, string(e.buf[i+1 : e.cursor])
	}
	return menuNone, 0, ""
}

// menuOpen reports whether either live dropdown should be visible.
func (e *editor) menuOpen() bool {
	mode, _, _ := e.currentMenuMode()
	return mode != menuNone
}

// matches returns the dropdown entries for the current buffer: slash
// commands (prefix match; bare "/" lists everything) or, mid-message after
// an "@", matching project files (filematch.go).
func (e *editor) matches() []commandSpec {
	mode, _, query := e.currentMenuMode()
	switch mode {
	case menuSlash:
		cmds := slashCommands()
		typed := strings.ToLower(string(e.buf))
		if typed == "/" {
			return cmds
		}
		var out []commandSpec
		for _, c := range cmds {
			if strings.HasPrefix(c.label, typed) {
				out = append(out, c)
			}
		}
		return out
	case menuAt:
		return fileMatches(e.projectPath, query)
	default:
		return nil
	}
}

// applySelection inserts the chosen entry: a full-line replacement for a
// slash command (it IS the whole line), or a splice of just the "@" token
// at atStart..cursor for a file mention (everything else the user typed
// before/after it is left alone).
func (e *editor) applySelection(mode menuMode, atStart int, choice commandSpec) {
	switch mode {
	case menuSlash:
		e.buf = []rune(choice.label)
		e.cursor = len(e.buf)
	case menuAt:
		token := []rune("@" + choice.label)
		tail := append([]rune{}, e.buf[e.cursor:]...)
		e.buf = append(append(e.buf[:atStart:atStart], token...), tail...)
		e.cursor = atStart + len(token)
	}
}

// render redraws the input line and whatever lives under it (dropdown or
// notice), leaving the terminal cursor at the editing position. It always
// starts from column 0 of the input row and wipes downward, so it is safe to
// call after any state change.
func (e *editor) render(prompt string) {
	w := 80
	if e.width != nil {
		if got := e.width(); got > 0 {
			w = got
		}
	}
	promptW := len([]rune(stripANSI(prompt)))

	// Horizontal scroll window: the cursor must always be visible.
	maxVis := max(w-promptW-1, 8)
	start := 0
	if e.cursor > maxVis {
		start = e.cursor - maxVis
	}
	end := min(start+maxVis, len(e.buf))
	visible := string(e.buf[start:end])
	curCol := promptW + (e.cursor - start)

	var b strings.Builder
	b.WriteString("\r\033[J")
	b.WriteString(prompt)
	if len(visible) > 0 {
		b.WriteString(userInputStart + visible + colorReset)
	}

	rows := 0
	if m := e.matches(); len(m) > 0 {
		mode, _, _ := e.currentMenuMode()
		for i, c := range m {
			b.WriteString("\n")
			label := c.label
			if i == e.menuSel {
				b.WriteString(bold(gold("  ▶ " + label)))
			} else {
				b.WriteString("    " + label)
			}
			if pad := 18 - len(label); pad > 0 {
				b.WriteString(strings.Repeat(" ", pad))
			}
			b.WriteString("  " + dim(c.hint))
			rows++
		}
		hint := t("dropdown_hint_slash")
		if mode == menuAt {
			hint = t("dropdown_hint_at")
		}
		b.WriteString("\n  " + dim(hint))
		rows++
	} else if e.notice != "" {
		b.WriteString("\n  " + dim(e.notice))
		rows++
	}
	// A persistent status bar under the input line, always visible
	// regardless of dropdown/notice state — the terminal equivalent of
	// Claude Code's bottom "manual mode on · ? for shortcuts" bar.
	b.WriteString("\n  " + e.statusBarLine())
	rows++
	e.rowsBelow = rows

	// Park the terminal cursor back at the edit position.
	if rows > 0 {
		fmt.Fprintf(&b, "\033[%dA", rows)
	}
	b.WriteString("\r")
	if curCol > 0 {
		fmt.Fprintf(&b, "\033[%dC", curCol)
	}
	fmt.Fprint(e.out, b.String())
}

// finish clears anything under the input line, redraws the final state of
// the line and moves to the next row — the reply (or the next prompt) starts
// on a clean line with no dropdown leftovers above it.
func (e *editor) finish(prompt string) {
	line := prompt
	if len(e.buf) > 0 {
		line += userInputStart + string(e.buf) + colorReset
	}
	fmt.Fprint(e.out, "\r\033[J"+line+"\n")
	e.rowsBelow = 0
}

// stripANSI removes SGR escape sequences so display widths can be computed
// from styled strings.
func stripANSI(s string) string {
	var b strings.Builder
	inSeq := false
	for _, r := range s {
		switch {
		case inSeq:
			if (r >= 0x40 && r <= 0x7E) && r != '[' {
				inSeq = false
			}
		case r == 0x1b:
			inSeq = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// crlfWriter rewrites "\n" as "\r\n" on the way out. With the terminal in
// raw mode for the whole REPL session, ONLCR post-processing is off — a bare
// "\n" moves down a row but keeps the column, stair-stepping every multi-line
// print. Wrapping the output once here means the rest of the package can keep
// printing plain "\n" exactly as before.
type crlfWriter struct {
	w io.Writer
}

func (c crlfWriter) Write(p []byte) (int, error) {
	var b []byte
	last := byte(0)
	for _, ch := range p {
		if ch == '\n' && last != '\r' {
			b = append(b, '\r')
		}
		b = append(b, ch)
		last = ch
	}
	if _, err := c.w.Write(b); err != nil {
		return 0, err
	}
	return len(p), nil
}
