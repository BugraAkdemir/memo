package taskloop

import (
	"fmt"
	"sort"
	"strings"
)

// This file is the single human/LLM-facing definition of the Task.md format.
// ParseTaskMd (taskmd.go) is the machine enforcer of the same format;
// schema_parse_sync_test.go round-trips TaskMdTemplate() through ParseTaskMd so
// the two views cannot drift.

// TaskMdItem is one checkbox line, optionally with one level of nested
// sub-items.
type TaskMdItem struct {
	Text     string
	Done     bool
	Children []TaskMdItem
}

// TaskMdDoc is the structured form of a Task.md. RenderTaskMd turns it into
// file text; the create_task_md / edit_task_md agent tools build and mutate it.
// It is a superset of what ParseTaskMd extracts back out (ParseTaskMd ignores
// Intro and Notes — the planner reads those from the raw file).
type TaskMdDoc struct {
	// Headers are the "# key: value" lines at the top. Keys are lower-case.
	Headers map[string]string
	// Intro is the free-text goal/context paragraph after the headers.
	Intro string
	Items []TaskMdItem
	// Notes is rendered after a "---" separator; ParseTaskMd stops at it.
	Notes string
}

// headerOrder is the stable render order for recognised header keys; unknown
// keys follow, sorted.
var headerOrder = []string{
	"bildirim", "mod", "planlayıcı", "kodlayıcı", "doğrulayıcı", "hafıza", "onay",
}

// RenderTaskMd serialises a TaskMdDoc to Task.md file text. The result parses
// back through ParseTaskMd with the same items and headers.
func RenderTaskMd(doc TaskMdDoc) string {
	var b strings.Builder

	written := map[string]bool{}
	for _, k := range headerOrder {
		if v, ok := doc.Headers[k]; ok && strings.TrimSpace(v) != "" {
			fmt.Fprintf(&b, "# %s: %s\n", k, strings.TrimSpace(v))
			written[k] = true
		}
	}
	var extra []string
	for k := range doc.Headers {
		if !written[k] && strings.TrimSpace(doc.Headers[k]) != "" {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	for _, k := range extra {
		fmt.Fprintf(&b, "# %s: %s\n", k, strings.TrimSpace(doc.Headers[k]))
	}

	if intro := strings.TrimSpace(doc.Intro); intro != "" {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(intro)
		b.WriteByte('\n')
	}

	if len(doc.Items) > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		writeItems(&b, doc.Items, 0)
	}

	if notes := strings.TrimSpace(doc.Notes); notes != "" {
		b.WriteString("\n---\n\n")
		b.WriteString(notes)
		b.WriteByte('\n')
	}

	return b.String()
}

func writeItems(b *strings.Builder, items []TaskMdItem, depth int) {
	indent := strings.Repeat("  ", depth)
	for _, it := range items {
		mark := " "
		if it.Done {
			mark = "x"
		}
		fmt.Fprintf(b, "%s- [%s] %s\n", indent, mark, strings.TrimSpace(it.Text))
		if len(it.Children) > 0 && depth == 0 {
			writeItems(b, it.Children, depth+1)
		}
	}
}

// TaskMdTemplate is a minimal, valid starter file. Used by the "write a
// starter Task.md here" affordance and shown as an example in the schema doc.
func TaskMdTemplate() string {
	return RenderTaskMd(TaskMdDoc{
		Headers: map[string]string{"bildirim": string(NotifyImportant)},
		Intro:   "One or two sentences on the goal and any context the worker needs.",
		Items: []TaskMdItem{
			{Text: "First concrete, independently checkable deliverable"},
			{Text: "Second deliverable"},
			{Text: "A larger piece [parallel] — split into sub-steps automatically", Children: []TaskMdItem{
				{Text: "sub-step A"},
				{Text: "sub-step B"},
			}},
		},
		Notes: "Anything after the --- line is free notes and is ignored by the parser.",
	})
}

// TaskMdSchemaDoc is the format spec, shown to the model (in the agent system
// prompt) and to the user (Tasks-tab help panel).
func TaskMdSchemaDoc() string {
	return strings.TrimSpace(`
# Task.md format

A Task.md drives a Self-Driving task list. Structure, top to bottom:

1. **Header lines** (optional, one per line, any order), each "# key: value":
   - ` + "`# bildirim: sadece-bitince | önemli | her-şey`" + ` — how chatty notifications are (default: önemli).
   - ` + "`# mod: worker | planlayıcı`" + ` — worker (default) runs each item in one agent turn; planlayıcı splits the work into a reviewed plan of small steps.
   - ` + "`# planlayıcı: <model>`" + `, ` + "`# kodlayıcı: <model>`" + `, ` + "`# doğrulayıcı: <model>`" + ` — pin the model for a role (e.g. "local", "claude", "openai/gpt-5"). Unset → asked once, then remembered in AGENTS.md.
   - ` + "`# hafıza: açık | kapalı`" + ` — whether the task's turns get memory/RAG context (default: kapalı).
   - ` + "`# onay: otomatik`" + ` — skip the plan approval gate (planlayıcı mode only).
2. **Intro paragraph** (optional) — the goal and any context. Free text; read by the planner, ignored by the parser.
3. **Checkbox items** — one per line: "- [ ] text" for pending, "- [x] text" for done. These are the unit of work; each should be independently verifiable. Put "[parallel]" anywhere in an item's text to let it fan out to sub-agents / multiple steps.
4. **Nested sub-items** (optional, one level) — "  - [ ] sub-item" under a parent. This is what planlayıcı mode writes when it decomposes a big item.
5. **Notes** — everything after a line that is exactly "---" is free notes, ignored by the parser.

Rules: keep each item small and concrete enough that "done" is checkable; do not number the items (the "- [ ]" bullet is enough); the file must contain at least one checkbox item to be runnable.
`)
}
