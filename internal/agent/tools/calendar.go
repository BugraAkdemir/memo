package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CalendarClient is the interface tools use to read calendar events. Set by
// App after initialization.
var CalendarClient interface {
	ListEvents(ctx context.Context, from, to time.Time) ([]CalendarEvent, error)
}

// CalendarEvent mirrors the fields of a stored calendar event that are
// useful to surface to the model.
type CalendarEvent struct {
	Title     string
	StartTime time.Time
	Source    string
}

type CalendarEventsArgs struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// calendarDateLayouts are tried in order when parsing the from/to arguments —
// an LLM may supply a bare date or a full timestamp.
var calendarDateLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02",
}

// GetCalendarEvents is a read-only tool that queries the real calendar store
// (events.db) instead of letting the model guess from chat/RAG memory — see
// BUG-M5: without this tool, the model had no ground truth to distinguish a
// genuinely saved event from something merely discussed or a habit that
// failed to save. from/to default to a week-wide window (yesterday through
// 7 days out) when omitted or unparsable.
func GetCalendarEvents(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args CalendarEventsArgs
	if len(argsJSON) > 0 {
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
	}
	if CalendarClient == nil {
		return "Takvim başlatılmamış", nil
	}

	from, to := defaultCalendarRange()
	if t, err := parseCalendarDate(args.From); err == nil {
		from = t
	}
	if t, err := parseCalendarDate(args.To); err == nil {
		to = t
	}

	events, err := CalendarClient.ListEvents(ctx, from, to)
	if err != nil {
		return "", fmt.Errorf("takvim okunamadı: %w", err)
	}
	if len(events) == 0 {
		return "Bu tarih aralığında kayıtlı hiçbir etkinlik yok", nil
	}

	lines := make([]string, 0, len(events))
	for _, e := range events {
		lines = append(lines, fmt.Sprintf("%s — %s (kaynak: %s)", e.StartTime.Format("2006-01-02 15:04"), e.Title, e.Source))
	}
	return strings.Join(lines, "\n"), nil
}

func defaultCalendarRange() (time.Time, time.Time) {
	now := time.Now()
	return now.Add(-24 * time.Hour), now.Add(7 * 24 * time.Hour)
}

func parseCalendarDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	var lastErr error
	for _, l := range calendarDateLayouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, lastErr
}
