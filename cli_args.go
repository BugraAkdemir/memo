package main

import "strings"

// splitFlagsAndPositional separates a subcommand's remaining args into
// flag-related tokens (in order, ready for flag.FlagSet.Parse) and
// positional tokens (also in order) — needed because Go's flag package
// stops parsing at the first non-flag argument, so a flag placed after a
// positional one (a very natural place to put it — "add-device MyPhone
// --port 9090" reads more naturally than "--port 9090 add-device MyPhone")
// would otherwise end up unparsed inside fs.Args() instead of being
// recognized as a flag at all. boolFlags lists (without leading dashes)
// which flag names take no value, so their token isn't mistaken for one
// that does.
func splitFlagsAndPositional(args []string, boolFlags map[string]bool) (flagArgs, positional []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			positional = append(positional, a)
			continue
		}
		flagArgs = append(flagArgs, a)
		name := strings.TrimLeft(a, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			continue // "--port=1234" already carries its own value
		}
		if boolFlags[name] {
			continue // boolean flag, no separate value token to consume
		}
		if i+1 < len(args) {
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}
	return flagArgs, positional
}
