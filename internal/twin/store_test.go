package twin

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestExplicitIdentityMergeAndRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, e := NewStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	at := time.Date(2026, 10, 31, 23, 50, 0, 0, time.FixedZone("TR", 10800))
	for i, m := range []Message{
		{Channel: "mail", Account: "inbox", EventID: "1", Address: "Ali <ali@example.com>", Body: "incoming must not set style", SentAt: at},
		{Channel: "whatsapp", Account: "device", EventID: "2", Address: "905551234567", Body: "selam kanka", Outgoing: true, SentAt: at.Add(time.Minute)},
		{Channel: "mail", Account: "inbox", EventID: "3", Address: "other@example.com", Body: "private other", SentAt: at},
	} {
		if e = s.RecordMessage(ctx, m, "Same Name"); e != nil {
			t.Fatalf("%d: %v", i, e)
		}
	}
	mailID, _ := s.Resolve(ctx, Alias{"mail", "ALI@example.com"})
	waID, _ := s.Resolve(ctx, Alias{"whatsapp", "905551234567"})
	if mailID == waID {
		t.Fatal("names silently linked identities")
	}
	if _, e = s.SaveContact(ctx, Contact{Name: "steal", Aliases: []Alias{{"mail", "ali@example.com"}}}); !errors.Is(e, ErrAliasConflict) {
		t.Fatal(e)
	}
	if e = s.Merge(ctx, mailID, waID); e != nil {
		t.Fatal(e)
	}
	list, e := s.Timeline(ctx, mailID, 50)
	if e != nil || len(list) != 2 {
		t.Fatalf("%+v %v", list, e)
	}
	if list[0].Body != "selam kanka" || !list[0].SentAt.Equal(at.Add(time.Minute)) {
		t.Fatal(list)
	}
	_, offset := list[0].SentAt.Zone()
	if offset != 10800 {
		t.Fatal("source timezone lost", list[0].SentAt)
	}
	p := InferStyle(list)
	if p.Samples != 1 || p.Tone != "informal" || p.Sufficient {
		t.Fatal(p)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e = NewStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	list, e = s.Timeline(ctx, mailID, 50)
	if e != nil || len(list) != 2 {
		t.Fatalf("restart: %v %v", list, e)
	}
	c, e := s.Contacts(ctx)
	if e != nil || len(c) != 2 {
		t.Fatalf("contacts %v %v", c, e)
	}
	if _, e = s.Timeline(ctx, waID, 50); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
}
func TestAtomicDedupAndCC(t *testing.T) {
	s, e := NewStore(filepath.Join(t.TempDir(), "nested"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e := s.RecordMessage(ctx, Message{Channel: "mail", Account: "a", EventID: "uid1", Address: "a@example.com", Body: "one", SentAt: time.Now()}, "")
			if e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if e = s.RecordMessage(ctx, Message{Channel: "mail", Account: "a", EventID: "uid1", Address: "cc@example.com", Body: "one", SentAt: time.Now()}, ""); e != nil {
		t.Fatal(e)
	}
	c, e := s.Contacts(ctx)
	if e != nil || len(c) != 2 {
		t.Fatalf("%v %v", c, e)
	}
	for _, v := range c {
		m, e := s.Timeline(ctx, v.ID, 100)
		if e != nil || len(m) != 1 {
			t.Fatalf("%v %v", m, e)
		}
	}
}
func TestValidationAndPolicyMerge(t *testing.T) {
	s, e := NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx := context.Background()
	for _, a := range []Alias{{"telegram", "@somebody"}, {"telegram", "-12"}, {"whatsapp", "123@g.us"}, {"mail", "a@b.com\nBcc: steal"}, {"shell", "x"}} {
		if _, e = NormalizeAlias(a); e == nil {
			t.Fatal(a)
		}
	}
	p, _ := s.Preferences(ctx)
	if p.AllowProjectInspection {
		t.Fatal("default must be off")
	}
	if e = s.SavePreferences(ctx, Preferences{Timezone: "Europe/Istanbul", WorkStart: "18:00", WorkEnd: "09:00"}); e == nil {
		t.Fatal("invalid hours")
	}
	p.Timezone = "Europe/Istanbul"
	p.AllowProjectInspection = true
	if e = s.SavePreferences(ctx, p); e != nil {
		t.Fatal(e)
	}
	id, e := s.SaveContact(ctx, Contact{Name: "A", AutonomyPolicy: "never", Aliases: []Alias{{"mail", "a@example.com"}}})
	if e != nil {
		t.Fatal(e)
	}
	second, e := s.SaveContact(ctx, Contact{Name: "B", Aliases: []Alias{{"mail", "b@example.com"}}})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Merge(ctx, second, id); e != nil {
		t.Fatal(e)
	}
	c, _ := s.Contacts(ctx)
	if len(c) != 1 || c[0].AutonomyPolicy != "never" {
		t.Fatal(c)
	}
}
func TestStyleUsesOnlyOutgoingAndStableGreeting(t *testing.T) {
	p := InferStyle([]Message{{Body: "sayın dear regards"}, {Body: "selam kanka", Outgoing: true}, {Body: "selam dostum", Outgoing: true}, {Body: "merhaba görüşürüz", Outgoing: true}})
	if p.Tone != "informal" || p.Language != "tr" || p.Greeting != "selam" || p.Samples != 3 || !p.Sufficient {
		t.Fatal(p)
	}
}

func TestFreshStoreDoesNotActivateToolTurns(t *testing.T) {
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if s.Active(ctx) {
		t.Fatal("empty twin activated tools")
	}
	p, err := s.Preferences(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.Active(ctx) {
		t.Fatal("reading defaults activated tools")
	}
	if err = s.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
	if !s.Active(ctx) {
		t.Fatal("explicit preferences did not activate tools")
	}
}

func TestMergedAliasesDoNotDoubleCountOneCCMessage(t *testing.T) {
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	at := time.Now()
	for _, address := range []string{"a@example.com", "a.work@example.com"} {
		if err = s.RecordMessage(ctx, Message{Channel: "mail", Account: "owner", EventID: "one-send", Address: address, Outgoing: true, Body: "selam", SentAt: at}, ""); err != nil {
			t.Fatal(err)
		}
	}
	first, _ := s.Resolve(ctx, Alias{"mail", "a@example.com"})
	second, _ := s.Resolve(ctx, Alias{"mail", "a.work@example.com"})
	if err = s.Merge(ctx, first, second); err != nil {
		t.Fatal(err)
	}
	messages, err := s.Timeline(ctx, first, 100)
	if err != nil || len(messages) != 1 {
		t.Fatal(messages, err)
	}
	contacts, err := s.Contacts(ctx)
	if err != nil || len(contacts) != 1 || contacts[0].Style.Samples != 1 {
		t.Fatal(contacts, err)
	}
}

func TestTimelineOrdersInstantsAcrossOffsetsWithoutChangingSourceDay(t *testing.T) {
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	source, err := time.Parse(time.RFC3339, "2026-11-01T00:30:00+03:00")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []Message{
		{Channel: "mail", Account: "a", EventID: "a", Address: "friend@example.com", Body: "tomorrow", SentAt: source},
		{Channel: "mail", Account: "a", EventID: "b", Address: "friend@example.com", Body: "later", SentAt: source.Add(time.Minute).UTC()},
	} {
		if err = s.RecordMessage(ctx, m, ""); err != nil {
			t.Fatal(err)
		}
	}
	id, _ := s.Resolve(ctx, Alias{"mail", "friend@example.com"})
	messages, err := s.Timeline(ctx, id, 50)
	if err != nil || len(messages) != 2 || messages[0].Body != "later" || messages[1].SentAt.Format(time.RFC3339) != "2026-11-01T00:30:00+03:00" {
		t.Fatal(messages, err)
	}
}

func TestAutonomyCandidateBudgetSurvivesReopenAndChannels(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ok, err := s.ReserveCandidate(ctx, "same-contact", "mail-thread", "a", now, true)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if ok, err = s.ReserveCandidate(ctx, "same-contact", "wa-thread", "b", now.Add(time.Minute), true); err != nil || ok {
		t.Fatal(ok, err)
	}
	for i := 1; i <= 2; i++ {
		if ok, err = s.ReserveCandidate(ctx, "same-contact", "another", fmt.Sprint(i), now.Add(time.Duration(i)*2*time.Hour), true); err != nil || !ok {
			t.Fatal(ok, err)
		}
	}
	if ok, err = s.ReserveCandidate(ctx, "same-contact", "another", "fourth", now.Add(8*time.Hour), true); err != nil || ok {
		t.Fatal(ok, err)
	}
}

func TestNewContactsInheritAndExplicitChoicesSurviveIngress(t *testing.T) {
	ctx := context.Background()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	m := Message{Channel: "mail", Account: "inbox", EventID: "new", Address: "new@example.com", Body: "Question", SentAt: time.Now()}
	if err := s.RecordMessage(ctx, m, "New"); err != nil {
		t.Fatal(err)
	}
	contacts, err := s.Contacts(ctx)
	if err != nil || len(contacts) != 1 || contacts[0].AutonomyPolicy != "default" {
		t.Fatalf("%+v %v", contacts, err)
	}
	c := contacts[0]
	c.AutonomyPolicy = "draft_only"
	if _, err := s.SaveContact(ctx, c); err != nil {
		t.Fatal(err)
	}
	m.EventID = "next"
	if err := s.RecordMessage(ctx, m, "New"); err != nil {
		t.Fatal(err)
	}
	contacts, err = s.Contacts(ctx)
	if err != nil || contacts[0].AutonomyPolicy != "draft_only" {
		t.Fatalf("override lost: %+v %v", contacts, err)
	}
}
