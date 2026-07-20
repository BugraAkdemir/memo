package replcli

import (
	"fmt"
	"io"
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

// startupTips lists the composer tricks worth surfacing right away — the
// terminal equivalent of Claude Code's "Tips for getting started" panel.
// Deliberately no "what's new"/announcement section next to it: a terminal
// client has no changelog surface to point at, and the user explicitly asked
// for it to stay out of this redesign.
func startupTips() []tipEntry {
	return []tipEntry{
		{"/help", t("tip_help")},
		{"@", t("tip_at")},
		{"Esc", t("tip_stop")},
		{"Ctrl+D", t("tip_exit")},
	}
}

// boxWriter accumulates rows for a single fully-bordered panel (welcomePanel's
// tips box — the title card uses its own lighter left-rule style below,
// matching the reference screenshot's two different border weights), all
// sharing one interior width computed from whatever's been added so far.
// Every row is stored as (plain-text, styled) pairs: plain drives
// width/padding math (raw rune count, no ANSI), styled is what's actually
// printed, so colored segments never throw off alignment.
type boxWriter struct {
	rows  []func(width int) string
	width int
}

func newBoxWriter() *boxWriter { return &boxWriter{width: 20} }

// left adds a left-aligned row with a 2-column margin, styled independently
// of the plain text used for width accounting.
func (b *boxWriter) left(plain, styled string) {
	b.width = max(b.width, len([]rune(plain))+4)
	b.rows = append(b.rows, func(width int) string {
		pad := strings.Repeat(" ", max(width-2-len([]rune(plain)), 0))
		return bronze("│") + "  " + styled + pad + bronze("│")
	})
}

// blank adds an empty spacer row.
func (b *boxWriter) blank() {
	b.rows = append(b.rows, func(width int) string {
		return bronze("│") + strings.Repeat(" ", width) + bronze("│")
	})
}

// render draws the finished box: top/bottom borders plus every queued row,
// each padded out to the box's final (widest-row-driven) interior width.
func (b *boxWriter) render() string {
	var out strings.Builder
	fmt.Fprintln(&out, bronze("╭"+strings.Repeat("─", b.width)+"╮"))
	for _, row := range b.rows {
		fmt.Fprintln(&out, row(b.width))
	}
	fmt.Fprint(&out, bronze("╰"+strings.Repeat("─", b.width)+"╯"))
	return out.String()
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

// welcomePanel renders the startup panel: a title card in Claude Code's own
// minimal style (a top rule, not a closed box — left edge only, no right or
// bottom border) with centered content and a small mascot, followed by a
// separate, fully bordered box listing composer tips. The two panels
// deliberately use different border weights, matching the reference
// screenshot exactly rather than forcing one consistent box style: Claude
// Code's own welcome card is a light rule + left edge, but its tips/updates
// panel is a real closed box. Deliberately no third "what's new" box: a
// terminal client has no changelog surface to point at, and it was asked to
// stay out of this redesign explicitly. Both panels are bronze
// (colorBronze), not a neutral dim gray — an earlier pass's plain gray
// border was the single biggest reason it read as generic rather than
// deliberately themed. version/projectPath may be empty (version fetch
// failed, or this run has no project root) — both rows are simply omitted.
func welcomePanel(version, projectPath, model, memory string, memoryActive bool) string {
	title := "✳ Memo CLI"
	if version != "" {
		// The raw version string (build_releases.sh's `version` file)
		// conventionally already carries its own leading V/v — trim it
		// before adding ours so the title never doubles up ("vV3.3.3").
		title += " v" + strings.TrimLeft(version, "Vv")
	}
	memoryColor := yellow
	if memoryActive {
		memoryColor = green
	}

	// Only the centered rows (title, mascot, welcome line) drive this width —
	// the Model:/Memory:/path status block below stays left-aligned and can
	// run wider than it without affecting how the centered rows are padded,
	// same reasoning as the previous pass: real key:value data centers with
	// a ragged, harder-to-scan left edge, so it deliberately doesn't.
	centerWidth := len([]rune(title))
	centerWidth = max(centerWidth, len([]rune(t("welcome_back"))))
	for _, row := range memoMascot {
		centerWidth = max(centerWidth, len([]rune(row)))
	}
	center := func(s string) string {
		pad := max((centerWidth-len([]rune(s)))/2, 0)
		return strings.Repeat(" ", pad) + s
	}

	var card strings.Builder
	fmt.Fprintln(&card, bronze("╭─")+" "+bold(gold(title)))
	line := func(s string) { fmt.Fprintln(&card, bronze("│")+"  "+s) }
	blank := func() { fmt.Fprintln(&card, bronze("│")) }

	blank()
	for _, row := range memoMascot {
		line(bronze(center(row)))
	}
	blank()
	line(center(t("welcome_back")))
	blank()
	line(bold(t("label_model")) + model)
	line(bold(t("label_memory")) + memoryColor(memory))
	if projectPath != "" {
		line(dim(projectPath))
	}

	tips := newBoxWriter()
	tips.left(t("tips_title"), bold(t("tips_title")))
	tips.blank()
	const labelWidth = 8
	for _, tip := range startupTips() {
		padded := tip.label
		if pad := labelWidth - len([]rune(tip.label)); pad > 0 {
			padded += strings.Repeat(" ", pad)
		}
		plain := padded + tip.desc
		styled := bold(gold(tip.label))
		if pad := labelWidth - len([]rune(tip.label)); pad > 0 {
			styled += strings.Repeat(" ", pad)
		}
		styled += dim(tip.desc)
		tips.left(plain, styled)
	}

	return strings.TrimRight(card.String(), "\n") + "\n\n" + tips.render()
}
