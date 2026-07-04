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
)

func colorize(code, s string) string {
	return code + s + colorReset
}

func bold(s string) string   { return colorize(colorBold, s) }
func dim(s string) string    { return colorize(colorDim, s) }
func red(s string) string    { return colorize(colorRed, s) }
func green(s string) string  { return colorize(colorGreen, s) }
func yellow(s string) string { return colorize(colorYellow, s) }
func cyan(s string) string   { return colorize(colorCyan, s) }

func errorf(format string, args ...any) string {
	return red(fmt.Sprintf(format, args...))
}

// clearScreen wipes the terminal (including scrollback, where supported) so
// `memo` always opens on a clean screen instead of appending below whatever
// was already in the terminal.
func clearScreen(out io.Writer) {
	fmt.Fprint(out, "\033[H\033[2J\033[3J")
}

// banner renders the small dotted-border "memo cli" wordmark shown once at
// startup. Built from matching plain/colored segment pairs so the border
// width is computed from visible characters only — ANSI codes never affect
// the box alignment.
func banner() string {
	segDots := "  ·  ·  ·  "
	segMemo := "memo"
	segGap := " "
	segCli := "cli"

	plainInner := segDots + segMemo + segGap + segCli + strings.TrimRight(segDots, " ") + "  "
	coloredInner := dim(segDots) + bold(cyan(segMemo)) + segGap + dim(segCli) + dim(strings.TrimRight(segDots, " ")) + "  "

	border := strings.Repeat("─", len([]rune(plainInner)))
	return fmt.Sprintf("%s\n%s\n%s",
		dim("╭"+border+"╮"),
		dim("│")+coloredInner+dim("│"),
		dim("╰"+border+"╯"),
	)
}
