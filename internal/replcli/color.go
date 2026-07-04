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

// welcomePanel renders the bordered startup panel: the ASCII banner plus a
// small info block showing which model/provider will actually answer and
// whether embedding/RAG memory is active — so that's never a guessing game.
// Padding is computed from plain-text lengths; colored segments are wrapped
// afterward so ANSI codes never throw off the box alignment. memoryActive
// tints the memory line green/yellow without affecting its width.
func welcomePanel(model, memory string, memoryActive bool) string {
	bannerLines := strings.Split(asciiBanner, "\n")
	plainModel := "Model:  " + model
	plainMemory := "Hafıza: " + memory

	width := 0
	for _, l := range bannerLines {
		width = max(width, len([]rune(l)))
	}
	width = max(width, len([]rune(plainModel)))
	width = max(width, len([]rune(plainMemory)))
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
	for _, l := range bannerLines {
		fmt.Fprint(&b, row(bold(cyan(l)), l))
	}
	fmt.Fprint(&b, row("", ""))
	fmt.Fprint(&b, row(bold("Model:  ")+model, plainModel))
	fmt.Fprint(&b, row(bold("Hafıza: ")+memoryColor(memory), plainMemory))
	fmt.Fprintf(&b, "%s\n", dim("╰"+strings.Repeat("─", width)+"╯"))
	fmt.Fprint(&b, dim("Çıkmak için /exit ya da Ctrl+D  ·  Komutlar için /help ya da /"))
	return b.String()
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
