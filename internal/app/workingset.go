package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"memo/internal/agent"
	"memo/internal/truncate"
)

// workingSet is a per-chat digest of the files and shell commands an agent
// turn has already touched this session. It is injected, compactly, into
// later turns so the model doesn't re-read a file or re-run a command just
// because that tool's raw output has since fallen out of the message
// history — the single biggest source of wasted iterations in a long
// agentic session (AGENTS.md streaming/agent notes, Phase 2 of the
// long-session plan).
//
// In-memory and session-scoped: lost on a backend restart, which only costs
// one redundant re-read, and never persisted or synced.
type workingSet struct {
	mu    sync.Mutex
	files map[string]*wsFile
	cmds  []wsCmd
	order int64 // monotonic, for "most recently touched" ordering of files
}

type wsFile struct {
	path   string
	action string // read | wrote | edited
	lines  int    // best-effort line count, 0 = unknown
	seq    int64
}

type wsCmd struct {
	cmd  string
	ok   bool
	tail string
}

const (
	wsMaxFiles       = 12
	wsMaxCmds        = 5
	wsCmdTailChars   = 140
	wsFileActionRead = "read"
)

func newWorkingSet() *workingSet {
	return &workingSet{files: map[string]*wsFile{}}
}

// record folds one agent tool event into the set. Only file and command
// tools contribute; searches, directory listings, web fetches etc. are
// ignored (their staleness cost is low and their output is cheap to
// re-derive).
func (w *workingSet) record(ev agent.AgentEvent) {
	if ev.Type != agent.EventToolResult && ev.Type != agent.EventToolError {
		return
	}
	ok := ev.Type == agent.EventToolResult

	switch ev.ToolName {
	case "read_file":
		if p := argString(ev.Args, "path"); p != "" {
			w.touchFile(p, wsFileActionRead, countLines(ev.Result))
		}
	case "write_file":
		if p := argString(ev.Args, "path"); p != "" {
			w.touchFile(p, "wrote", countLines(argString(ev.Args, "content")))
		}
	case "edit_file", "insert_line", "delete_lines", "replace_in_file":
		if p := argString(ev.Args, "path"); p != "" {
			w.touchFile(p, "edited", 0)
		}
	case "run_command", "run_command_readonly":
		cmd := argString(ev.Args, "command")
		if cmd == "" {
			cmd = argString(ev.Args, "cmd")
		}
		if cmd == "" {
			return
		}
		body := ev.Result
		if !ok && ev.Error != "" {
			body = ev.Error
		}
		w.addCmd(cmd, ok, body)
	}
}

func (w *workingSet) touchFile(path, action string, lines int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.order++
	f := w.files[path]
	if f == nil {
		f = &wsFile{path: path}
		w.files[path] = f
	}
	f.action = action
	f.seq = w.order
	if lines > 0 {
		f.lines = lines
	}
	// Bound the map: drop the least-recently-touched entries.
	if len(w.files) > wsMaxFiles {
		var all []*wsFile
		for _, v := range w.files {
			all = append(all, v)
		}
		sort.Slice(all, func(i, j int) bool { return all[i].seq < all[j].seq })
		for _, v := range all[:len(all)-wsMaxFiles] {
			delete(w.files, v.path)
		}
	}
}

func (w *workingSet) addCmd(cmd string, ok bool, body string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	tail := strings.TrimSpace(body)
	if len(tail) > wsCmdTailChars {
		tail = "…" + tail[len(tail)-wsCmdTailChars:]
	}
	tail = strings.ReplaceAll(tail, "\n", " ")
	w.cmds = append(w.cmds, wsCmd{cmd: strings.TrimSpace(cmd), ok: ok, tail: tail})
	if len(w.cmds) > wsMaxCmds {
		w.cmds = w.cmds[len(w.cmds)-wsMaxCmds:]
	}
}

// render produces the block injected into the system prompt, hard-capped at
// maxTokens (truncate.EstimateTokens units). Empty when nothing has been
// recorded yet.
func (w *workingSet) render(maxTokens int) string {
	w.mu.Lock()
	files := make([]*wsFile, 0, len(w.files))
	for _, f := range w.files {
		files = append(files, f)
	}
	cmds := append([]wsCmd(nil), w.cmds...)
	w.mu.Unlock()

	if len(files) == 0 && len(cmds) == 0 {
		return ""
	}
	sort.Slice(files, func(i, j int) bool { return files[i].seq > files[j].seq })

	var b strings.Builder
	b.WriteString("\n\n[Working set — files and commands already handled earlier this session. Don't re-read or re-run these unless you expect the result to have changed.]")
	if len(files) > 0 {
		b.WriteString("\nFiles:")
		for _, f := range files {
			if f.lines > 0 {
				fmt.Fprintf(&b, "\n- %s (%d lines) — %s", f.path, f.lines, f.action)
			} else {
				fmt.Fprintf(&b, "\n- %s — %s", f.path, f.action)
			}
		}
	}
	if len(cmds) > 0 {
		b.WriteString("\nCommands:")
		for _, c := range cmds {
			status := "ok"
			if !c.ok {
				status = "failed"
			}
			if c.tail != "" {
				fmt.Fprintf(&b, "\n- `%s` → %s: %s", c.cmd, status, c.tail)
			} else {
				fmt.Fprintf(&b, "\n- `%s` → %s", c.cmd, status)
			}
		}
	}

	out := b.String()
	if maxTokens > 0 && truncate.EstimateTokens(out) > maxTokens {
		lines := strings.Split(out, "\n")
		var kept []string
		total := 0
		for _, ln := range lines {
			t := truncate.EstimateTokens(ln)
			if total+t > maxTokens {
				kept = append(kept, "- … (working set truncated)")
				break
			}
			total += t
			kept = append(kept, ln)
		}
		out = strings.Join(kept, "\n")
	}
	return out
}

func argString(raw json.RawMessage, key string) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// --- App glue -------------------------------------------------------------

func (a *App) workingSetFor(chatID string) *workingSet {
	if chatID == "" {
		return nil
	}
	a.workingSetMu.Lock()
	defer a.workingSetMu.Unlock()
	if a.workingSets == nil {
		a.workingSets = map[string]*workingSet{}
	}
	ws := a.workingSets[chatID]
	if ws == nil {
		ws = newWorkingSet()
		a.workingSets[chatID] = ws
	}
	return ws
}

// recordWorkingSetEvent is called from callAgentStream's onEvent for every
// tool event in an agent turn.
func (a *App) recordWorkingSetEvent(chatID string, ev agent.AgentEvent) {
	a.cfgMu.RLock()
	enabled := a.cfg.AgentMode.WorkingSetEnabled
	a.cfgMu.RUnlock()
	if !enabled {
		return
	}
	if ws := a.workingSetFor(chatID); ws != nil {
		ws.record(ev)
	}
}

// renderWorkingSet is the block buildMessagesForSession injects. Empty
// unless the feature is on and this chat has recorded something.
func (a *App) renderWorkingSet(chatID string) string {
	a.cfgMu.RLock()
	enabled := a.cfg.AgentMode.WorkingSetEnabled
	maxTok := a.cfg.AgentMode.WorkingSetMaxTokens
	a.cfgMu.RUnlock()
	if !enabled || chatID == "" {
		return ""
	}
	a.workingSetMu.Lock()
	ws := a.workingSets[chatID]
	a.workingSetMu.Unlock()
	if ws == nil {
		return ""
	}
	return ws.render(maxTok)
}

// clearWorkingSet drops a chat's working set and cached conversation summary
// (e.g. when the chat is deleted). Safe to call for an unknown chatID.
func (a *App) clearWorkingSet(chatID string) {
	a.workingSetMu.Lock()
	delete(a.workingSets, chatID)
	a.workingSetMu.Unlock()
	a.convSummaryMu.Lock()
	delete(a.convSummaries, chatID)
	a.convSummaryMu.Unlock()
}
