package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type fakeCalendarClient struct {
	events         []CalendarEvent
	gotFrom, gotTo time.Time
}

func (f *fakeCalendarClient) ListEvents(ctx context.Context, from, to time.Time) ([]CalendarEvent, error) {
	f.gotFrom, f.gotTo = from, to
	return f.events, nil
}

// TestGetCalendarEvents_NotInitialized is the regression guard for BUG-M5:
// before this tool existed, the model had no way to query real calendar
// events at all and fell back to guessing from RAG/chat memory. A nil client
// (calendar not initialized) must return a clear message, not a panic.
func TestGetCalendarEvents_NotInitialized(t *testing.T) {
	old := CalendarClient
	CalendarClient = nil
	defer func() { CalendarClient = old }()
	// The assertion below matches the Turkish wording; pin the language so
	// the default-English unset state doesn't flip the message.
	SetUILanguage("tr")
	defer SetUILanguage("")
	got, err := GetCalendarEvents(context.Background(), nil, "", nil)
	if err != nil {
		t.Fatalf("GetCalendarEvents() error = %v", err)
	}
	if !strings.Contains(got, "başlatılmamış") {
		t.Errorf("GetCalendarEvents() = %q, want a not-initialized message", got)
	}
}

func TestGetCalendarEvents_ReturnsRealEvents(t *testing.T) {
	fake := &fakeCalendarClient{events: []CalendarEvent{
		{Title: "Dentist appointment", StartTime: time.Date(2026, 7, 18, 15, 0, 0, 0, time.Local), Source: "chat"},
	}}
	old := CalendarClient
	CalendarClient = fake
	defer func() { CalendarClient = old }()

	got, err := GetCalendarEvents(context.Background(), nil, "", nil)
	if err != nil {
		t.Fatalf("GetCalendarEvents() error = %v", err)
	}
	if !strings.Contains(got, "Dentist appointment") {
		t.Errorf("GetCalendarEvents() = %q, want it to contain the real event title", got)
	}
}

func TestGetCalendarEvents_NoEventsInRange(t *testing.T) {
	fake := &fakeCalendarClient{events: nil}
	old := CalendarClient
	CalendarClient = fake
	defer func() { CalendarClient = old }()

	got, err := GetCalendarEvents(context.Background(), nil, "", nil)
	if err != nil {
		t.Fatalf("GetCalendarEvents() error = %v", err)
	}
	if strings.Contains(got, "—") {
		t.Errorf("GetCalendarEvents() = %q, want an empty-range message, not event-formatted output", got)
	}
}

func TestGetCalendarEvents_UsesGivenDateRange(t *testing.T) {
	fake := &fakeCalendarClient{}
	old := CalendarClient
	CalendarClient = fake
	defer func() { CalendarClient = old }()

	args, _ := json.Marshal(CalendarEventsArgs{From: "2026-01-01", To: "2026-01-31"})
	if _, err := GetCalendarEvents(context.Background(), args, "", nil); err != nil {
		t.Fatalf("GetCalendarEvents() error = %v", err)
	}
	if fake.gotFrom.Year() != 2026 || fake.gotFrom.Month() != 1 || fake.gotFrom.Day() != 1 {
		t.Errorf("gotFrom = %v, want 2026-01-01", fake.gotFrom)
	}
	if fake.gotTo.Day() != 31 {
		t.Errorf("gotTo = %v, want day 31", fake.gotTo)
	}
}

func TestParseCalendarDate_InvalidReturnsError(t *testing.T) {
	if _, err := parseCalendarDate("not-a-date"); err == nil {
		t.Error("parseCalendarDate(\"not-a-date\") = nil error, want error")
	}
	if _, err := parseCalendarDate(""); err == nil {
		t.Error("parseCalendarDate(\"\") = nil error, want error")
	}
}
