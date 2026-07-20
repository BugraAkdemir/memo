package replcli

import (
	"fmt"
	"io"
	"math/rand"
	"strings"
)

// Minimal ANSI color helpers — no library, just escape codes. Kept tiny on
// purpose: the terminal REPL is meant to stay dependency-free and simple.
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"

	// 256-color (8-bit) codes below, not the basic 16-color ones. Most
	// terminal themes remap the basic 16 (including "bright" 90-97/100-107)
	// to custom palette colors, which is exactly what made the first version
	// of the user-message background unreadable in some setups — 256-color
	// codes address a fixed, standard palette instead and render the same
	// everywhere.
	colorBrightCyan    = "\033[38;5;51m"
	colorBrightMagenta = "\033[38;5;213m"

	// Bronze/gold — Memo's brand accent (README's #B08D57), used for the
	// terminal's structural chrome (welcome panel, prompt glyph, menu/dropdown
	// selection, status bar) since the 2026-07-20 Claude-Code-inspired
	// redesign. 256-color approximations of the true hex, picked from the
	// nearest points on the xterm color cube to RGB(176,141,87): index 137 ≈
	// (175,135,95) for the base tone, index 180 ≈ (215,175,135) for the
	// brighter selection/emphasis tone.
	colorBronze = "\033[38;5;137m"
	colorGold   = "\033[38;5;180m"
)

func colorize(code, s string) string {
	return code + s + colorReset
}

func bold(s string) string          { return colorize(colorBold, s) }
func dim(s string) string           { return colorize(colorDim, s) }
func red(s string) string           { return colorize(colorRed, s) }
func green(s string) string         { return colorize(colorGreen, s) }
func yellow(s string) string        { return colorize(colorYellow, s) }
func cyan(s string) string          { return colorize(colorCyan, s) }
func brightCyan(s string) string    { return colorize(colorBrightCyan, s) }
func brightMagenta(s string) string { return colorize(colorBrightMagenta, s) }
func bronze(s string) string        { return colorize(colorBronze, s) }
func gold(s string) string          { return colorize(colorGold, s) }

// userInputStart marks the user's own typed/echoed text — a tty prints
// keystrokes using whatever SGR state is currently active when each one is
// drawn, so this styles the user's raw input without the program ever
// touching what they typed. Bold only, no background fill and no forced
// foreground color: an earlier version filled a solid background block
// (blue background, near-black text) — reported unreadable, the second such
// report after an earlier 16→256-color fix already tried to address the
// same complaint once. A fixed bg/fg pair can only ever be tuned for one
// kind of terminal (dark or light), and there's no way to detect which the
// user is actually running — bold-only sidesteps the whole class of bug by
// never overriding color at all, so it reads correctly against whatever
// foreground/background the user's own terminal already uses. Always pair
// with colorReset immediately after the line is read, so nothing printed
// afterward (blank line, reply, command output) inherits the bold weight.
const userInputStart = colorBold

func errorf(format string, args ...any) string {
	return red(fmt.Sprintf(format, args...))
}

// clearScreen wipes the terminal (including scrollback, where supported) so
// `memo` always opens on a clean screen instead of appending below whatever
// was already in the terminal.
func clearScreen(out io.Writer) {
	fmt.Fprint(out, "\033[H\033[2J\033[3J")
}

// tipEntry is one row of the boxed tips panel: a short key/gesture and what
// it does, rendered as two aligned columns (labelWidth below).
type tipEntry struct {
	label string
	desc  string
}

// allTips is the full pool the welcome panel draws from — every command and
// composer gesture worth a passing mention, not just the four most
// essential ones. randomTips below samples from this each launch instead of
// always showing the same fixed set, the terminal equivalent of Claude
// Code's "Tips for getting started" panel but rotating rather than static.
func allTips() []tipEntry {
	return []tipEntry{
		{"/help", t("tip_help")},
		{"@", t("tip_at")},
		{"Esc", t("tip_stop")},
		{"Ctrl+D", t("tip_exit")},
		{"/models", t("tip_models")},
		{"/model", t("tip_model")},
		{"/embedding", t("tip_embedding")},
		{"/connect", t("tip_connect")},
		{"/clear", t("tip_clear")},
		{"/session", t("tip_session")},
		{"/tasklist", t("tip_tasklist")},
		{"/remote", t("tip_remote")},
		{"/gui", t("tip_gui")},
		{"/model-download", t("tip_model_download")},
		{"/update", t("tip_update")},
		{"Tab", t("tip_tab")},
		{"↑↓", t("tip_history")},
		{"Ctrl+L", t("tip_clear_screen")},
		{"Ctrl+W", t("tip_delete_word")},
		{"y/a/n", t("tip_permission")},
	}
}

// randomTips returns n distinct tips picked at random from allTips, a fresh
// draw every call — math/rand's top-level functions have used a random seed
// by default since Go 1.20, so this genuinely varies launch to launch with
// no manual seeding needed. n is clamped to the pool size.
func randomTips(n int) []tipEntry {
	all := allTips()
	rand.Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}

// Welcome-panel geometry. Fixed on purpose, NOT derived from the terminal
// width: growing or shrinking the window must not reflow the box or shift
// its contents around ("terminali büyütüp küçültünce tasarım çok kaymasın").
// A terminal genuinely too narrow for the box falls back to plain unboxed
// lines (narrowPanel), where there is no alignment left to break.
const (
	panelLeftW  = 44 // left column's content width
	panelRightW = 36 // right column's content width
	// Borders and margins: │ + 2 + left + 1 + │ + 2 + right + 1 + │
	panelWidth = panelLeftW + panelRightW + 9
)

// fitTo hard-truncates s to w cells, marking the cut with "…", so an
// over-long value (a deep project path, a verbose model name) can never
// push the box out of alignment.
func fitTo(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return string(r[:w-1]) + "…"
}

// wrapTo greedily word-wraps s into lines of at most w cells, hard-splitting
// any single word too long to ever fit.
func wrapTo(s string, w int) []string {
	if w <= 0 {
		return nil
	}
	var out []string
	cur := ""
	splitOverlong := func() {
		for len([]rune(cur)) > w {
			r := []rune(cur)
			out = append(out, string(r[:w]))
			cur = string(r[w:])
		}
	}
	for _, word := range strings.Fields(s) {
		switch {
		case cur == "":
			cur = word
		case len([]rune(cur))+1+len([]rune(word)) <= w:
			cur += " " + word
		default:
			out = append(out, cur)
			cur = word
		}
		splitOverlong()
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// panelCell is one column's content for one row, carried as a (plain,
// styled) pair: plain drives the width/padding math (raw rune count, no
// ANSI), styled is what actually gets printed — so colored segments can
// never throw the column alignment off.
type panelCell struct {
	plain  string
	styled string
}

func (c panelCell) pad(w int) string {
	return c.styled + strings.Repeat(" ", max(w-len([]rune(c.plain)), 0))
}

// memoMascot is the welcome card's signature element — a small pixel-face,
// Memo's own (not a copy of Claude Code's llama), sized to stay legible at
// this scale. Every row must be the same rune width (7) so center() below
// can align them as one block.
var memoMascot = []string{
	" ▄▄▄▄▄ ",
	"█ o o █",
	"█  -  █",
	" ▀▀▀▀▀ ",
}

// leftColumn builds the box's left half: the mascot and the welcome line
// centered, then the chat model and project path flush left. Centering
// short standalone phrases reads well; centering real key:value data gives
// a ragged left edge that is harder to scan, so the status block
// deliberately stays left-aligned.
//
// There is deliberately no memory/embedding status row here. It used to
// show one, and it was actively misleading: the backend's embedding status
// falls back to a bare port ping (GetStatus/pingPort, internal/app), which
// reports "running" whenever anything at all is listening on that port —
// so the panel would cheerfully print "Hafıza: açık" for a session whose
// memory was not working. A status line nobody can trust is worse than no
// status line, so what the panel offers instead is a single actionable
// warning (printWelcome, repl.go) telling the user to run /embedding.
func leftColumn(model, projectPath string) []panelCell {
	var out []panelCell
	add := func(plain, styled string) { out = append(out, panelCell{plain, styled}) }
	center := func(plain, styled string) {
		sp := strings.Repeat(" ", max((panelLeftW-len([]rune(plain)))/2, 0))
		add(sp+plain, sp+styled)
	}

	add("", "")
	for _, row := range memoMascot {
		center(row, bronze(row))
	}
	add("", "")
	center(t("welcome_back"), bold(t("welcome_back")))
	add("", "")

	// Truncate the value, never the label: a clipped "Model:" would be
	// unreadable, a clipped value still says which field it belongs to.
	label := t("label_model")
	add(label+fitTo(model, panelLeftW-len([]rune(label))),
		bold(label)+fitTo(model, panelLeftW-len([]rune(label))))
	if projectPath != "" {
		p := fitTo(projectPath, panelLeftW)
		add(p, dim(p))
	}
	return out
}

// rightColumn builds the box's right half: a heading, then tips drawn at
// random for this launch, then the update notice — or, when there's nothing
// to update, one extra tip so the slot is never left blank.
func rightColumn(picks []tipEntry, updateNotice string) []panelCell {
	var out []panelCell
	add := func(plain, styled string) { out = append(out, panelCell{plain, styled}) }

	shown := picks
	if updateNotice != "" && len(shown) > 3 {
		shown = shown[:3]
	}

	// Sized from the tips actually drawn this launch rather than a fixed
	// constant: the pool's labels run from "@" to "/model-download", so a
	// constant either wastes the column or (this shipped once already) runs
	// a long label straight into its description with no gap at all. Capped
	// so the description always keeps room to wrap into.
	labelW := 0
	for _, tp := range shown {
		labelW = max(labelW, len([]rune(tp.label)))
	}
	labelW = min(labelW, panelRightW/2)

	add(t("tips_title"), bold(t("tips_title")))
	add("", "")
	// One row per tip, description truncated rather than wrapped — the same
	// choice the reference screenshot makes ("…create a CLAUDE.md file with
	// instructio…"). Wrapping instead made a single tip eat three rows and
	// left the panel tall and ragged.
	for _, tp := range shown {
		label := fitTo(tp.label, labelW)
		gap := strings.Repeat(" ", max(labelW-len([]rune(label)), 0)+1)
		desc := fitTo(tp.desc, panelRightW-labelW-1)
		add(label+gap+desc, bold(gold(label))+gap+dim(desc))
	}
	if updateNotice != "" {
		add("", "")
		for _, l := range wrapTo(updateNotice, panelRightW) {
			add(l, yellow(l))
		}
	}
	return out
}

// boxedPanel draws the finished two-column box: the title inlined into the
// top border (the way the reference screenshot has it, rather than as a row
// inside the box), then every row zipped from the two columns — the shorter
// column padded out with blanks so the divider stays flush all the way down.
func boxedPanel(title string, left, right []panelCell) string {
	head := fitTo(title, panelLeftW)
	var b strings.Builder
	fmt.Fprintln(&b, bronze("╭─ ")+bold(gold(head))+bronze(" "+
		strings.Repeat("─", max(panelLeftW-len([]rune(head)), 0))+"┬"+
		strings.Repeat("─", panelRightW+3)+"╮"))

	cell := func(cells []panelCell, i, w int) string {
		if i < len(cells) {
			return cells[i].pad(w)
		}
		return strings.Repeat(" ", w)
	}
	for i := range max(len(left), len(right)) {
		fmt.Fprintln(&b, bronze("│")+"  "+cell(left, i, panelLeftW)+" "+
			bronze("│")+"  "+cell(right, i, panelRightW)+" "+bronze("│"))
	}
	fmt.Fprint(&b, bronze("╰"+strings.Repeat("─", panelLeftW+3)+"┴"+
		strings.Repeat("─", panelRightW+3)+"╯"))
	return b.String()
}

// narrowPanel is the fallback for a terminal too narrow to hold the box:
// the same content as plain unboxed lines, where there is no column
// alignment left to break in the first place.
func narrowPanel(title string, left, right []panelCell) string {
	var b strings.Builder
	fmt.Fprintln(&b, bold(gold(title)))
	all := make([]panelCell, 0, len(left)+len(right))
	all = append(all, left...)
	all = append(all, right...)
	for _, c := range all {
		if strings.TrimSpace(c.plain) == "" {
			fmt.Fprintln(&b)
			continue
		}
		fmt.Fprintln(&b, "  "+c.styled)
	}
	return strings.TrimRight(b.String(), "\n")
}

// welcomePanel renders the startup panel as ONE box split by a vertical
// divider, matching the layout of the reference screenshot: the title sits
// inlined in the top border, the left column carries the mascot, the
// welcome line and the chat model / project path, and the right column
// carries tips drawn at random per launch plus — when GET /api/version/check
// reports a newer release — an update notice pointing at /update. With
// nothing to update, that slot takes one more tip instead so it is never
// blank. Everything is bronze (colorBronze), Memo's brand accent.
//
// version and projectPath may be empty (the version fetch failed, or this
// run has no project root); both are simply omitted rather than shown blank.
// termWidth <= 0 means "unknown" (piped input, a pty that never reported a
// size) and takes the full box, since nothing is going to reflow anyway.
func welcomePanel(version, projectPath, model, updateNotice string, termWidth int) string {
	title := "✳ Memo CLI"
	if version != "" {
		// The raw version string (build_releases.sh's `version` file)
		// conventionally already carries its own leading V/v — trim it
		// before adding ours so the title never doubles up ("vV3.3.3").
		title += " v" + strings.TrimLeft(version, "Vv")
	}

	// Drawn from one shuffle rather than two calls, so the extra tip that
	// fills the update slot when there's nothing to update can never repeat
	// one of the three already listed above it.
	picks := randomTips(4)

	left := leftColumn(model, projectPath)
	right := rightColumn(picks, updateNotice)

	if termWidth > 0 && termWidth < panelWidth {
		return narrowPanel(title, left, right)
	}
	return boxedPanel(title, left, right)
}
