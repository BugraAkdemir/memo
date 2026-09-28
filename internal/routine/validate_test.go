package routine

import (
	"errors"
	"testing"
	"time"
)

// Found live: routines with these schedules were stored as enabled and never
// fired — the only trace was a "bad schedule" log line on every tick.
func TestRoutineValidate(t *testing.T) {
	ok := []Routine{
		{Prompt: "p", Schedule: Schedule{TimeOfDay: "08:00"}},
		{Prompt: "p", Schedule: Schedule{TimeOfDay: "8:05", Weekdays: []time.Weekday{time.Sunday, time.Saturday}}},
		{Prompt: "p", Schedule: Schedule{TimeOfDay: "23:59"}},
	}
	for _, r := range ok {
		if err := r.Validate(); err != nil {
			t.Errorf("Validate(%+v) = %v, want nil", r.Schedule, err)
		}
		// Anything Validate accepts, the scheduler must be able to read.
		if _, err := ParseFireTime(r.Schedule.TimeOfDay, nil, time.Now()); err != nil {
			t.Errorf("accepted %q but ParseFireTime rejects it: %v", r.Schedule.TimeOfDay, err)
		}
	}
	bad := []Routine{
		{Prompt: "p", Schedule: Schedule{TimeOfDay: "25:99"}},
		{Prompt: "p", Schedule: Schedule{TimeOfDay: "20.00"}},
		{Prompt: "p", Schedule: Schedule{TimeOfDay: "8 PM"}},
		{Prompt: "p", Schedule: Schedule{TimeOfDay: ""}},
		{Prompt: "p", Schedule: Schedule{TimeOfDay: "08:00", Weekdays: []time.Weekday{9}}},
		{Prompt: "  ", Schedule: Schedule{TimeOfDay: "08:00"}},
	}
	for _, r := range bad {
		if err := r.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("Validate(%+v, prompt %q) = %v, want ErrInvalid", r.Schedule, r.Prompt, err)
		}
	}
}
