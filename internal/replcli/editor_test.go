package replcli

import (
	"bytes"
	"strings"
	"testing"
)

// newTestEditor builds an editor reading the scripted keystrokes, with a
// fixed width so rendering is deterministic.
func newTestEditor(input string) (*editor, *bytes.Buffer) {
	var out bytes.Buffer
	return &editor{
		out:   &out,
		keys:  keysFrom(input),
		width: func() int { return 80 },
	}, &out
}

func TestEditor_PlainLine(t *testing.T) {
	ed, _ := newTestEditor("merhaba dünya\r")
	line, ok := ed.readLine("> ")
	if !ok || line != "merhaba dünya" {
		t.Fatalf("readLine = %q, %v", line, ok)
	}
}

func TestEditor_CursorEditing(t *testing.T) {
	// "ab", left, insert "c" → "acb"
	ed, _ := newTestEditor("ab\x1b[Dc\r")
	line, ok := ed.readLine("> ")
	if !ok || line != "acb" {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, "acb")
	}
}

func TestEditor_BackspaceMidLine(t *testing.T) {
	// "abc", left, backspace (deletes 'b') → "ac"
	ed, _ := newTestEditor("abc\x1b[D\x7f\r")
	line, ok := ed.readLine("> ")
	if !ok || line != "ac" {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, "ac")
	}
}

func TestEditor_SlashDropdown_ArrowThenEnterRunsSelection(t *testing.T) {
	// "/" opens the dropdown immediately — down once selects the second
	// command, Enter runs it. This is the flow that used to require sending
	// "/" first and then navigating a separate menu.
	ed, _ := newTestEditor("/\x1b[B\r")
	line, ok := ed.readLine("> ")
	if !ok || line != slashCommands[1].label {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, slashCommands[1].label)
	}
}

func TestEditor_SlashDropdown_PrefixEnterRunsFirstMatch(t *testing.T) {
	// "/mo" narrows to /models, /model, /model-download — Enter runs the
	// highlighted first match even though only a prefix was typed.
	ed, _ := newTestEditor("/mo\r")
	line, ok := ed.readLine("> ")
	if !ok || line != "/models" {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, "/models")
	}
}

func TestEditor_SlashDropdown_TabCompletes(t *testing.T) {
	ed, _ := newTestEditor("/emb\t\r")
	line, ok := ed.readLine("> ")
	if !ok || line != "/embedding" {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, "/embedding")
	}
}

func TestEditor_SlashDropdown_EscSendsLiteralText(t *testing.T) {
	// Esc closes the dropdown; Enter then submits exactly what was typed.
	ed, _ := newTestEditor("/mo\x1b\r")
	line, ok := ed.readLine("> ")
	if !ok || line != "/mo" {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, "/mo")
	}
}

func TestEditor_SlashDropdown_ArgumentsCloseIt(t *testing.T) {
	// Once a space is typed the dropdown is gone: Enter submits the full
	// argument form, and the arrow keys fall through to history (a no-op
	// here) instead of a hidden menu.
	ed, _ := newTestEditor("/model llama\x1b[A\r")
	line, ok := ed.readLine("> ")
	if !ok || line != "/model llama" {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, "/model llama")
	}
}

func TestEditor_DropdownRendersMatches(t *testing.T) {
	ed, out := newTestEditor("/mo\r")
	ed.readLine("> ")
	rendered := out.String()
	for _, want := range []string{"/models", "/model-download", "Tab tamamla"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("dropdown output missing %q", want)
		}
	}
}

func TestEditor_History(t *testing.T) {
	// First line submits "selam"; second prompt recalls it with Up.
	ed, _ := newTestEditor("selam\r\x1b[A\r")
	if line, ok := ed.readLine("> "); !ok || line != "selam" {
		t.Fatalf("first readLine = %q, %v", line, ok)
	}
	if line, ok := ed.readLine("> "); !ok || line != "selam" {
		t.Fatalf("recalled readLine = %q, %v, want %q", line, ok, "selam")
	}
}

func TestEditor_HistoryDownRestoresDraft(t *testing.T) {
	// Type a draft, go up into history, come back down — the draft returns.
	ed, _ := newTestEditor("eski\rtaslak\x1b[A\x1b[B\r")
	if line, ok := ed.readLine("> "); !ok || line != "eski" {
		t.Fatalf("first readLine = %q, %v", line, ok)
	}
	if line, ok := ed.readLine("> "); !ok || line != "taslak" {
		t.Fatalf("draft readLine = %q, %v, want %q", line, ok, "taslak")
	}
}

func TestEditor_CtrlC_ClearsNonEmptyLine(t *testing.T) {
	ed, _ := newTestEditor("yanlış\x03doğru\r")
	line, ok := ed.readLine("> ")
	if !ok || line != "doğru" {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, "doğru")
	}
}

func TestEditor_DoubleCtrlC_Exits(t *testing.T) {
	ed, _ := newTestEditor("\x03\x03")
	if line, ok := ed.readLine("> "); ok {
		t.Fatalf("readLine = %q, ok=true — double Ctrl+C on empty line must exit", line)
	}
}

func TestEditor_CtrlD_OnEmptyLineExits(t *testing.T) {
	ed, _ := newTestEditor("\x04")
	if _, ok := ed.readLine("> "); ok {
		t.Fatal("Ctrl+D on empty line must exit")
	}
}

func TestEditor_CtrlU_KillsToStart(t *testing.T) {
	ed, _ := newTestEditor("hepsi silinsin\x15kalan\r")
	line, ok := ed.readLine("> ")
	if !ok || line != "kalan" {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, "kalan")
	}
}

func TestEditor_BracketedPaste_InsertsAsOneChunkNotMultipleEnters(t *testing.T) {
	// Before bracketed-paste support the embedded newlines in a pasted
	// multi-line block each acted as a real Enter, submitting the paste as
	// several separate messages. A wrapped paste must now land as one
	// edited buffer, submitted only when the user actually presses Enter.
	ed, _ := newTestEditor("\x1b[200~line one\nline two\x1b[201~\r")
	line, ok := ed.readLine("> ")
	want := "line one line two"
	if !ok || line != want {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, want)
	}
}

func TestEditor_BracketedPaste_InsertsAtCursor(t *testing.T) {
	// "ac", left once (cursor between a and c), paste "b" — must land at the
	// cursor position like a typed rune, not appended to the end.
	ed, _ := newTestEditor("ac\x1b[D\x1b[200~b\x1b[201~\r")
	line, ok := ed.readLine("> ")
	if !ok || line != "abc" {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, "abc")
	}
}

func TestEditor_ReadLinePlain_NoDropdownNoHistory(t *testing.T) {
	// A "/" in a plain prompt (y/n answers, search terms) is literal text,
	// and Up must not pull chat history into the answer.
	ed, _ := newTestEditor("chat\r/evet\x1b[A\r")
	if line, ok := ed.readLine("> "); !ok || line != "chat" {
		t.Fatalf("chat readLine = %q, %v", line, ok)
	}
	line, ok := ed.readLinePlain("? ")
	if !ok || line != "/evet" {
		t.Fatalf("readLinePlain = %q, %v, want %q", line, ok, "/evet")
	}
}

func TestSelectFromMenu_ArrowNavigation(t *testing.T) {
	items := []menuItem{{Label: "bir"}, {Label: "iki"}, {Label: "üç"}}
	var out bytes.Buffer

	// down, down, enter → index 2
	if got := selectFromMenu(&out, keysFrom("\x1b[B\x1b[B\r"), "Seç", items); got != 2 {
		t.Errorf("arrow selection = %d, want 2", got)
	}
	// wrap: up from the top lands on the last item
	if got := selectFromMenu(&out, keysFrom("\x1b[A\r"), "Seç", items); got != 2 {
		t.Errorf("wrap selection = %d, want 2", got)
	}
	// number shortcut
	if got := selectFromMenu(&out, keysFrom("2"), "Seç", items); got != 1 {
		t.Errorf("number selection = %d, want 1", got)
	}
	// Esc cancels
	if got := selectFromMenu(&out, keysFrom("\x1b"), "Seç", items); got != -1 {
		t.Errorf("esc = %d, want -1", got)
	}
	// nil keys (piped input) falls back
	if got := selectFromMenu(&out, nil, "Seç", items); got != -1 {
		t.Errorf("nil keys = %d, want -1", got)
	}
}

func TestSelectFromMenu_NestedMenusShareKeySource(t *testing.T) {
	// The regression that motivated the rewrite: a menu opened right after
	// another menu's selection must keep seeing arrow keys. Both menus read
	// from the same keySource here — first picks index 1, second picks 0
	// via SS3-style arrows (application cursor mode).
	items := []menuItem{{Label: "bir"}, {Label: "iki"}}
	var out bytes.Buffer
	ks := keysFrom("\x1b[B\r\x1bOB\x1bOA\r")

	if got := selectFromMenu(&out, ks, "İlk", items); got != 1 {
		t.Fatalf("first menu = %d, want 1", got)
	}
	if got := selectFromMenu(&out, ks, "İkinci", items); got != 0 {
		t.Fatalf("second menu = %d, want 0", got)
	}
}

func TestStripANSI(t *testing.T) {
	in := bold(brightCyan("❯ "))
	if got := stripANSI(in); got != "❯ " {
		t.Errorf("stripANSI(%q) = %q, want %q", in, got, "❯ ")
	}
}
