package replcli

import (
	"fmt"
	"io"
	"strings"
)

// menuItem is one selectable line in an interactive arrow-key menu.
type menuItem struct {
	Label string
	Hint  string // dim secondary text shown after Label
}

// selectFromMenu renders items and lets the user pick one with the Up/Down
// arrows and Enter, redrawing in place. Number keys 1-9 jump straight to an
// entry. Returns the chosen index, or -1 if the user cancelled (Esc, q or
// Ctrl+C) or when keys is nil (piped input, test harnesses) — callers fall
// back to a plain text prompt.
//
// Keys come from the session's shared keySource — the same one the line
// editor uses — never from a private os.Stdin reader. That is what makes
// menus stackable: a menu opened from another menu's selection (model pick
// after the "/" menu, file pick after the repo pick) reads from the same
// decoded key stream instead of fighting over raw stdin bytes.
func selectFromMenu(out io.Writer, keys *keySource, title string, items []menuItem) int {
	if len(items) == 0 || keys == nil {
		return -1
	}

	selected := 0
	rows := len(items) + 2 // title + items + footer
	render := func(first bool) {
		var b strings.Builder
		if !first {
			// The cursor rests at the end of the footer (the last of the
			// rows lines, no trailing newline) — the title row is rows-1
			// lines up.
			fmt.Fprintf(&b, "\033[%dA", rows-1)
		}
		b.WriteString("\r\033[J")
		b.WriteString(bold(title) + "\n")
		for i, it := range items {
			line := "    " + it.Label
			if i == selected {
				line = bold(brightCyan("  ▶ " + it.Label))
			}
			if it.Hint != "" {
				line += "  " + dim(it.Hint)
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("  " + dim("↑↓ gezin · Enter seç · Esc iptal"))
		fmt.Fprint(out, b.String())
	}
	render(true)

	finish := func() {
		// Wipe the menu so the picked action's output doesn't stack under
		// a dead selector UI.
		fmt.Fprintf(out, "\033[%dA\r\033[J", rows-1)
	}

	for {
		k := keys.readKey()
		switch k.kind {
		case keyEOF, keyCtrlC, keyEsc:
			finish()
			return -1
		case keyEnter:
			finish()
			return selected
		case keyUp:
			selected = (selected - 1 + len(items)) % len(items)
			render(false)
		case keyDown:
			selected = (selected + 1) % len(items)
			render(false)
		case keyHome:
			selected = 0
			render(false)
		case keyEnd:
			selected = len(items) - 1
			render(false)
		case keyRune:
			switch {
			case k.r == 'q':
				finish()
				return -1
			case k.r >= '1' && k.r <= '9' && int(k.r-'1') < len(items):
				finish()
				return int(k.r - '1')
			}
		}
	}
}
