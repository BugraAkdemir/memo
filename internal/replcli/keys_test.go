package replcli

import (
	"io"
	"strings"
	"testing"
	"time"
)

func keysFrom(input string) *keySource {
	return newKeySource(strings.NewReader(input))
}

func TestKeySource_DecodesBasicKeys(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []key
	}{
		{"ascii rune", "a", []key{{kind: keyRune, r: 'a'}}},
		{"utf8 turkish rune", "ç", []key{{kind: keyRune, r: 'ç'}}},
		{"enter cr", "\r", []key{{kind: keyEnter}}},
		{"enter lf", "\n", []key{{kind: keyEnter}}},
		{"backspace del", "\x7f", []key{{kind: keyBackspace}}},
		{"tab", "\t", []key{{kind: keyTab}}},
		{"ctrl-c", "\x03", []key{{kind: keyCtrlC}}},
		{"ctrl-d", "\x04", []key{{kind: keyCtrlD}}},
		{"csi up", "\x1b[A", []key{{kind: keyUp}}},
		{"csi down", "\x1b[B", []key{{kind: keyDown}}},
		{"csi right", "\x1b[C", []key{{kind: keyRight}}},
		{"csi left", "\x1b[D", []key{{kind: keyLeft}}},
		{"csi home", "\x1b[H", []key{{kind: keyHome}}},
		{"csi end", "\x1b[F", []key{{kind: keyEnd}}},
		{"csi delete", "\x1b[3~", []key{{kind: keyDelete}}},
		{"csi home tilde", "\x1b[1~", []key{{kind: keyHome}}},
		// SS3 variants — sent by terminals in application cursor mode; the
		// old parser rejected these outright, which is why arrow keys
		// "randomly" didn't work in some terminals.
		{"ss3 up", "\x1bOA", []key{{kind: keyUp}}},
		{"ss3 down", "\x1bOB", []key{{kind: keyDown}}},
		{"bare esc at end of input", "\x1b", []key{{kind: keyEsc}}},
		{"arrow then rune", "\x1b[Bx", []key{{kind: keyDown}, {kind: keyRune, r: 'x'}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ks := keysFrom(tt.input)
			for i, want := range tt.want {
				got := ks.readKey()
				if got != want {
					t.Errorf("key %d = %+v, want %+v", i, got, want)
				}
			}
			if got := ks.readKey(); got.kind != keyEOF {
				t.Errorf("after input, key = %+v, want keyEOF", got)
			}
		})
	}
}

func TestKeySource_BracketedPaste_CollapsesEmbeddedNewlines(t *testing.T) {
	// Before bracketed-paste support, a multi-line paste decoded as N
	// separate Enter presses (splitting one paste into N sent messages, and
	// running any "/"-prefixed line inside it as a command). The terminal
	// wraps the pasted bytes in ESC[200~ ... ESC[201~; readBracketedPaste
	// must consume exactly that block and collapse the embedded breaks.
	ks := keysFrom("\x1b[200~first line\nsecond line\r\nthird\x1b[201~x")
	got := ks.readKey()
	want := "first line second line third"
	if got.kind != keyPaste || got.text != want {
		t.Fatalf("first key = %+v, want {kind: keyPaste, text: %q}", got, want)
	}
	if got := ks.readKey(); got.kind != keyRune || got.r != 'x' {
		t.Fatalf("trailing key = %+v, want rune x (bytes after the terminator must not be swallowed)", got)
	}
}

func TestKeySource_BracketedPaste_PreservesUTF8AndPunctuation(t *testing.T) {
	ks := keysFrom("\x1b[200~türkçe metin, virgül de var\x1b[201~")
	got := ks.readKey()
	want := "türkçe metin, virgül de var"
	if got.kind != keyPaste || got.text != want {
		t.Fatalf("key = %+v, want {kind: keyPaste, text: %q}", got, want)
	}
}

func TestKeySource_BracketedPaste_EmptyPasteYieldsNone(t *testing.T) {
	ks := keysFrom("\x1b[200~\x1b[201~x")
	if got := ks.readKey(); got.kind != keyNone {
		t.Fatalf("first key = %+v, want keyNone for an empty paste", got)
	}
	if got := ks.readKey(); got.kind != keyRune || got.r != 'x' {
		t.Fatalf("second key = %+v, want rune x", got)
	}
}

func TestCollapsePasteNewlines(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"a\nb", "a b"},
		{"a\r\nb", "a b"},
		{"a\n\nb", "a b"},
		{"a    b", "a    b"}, // regular whitespace, no newline involved — left untouched
		{"no newlines here", "no newlines here"},
		{"\nleading and trailing\n", "leading and trailing"},
	}
	for _, tt := range tests {
		if got := collapsePasteNewlines(tt.in); got != tt.want {
			t.Errorf("collapsePasteNewlines(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestKeySource_UnknownCSISwallowedWhole(t *testing.T) {
	// An exotic sequence must be consumed entirely (keyNone), never leak
	// its bytes into the stream as if the user typed them.
	ks := keysFrom("\x1b[12;34Rx")
	if got := ks.readKey(); got.kind != keyNone {
		t.Fatalf("first key = %+v, want keyNone", got)
	}
	if got := ks.readKey(); got.kind != keyRune || got.r != 'x' {
		t.Fatalf("second key = %+v, want rune x", got)
	}
}

func TestWatchInterrupt_CancelsOnCtrlC(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	ks := newKeySource(pr)

	fired := make(chan struct{}, 4)
	w := ks.watchInterrupt(func() { fired <- struct{}{} })

	pw.Write([]byte("\x03"))
	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("Ctrl+C did not trigger the interrupt")
	}

	// Ordinary keys mid-stream are swallowed, not treated as interrupts.
	pw.Write([]byte("h"))
	select {
	case <-fired:
		t.Fatal("plain rune must not trigger the interrupt")
	case <-time.After(150 * time.Millisecond):
	}

	w.Stop()
}

func TestWatchInterrupt_ArrowKeyDoesNotCancel(t *testing.T) {
	// Arrows start with ESC — the watcher must decode the full sequence and
	// ignore it, not misread the leading ESC as a cancel request.
	pr, pw := io.Pipe()
	defer pw.Close()
	ks := newKeySource(pr)

	fired := make(chan struct{}, 4)
	w := ks.watchInterrupt(func() { fired <- struct{}{} })

	pw.Write([]byte("\x1b[A"))
	select {
	case <-fired:
		t.Fatal("arrow key must not trigger the interrupt")
	case <-time.After(200 * time.Millisecond):
	}

	w.Stop()
}

func TestCRLFWriter(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"a\nb", "a\r\nb"},
		{"a\r\nb", "a\r\nb"}, // already-correct pairs stay untouched
		{"\n\n", "\r\n\r\n"},
		{"düz metin", "düz metin"},
	}
	for _, tt := range tests {
		var sb strings.Builder
		n, err := crlfWriter{&sb}.Write([]byte(tt.in))
		if err != nil {
			t.Fatalf("Write(%q) error = %v", tt.in, err)
		}
		if n != len(tt.in) {
			t.Errorf("Write(%q) n = %d, want %d (must report input length)", tt.in, n, len(tt.in))
		}
		if sb.String() != tt.want {
			t.Errorf("Write(%q) = %q, want %q", tt.in, sb.String(), tt.want)
		}
	}
}
