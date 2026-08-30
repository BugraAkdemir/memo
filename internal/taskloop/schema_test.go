package taskloop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeParse renders a doc, writes it, and parses it back.
func writeParse(t *testing.T, doc TaskMdDoc) *ParsedTaskMd {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "Task.md")
	if err := os.WriteFile(path, []byte(RenderTaskMd(doc)), 0644); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseTaskMd(path)
	if err != nil {
		t.Fatalf("ParseTaskMd: %v", err)
	}
	return parsed
}

func TestRenderTaskMd_RoundTrip(t *testing.T) {
	doc := TaskMdDoc{
		Headers: map[string]string{
			"bildirim":  "her-şey",
			"mod":       "planlayıcı",
			"kodlayıcı": "local",
			"custom":    "keep-me",
		},
		Intro: "Build the thing.",
		Items: []TaskMdItem{
			{Text: "first"},
			{Text: "second [parallel]", Children: []TaskMdItem{
				{Text: "sub a"},
				{Text: "sub b", Done: true},
			}},
			{Text: "third", Done: true},
		},
		Notes: "ignored tail",
	}
	parsed := writeParse(t, doc)

	if parsed.NotifyLevel != NotifyEverything {
		t.Fatalf("NotifyLevel = %q, want %q", parsed.NotifyLevel, NotifyEverything)
	}
	if parsed.Headers["mod"] != "planlayıcı" || parsed.Headers["kodlayıcı"] != "local" {
		t.Fatalf("headers not captured: %#v", parsed.Headers)
	}
	if parsed.Headers["custom"] != "keep-me" {
		t.Fatalf("unknown header dropped: %#v", parsed.Headers)
	}
	// 3 top-level + 2 nested = 5 checkbox lines, in file order.
	if len(parsed.Items) != 5 {
		t.Fatalf("Items = %d, want 5", len(parsed.Items))
	}
	if parsed.Items[0].Text != "first" || parsed.Items[0].Indent != 0 {
		t.Fatalf("Items[0] = %+v", parsed.Items[0])
	}
	if parsed.Items[1].Text != "second [parallel]" || parsed.Items[1].Indent != 0 {
		t.Fatalf("Items[1] = %+v", parsed.Items[1])
	}
	if parsed.Items[2].Text != "sub a" || parsed.Items[2].Indent == 0 {
		t.Fatalf("nested item not indented: %+v", parsed.Items[2])
	}
	if parsed.Items[3].Text != "sub b" || parsed.Items[3].Status != "done" {
		t.Fatalf("Items[3] = %+v", parsed.Items[3])
	}
	if parsed.Items[4].Text != "third" || parsed.Items[4].Status != "done" {
		t.Fatalf("Items[4] = %+v", parsed.Items[4])
	}
	// Notes must not leak into items.
	for _, it := range parsed.Items {
		if strings.Contains(it.Text, "ignored") {
			t.Fatalf("note text leaked into an item: %+v", it)
		}
	}
}

func TestTaskMdTemplate_ParsesClean(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Task.md")
	if err := os.WriteFile(path, []byte(TaskMdTemplate()), 0644); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseTaskMd(path)
	if err != nil {
		t.Fatalf("ParseTaskMd(template): %v", err)
	}
	if len(parsed.Items) == 0 {
		t.Fatal("template produced no checkbox items")
	}
	if len(parsed.Warnings) != 0 {
		t.Fatalf("template produced warnings: %v", parsed.Warnings)
	}
	if parsed.NotifyLevel != NotifyImportant {
		t.Fatalf("template NotifyLevel = %q, want %q", parsed.NotifyLevel, NotifyImportant)
	}
}

func TestTaskMdSchemaDoc_MentionsCoreRules(t *testing.T) {
	doc := TaskMdSchemaDoc()
	for _, must := range []string{"# bildirim:", "# mod:", "[parallel]", "- [ ]", "---"} {
		if !strings.Contains(doc, must) {
			t.Fatalf("schema doc missing %q", must)
		}
	}
}

func TestParseTaskMd_SeparatorOnlyCutsAfterItems(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Task.md")
	// A leading "---" (e.g. stray frontmatter fence) must NOT swallow the list.
	writeFile(t, path, "---\n# bildirim: önemli\n---\n- [ ] real item\n---\n- [ ] note, not an item\n")
	parsed, err := ParseTaskMd(path)
	if err != nil {
		t.Fatalf("ParseTaskMd: %v", err)
	}
	if len(parsed.Items) != 1 || parsed.Items[0].Text != "real item" {
		t.Fatalf("items = %+v, want exactly [real item]", parsed.Items)
	}
}
