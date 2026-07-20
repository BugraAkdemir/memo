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
	colorBgUser        = "\033[48;5;33m" // vivid blue background — tints the user's own sent message
	colorFgUser        = "\033[38;5;16m" // near-black — strongest possible contrast against colorBgUser

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

// userInputStart begins a background tint that "bleeds" into the terminal's
// own echo of whatever the user types next — a tty prints keystrokes using
// whatever SGR state is currently active when each one is drawn, so this
// colors the user's raw input without the program ever touching what they
// typed. Bold near-black on vivid blue, rather than white-on-blue, so it
// stays legible even under a semi-transparent terminal window blending the
// background toward whatever's behind it. Always pair with colorReset
// immediately after the line is read, so nothing printed afterward (blank
// line, reply, command output) inherits it.
const userInputStart = colorBgUser + colorBold + colorFgUser

func errorf(format string, args ...any) string {
	return red(fmt.Sprintf(format, args...))
}

// clearScreen wipes the terminal (including scrollback, where supported) so
// `memo` always opens on a clean screen instead of appending below whatever
// was already in the terminal.
func clearScreen(out io.Writer) {
	fmt.Fprint(out, "\033[H\033[2J\033[3J")
}

// startupTips lists the composer tricks worth surfacing right away — the
// terminal equivalent of Claude Code's "Tips for getting started" panel.
// Deliberately no "what's new"/announcement section next to it: a terminal
// client has no changelog surface to point at, and the user explicitly asked
// for it to stay out of this redesign.
func startupTips() []string {
	return []string{
		dim("/help") + t("tip_help"),
		dim("@") + t("tip_at"),
		dim("Esc") + t("tip_stop") + dim("Ctrl+D") + t("tip_exit"),
	}
}

// welcomePanel renders the startup panel in the Claude-Code-inspired style
// adopted 2026-07-20: a bordered title box (app name + version, a welcome
// line, the model/memory status that was already here, and the active
// project directory) followed by a short tips list. Padding is computed from
// plain-text lengths; colored segments are wrapped afterward so ANSI codes
// never throw off the box alignment. memoryActive tints the memory line
// green/yellow without affecting its width. version/projectPath may be
// empty (version fetch failed, or this run has no project root) — both rows
// are simply omitted rather than shown blank.
func welcomePanel(version, projectPath, model, memory string, memoryActive bool) string {
	title := "✳ Memo CLI"
	if version != "" {
		// The raw version string (build_releases.sh's `version` file)
		// conventionally already carries its own leading V/v — trim it
		// before adding ours so the title never doubles up ("vV3.3.3").
		title += " v" + strings.TrimLeft(version, "Vv")
	}
	welcomeLine := t("welcome_back")
	plainModel := t("label_model") + model
	plainMemory := t("label_memory") + memory

	width := len([]rune(title))
	width = max(width, len([]rune(welcomeLine)))
	width = max(width, len([]rune(plainModel)))
	width = max(width, len([]rune(plainMemory)))
	width = max(width, len([]rune(projectPath)))
	width += 4 // 2-char left margin + at least 2-char right margin

	pad := func(plain string) string {
		return strings.Repeat(" ", max(width-2-len([]rune(plain)), 0))
	}
	row := func(colored, plainForWidth string) string {
		return dim("│") + "  " + colored + pad(plainForWidth) + dim("│") + "\n"
	}

	memoryColor := yellow
	if memoryActive {
		memoryColor = green
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", dim("╭"+strings.Repeat("─", width)+"╮"))
	fmt.Fprint(&b, row(bold(gold(title)), title))
	fmt.Fprint(&b, row("", ""))
	fmt.Fprint(&b, row(welcomeLine, welcomeLine))
	fmt.Fprint(&b, row(bold(t("label_model"))+model, plainModel))
	fmt.Fprint(&b, row(bold(t("label_memory"))+memoryColor(memory), plainMemory))
	if projectPath != "" {
		fmt.Fprint(&b, row(dim(projectPath), projectPath))
	}
	fmt.Fprintf(&b, "%s\n", dim("╰"+strings.Repeat("─", width)+"╯"))
	fmt.Fprint(&b, "\n"+bold(t("tips_title"))+"\n")
	for _, tip := range startupTips() {
		fmt.Fprint(&b, "  "+tip+"\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
