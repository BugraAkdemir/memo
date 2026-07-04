package replcli

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// menuItem is one selectable line in an interactive arrow-key menu.
type menuItem struct {
	Label string
	Hint  string // dim secondary text shown after Label
}

// selectFromMenu renders items and lets the user pick one with the Up/Down
// arrows and Enter, redrawing in place. Returns the chosen index, or -1 if
// the user cancelled (Ctrl+C — the reliable cancel key) or if stdin isn't a
// real terminal we can put in raw mode (piped input, test harnesses). -1
// means "no selection" — callers fall back to a plain text prompt.
func selectFromMenu(out io.Writer, title string, items []menuItem) int {
	if len(items) == 0 {
		return -1
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return -1
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return -1
	}
	defer term.Restore(fd, oldState)

	selected := 0
	render := func(first bool) {
		if !first {
			fmt.Fprintf(out, "\033[%dA", len(items)+1)
		}
		fmt.Fprintf(out, "\033[K%s\r\n", bold(title))
		for i, it := range items {
			fmt.Fprint(out, "\033[K")
			line := "  " + it.Label
			if i == selected {
				line = green("▶ " + it.Label)
			}
			if it.Hint != "" {
				line += "  " + dim(it.Hint)
			}
			fmt.Fprint(out, line+"\r\n")
		}
	}
	render(true)

	b := make([]byte, 1)
	for {
		if _, err := os.Stdin.Read(b); err != nil {
			return -1
		}
		switch b[0] {
		case 3, 'q': // Ctrl+C or q — reliable, non-blocking cancel
			return -1
		case '\r', '\n':
			return selected
		case 27: // Esc — start of an arrow-key sequence (ESC [ A/B)
			rest := make([]byte, 2)
			n, _ := os.Stdin.Read(rest)
			if n < 2 || rest[0] != '[' {
				return -1
			}
			switch rest[1] {
			case 'A':
				selected = (selected - 1 + len(items)) % len(items)
				render(false)
			case 'B':
				selected = (selected + 1) % len(items)
				render(false)
			}
		}
	}
}
