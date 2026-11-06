package autonomy

import (
	"context"
	"strings"
	"testing"
	"time"

	"memo/internal/calendar"
	"memo/internal/twin"
)

// Istanbul is UTC+3 all year, which keeps the local-time cases unambiguous.
var istanbul = time.FixedZone("TRT", 3*3600)

type briefFixture struct {
	b   *Briefer
	cal *calendar.Store
	now *time.Time
}

func newBriefer(t *testing.T, morning, meeting bool) *briefFixture {
	t.Helper()
	ctx := context.Background()
	s, err := twin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	cal, err := calendar.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cal.Close() })
	p := twin.Preferences{Timezone: "Europe/Istanbul", WorkStart: "09:00", WorkEnd: "18:00"}
	p.Briefing = twin.BriefingSettings{Morning: morning, MorningTime: "09:00", PreMeeting: meeting, PreMeetingMinutes: 15}
	if err = s.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 3, 9, 30, 0, 0, istanbul)
	f := &briefFixture{cal: cal, now: &now}
	f.b = &Briefer{Store: s, Calendar: cal, Now: func() time.Time { return *f.now }}
	return f
}

// realClock lets a test use real timestamps (the store stamps actions and
// accepts promises against the wall clock) and makes the morning always due.
func realClock(t *testing.T, f *briefFixture, ahead time.Duration) {
	t.Helper()
	ctx := context.Background()
	p, _ := f.b.Store.Preferences(ctx)
	p.Briefing.MorningTime = "00:00"
	if err := f.b.Store.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
	*f.now = time.Now().Add(ahead).In(istanbul)
}

func section(b *twin.Briefing, code string) *twin.BriefingSection {
	for i := range b.Sections {
		if b.Sections[i].Heading == code {
			return &b.Sections[i]
		}
	}
	return nil
}

func sendTo(t *testing.T, s *twin.Store, addr, body, event string, at time.Time) {
	t.Helper()
	m := twin.Message{Channel: "mail", Account: "acct", EventID: event, Address: addr, Outgoing: true, Body: body, SentAt: at}
	if err := s.RecordMessage(context.Background(), m, addr); err != nil {
		t.Fatal(err)
	}
}

func TestMorningBriefingWaitsForItsTimeAndComesOncePerDay(t *testing.T) {
	ctx := context.Background()
	f := newBriefer(t, true, false)
	*f.now = time.Date(2026, 11, 3, 8, 59, 0, 0, istanbul)
	if b, err := f.b.Morning(ctx); err != nil || b != nil {
		t.Fatalf("before 09:00 local: %+v err=%v", b, err)
	}
	*f.now = time.Date(2026, 11, 3, 9, 0, 0, 0, istanbul)
	b, err := f.b.Morning(ctx)
	if err != nil || b == nil || b.Key != "morning:2026-11-03" || b.Kind != "morning" {
		t.Fatalf("at 09:00: %+v err=%v", b, err)
	}
	if section(b, "all_clear") == nil {
		t.Fatalf("an empty day must still say so: %+v", b.Sections)
	}
	*f.now = time.Date(2026, 11, 3, 15, 0, 0, 0, istanbul)
	if again, _ := f.b.Morning(ctx); again != nil {
		t.Fatalf("briefed twice in a day: %+v", again)
	}
	*f.now = time.Date(2026, 11, 4, 9, 5, 0, 0, istanbul)
	if next, err := f.b.Morning(ctx); err != nil || next == nil || next.Key != "morning:2026-11-04" {
		t.Fatalf("next day: %+v err=%v", next, err)
	}
	if list, _ := f.b.Store.Briefings(ctx, 10); len(list) != 2 {
		t.Fatalf("stored %d briefings, want 2", len(list))
	}
}

func TestMorningBriefingUsesTheOwnersTimezoneNotUTC(t *testing.T) {
	ctx := context.Background()
	f := newBriefer(t, true, false)
	// 06:30 UTC is already 09:30 in Istanbul.
	*f.now = time.Date(2026, 11, 3, 6, 30, 0, 0, time.UTC)
	if b, _ := f.b.Morning(ctx); b == nil || b.Key != "morning:2026-11-03" {
		t.Fatalf("local morning missed: %+v", b)
	}
}

func TestMorningBriefingIsOffUnlessEnabled(t *testing.T) {
	f := newBriefer(t, false, true)
	if b, err := f.b.Morning(context.Background()); err != nil || b != nil {
		t.Fatalf("%+v %v", b, err)
	}
}

func TestMorningBriefingGathersTheDay(t *testing.T) {
	ctx := context.Background()
	f := newBriefer(t, true, false)
	s := f.b.Store
	// A pending draft, an overdue awaited reply and a promise due today.
	if err := s.SaveAction(ctx, twin.Action{ID: "a1", Channel: "mail", Account: "acct", Recipient: "ali@example.com", Subject: "Teklif", Body: "x", State: "pending", Mode: "awaiting_approval", Reason: "drafted"}); err != nil {
		t.Fatal(err)
	}
	pref, _ := s.Preferences(ctx)
	pref.Radar = twin.RadarSettings{Enabled: true, FollowUpHours: 1, TrackPromises: true}
	if err := s.SavePreferences(ctx, pref); err != nil {
		t.Fatal(err)
	}
	real := time.Now()
	sendTo(t, s, "veli@example.com", "Teklifi gördün mü?", "w1", real.Add(-10*time.Minute))
	if ok, err := s.AddPromise(ctx, twin.Promise{Channel: "mail", Account: "acct", Address: "veli@example.com", Summary: "Raporu gönder", SourceRef: "w1", DueAt: real.Add(time.Hour)}); err != nil || !ok {
		t.Fatalf("promise ok=%v err=%v", ok, err)
	}
	if _, err := s.AddPromise(ctx, twin.Promise{Channel: "mail", Account: "acct", Address: "veli@example.com", Summary: "Gelecek haftanın işi", SourceRef: "w1", DueAt: real.Add(10 * 24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	realClock(t, f, 2*time.Hour) // two hours on: the one-hour wait for veli has run out
	b, err := f.b.Morning(ctx)
	if err != nil || b == nil {
		t.Fatalf("b=%+v err=%v", b, err)
	}
	if sec := section(b, "pending_drafts"); sec == nil || len(sec.Items) != 1 || !strings.Contains(sec.Items[0], "ali@example.com") {
		t.Fatalf("pending drafts: %+v", sec)
	}
	if sec := section(b, "waiting_overdue"); sec == nil || len(sec.Items) == 0 || !strings.Contains(sec.Items[0], "veli@example.com") {
		t.Fatalf("overdue replies: %+v", sec)
	}
	sec := section(b, "promises_due")
	if sec == nil || len(sec.Items) != 1 || !strings.Contains(sec.Items[0], "Raporu gönder") {
		t.Fatalf("promises due today must hold only today's: %+v", sec)
	}
	for _, s := range b.Sections {
		for _, it := range s.Items {
			if strings.Contains(it, "Gelecek haftanın işi") {
				t.Fatal("a promise due next week was listed for today")
			}
		}
	}
	if section(b, "all_clear") != nil {
		t.Fatal("a busy day was reported as clear")
	}
}

func TestMorningBriefingListsTodaysMeetingsOnly(t *testing.T) {
	ctx := context.Background()
	f := newBriefer(t, true, false)
	day := time.Date(2026, 11, 3, 14, 0, 0, 0, istanbul)
	if _, err := f.cal.Add(ctx, calendar.Event{Title: "Backend toplantısı", StartTime: day}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.cal.Add(ctx, calendar.Event{Title: "Yarınki toplantı", StartTime: day.Add(24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.cal.Add(ctx, calendar.Event{Title: "Dünkü toplantı", StartTime: day.Add(-24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	b, err := f.b.Morning(ctx)
	if err != nil || b == nil {
		t.Fatalf("b=%+v err=%v", b, err)
	}
	sec := section(b, "meetings_today")
	if sec == nil || len(sec.Items) != 1 || !strings.Contains(sec.Items[0], "Backend toplantısı") || !strings.HasPrefix(sec.Items[0], "14:00") {
		t.Fatalf("meetings: %+v", sec)
	}
}

func TestMorningBriefingCountsTheLastNightsWork(t *testing.T) {
	ctx := context.Background()
	f := newBriefer(t, true, false)
	s := f.b.Store
	for i, st := range []string{"pending", "pending", "skipped", "blocked", "failed"} {
		a := twin.Action{ID: "n" + string(rune('a'+i)), Channel: "mail", Account: "acct", Recipient: "x@example.com", Body: "b", State: st, Mode: "awaiting_approval", Reason: "drafted"}
		if err := s.SaveAction(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	realClock(t, f, 0) // the actions were just saved, so they fall in the last 24 hours
	b, err := f.b.Morning(ctx)
	if err != nil || b == nil {
		t.Fatalf("b=%+v err=%v", b, err)
	}
	sec := section(b, "overnight")
	if sec == nil || sec.Counts["pending"] != 2 || sec.Counts["skipped"] != 1 || sec.Counts["blocked"] != 1 || sec.Counts["failed"] != 1 {
		t.Fatalf("overnight: %+v", sec)
	}
}

func TestMorningBriefingIncludesUnfinishedCodeWhenProvided(t *testing.T) {
	ctx := context.Background()
	f := newBriefer(t, true, false)
	f.b.Dev = func(context.Context) []string { return []string{"memo · 3 değişen dosya"} }
	b, err := f.b.Morning(ctx)
	if err != nil || b == nil {
		t.Fatalf("b=%+v err=%v", b, err)
	}
	if sec := section(b, "unfinished_code"); sec == nil || sec.Items[0] != "memo · 3 değişen dosya" {
		t.Fatalf("code: %+v", sec)
	}
}

func meetingIn(t *testing.T, f *briefFixture, title, contact string, startsIn time.Duration) calendar.Event {
	t.Helper()
	e, err := f.cal.Add(context.Background(), calendar.Event{Title: title, ContactName: contact, StartTime: f.now.Add(startsIn)})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestPreMeetingNoteComesOnlyInsideTheWindowAndOnce(t *testing.T) {
	ctx := context.Background()
	f := newBriefer(t, false, true)
	soon := meetingIn(t, f, "Yakın toplantı", "", 10*time.Minute)
	meetingIn(t, f, "Uzak toplantı", "", 90*time.Minute)
	started := meetingIn(t, f, "Başlamış toplantı", "", -5*time.Minute)
	_ = started
	notes, err := f.b.MeetingNotes(ctx)
	if err != nil || len(notes) != 1 || notes[0].Key != "meeting:"+soon.ID || notes[0].Title != "Yakın toplantı" {
		t.Fatalf("notes=%+v err=%v", notes, err)
	}
	if again, _ := f.b.MeetingNotes(ctx); len(again) != 0 {
		t.Fatalf("the same meeting was briefed twice: %+v", again)
	}
	*f.now = f.now.Add(80 * time.Minute) // the far one is now 10 minutes away
	if later, _ := f.b.MeetingNotes(ctx); len(later) != 1 || later[0].Title != "Uzak toplantı" {
		t.Fatalf("later=%+v", later)
	}
}

func TestPreMeetingNoteIsOffUnlessEnabled(t *testing.T) {
	f := newBriefer(t, true, false)
	meetingIn(t, f, "Toplantı", "", 5*time.Minute)
	if notes, err := f.b.MeetingNotes(context.Background()); err != nil || len(notes) != 0 {
		t.Fatalf("%+v %v", notes, err)
	}
}

func TestPreMeetingNoteIsLinkedOnlyByAnExactKnownAddress(t *testing.T) {
	ctx := context.Background()
	f := newBriefer(t, false, true)
	s := f.b.Store
	real := time.Now()
	sendTo(t, s, "ali@example.com", "Mimari taslağını gördün mü?", "e1", real.Add(-2*time.Hour))
	sendTo(t, s, "veli@example.com", "VELİ'YE ÖZEL GİZLİ NOT", "e2", real.Add(-time.Hour))
	meetingIn(t, f, "Ali ile mimari", "ali@example.com", 10*time.Minute)
	meetingIn(t, f, "Ali Yılmaz ile kahve", "Ali", 12*time.Minute) // a name never links an identity
	notes, err := f.b.MeetingNotes(ctx)
	if err != nil || len(notes) != 2 {
		t.Fatalf("notes=%+v err=%v", notes, err)
	}
	var linked, unlinked *twin.Briefing
	for i := range notes {
		if notes[i].Title == "Ali ile mimari" {
			linked = &notes[i]
		} else {
			unlinked = &notes[i]
		}
	}
	if sec := section(linked, "meeting_recent"); sec == nil || !strings.Contains(strings.Join(sec.Items, "|"), "Mimari taslağını") {
		t.Fatalf("linked meeting lacks the shared history: %+v", linked.Sections)
	}
	for _, sec := range linked.Sections {
		for _, it := range sec.Items {
			if strings.Contains(it, "GİZLİ NOT") {
				t.Fatal("another contact's message appeared in this contact's note")
			}
		}
	}
	if section(unlinked, "meeting_unlinked") == nil || section(unlinked, "meeting_recent") != nil {
		t.Fatalf("a name-only match must not pull history: %+v", unlinked.Sections)
	}
}

// Found live: with seven promises to one person, all five lines were promises and
// the recent conversation, which is what a meeting needs, never appeared.
func TestManyPromisesDoNotCrowdOutTheRecentConversation(t *testing.T) {
	ctx := context.Background()
	f := newBriefer(t, false, true)
	s := f.b.Store
	real := time.Now()
	pref, _ := s.Preferences(ctx)
	pref.Radar = twin.RadarSettings{Enabled: true, FollowUpHours: 48, TrackPromises: true}
	if err := s.SavePreferences(ctx, pref); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		sendTo(t, s, "ali@example.com", "Konuşma mesajı "+string(rune('A'+i)), "m"+string(rune('0'+i)), real.Add(-time.Duration(i+1)*time.Hour))
	}
	for i := 0; i < 7; i++ {
		if _, err := s.AddPromise(ctx, twin.Promise{Channel: "mail", Account: "acct", Address: "ali@example.com", Summary: "Söz " + string(rune('A'+i)), SourceRef: "m0", DueAt: real.Add(time.Duration(i+2) * time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	meetingIn(t, f, "Ali ile mimari", "ali@example.com", 10*time.Minute)
	notes, err := f.b.MeetingNotes(ctx)
	if err != nil || len(notes) != 1 {
		t.Fatalf("notes=%+v err=%v", notes, err)
	}
	counts := map[string]int{}
	total := 0
	for _, sec := range notes[0].Sections {
		if sec.Heading != "meeting_when" {
			counts[sec.Heading] = len(sec.Items)
			total += len(sec.Items)
		}
	}
	if total != 5 || counts["meeting_promises"] > 2 || counts["meeting_recent"] < 3 {
		t.Fatalf("promises crowded out the conversation: %v", counts)
	}
}

// When a category has nothing, its slots go to the next one instead of staying empty.
func TestUnusedQuotaSlotsAreFilledByTheNextCategory(t *testing.T) {
	ctx := context.Background()
	f := newBriefer(t, false, true)
	s := f.b.Store
	real := time.Now()
	for i := 0; i < 8; i++ {
		sendTo(t, s, "ali@example.com", "Sadece yazışma "+string(rune('A'+i)), "h"+string(rune('0'+i)), real.Add(-time.Duration(i+1)*time.Hour))
	}
	meetingIn(t, f, "Ali ile mimari", "ali@example.com", 10*time.Minute)
	notes, _ := f.b.MeetingNotes(ctx)
	if len(notes) != 1 {
		t.Fatalf("%+v", notes)
	}
	if sec := section(&notes[0], "meeting_recent"); sec == nil || len(sec.Items) != 5 {
		t.Fatalf("with no promises, waiting replies or drafts the five lines should all be history: %+v", notes[0].Sections)
	}
}

func TestPreMeetingNoteHasAtMostFiveContactLines(t *testing.T) {
	ctx := context.Background()
	f := newBriefer(t, false, true)
	s := f.b.Store
	real := time.Now()
	for i := 0; i < 9; i++ {
		sendTo(t, s, "ali@example.com", "Mesaj numarası "+string(rune('A'+i)), "e"+string(rune('0'+i)), real.Add(-time.Duration(i+1)*time.Hour))
	}
	pref, _ := s.Preferences(ctx)
	pref.Radar = twin.RadarSettings{Enabled: true, FollowUpHours: 48, TrackPromises: true}
	if err := s.SavePreferences(ctx, pref); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddPromise(ctx, twin.Promise{Channel: "mail", Account: "acct", Address: "ali@example.com", Summary: "Taslağı gönder", SourceRef: "e0", DueAt: real.Add(48 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	meetingIn(t, f, "Ali ile mimari", "ali@example.com", 10*time.Minute)
	notes, err := f.b.MeetingNotes(ctx)
	if err != nil || len(notes) != 1 {
		t.Fatalf("notes=%+v err=%v", notes, err)
	}
	total := 0
	for _, sec := range notes[0].Sections {
		if strings.HasPrefix(sec.Heading, "meeting_") && sec.Heading != "meeting_when" {
			total += len(sec.Items)
		}
	}
	if total != 5 {
		t.Fatalf("contact lines=%d, want exactly 5 from the nine messages and a promise: %+v", total, notes[0].Sections)
	}
	// The open promise outranks plain history.
	if sec := section(&notes[0], "meeting_promises"); sec == nil || len(sec.Items) != 1 {
		t.Fatalf("promise missing from the top of the note: %+v", notes[0].Sections)
	}
}

// setBriefing rewrites the briefing settings of a fixture, keeping the morning due.
func setBriefing(t *testing.T, f *briefFixture, mutate func(*twin.BriefingSettings)) {
	t.Helper()
	ctx := context.Background()
	p, _ := f.b.Store.Preferences(ctx)
	mutate(&p.Briefing)
	if err := f.b.Store.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
}

func pendingDraft(t *testing.T, f *briefFixture) {
	t.Helper()
	a := twin.Action{ID: "a1", Channel: "mail", Account: "acct", Recipient: "ali@example.com", Subject: "Teklif", Body: "x", State: "pending", Mode: "awaiting_approval", Reason: "drafted"}
	if err := f.b.Store.SaveAction(context.Background(), a); err != nil {
		t.Fatal(err)
	}
}

func TestMorningBriefingHoldsOnlyTheChosenSections(t *testing.T) {
	ctx := context.Background()
	f := newBriefer(t, true, false)
	pendingDraft(t, f)
	setBriefing(t, f, func(b *twin.BriefingSettings) { b.Sections = []string{"drafts"} })
	b, err := f.b.Morning(ctx)
	if err != nil || b == nil {
		t.Fatalf("b=%+v err=%v", b, err)
	}
	if len(b.Sections) != 1 || b.Sections[0].Heading != "pending_drafts" {
		t.Fatalf("only the drafts section was chosen: %+v", b.Sections)
	}
}

func TestMorningBriefingCustomItemsStandAloneWithoutAModel(t *testing.T) {
	ctx := context.Background()
	f := newBriefer(t, true, false)
	setBriefing(t, f, func(b *twin.BriefingSettings) {
		b.Sections = []string{}
		b.Custom = []twin.BriefingCustom{{Title: "Water", Prompt: "Drink a glass of water"}}
	})
	b, err := f.b.Morning(ctx)
	if err != nil || b == nil {
		t.Fatalf("b=%+v err=%v", b, err)
	}
	sec := section(b, "custom")
	if sec == nil || sec.Label != "Water" || len(sec.Items) != 1 || sec.Items[0] != "Drink a glass of water" {
		t.Fatalf("a custom item must show as the owner wrote it: %+v", b.Sections)
	}
	if section(b, "all_clear") != nil {
		t.Fatal("a briefing with a custom item is not 'all clear'")
	}
}

func TestMorningBriefingModelWritesSummaryAndAnswersCustomItems(t *testing.T) {
	ctx := context.Background()
	f := newBriefer(t, true, false)
	pendingDraft(t, f)
	setBriefing(t, f, func(b *twin.BriefingSettings) {
		b.Model = twin.BriefingModel{Provider: "p", Model: "m"}
		b.Custom = []twin.BriefingCustom{{Title: "Focus", Prompt: "What is the one thing to do first?"}}
	})
	var got ComposeInput
	f.b.Compose = func(_ context.Context, in ComposeInput) (ComposeOutput, error) {
		got = in
		return ComposeOutput{Summary: "A calm day.", Custom: []string{"Answer the offer."}}, nil
	}
	b, err := f.b.Morning(ctx)
	if err != nil || b == nil {
		t.Fatalf("b=%+v err=%v", b, err)
	}
	if b.Summary != "A calm day." {
		t.Fatalf("summary: %q", b.Summary)
	}
	if sec := section(b, "custom"); sec == nil || sec.Items[0] != "Answer the offer." {
		t.Fatalf("custom answer: %+v", b.Sections)
	}
	if len(got.Custom) != 1 || got.Custom[0].Prompt != "What is the one thing to do first?" || section(&twin.Briefing{Sections: got.Sections}, "pending_drafts") == nil {
		t.Fatalf("the model must get the day's data and the owner's instructions: %+v", got)
	}
}

func TestMorningBriefingDoesNotCallTheModelWhenNoneIsChosen(t *testing.T) {
	ctx := context.Background()
	f := newBriefer(t, true, false)
	pendingDraft(t, f)
	called := false
	f.b.Compose = func(context.Context, ComposeInput) (ComposeOutput, error) { called = true; return ComposeOutput{}, nil }
	if b, err := f.b.Morning(ctx); err != nil || b == nil {
		t.Fatalf("b=%+v err=%v", b, err)
	}
	if called {
		t.Fatal("no briefing model is chosen, so no model may be called")
	}
}

func TestMorningBriefingSurvivesAFailingModel(t *testing.T) {
	ctx := context.Background()
	f := newBriefer(t, true, false)
	pendingDraft(t, f)
	setBriefing(t, f, func(b *twin.BriefingSettings) {
		b.Model = twin.BriefingModel{Provider: "p", Model: "m"}
		b.Custom = []twin.BriefingCustom{{Title: "Focus", Prompt: "Plan the day"}}
	})
	f.b.Compose = func(context.Context, ComposeInput) (ComposeOutput, error) {
		return ComposeOutput{}, context.DeadlineExceeded
	}
	b, err := f.b.Morning(ctx)
	if err != nil || b == nil {
		t.Fatalf("a model outage must not cost the owner the briefing: b=%+v err=%v", b, err)
	}
	if b.Summary != "" || section(b, "pending_drafts") == nil {
		t.Fatalf("plain card expected: %+v", b)
	}
	if sec := section(b, "custom"); sec == nil || sec.Items[0] != "Plan the day" {
		t.Fatalf("the reminder must survive as written: %+v", b.Sections)
	}
}
