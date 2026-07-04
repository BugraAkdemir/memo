package replcli

import "fmt"

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
