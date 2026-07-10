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
// menu and tab completion.
var slashCommands = []commandSpec{
	{"/help", "komut listesini göster"},
	{"/models", "modelleri ve sağlayıcıları listele"},
	{"/model", "bir sohbet modeli seç ve başlat"},
	{"/embedding", "bir embedding modeli seç ve başlat"},
	{"/model-download", "model indirmek için masaüstü uygulamasını aç"},
	{"/connect", "harici bir API sağlayıcısına bağlan"},
	{"/gui", "masaüstü uygulamasını aç"},
	{"/clear", "sohbeti temizle, yeni bir sohbet başlat"},
	{"/session", "bu projedeki sohbetler arasında geç"},
	{"/tasklist", "görev listelerini yönet (list/create/start/stop/delete/show)"},
	{"/remote", "ngrok ile uzak erişim tüneli aç"},
	{"/exit", "çık"},
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
				e.notice = "çıkmak için tekrar Ctrl+C"
			}

		case keyEnter:
			if m := e.matches(); e.menuOpen() && len(m) > 0 {
				// Enter runs the highlighted command, even if only a prefix
				// was typed ("/mo" + Enter → the selected entry).
				e.buf = []rune(m[e.menuSel].label)
				e.cursor = len(e.buf)
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
			if m := e.matches(); e.menuOpen() && len(m) > 0 {
				e.buf = []rune(m[e.menuSel].label)
				e.cursor = len(e.buf)
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

// menuOpen reports whether the dropdown should be visible: a single
// "/"-prefixed token, not dismissed with Esc.
func (e *editor) menuOpen() bool {
	if !e.menuEnable || e.menuSuppress || len(e.buf) == 0 || e.buf[0] != '/' {
		return false
	}
	return !strings.ContainsRune(string(e.buf), ' ')
}

// matches returns the dropdown entries for the current buffer (prefix match;
// bare "/" lists everything).
func (e *editor) matches() []commandSpec {
	if !e.menuOpen() {
		return nil
	}
	typed := strings.ToLower(string(e.buf))
	if typed == "/" {
		return slashCommands
	}
	var out []commandSpec
	for _, c := range slashCommands {
		if strings.HasPrefix(c.label, typed) {
			out = append(out, c)
		}
	}
	return out
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
		for i, c := range m {
			b.WriteString("\n")
			label := c.label
			if i == e.menuSel {
				b.WriteString(bold(brightCyan("  ▶ " + label)))
			} else {
				b.WriteString("    " + label)
			}
			if pad := 18 - len(label); pad > 0 {
				b.WriteString(strings.Repeat(" ", pad))
			}
			b.WriteString("  " + dim(c.hint))
			rows++
		}
		b.WriteString("\n  " + dim("↑↓ gezin · Tab tamamla · Enter çalıştır · Esc kapat"))
		rows++
	} else if e.notice != "" {
		b.WriteString("\n  " + dim(e.notice))
		rows++
	}
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
