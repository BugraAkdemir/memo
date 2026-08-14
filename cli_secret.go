package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// promptSecret reads a secret value (password, API key) from the terminal
// with input hidden (no echo), the same way sudo/ssh prompt for a password.
//
// Used whenever a flag that carries a secret (--password, --key) is left
// empty on the command line. Accepting a secret as a plain flag *value*
// leaves it visible to any other user on the same machine via `ps`/
// `/proc/<pid>/cmdline`, and permanently in shell history — a real, standing
// risk specifically because every `memo <subcommand>` this is used from
// (remote, provider) exists for SSH-only, often multi-user, self-hosted
// boxes — the exact environment where "anyone else logged into this machine
// can read your API key/password off the process list" actually matters.
//
// Returns an error instead of blocking forever if stdin isn't a real
// terminal (a script or pipe) — a scripted/automated caller gets a clear
// message and must pass the value directly via the flag instead. Same
// explicit-only-when-scripted trade-off runPrintMode's --auto-allow already
// makes for permission prompts (main.go).
func promptSecret(label string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("%s bir terminalden interaktif olarak istenemedi (stdin bir terminal değil) — script içinde ilgili flag'i doğrudan verin / %s cannot be prompted for interactively (stdin is not a terminal) — pass the flag directly when scripting", label, label)
	}
	fmt.Fprintf(os.Stderr, "%s: ", label)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("okunamadı / could not read: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}
