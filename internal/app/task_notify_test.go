package app

import (
	"strings"
	"testing"

	"memo/internal/taskloop"
)

func TestFormatTaskNotification_UsesBodyVerbatim(t *testing.T) {
	report := "2/3 madde tamamlandı.\ngreeting.txt ve notes.md yazıldı.\n3. madde takıldı: model boş yanıt verdi."
	got := formatTaskNotification(taskloop.Notification{
		Event:     "finished",
		ListTitle: "Task.md",
		Body:      report,
	})
	if got != report {
		t.Fatalf("Body not used verbatim.\n got: %q\nwant: %q", got, report)
	}
}

func TestFormatTaskNotification_FallsBackToOneLinerWithoutBody(t *testing.T) {
	got := formatTaskNotification(taskloop.Notification{Event: "item_stuck", ListTitle: "T", Detail: "x"})
	if !strings.Contains(got, "T") || !strings.Contains(got, "takıldı") {
		t.Fatalf("one-liner fallback wrong: %q", got)
	}
}

func TestFactualTaskRollup(t *testing.T) {
	tl := &taskloop.TaskList{
		Title: "T",
		Items: []taskloop.TaskItem{
			{Text: "a", Status: "done"},
			{Text: "b", Status: "done"},
			{Text: "c", Status: "stuck", Note: "boş yanıt"},
		},
	}
	got := factualTaskRollup(tl)
	if !strings.Contains(got, "2/3") || !strings.Contains(got, "1 takıldı") || !strings.Contains(got, "boş yanıt") {
		t.Fatalf("rollup = %q", got)
	}
}
