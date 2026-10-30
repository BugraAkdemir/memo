package autonomy

import (
	"memo/internal/calendar"
	"time"
)

type BusyWindow struct {
	Start time.Time  `json:"start"`
	End   *time.Time `json:"end,omitempty"`
}

// MaskCalendarEvents deliberately has no title, description, source or contact
// fields in its output. External availability tools must return this shape.
func MaskCalendarEvents(events []calendar.Event) []BusyWindow {
	out := make([]BusyWindow, 0, len(events))
	for _, e := range events {
		out = append(out, BusyWindow{e.StartTime, e.EndTime})
	}
	return out
}

// ContactScope fails closed on unknown identities; graph alias resolution is
// owned by the next plan step and must happen before calling this guard.
type ContactScope struct{ Sender string }

func (s ContactScope) Allows(target string) bool {
	sender := address(s.Sender)
	return sender != "" && sender == address(target)
}
