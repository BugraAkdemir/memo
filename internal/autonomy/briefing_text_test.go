package autonomy

import (
	"strings"
	"testing"
	"time"

	"memo/internal/twin"
)

func renderFixture() twin.Briefing {
	return twin.Briefing{
		Key: "morning:2026-11-07", Kind: "morning", Title: "2026-11-07", CreatedAt: time.Now(),
		Summary: "Two drafts wait.",
		Sections: []twin.BriefingSection{
			{Heading: "meetings_today", Items: []string{"10:00 Standup"}},
			{Heading: "overnight", Counts: map[string]int{"sent": 2, "cancelled": 1}},
			{Heading: "custom", Label: "Water", Items: []string{"Drink a glass of water"}},
		},
	}
}

func TestRenderMorningBriefingInTurkishAndEnglish(t *testing.T) {
	subject, body := RenderMorningBriefing(renderFixture(), "tr")
	if subject != "Memo · Sabah brifingi · 2026-11-07" {
		t.Fatalf("subject: %q", subject)
	}
	for _, want := range []string{"Two drafts wait.", "Bugünkü toplantılar", "• 10:00 Standup", "Gönderildi: 2", "İptal edildi: 1", "Water\nDrink a glass of water"} {
		if !strings.Contains(body, want) {
			t.Errorf("tr body misses %q:\n%s", want, body)
		}
	}
	_, body = RenderMorningBriefing(renderFixture(), "en")
	for _, want := range []string{"Today's meetings", "Sent: 2", "Cancelled: 1"} {
		if !strings.Contains(body, want) {
			t.Errorf("en body misses %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "Bugünkü") {
		t.Fatal("English output must not carry Turkish headings")
	}
}

func TestRenderMorningBriefingKeepsUnknownCodesVisibleAndCustomItemsUnbulleted(t *testing.T) {
	b := twin.Briefing{Title: "d", Sections: []twin.BriefingSection{
		{Heading: "future_section", Items: []string{"x"}},
		{Heading: "overnight", Counts: map[string]int{"new_state": 3}},
		{Heading: "custom", Label: "Mine", Items: []string{"plain line"}},
	}}
	_, body := RenderMorningBriefing(b, "en")
	for _, want := range []string{"future_section", "new_state: 3", "Mine\nplain line"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "• plain line") {
		t.Fatal("a custom item is the owner's own line, not a list bullet")
	}
}

func TestRenderMorningBriefingAllClear(t *testing.T) {
	_, body := RenderMorningBriefing(twin.Briefing{Title: "d", Sections: []twin.BriefingSection{{Heading: "all_clear", Items: []string{}}}}, "tr")
	if !strings.Contains(body, "Bugün için bekleyen bir şey yok.") {
		t.Fatalf("all clear: %s", body)
	}
}
