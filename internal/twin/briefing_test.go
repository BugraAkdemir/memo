package twin

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func newBriefingStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestBriefingSettingsDefaultOffAndAreValidated(t *testing.T) {
	ctx := context.Background()
	s := newBriefingStore(t)
	if p, _ := s.Preferences(ctx); p.Briefing.Morning || p.Briefing.PreMeeting {
		t.Fatalf("briefings must default to off: %+v", p.Briefing)
	}
	base := Preferences{Timezone: "UTC", WorkStart: "09:00", WorkEnd: "18:00"}
	base.Briefing = BriefingSettings{Morning: true, PreMeeting: true}
	if err := s.SavePreferences(ctx, base); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Preferences(ctx)
	if got.Briefing.MorningTime != "09:00" || got.Briefing.PreMeetingMinutes != 15 {
		t.Fatalf("defaults not applied: %+v", got.Briefing)
	}
	for name, mutate := range map[string]func(*BriefingSettings){
		"bad time":        func(b *BriefingSettings) { b.MorningTime = "9am" },
		"time past 24h":   func(b *BriefingSettings) { b.MorningTime = "25:00" },
		"lead too short":  func(b *BriefingSettings) { b.PreMeetingMinutes = 2 },
		"lead too long":   func(b *BriefingSettings) { b.PreMeetingMinutes = 500 },
		"negative lead":   func(b *BriefingSettings) { b.PreMeetingMinutes = -5 },
		"seconds in time": func(b *BriefingSettings) { b.MorningTime = "09:00:30" },
	} {
		p := base
		p.Briefing = BriefingSettings{Morning: true, PreMeeting: true, MorningTime: "08:30", PreMeetingMinutes: 20}
		mutate(&p.Briefing)
		if err := s.SavePreferences(ctx, p); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s accepted: %v", name, err)
		}
	}
}

func TestBriefingIsStoredOncePerKeyAndListedNewestFirst(t *testing.T) {
	ctx := context.Background()
	s := newBriefingStore(t)
	mk := func(key string, at time.Time) Briefing {
		return Briefing{Key: key, Kind: "morning", Title: key, CreatedAt: at, Sections: []BriefingSection{{Heading: "h", Items: []string{"a"}}}}
	}
	now := time.Now()
	created, err := s.SaveBriefing(ctx, mk("morning:2026-11-03", now))
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if created, _ = s.SaveBriefing(ctx, mk("morning:2026-11-03", now.Add(time.Hour))); created {
		t.Fatal("the same briefing key was stored twice")
	}
	if _, err = s.SaveBriefing(ctx, mk("meeting:abc", now.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}
	list, err := s.Briefings(ctx, 10)
	if err != nil || len(list) != 2 || list[0].Key != "meeting:abc" || list[1].Key != "morning:2026-11-03" || list[0].Read {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	if list[1].Sections[0].Items[0] != "a" {
		t.Fatalf("sections lost: %+v", list[1])
	}
	if ok, err := s.HasBriefing(ctx, "meeting:abc"); err != nil || !ok {
		t.Fatalf("HasBriefing=%v err=%v", ok, err)
	}
	if ok, _ := s.HasBriefing(ctx, "meeting:none"); ok {
		t.Fatal("unknown key reported as present")
	}
	if err = s.MarkBriefingRead(ctx, "meeting:abc"); err != nil {
		t.Fatal(err)
	}
	if list, _ = s.Briefings(ctx, 10); !list[0].Read || list[1].Read {
		t.Fatalf("read flag wrong: %+v", list)
	}
	if err = s.MarkBriefingRead(ctx, "meeting:none"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown key: %v", err)
	}
}

func TestBriefingsKeepOnlyTheMostRecent(t *testing.T) {
	ctx := context.Background()
	s := newBriefingStore(t)
	base := time.Now().Add(-100 * time.Hour)
	for i := 0; i < briefingKeep+10; i++ {
		b := Briefing{Key: fmt.Sprintf("meeting:%03d", i), Kind: "meeting", Title: "t", CreatedAt: base.Add(time.Duration(i) * time.Minute)}
		if _, err := s.SaveBriefing(ctx, b); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.Briefings(ctx, 1000)
	if err != nil || len(list) != briefingKeep {
		t.Fatalf("kept %d, want %d (err=%v)", len(list), briefingKeep, err)
	}
	if list[0].Key != fmt.Sprintf("meeting:%03d", briefingKeep+9) {
		t.Fatalf("newest was dropped: %s", list[0].Key)
	}
	if ok, _ := s.HasBriefing(ctx, "meeting:000"); ok {
		t.Fatal("oldest briefing was not evicted")
	}
}

func TestBriefingTextIsBoundedBeforeStorage(t *testing.T) {
	ctx := context.Background()
	s := newBriefingStore(t)
	huge := make([]string, 200)
	for i := range huge {
		huge[i] = string(make([]rune, 2000))
	}
	b := Briefing{Key: "morning:x", Kind: "morning", Title: string(make([]rune, 2000)), CreatedAt: time.Now(), Sections: []BriefingSection{{Heading: "h", Items: huge}}}
	if _, err := s.SaveBriefing(ctx, b); err != nil {
		t.Fatal(err)
	}
	list, _ := s.Briefings(ctx, 1)
	if len([]rune(list[0].Title)) > 200 || len(list[0].Sections[0].Items) > briefingMaxItems {
		t.Fatalf("unbounded: title=%d items=%d", len([]rune(list[0].Title)), len(list[0].Sections[0].Items))
	}
	for _, it := range list[0].Sections[0].Items {
		if len([]rune(it)) > 400 {
			t.Fatalf("item not truncated: %d", len([]rune(it)))
		}
	}
	if _, err := s.SaveBriefing(ctx, Briefing{Kind: "morning", Title: "x"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a briefing without a key was accepted: %v", err)
	}
	if _, err := s.SaveBriefing(ctx, Briefing{Key: "k", Kind: "weird", Title: "x"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("an unknown kind was accepted: %v", err)
	}
}

func briefingPrefs(b BriefingSettings) Preferences {
	return Preferences{Timezone: "UTC", WorkStart: "09:00", WorkEnd: "18:00", Briefing: b}
}

func TestBriefingDeliverySettingsRoundTripAndKeepTheOwnersOrder(t *testing.T) {
	ctx := context.Background()
	s := newBriefingStore(t)
	in := BriefingSettings{
		Morning:     true,
		Sections:    []string{"drafts", "meetings", "drafts"},
		Channels:    []string{"telegram", "mail", "telegram"},
		MailAccount: " acc-1 ",
		Model:       BriefingModel{Provider: " p ", Model: " m "},
		Custom:      []BriefingCustom{{Title: " Water ", Prompt: " Remind me to drink "}, {}},
	}
	if err := s.SavePreferences(ctx, briefingPrefs(in)); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Preferences(ctx)
	b := got.Briefing
	if fmt.Sprint(b.Sections) != "[drafts meetings]" || fmt.Sprint(b.Channels) != "[telegram mail]" {
		t.Fatalf("lists not cleaned: %+v", b)
	}
	if b.MailAccount != "acc-1" || b.Model != (BriefingModel{"p", "m"}) {
		t.Fatalf("not trimmed: %+v", b)
	}
	if len(b.Custom) != 1 || b.Custom[0] != (BriefingCustom{"Water", "Remind me to drink"}) {
		t.Fatalf("custom items: %+v", b.Custom)
	}
}

func TestBriefingSectionsMissingMeansAllAndEmptyMeansNone(t *testing.T) {
	ctx := context.Background()
	s := newBriefingStore(t)
	if err := s.SavePreferences(ctx, briefingPrefs(BriefingSettings{Morning: true})); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Preferences(ctx)
	for _, id := range BriefingSectionIDs {
		if !got.Briefing.SectionEnabled(id) {
			t.Errorf("a briefing saved before sections existed must keep %s", id)
		}
	}
	if err := s.SavePreferences(ctx, briefingPrefs(BriefingSettings{Morning: true, Sections: []string{}})); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Preferences(ctx)
	for _, id := range BriefingSectionIDs {
		if got.Briefing.SectionEnabled(id) {
			t.Errorf("an explicit empty list must switch %s off", id)
		}
	}
}

func TestBriefingDeliverySettingsAreValidated(t *testing.T) {
	ctx := context.Background()
	s := newBriefingStore(t)
	for name, mutate := range map[string]func(*BriefingSettings){
		"unknown section": func(b *BriefingSettings) { b.Sections = []string{"weather"} },
		"unknown channel": func(b *BriefingSettings) { b.Channels = []string{"sms"} },
		"six custom items": func(b *BriefingSettings) {
			b.Custom = make([]BriefingCustom, 6)
			for i := range b.Custom {
				b.Custom[i] = BriefingCustom{"t", "p"}
			}
		},
		"title only":       func(b *BriefingSettings) { b.Custom = []BriefingCustom{{Title: "t"}} },
		"prompt only":      func(b *BriefingSettings) { b.Custom = []BriefingCustom{{Prompt: "p"}} },
		"title too long":   func(b *BriefingSettings) { b.Custom = []BriefingCustom{{Title: fmt.Sprintf("%061d", 0), Prompt: "p"}} },
		"provider only":    func(b *BriefingSettings) { b.Model = BriefingModel{Provider: "p"} },
		"model only":       func(b *BriefingSettings) { b.Model = BriefingModel{Model: "m"} },
		"account too long": func(b *BriefingSettings) { b.MailAccount = fmt.Sprintf("%0201d", 0) },
	} {
		b := BriefingSettings{Morning: true}
		mutate(&b)
		if err := s.SavePreferences(ctx, briefingPrefs(b)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s accepted: %v", name, err)
		}
	}
}
