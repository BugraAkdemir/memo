package autonomy

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"
	"unicode"

	"memo/internal/calendar"
	"memo/internal/twin"
)

// BriefingCalendar is the part of the calendar the briefer reads.
type BriefingCalendar interface {
	List(ctx context.Context, from, to time.Time) ([]calendar.Event, error)
}

// Briefer builds the morning briefing and the pre-meeting note. Both are
// assembled from local data with no model call: they cost nothing, cannot
// invent facts and never send text anywhere.
//
// Headings are codes and items are language-neutral data (times, names,
// addresses), so the UI translates them in the owner's language.
type Briefer struct {
	Store    *twin.Store
	Calendar BriefingCalendar
	// Dev optionally lists unfinished code work, one line per project.
	Dev func(context.Context) []string
	Now func() time.Time
	// Compose optionally lets the owner's briefing model write a short summary
	// and answer the custom items. It is only called when a briefing model is
	// chosen, and its failure never blocks the briefing.
	Compose Composer
}

// ComposeInput is what the briefing model is given: the day's collected
// sections (data, which may quote other people's mail) and the owner's own
// custom instructions.
type ComposeInput struct {
	Title    string
	Sections []twin.BriefingSection
	Custom   []twin.BriefingCustom
}

// ComposeOutput is what the model returns: a short summary and one answer per
// custom item, in the same order.
type ComposeOutput struct {
	Summary string
	Custom  []string
}

// Composer writes the model-made part of a briefing.
type Composer func(ctx context.Context, in ComposeInput) (ComposeOutput, error)

const (
	briefLines        = 5 // list lines per section in the morning briefing
	meetingLines      = 5 // contact lines in a pre-meeting note
	briefingSnippet   = 120
	briefingOvernight = 24 * time.Hour
)

func (b *Briefer) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

func location(p twin.Preferences) *time.Location {
	if loc, err := time.LoadLocation(p.Timezone); err == nil {
		return loc
	}
	return time.UTC
}

func clip(s string, n int) string {
	s = strings.Join(strings.FieldsFunc(s, func(r rune) bool { return unicode.IsSpace(r) }), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func join(parts ...string) string {
	out := []string{}
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " · ")
}

// Morning returns today's briefing the first time it is due (the owner's local
// clock has reached the configured time and none exists for the date), saving
// it. It returns nil when nothing is due.
func (b *Briefer) Morning(ctx context.Context) (*twin.Briefing, error) {
	prefs, err := b.Store.Preferences(ctx)
	if err != nil || !prefs.Briefing.Morning {
		return nil, err
	}
	loc := location(prefs)
	now := b.now()
	local := now.In(loc)
	if local.Format("15:04") < prefs.Briefing.MorningTime {
		return nil, nil
	}
	key := "morning:" + local.Format("2006-01-02")
	if has, err := b.Store.HasBriefing(ctx, key); err != nil || has {
		return nil, err
	}
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	dayEnd := dayStart.AddDate(0, 0, 1).Add(-time.Second)
	out := twin.Briefing{Key: key, Kind: "morning", Title: local.Format("2006-01-02"), CreatedAt: now}
	add := func(s twin.BriefingSection) {
		if len(s.Items) > 0 || len(s.Counts) > 0 {
			out.Sections = append(out.Sections, s)
		}
	}

	set := prefs.Briefing
	if b.Calendar != nil && set.SectionEnabled("meetings") {
		events, err := b.Calendar.List(ctx, dayStart, dayEnd)
		if err != nil {
			return nil, err
		}
		sort.Slice(events, func(i, j int) bool { return events[i].StartTime.Before(events[j].StartTime) })
		items := []string{}
		for _, e := range events {
			items = append(items, e.StartTime.In(loc).Format("15:04")+" "+clip(e.Title, 120))
		}
		add(twin.BriefingSection{Heading: "meetings_today", Items: items})
	}

	actions, err := b.Store.Actions(ctx)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	drafts := []string{}
	for _, a := range actions {
		if a.State != "preparing" && now.Sub(a.UpdatedAt) <= briefingOvernight && !a.UpdatedAt.After(now.Add(time.Minute)) {
			counts[a.State]++
		}
		if a.State == "pending" && len(drafts) < briefLines {
			drafts = append(drafts, join(a.Recipient, clip(a.Subject, 100)))
		}
	}
	if set.SectionEnabled("overnight") {
		add(twin.BriefingSection{Heading: "overnight", Counts: counts})
	}
	if set.SectionEnabled("drafts") {
		add(twin.BriefingSection{Heading: "pending_drafts", Items: drafts})
	}

	open, err := b.Store.Commitments(ctx)
	if err != nil {
		return nil, err
	}
	waiting, promises := []string{}, []string{}
	for _, c := range open {
		switch c.Kind {
		case "waiting_on_other":
			overdue := c.Status == "reminded" || (c.Status == "open" && !c.DueAt.After(now))
			if overdue && len(waiting) < briefLines {
				waiting = append(waiting, join(c.Address, clip(c.Subject, 100)))
			}
		case "my_promise":
			if !c.DueAt.After(dayEnd) && len(promises) < briefLines {
				promises = append(promises, join(clip(c.Body, 150), c.Address))
			}
		}
	}
	if set.SectionEnabled("waiting") {
		add(twin.BriefingSection{Heading: "waiting_overdue", Items: waiting})
	}
	if set.SectionEnabled("promises") {
		add(twin.BriefingSection{Heading: "promises_due", Items: promises})
	}

	if b.Dev != nil && set.SectionEnabled("code") {
		lines := b.Dev(ctx)
		if len(lines) > briefLines {
			lines = lines[:briefLines]
		}
		add(twin.BriefingSection{Heading: "unfinished_code", Items: lines})
	}
	b.addCustom(ctx, &out, set)
	if len(out.Sections) == 0 && out.Summary == "" {
		out.Sections = []twin.BriefingSection{{Heading: "all_clear", Items: []string{}}}
	}
	created, err := b.Store.SaveBriefing(ctx, out)
	if err != nil || !created {
		return nil, err
	}
	return &out, nil
}

// addCustom appends the owner's custom items and, when a briefing model is
// chosen, lets it write the summary and answer them. Without a model, or when
// the model fails, each item is shown as the owner wrote it, so a reminder is
// never lost to a model outage.
func (b *Briefer) addCustom(ctx context.Context, out *twin.Briefing, set twin.BriefingSettings) {
	custom := set.Custom
	answers := make([]string, len(custom))
	for i, c := range custom {
		answers[i] = c.Prompt
	}
	if b.Compose != nil && set.Model.Provider != "" && (len(custom) > 0 || len(out.Sections) > 0) {
		if res, err := b.Compose(ctx, ComposeInput{Title: out.Title, Sections: out.Sections, Custom: custom}); err == nil {
			out.Summary = strings.TrimSpace(res.Summary)
			for i := range answers {
				if i < len(res.Custom) && strings.TrimSpace(res.Custom[i]) != "" {
					answers[i] = strings.TrimSpace(res.Custom[i])
				}
			}
		}
	}
	for i, c := range custom {
		out.Sections = append(out.Sections, twin.BriefingSection{Heading: "custom", Label: c.Title, Items: []string{answers[i]}})
	}
}

// MeetingNotes returns a note for every meeting that starts within the
// configured lead time and has not been briefed yet.
func (b *Briefer) MeetingNotes(ctx context.Context) ([]twin.Briefing, error) {
	prefs, err := b.Store.Preferences(ctx)
	if err != nil || !prefs.Briefing.PreMeeting || b.Calendar == nil {
		return nil, err
	}
	loc := location(prefs)
	now := b.now()
	until := now.Add(time.Duration(prefs.Briefing.PreMeetingMinutes) * time.Minute)
	events, err := b.Calendar.List(ctx, now, until)
	if err != nil {
		return nil, err
	}
	notes := []twin.Briefing{}
	for _, e := range events {
		if !e.StartTime.After(now) || e.StartTime.After(until) {
			continue
		}
		key := "meeting:" + e.ID
		if has, err := b.Store.HasBriefing(ctx, key); err != nil {
			return notes, err
		} else if has {
			continue
		}
		note, err := b.meetingNote(ctx, e, key, loc, now)
		if err != nil {
			return notes, err
		}
		created, err := b.Store.SaveBriefing(ctx, note)
		if err != nil {
			return notes, err
		}
		if created {
			notes = append(notes, note)
		}
	}
	return notes, nil
}

// linkedContact finds the contact only when the event's contact field is an
// exact, already-known mail address. A display name never links an identity.
func (b *Briefer) linkedContact(ctx context.Context, field string) (string, bool, error) {
	field = strings.TrimSpace(field)
	if !strings.Contains(field, "@") {
		return "", false, nil
	}
	addr, err := mail.ParseAddress(field)
	if err != nil || !strings.EqualFold(addr.Address, field) {
		return "", false, nil
	}
	id, err := b.Store.Resolve(ctx, twin.Alias{Channel: "mail", Address: addr.Address})
	if errors.Is(err, twin.ErrNotFound) {
		return "", false, nil
	}
	return id, err == nil, err
}

func (b *Briefer) meetingNote(ctx context.Context, e calendar.Event, key string, loc *time.Location, now time.Time) (twin.Briefing, error) {
	when := e.StartTime.In(loc).Format("15:04")
	if e.EndTime != nil {
		when += "–" + e.EndTime.In(loc).Format("15:04")
	}
	note := twin.Briefing{Key: key, Kind: "meeting", Title: clip(e.Title, 160), CreatedAt: now,
		Sections: []twin.BriefingSection{{Heading: "meeting_when", Items: []string{when}}}}
	contact, linked, err := b.linkedContact(ctx, e.ContactName)
	if err != nil {
		return note, err
	}
	if !linked {
		note.Sections = append(note.Sections, twin.BriefingSection{Heading: "meeting_unlinked", Items: []string{}})
		return note, nil
	}
	lines := []meetingLine{}
	open, err := b.Store.Commitments(ctx)
	if err != nil {
		return note, err
	}
	for _, c := range open {
		if c.ContactID == contact && c.Kind == "my_promise" {
			lines = append(lines, meetingLine{"meeting_promises", join(clip(c.Body, 150), c.DueAt.In(loc).Format("02.01 15:04"))})
		}
	}
	for _, c := range open {
		if c.ContactID == contact && c.Kind == "waiting_on_other" {
			lines = append(lines, meetingLine{"meeting_waiting", join(clip(c.Subject, 100), c.CreatedAt.In(loc).Format("02.01 15:04"))})
		}
	}
	actions, err := b.Store.Actions(ctx)
	if err != nil {
		return note, err
	}
	for _, a := range actions {
		if a.ContactID == contact && a.State == "pending" {
			lines = append(lines, meetingLine{"meeting_drafts", join(a.Recipient, clip(a.Subject, 100))})
		}
	}
	history, err := b.Store.Timeline(ctx, contact, 10)
	if err != nil {
		return note, err
	}
	for _, m := range history {
		arrow := "←"
		if m.Outgoing {
			arrow = "→"
		}
		lines = append(lines, meetingLine{"meeting_recent", fmt.Sprintf("%s %s %s", m.SentAt.In(loc).Format("02.01 15:04"), arrow, clip(m.Body, briefingSnippet))})
	}
	lines = pickMeetingLines(lines, meetingLines)
	index := map[string]int{}
	for _, l := range lines {
		i, ok := index[l.heading]
		if !ok {
			note.Sections = append(note.Sections, twin.BriefingSection{Heading: l.heading, Items: []string{}})
			i = len(note.Sections) - 1
			index[l.heading] = i
		}
		note.Sections[i].Items = append(note.Sections[i].Items, l.text)
	}
	return note, nil
}

type meetingLine struct{ heading, text string }

// pickMeetingLines chooses the lines of a pre-meeting note. A meeting needs the
// recent conversation as much as the open promises, so each category has a
// quota (two promises, one awaited reply, one draft) and the conversation gets
// every remaining slot; a category with nothing hands its slots to the next one
// in priority order. Found live: seven promises used all five lines.
func pickMeetingLines(all []meetingLine, max int) []meetingLine {
	order := []string{"meeting_promises", "meeting_waiting", "meeting_drafts", "meeting_recent"}
	quota := map[string]int{"meeting_promises": 2, "meeting_waiting": 1, "meeting_drafts": 1, "meeting_recent": max}
	byKind := map[string][]meetingLine{}
	for _, l := range all {
		byKind[l.heading] = append(byKind[l.heading], l)
	}
	taken := map[string]int{}
	total := 0
	for _, k := range order {
		n := quota[k]
		if n > len(byKind[k]) {
			n = len(byKind[k])
		}
		if n > max-total {
			n = max - total
		}
		taken[k] = n
		total += n
	}
	// Unused slots go to whichever category still has lines, in priority order.
	for _, k := range order {
		for total < max && taken[k] < len(byKind[k]) {
			taken[k]++
			total++
		}
	}
	out := make([]meetingLine, 0, total)
	for _, k := range order {
		out = append(out, byKind[k][:taken[k]]...)
	}
	return out
}
