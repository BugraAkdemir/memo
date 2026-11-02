package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"memo/internal/calendar"
	"memo/internal/twin"
)

func briefingApp(t *testing.T, morning, meeting, inspect bool) (*App, *twin.Store, *calendar.Store) {
	t.Helper()
	ctx := context.Background()
	a, s, _ := radarApp(t, 48)
	cal, err := calendar.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cal.Close() })
	p, _ := s.Preferences(ctx)
	p.AllowProjectInspection = inspect
	p.Briefing = twin.BriefingSettings{Morning: morning, MorningTime: "00:00", PreMeeting: meeting, PreMeetingMinutes: 15}
	if err = s.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
	a.calendarStore = cal
	a.events = &eventRing{}
	return a, s, cal
}

func TestBriefingScanCreatesEachCardOnceAndAnnouncesWithoutContent(t *testing.T) {
	ctx := context.Background()
	a, s, cal := briefingApp(t, true, true, false)
	now := time.Now()
	if _, err := cal.Add(ctx, calendar.Event{Title: "GİZLİ YATIRIMCI GÖRÜŞMESİ", StartTime: now.Add(10 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	n, err := a.runBriefings(ctx, now)
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v, want the morning card and one meeting note", n, err)
	}
	list, _ := s.Briefings(ctx, 10)
	kinds := map[string]bool{}
	for _, b := range list {
		kinds[b.Kind] = true
	}
	if len(list) != 2 || !kinds["morning"] || !kinds["meeting"] {
		t.Fatalf("stored=%+v", list)
	}
	events := a.events.snapshot()
	if len(events) != 2 {
		t.Fatalf("events=%+v", events)
	}
	for _, e := range events {
		if e.Name != "autonomy:briefing" || strings.Contains(e.Data, "GİZLİ") {
			t.Fatalf("the ambient bus must carry an id and a kind only: %+v", e)
		}
	}
	if n, _ = a.runBriefings(ctx, now.Add(time.Minute)); n != 0 {
		t.Fatalf("briefed again: %d", n)
	}
}

func TestBriefingScanIsOffUnlessTheOwnerTurnedItOn(t *testing.T) {
	ctx := context.Background()
	a, _, cal := briefingApp(t, false, false, false)
	if _, err := cal.Add(ctx, calendar.Event{Title: "Toplantı", StartTime: time.Now().Add(5 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if n, err := a.runBriefings(ctx, time.Now()); err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestBriefingWorksWithoutACalendarStore(t *testing.T) {
	ctx := context.Background()
	a, s, _ := briefingApp(t, true, true, false)
	a.calendarStore = nil
	n, err := a.runBriefings(ctx, time.Now())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v, want only the morning card", n, err)
	}
	if list, _ := s.Briefings(ctx, 5); len(list) != 1 || list[0].Kind != "morning" {
		t.Fatalf("%+v", list)
	}
}

func TestStateCarriesBriefingsAndTheOwnerCanMarkThemRead(t *testing.T) {
	ctx := context.Background()
	a, s, _ := briefingApp(t, true, false, false)
	if _, err := a.runBriefings(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	raw, err := a.CallTwin(ctx, "state", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := json.Marshal(raw)
	var got struct {
		Briefings []struct {
			Key  string `json:"key"`
			Kind string `json:"kind"`
			Read bool   `json:"read"`
		} `json:"briefings"`
		Preferences twin.Preferences `json:"preferences"`
	}
	if err = json.Unmarshal(blob, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Briefings) != 1 || got.Briefings[0].Kind != "morning" || got.Briefings[0].Read || !got.Preferences.Briefing.Morning {
		t.Fatalf("state=%s", blob)
	}
	key := got.Briefings[0].Key
	if _, err = a.CallTwin(ctx, "briefing_read", json.RawMessage(`{"key":"`+key+`"}`)); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.Briefings(ctx, 5); !list[0].Read {
		t.Fatal("not marked read")
	}
	if _, err = a.CallTwin(ctx, "briefing_read", json.RawMessage(`{"key":"nope"}`)); err == nil {
		t.Fatal("an unknown key was accepted")
	}
	if _, err = a.CallTwin(ctx, "briefing_read", json.RawMessage(`{"key":"x","extra":1}`)); err == nil {
		t.Fatal("unknown fields must be rejected")
	}
}

func TestPreferencesSaveRoundTripsBriefingSettings(t *testing.T) {
	ctx := context.Background()
	a, s, _ := radarApp(t, 48)
	body := `{"timezone":"UTC","work_start":"09:00","work_end":"18:00","allow_project_inspection":false,"autonomy":{},"radar":{"enabled":true,"follow_up_hours":48},"briefing":{"morning":true,"morning_time":"08:15","pre_meeting":true,"pre_meeting_minutes":20}}`
	if _, err := a.CallTwin(ctx, "preferences", json.RawMessage(body)); err != nil {
		t.Fatal(err)
	}
	p, _ := s.Preferences(ctx)
	if !p.Briefing.Morning || p.Briefing.MorningTime != "08:15" || !p.Briefing.PreMeeting || p.Briefing.PreMeetingMinutes != 20 {
		t.Fatalf("briefing settings lost: %+v", p.Briefing)
	}
}
