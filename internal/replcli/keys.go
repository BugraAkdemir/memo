package replcli

import (
	"io"
	"memo/internal/logx"
	"strings"
	"time"
	"unicode/utf8"
)

// keyKind classifies one decoded keypress.
type keyKind int

const (
	keyNone keyKind = iota // unrecognized/incomplete sequence — callers ignore it
	keyRune
	keyEnter
	keyBackspace
	keyDelete
	keyUp
	keyDown
	keyLeft
	keyRight
	keyHome
	keyEnd
	keyTab
	keyShiftTab
	keyEsc
	keyCtrlC
	keyCtrlD
	keyCtrlK
	keyCtrlL
	keyCtrlU
	keyCtrlW
	keyEOF
	keyPaste // a decoded bracketed-paste block; see readBracketedPaste
)

type key struct {
	kind keyKind
	r    rune   // set only for keyRune
	text string // set only for keyPaste
}

// keySource turns a raw-mode byte stream into decoded keypresses. A single
// background goroutine owns all reads from the underlying reader and feeds a
// buffered channel; every consumer (line editor, arrow-key menus, the
// mid-stream interrupt watcher) takes bytes from that one channel, so two
// readers can never race for the same keystrokes — exactly the bug that made
// nested menus ignore arrow keys before.
type keySource struct {
	ch chan byte

	// pending holds bytes consumed while disambiguating an ESC that turned
	// out to belong to the NEXT keypress (Esc immediately followed by Enter,
	// pasted text). They are replayed before the channel is read again, so
	// no keystroke is ever silently dropped. Only ever touched by the single
	// active consumer, never the feeder goroutine.
	pending []byte

	// escWait is how long an ESC byte waits for a follow-up before being
	// reported as a bare Esc press. Arrow keys arrive as one 3-byte burst,
	// so anything already in flight lands well within this window.
	escWait time.Duration
}

func newKeySource(r io.Reader) *keySource {
	k := &keySource{ch: make(chan byte, 256), escWait: 50 * time.Millisecond}
	go func() {
		defer logx.Recover("replcli.keySource reader")
		buf := make([]byte, 256)
		for {
			n, err := r.Read(buf)
			for _, b := range buf[:n] {
				k.ch <- b
			}
			if err != nil {
				close(k.ch)
				return
			}
		}
	}()
	return k
}

// readKey blocks until the next full keypress (or EOF) is available.
func (k *keySource) readKey() key {
	if len(k.pending) > 0 {
		b := k.pending[0]
		k.pending = k.pending[1:]
		return k.assemble(b)
	}
	b, ok := <-k.ch
	if !ok {
		return key{kind: keyEOF}
	}
	return k.assemble(b)
}

// nextByte waits up to escWait for a byte that should already be in flight
// (the tail of an escape sequence or a multi-byte UTF-8 rune).
func (k *keySource) nextByte() (byte, bool) {
	if len(k.pending) > 0 {
		b := k.pending[0]
		k.pending = k.pending[1:]
		return b, true
	}
	select {
	case b, ok := <-k.ch:
		return b, ok
	case <-time.After(k.escWait):
		return 0, false
	}
}

// assemble decodes one keypress starting from its first byte.
func (k *keySource) assemble(b byte) key {
	switch b {
	case 1: // Ctrl+A
		return key{kind: keyHome}
	case 3:
		return key{kind: keyCtrlC}
	case 4:
		return key{kind: keyCtrlD}
	case 5: // Ctrl+E
		return key{kind: keyEnd}
	case 9:
		return key{kind: keyTab}
	case 11:
		return key{kind: keyCtrlK}
	case 12:
		return key{kind: keyCtrlL}
	case 21:
		return key{kind: keyCtrlU}
	case 23:
		return key{kind: keyCtrlW}
	case '\r', '\n':
		return key{kind: keyEnter}
	case 8, 127:
		return key{kind: keyBackspace}
	case 27:
		return k.assembleEscape()
	}
	if b < 32 {
		return key{kind: keyNone}
	}
	if b < utf8.RuneSelf {
		return key{kind: keyRune, r: rune(b)}
	}
	return k.assembleUTF8(b)
}

// assembleEscape decodes what follows an ESC byte: a CSI sequence (ESC [ …),
// an SS3 sequence (ESC O …, sent for arrows by terminals in application
// cursor mode — the old parser rejected these, which is one reason arrows
// "randomly" didn't work), or nothing, meaning a bare Esc press.
func (k *keySource) assembleEscape() key {
	b, ok := k.nextByte()
	if !ok {
		return key{kind: keyEsc}
	}
	switch b {
	case '[':
		return k.assembleCSI()
	case 'O': // SS3
		f, ok := k.nextByte()
		if !ok {
			return key{kind: keyEsc}
		}
		return csiFinal(f, "")
	default:
		// Not a sequence — a bare Esc pressed right before another key
		// (or Alt+<key>). Report the Esc and replay the byte so the next
		// keypress isn't swallowed.
		k.pending = append(k.pending, b)
		return key{kind: keyEsc}
	}
}

// assembleCSI reads parameter bytes until the sequence's final byte
// (0x40–0x7E), then maps the few sequences the REPL cares about. Unknown
// sequences are consumed whole and reported as keyNone, so a fancy terminal
// key can never leak garbage bytes into the input buffer.
func (k *keySource) assembleCSI() key {
	params := ""
	for {
		b, ok := k.nextByte()
		if !ok {
			return key{kind: keyNone}
		}
		if b >= 0x40 && b <= 0x7E {
			if b == '~' && params == "200" {
				return k.readBracketedPaste()
			}
			return csiFinal(b, params)
		}
		params += string(rune(b))
		if len(params) > 16 { // runaway sequence — bail out
			return key{kind: keyNone}
		}
	}
}

// blockingByte waits indefinitely for the next byte (draining pending
// first). Unlike nextByte, it has no escWait timeout — paste bodies can be
// large and arrive slower than a keypress's escape-sequence tail, so a
// short timeout would truncate them.
func (k *keySource) blockingByte() (byte, bool) {
	if len(k.pending) > 0 {
		b := k.pending[0]
		k.pending = k.pending[1:]
		return b, true
	}
	b, ok := <-k.ch
	return b, ok
}

// readBracketedPaste consumes raw bytes up through the ESC[201~ terminator
// that closes a bracketed-paste block (xterm's paste-safety convention: the
// terminal wraps a paste in ESC[200~ ... ESC[201~, and everything in
// between is literal content, not something to interpret as further escape
// sequences or control keys). Without this, every embedded newline in a
// multi-line paste used to decode as a real Enter press, splitting one
// pasted paragraph into N separately-submitted messages — and running any
// "/"-prefixed line inside it as a command. Embedded line breaks are
// collapsed to spaces so the whole paste becomes one insertable chunk.
func (k *keySource) readBracketedPaste() key {
	const terminator = "\x1b[201~"
	var raw []byte
	matched := 0
	for {
		b, ok := k.blockingByte()
		if !ok {
			break
		}
		if b == terminator[matched] {
			matched++
			if matched == len(terminator) {
				break
			}
			continue
		}
		if matched > 0 {
			raw = append(raw, terminator[:matched]...)
			matched = 0
		}
		if b == terminator[0] {
			matched = 1
			continue
		}
		raw = append(raw, b)
	}
	text := collapsePasteNewlines(string(raw))
	if text == "" {
		return key{kind: keyNone}
	}
	return key{kind: keyPaste, text: text}
}

// collapsePasteNewlines turns any \r\n/\r/\n run in s into a single space —
// the composer's buffer is single-line, so a hard line break from the
// source (a copied paragraph, a multi-line code snippet) is flattened
// instead of being submitted as several separate messages.
func collapsePasteNewlines(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == '\r' || r == '\n' {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return strings.TrimSpace(b.String())
}

func csiFinal(final byte, params string) key {
	switch final {
	case 'A':
		return key{kind: keyUp}
	case 'B':
		return key{kind: keyDown}
	case 'C':
		return key{kind: keyRight}
	case 'D':
		return key{kind: keyLeft}
	case 'H':
		return key{kind: keyHome}
	case 'F':
		return key{kind: keyEnd}
	case 'Z': // Shift+Tab (CSI Z / "reverse tab")
		return key{kind: keyShiftTab}
	case '~':
		switch params {
		case "1", "7":
			return key{kind: keyHome}
		case "3":
			return key{kind: keyDelete}
		case "4", "8":
			return key{kind: keyEnd}
		}
	}
	return key{kind: keyNone}
}

// assembleUTF8 collects the continuation bytes of a multi-byte rune (Turkish
// input: ç, ğ, ı, ö, ş, ü…) and decodes it.
func (k *keySource) assembleUTF8(first byte) key {
	var need int
	switch {
	case first&0xE0 == 0xC0:
		need = 1
	case first&0xF0 == 0xE0:
		need = 2
	case first&0xF8 == 0xF0:
		need = 3
	default:
		return key{kind: keyNone}
	}
	bytes := []byte{first}
	for range need {
		b, ok := k.nextByte()
		if !ok {
			return key{kind: keyNone}
		}
		bytes = append(bytes, b)
	}
	r, _ := utf8.DecodeRune(bytes)
	if r == utf8.RuneError {
		return key{kind: keyNone}
	}
	return key{kind: keyRune, r: r}
}

// interruptWatch consumes keys while a reply is streaming so Esc / Ctrl+C can
// cancel the generation. All other keys pressed mid-stream are deliberately
// swallowed — they would otherwise pollute the next prompt.
type interruptWatch struct {
	stop chan struct{}
	done chan struct{}
}

// watchInterrupt starts the watcher; onInterrupt runs (once per keypress) for
// Esc or Ctrl+C. Stop must be called before anything else reads keys again.
func (k *keySource) watchInterrupt(onInterrupt func()) *interruptWatch {
	w := &interruptWatch{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(w.done)
		defer logx.Recover("replcli.keySource.watchInterrupt")
		for {
			select {
			case <-w.stop:
				return
			case b, ok := <-k.ch:
				if !ok {
					return
				}
				kk := k.assemble(b)
				if kk.kind == keyEsc || kk.kind == keyCtrlC {
					onInterrupt()
				}
			}
		}
	}()
	return w
}

// Stop halts the watcher and waits until it is fully detached from the byte
// channel, so the caller can safely resume reading keys.
func (w *interruptWatch) Stop() {
	close(w.stop)
	<-w.done
}
