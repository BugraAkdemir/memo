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

// asciiBanner is the "MEMO CLI" block-letter wordmark shown once at startup.
const asciiBanner = ` __  __ _____ __  __  ___        ____ _     _____
|  \/  | ____|  \/  |/ _ \      / ___| |   |_ _|
| |\/| |  _| | |\/| | | | |    | |   | |    | |
| |  | | |___| |  | | |_| |    | |___| |___ | |
|_|  |_|_____|_|  |_|\___/      \____|_____|___|`

func banner() string {
	return bold(cyan(asciiBanner))
}

// progressBar renders a fixed-width text progress bar for percent (0-100).
func progressBar(percent float64) string {
	const width = 24
	filled := min(int(percent/100*width), width)
	filled = max(filled, 0)
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

// humanSize formats a byte count as a human-readable size (e.g. "4.2 GiB").
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
