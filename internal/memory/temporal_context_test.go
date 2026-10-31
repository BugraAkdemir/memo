package memory

import (
	"strings"
	"testing"
	"time"
)

func TestMemoryPromptKeepsAbsoluteSourceAnchorInBothFormats(t *testing.T) {
	for _, format := range []func([]MemoryResult) string{FormatMemoriesForPrompt, FormatMemoriesUserOnly} {
		text := format([]MemoryResult{{UserMsg: "Yarın toplantı var", Content: "Yarın toplantı var", Timestamp: "2026-10-31T20:50:00Z"}})
		if !strings.Contains(text, "recorded_at=2026-10-31T20:50:00Z") || !strings.Contains(text, "source date, not today's clock") {
			t.Fatal(text)
		}
		if !strings.Contains(text, "Yarın toplantı var") {
			t.Fatal("rewrote source wording")
		}
		unknown := format([]MemoryResult{{UserMsg: "tomorrow", Timestamp: "invalid"}})
		if !strings.Contains(unknown, "recorded_at=unknown") {
			t.Fatal("invented timestamp", unknown)
		}
		mail := format([]MemoryResult{{Source: "mail", UserMsg: "[Mail | Date: 2026-10-31T23:50:00+03:00] Yarın 12:00", Content: "Yarın 12:00", Timestamp: "2026-11-01T02:00:00Z"}})
		if !strings.Contains(mail, "not recorded_at (ingestion)") || !strings.Contains(mail, "external_message") {
			t.Fatal(mail)
		}
	}
}

func TestMemoryAgeDescribesElapsedTimeWithoutCallingYesterdayToday(t *testing.T) {
	// Works on both sides of midnight: elapsed age is not a calendar-day label.
	stamp := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
	if got := formatMemoryAge(stamp); got != "less than 24 hours ago" {
		t.Fatal(got)
	}
	if got := formatMemoryAge(time.Now().Add(time.Hour).Format(time.RFC3339)); got != "future timestamp" {
		t.Fatal(got)
	}
}
