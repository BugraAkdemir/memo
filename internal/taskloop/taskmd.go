package taskloop

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// NotifyLevel controls how chatty a Self-Driving task is on its notification
// channels. It is parsed from a "# bildirim:" header at the top of Task.md and
// consumed by the NotifyBus (see notify.go).
type NotifyLevel string

const (
	// NotifyOnlyDone reports only terminal outcomes (finished / failed).
	NotifyOnlyDone NotifyLevel = "sadece-bitince"
	// NotifyImportant is the default: start, completion, failure, stuck item,
	// rate-limit wait, provider switch, self-config change.
	NotifyImportant NotifyLevel = "önemli"
	// NotifyEverything additionally reports every item transition and sub-agent
	// spawn — a live feed.
	NotifyEverything NotifyLevel = "her-şey"
)

// validNotifyLevel reports whether s is a recognized level string.
func validNotifyLevel(s string) bool {
	switch NotifyLevel(s) {
	case NotifyOnlyDone, NotifyImportant, NotifyEverything:
		return true
	default:
		return false
	}
}

// ParsedItem is one checkbox line lifted out of a Task.md file. ID is a
// synthetic ordinal ("1", "2", …) — the JSON store owns the real UUIDs; Line is
// the 1-based source line so MarkItemDone can rewrite exactly that line.
type ParsedItem struct {
	ID     string
	Text   string
	Status string // "pending" | "done"
	Line   int
}

// ParsedTaskMd is the result of reading a Task.md file: its notification level
// (defaulting to NotifyImportant) and its checkbox items in file order.
type ParsedTaskMd struct {
	NotifyLevel NotifyLevel
	Items       []ParsedItem
	Warnings    []string
}

// checkboxRe matches a checkbox list item after any leading whitespace:
//
//	- [ ] text        * [ ] text        + [ ] text
//	1. [ ] text        2) [ ] text
//
// Group 1 is the check mark (space, x or X), group 2 is the item text.
var checkboxRe = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+\[([ xX])\]\s*(.*?)\s*$`)

var bildirimRe = regexp.MustCompile(`(?i)^#\s*bildirim\s*:\s*(.+?)\s*$`)

// ParseTaskMd reads a Task.md file and extracts its "# bildirim:" header and
// checkbox items. A missing file is an error; a file with no checkboxes is not
// (Items is empty).
func ParseTaskMd(path string) (*ParsedTaskMd, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("taskloop: read Task.md: %w", err)
	}

	out := &ParsedTaskMd{NotifyLevel: NotifyImportant}
	lines := strings.Split(string(data), "\n")
	ordinal := 0
	for i, line := range lines {
		if m := bildirimRe.FindStringSubmatch(line); m != nil {
			lvl := strings.ToLower(strings.TrimSpace(m[1]))
			if validNotifyLevel(lvl) {
				out.NotifyLevel = NotifyLevel(lvl)
			} else {
				out.Warnings = append(out.Warnings,
					fmt.Sprintf("line %d: unknown '# bildirim:' level %q, using %q", i+1, lvl, NotifyImportant))
			}
			continue
		}
		if m := checkboxRe.FindStringSubmatch(line); m != nil {
			status := "pending"
			if m[1] != " " {
				status = "done"
			}
			ordinal++
			out.Items = append(out.Items, ParsedItem{
				ID:     strconv.Itoa(ordinal),
				Text:   m[2],
				Status: status,
				Line:   i + 1,
			})
		}
	}
	return out, nil
}

// MarkItemDone rewrites the "[ ]" on the given 1-based line to "[x]" in place,
// preserving indentation and everything after the checkbox. If the line has no
// unchecked box (already "[x]", or not a checkbox line) it is a no-op. An
// out-of-range line number is an error.
func MarkItemDone(path string, line int) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("taskloop: stat Task.md: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("taskloop: read Task.md: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	if line < 1 || line > len(lines) {
		return fmt.Errorf("taskloop: MarkItemDone line %d out of range (1..%d)", line, len(lines))
	}
	idx := line - 1
	if newLine, changed := markLineDone(lines[idx]); changed {
		lines[idx] = newLine
	} else {
		return nil
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), info.Mode().Perm())
}

// markLineDone replaces the first "[ ]" in s with "[x]". It only touches a line
// that actually parses as an unchecked checkbox so it can't corrupt prose that
// happens to contain "[ ]".
func markLineDone(s string) (string, bool) {
	m := checkboxRe.FindStringSubmatch(s)
	if m == nil || m[1] != " " {
		return s, false
	}
	return strings.Replace(s, "[ ]", "[x]", 1), true
}

// ruleFiles is the priority order in which repo rule files are read and
// concatenated. AGENTS.md first means its guidance leads the merged prompt.
var ruleFiles = []string{"AGENTS.md", "CLAUDE.md", "rules.md", "memo.md"}

// ReadRules concatenates whichever of AGENTS.md / CLAUDE.md / rules.md / memo.md
// exist under projectRoot, in that priority order, each under a "===== name ====="
// header. An empty projectRoot or a root with none of the files yields "".
func ReadRules(projectRoot string) (string, error) {
	if strings.TrimSpace(projectRoot) == "" {
		return "", nil
	}
	var b strings.Builder
	for _, name := range ruleFiles {
		data, err := os.ReadFile(filepath.Join(projectRoot, name))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("taskloop: read %s: %w", name, err)
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "===== %s =====\n\n%s", name, strings.TrimRight(string(data), "\n"))
	}
	return b.String(), nil
}
