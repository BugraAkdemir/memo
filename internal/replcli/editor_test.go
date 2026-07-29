package replcli

import (
	"bytes"
	"os"
	"path/filepath"
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
	if !ok || line != slashCommands()[1].label {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, slashCommands()[1].label)
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

func TestEditor_AtMention_TabInsertsPathAndKeepsComposing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ed, _ := newTestEditor("check @re\t done\r")
	ed.projectPath = root
	line, ok := ed.readLine("> ")
	want := "check @readme.txt done"
	if !ok || line != want {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, want)
	}
}

func TestEditor_AtMention_EnterInsertsWithoutSubmitting(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Enter while the "@" dropdown is open must insert the file, not submit
	// the line — only the trailing "\r" after " ok" should end readLine.
	ed, _ := newTestEditor("check @re\r ok\r")
	ed.projectPath = root
	line, ok := ed.readLine("> ")
	want := "check @readme.txt ok"
	if !ok || line != want {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, want)
	}
}

func TestEditor_AtMention_NoProjectPathFallsThroughToPlainSubmit(t *testing.T) {
	// No projectPath set (a run with no project root) — "@" still opens
	// menuAt mode, but fileMatches has nothing to offer, so Enter must fall
	// through to a normal submit instead of trying to insert an empty pick.
	ed, _ := newTestEditor("hi @x\r")
	line, ok := ed.readLine("> ")
	if !ok || line != "hi @x" {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, "hi @x")
	}
}

func TestEditor_AtMention_MidWordAtSignDoesNotTriggerDropdown(t *testing.T) {
	// "@" not at the start of its word (an email address, say) must not be
	// treated as a file mention — Enter submits the line exactly as typed.
	ed, _ := newTestEditor("mail me at a@b.com\r")
	ed.projectPath = t.TempDir()
	line, ok := ed.readLine("> ")
	if !ok || line != "mail me at a@b.com" {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, "mail me at a@b.com")
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

func TestEditor_ShiftTab_TogglesAutoPermissionViaCallback(t *testing.T) {
	var calls []bool
	ed, _ := newTestEditor("\x1b[Zhi\r")
	ed.onToggleAutoPermission = func(current bool) bool {
		calls = append(calls, current)
		return !current
	}
	line, ok := ed.readLine("> ")
	if !ok || line != "hi" {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, "hi")
	}
	if len(calls) != 1 || calls[0] != false {
		t.Fatalf("onToggleAutoPermission calls = %v, want a single call with false", calls)
	}
	if !ed.autoPermission {
		t.Fatal("autoPermission = false after toggle, want true")
	}
}

func TestEditor_ShiftTab_NilCallbackIsNoOp(t *testing.T) {
	// No onToggleAutoPermission wired (piped/non-terminal-style construction
	// missing the callback) — Shift+Tab must not panic or affect the buffer.
	ed, _ := newTestEditor("\x1b[Zhi\r")
	line, ok := ed.readLine("> ")
	if !ok || line != "hi" {
		t.Fatalf("readLine = %q, %v, want %q", line, ok, "hi")
	}
	if ed.autoPermission {
		t.Fatal("autoPermission = true with no callback wired, want false")
	}
}

func TestEditor_StatusBarLine_ReflectsAutoPermissionState(t *testing.T) {
	ed, _ := newTestEditor("")
	off := ed.statusBarLine()
	if off != dim(statusBarText()) {
		t.Fatalf("statusBarLine() off = %q, want plain status bar text", off)
	}
	ed.autoPermission = true
	on := ed.statusBarLine()
	if on == off {
		t.Fatal("statusBarLine() unchanged after autoPermission=true")
	}
}

// TestEditor_StatusBarLine_GTheme_ShowsLiveDataNotStaticHints covers the
// "g" theme's status bar: it must never fall back to the classic static
// command-hint text, and must reflect auto-permission state live even
// though liveStatusPrefix itself is only refreshed at checkpoints (see
// repl.go's refreshLiveStatus doc comment).
func TestEditor_StatusBarLine_GTheme_ShowsLiveDataNotStaticHints(t *testing.T) {
	ed, _ := newTestEditor("")
	ed.theme = themeDefault
	ed.liveStatusPrefix = dim("deepseek-v4") + dim("  ·  hafıza ") + green("●")

	off := ed.statusBarLine()
	if strings.Contains(off, "deepseek-v4") == false {
		t.Errorf("statusBarLine() = %q, want it to include the cached model prefix", off)
	}
	if strings.Contains(off, "/ komutlar") {
		t.Errorf("statusBarLine() = %q, g theme must not fall back to the classic static hint bar", off)
	}

	ed.autoPermission = true
	on := ed.statusBarLine()
	if on == off {
		t.Fatal("statusBarLine() unchanged after autoPermission=true under g theme")
	}
}

// TestEditor_StatusBarLine_GTheme_EmptyPrefixDoesNotPanic guards the case
// before the session's first refreshLiveStatus call (e.g. very early
// startup) — liveStatusPrefix is still its zero value.
func TestEditor_StatusBarLine_GTheme_EmptyPrefixDoesNotPanic(t *testing.T) {
	ed, _ := newTestEditor("")
	ed.theme = themeDefault
	if got := ed.statusBarLine(); got == "" {
		t.Error("statusBarLine() with empty liveStatusPrefix returned an empty string, want a fallback")
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
