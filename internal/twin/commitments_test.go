package twin

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func radarStore(t *testing.T, enabled bool, hours int) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	p := Preferences{Timezone: "UTC", WorkStart: "09:00", WorkEnd: "18:00"}
	p.Radar = RadarSettings{Enabled: enabled, FollowUpHours: hours}
	if err := s.SavePreferences(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	return s
}

func sentMail(event, to, body string, at time.Time) Message {
	return Message{Channel: "mail", Account: "acct", EventID: event, Address: to, Outgoing: true, Body: body, SentAt: at}
}

func TestOutgoingQuestionStartsWaitingCommitment(t *testing.T) {
	ctx := context.Background()
	s := radarStore(t, true, 48)
	sent := time.Now().Add(-time.Hour)
	m := sentMail("e1", "ali@example.com", "Fiyat teklifini inceledin mi?", sent)
	m.Subject, m.InReplyTo = "Teklif", "<m1@example.com>"
	if err := s.RecordMessage(ctx, m, "Ali"); err != nil {
		t.Fatal(err)
	}
	open, err := s.Commitments(ctx)
	if err != nil || len(open) != 1 {
		t.Fatalf("open=%v err=%v", open, err)
	}
	c := open[0]
	if c.Kind != "waiting_on_other" || c.Status != "open" || c.Channel != "mail" || c.Address != "ali@example.com" || c.Subject != "Teklif" || c.InReplyTo != "<m1@example.com>" || c.ContactID == "" {
		t.Fatalf("unexpected commitment: %+v", c)
	}
	if want := sent.Add(48 * time.Hour); c.DueAt.Sub(want) > time.Second || want.Sub(c.DueAt) > time.Second {
		t.Fatalf("due=%v want %v", c.DueAt, want)
	}
	// The same source event never creates a second record.
	if err := s.RecordMessage(ctx, m, "Ali"); err != nil {
		t.Fatal(err)
	}
	if open, _ = s.Commitments(ctx); len(open) != 1 {
		t.Fatalf("duplicate event produced %d commitments", len(open))
	}
}

func TestRadarIgnoresMessagesThatExpectNoReply(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	cases := []struct {
		name  string
		on    bool
		msg   Message
		group bool
	}{
		{"radar disabled", false, sentMail("a", "x@example.com", "Müsait misin?", now), false},
		{"no question", true, sentMail("b", "x@example.com", "Teşekkürler, görüşürüz.", now), false},
		{"group or cc", true, sentMail("c", "x@example.com", "Müsait misin?", now), true},
		{"incoming", true, Message{Channel: "mail", Account: "acct", EventID: "d", Address: "x@example.com", Body: "Müsait misin?", SentAt: now}, false},
		{"already overdue", true, sentMail("e", "x@example.com", "Müsait misin?", now.Add(-72*time.Hour)), false},
		{"a reminder must not breed another", true, func() Message { m := sentMail("g", "x@example.com", "Müsait misin?", now); m.NoTrack = true; return m }(), false},
		{"telegram has no delivery path", true, Message{Channel: "telegram", Account: "acct", EventID: "f", Address: "42", Outgoing: true, Body: "Müsait misin?", SentAt: now}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := radarStore(t, tc.on, 48)
			tc.msg.Group = tc.group
			if err := s.RecordMessage(ctx, tc.msg, "X"); err != nil {
				t.Fatal(err)
			}
			if open, err := s.Commitments(ctx); err != nil || len(open) != 0 {
				t.Fatalf("created %v err=%v", open, err)
			}
		})
	}
}

func TestQuestionMarksOfOtherScriptsCountAsQuestions(t *testing.T) {
	for _, body := range []string{"Are you free?", "ご都合いかがですか？", "هل أنت متاح؟", "Είσαι διαθέσιμος;"} {
		if !expectsReply(body) {
			t.Errorf("%q should expect a reply", body)
		}
	}
	for _, body := range []string{"", "Thanks", "See you tomorrow."} {
		if expectsReply(body) {
			t.Errorf("%q should not expect a reply", body)
		}
	}
}

// A reply on ANY channel of the same (merged) contact closes the commitment,
// but only a reply that came after the message it answers.
func TestReplyOnAnotherChannelResolvesCommitment(t *testing.T) {
	ctx := context.Background()
	s := radarStore(t, true, 48)
	sent := time.Now().Add(-2 * time.Hour)
	if err := s.RecordMessage(ctx, sentMail("e1", "ali@example.com", "Teklif uygun mu?", sent), "Ali"); err != nil {
		t.Fatal(err)
	}
	open, _ := s.Commitments(ctx)
	if len(open) != 1 {
		t.Fatal(open)
	}
	// The same person reaches us on WhatsApp; the owner links the identities.
	wa := Message{Channel: "whatsapp", Account: "wa", EventID: "w0", Address: "+905551112233", Body: "selam", SentAt: sent.Add(-time.Hour)}
	if err := s.RecordMessage(ctx, wa, "Ali WA"); err != nil {
		t.Fatal(err)
	}
	waContact, err := s.Resolve(ctx, Alias{Channel: "whatsapp", Address: "+905551112233"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Merge(ctx, open[0].ContactID, waContact); err != nil {
		t.Fatal(err)
	}
	// A message older than the question (backfill) must not close it.
	old := Message{Channel: "whatsapp", Account: "wa", EventID: "w0b", Address: "+905551112233", Body: "eski", SentAt: sent.Add(-time.Minute)}
	if err = s.RecordMessage(ctx, old, ""); err != nil {
		t.Fatal(err)
	}
	if open, _ = s.Commitments(ctx); len(open) != 1 {
		t.Fatal("a message older than the question closed it")
	}
	reply := Message{Channel: "whatsapp", Account: "wa", EventID: "w1", Address: "+905551112233", Body: "Uygun, yarın konuşalım", SentAt: time.Now()}
	if err = s.RecordMessage(ctx, reply, ""); err != nil {
		t.Fatal(err)
	}
	if open, _ = s.Commitments(ctx); len(open) != 0 {
		t.Fatalf("still open after a reply: %+v", open)
	}
	done, err := s.CommitmentHistory(ctx, "resolved")
	if err != nil || len(done) != 1 || done[0].ResolvedAt == nil {
		t.Fatalf("resolved=%+v err=%v", done, err)
	}
}

func TestReplyFromAnotherContactDoesNotResolve(t *testing.T) {
	ctx := context.Background()
	s := radarStore(t, true, 48)
	if err := s.RecordMessage(ctx, sentMail("e1", "ali@example.com", "Uygun mu?", time.Now().Add(-time.Hour)), "Ali"); err != nil {
		t.Fatal(err)
	}
	other := Message{Channel: "mail", Account: "acct", EventID: "i1", Address: "veli@example.com", Body: "merhaba", SentAt: time.Now()}
	if err := s.RecordMessage(ctx, other, "Veli"); err != nil {
		t.Fatal(err)
	}
	if open, _ := s.Commitments(ctx); len(open) != 1 {
		t.Fatalf("unrelated sender resolved it: %v", open)
	}
}

func TestDueCommitmentsAndSingleClaim(t *testing.T) {
	ctx := context.Background()
	s := radarStore(t, true, 1)
	sent := time.Now().Add(-30 * time.Minute)
	if err := s.RecordMessage(ctx, sentMail("e1", "ali@example.com", "Uygun mu?", sent), "Ali"); err != nil {
		t.Fatal(err)
	}
	if due, err := s.DueCommitments(ctx, time.Now(), 10); err != nil || len(due) != 0 {
		t.Fatalf("not due yet but listed: %v %v", due, err)
	}
	later := time.Now().Add(45 * time.Minute)
	due, err := s.DueCommitments(ctx, later, 10)
	if err != nil || len(due) != 1 {
		t.Fatalf("due=%v err=%v", due, err)
	}
	if ok, err := s.SetCommitmentStatus(ctx, due[0].ID, "open", "reminded"); err != nil || !ok {
		t.Fatalf("first transition ok=%v err=%v", ok, err)
	}
	if ok, _ := s.SetCommitmentStatus(ctx, due[0].ID, "open", "reminded"); ok {
		t.Fatal("a second worker also won the transition")
	}
	if due, _ = s.DueCommitments(ctx, later, 10); len(due) != 0 {
		t.Fatalf("reminded commitment is still due: %v", due)
	}
}

func TestRadarSettingsValidationAndDefaults(t *testing.T) {
	ctx := context.Background()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got, _ := s.Preferences(ctx); got.Radar.Enabled {
		t.Fatal("radar must default to off")
	}
	p := Preferences{Timezone: "UTC", WorkStart: "09:00", WorkEnd: "18:00", Radar: RadarSettings{Enabled: true}}
	if err = s.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Preferences(ctx); got.Radar.FollowUpHours != 48 {
		t.Fatalf("default hours=%d, want 48", got.Radar.FollowUpHours)
	}
	for _, h := range []int{-1, 721} {
		p.Radar.FollowUpHours = h
		if err = s.SavePreferences(ctx, p); !errors.Is(err, ErrInvalid) {
			t.Fatalf("hours %d accepted: %v", h, err)
		}
	}
}

// The table shipped with six columns and was never written; opening a store
// created back then must gain the radar columns and keep any legacy row.
func TestCommitmentsMigrationKeepsLegacyRows(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = s.db.Write(ctx, func(tx *sql.Tx) error {
		for _, c := range []string{"channel", "account", "address", "subject", "in_reply_to", "source_ref", "created_at", "resolved_at"} {
			if _, e := tx.ExecContext(ctx, "ALTER TABLE commitments DROP COLUMN "+c); e != nil {
				return e
			}
		}
		_, e := tx.ExecContext(ctx, "INSERT INTO commitments(id,contact_id,kind,body,due_at,status) VALUES('legacy','c1','waiting_on_other','eski','2026-01-01T00:00:00.000000000Z','open')")
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewStore(dir)
	if err != nil {
		t.Fatalf("reopen after legacy schema: %v", err)
	}
	defer s.Close()
	open, err := s.Commitments(ctx)
	if err != nil || len(open) != 1 || open[0].ID != "legacy" || open[0].Body != "eski" {
		t.Fatalf("legacy row lost: %+v err=%v", open, err)
	}
	// And a second reopen is a no-op.
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if s, err = NewStore(dir); err != nil {
		t.Fatal(err)
	}
}

// ---- my_promise: what the owner promised someone else ----

func promiseStore(t *testing.T) (*Store, string) {
	t.Helper()
	ctx := context.Background()
	s := radarStore(t, true, 48)
	p, _ := s.Preferences(ctx)
	p.Radar.TrackPromises = true
	if err := s.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
	// The contact exists because the owner wrote to them.
	if err := s.RecordMessage(ctx, sentMail("m1", "ali@example.com", "Raporu yarın gönderirim.", time.Now()), "Ali"); err != nil {
		t.Fatal(err)
	}
	id, err := s.Resolve(ctx, Alias{Channel: "mail", Address: "ali@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	return s, id
}

func TestPromiseIsStoredListedAndDueOnTime(t *testing.T) {
	ctx := context.Background()
	s, contact := promiseStore(t)
	due := time.Now().Add(24 * time.Hour)
	ok, err := s.AddPromise(ctx, Promise{Channel: "mail", Account: "acct", Address: "ali@example.com", Summary: "Raporu gönder", SourceRef: "m1", DueAt: due})
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	// The same promise from the same message is never stored twice.
	if ok, _ = s.AddPromise(ctx, Promise{Channel: "mail", Account: "acct", Address: "ali@example.com", Summary: "Raporu gönder", SourceRef: "m1", DueAt: due}); ok {
		t.Fatal("duplicate promise stored")
	}
	all, err := s.Commitments(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var promise *Commitment
	for i := range all {
		if all[i].Kind == "my_promise" {
			promise = &all[i]
		}
	}
	if promise == nil || promise.Body != "Raporu gönder" || promise.ContactID != contact || promise.Status != "open" {
		t.Fatalf("promise missing or wrong: %+v", all)
	}
	if got, _ := s.DuePromises(ctx, time.Now(), 10); len(got) != 0 {
		t.Fatalf("not due yet: %+v", got)
	}
	got, err := s.DuePromises(ctx, due.Add(time.Minute), 10)
	if err != nil || len(got) != 1 || got[0].ID != promise.ID {
		t.Fatalf("due=%+v err=%v", got, err)
	}
	if ok, _ := s.SetCommitmentStatus(ctx, promise.ID, "open", "reminded"); !ok {
		t.Fatal("could not mark reminded")
	}
	if got, _ = s.DuePromises(ctx, due.Add(time.Hour), 10); len(got) != 0 {
		t.Fatalf("a reminded promise is due again: %+v", got)
	}
}

// A reply from the contact answers a question; it says nothing about whether
// the owner kept a promise, so it must never close one.
func TestContactReplyDoesNotCloseAPromise(t *testing.T) {
	ctx := context.Background()
	s, _ := promiseStore(t)
	if _, err := s.AddPromise(ctx, Promise{Channel: "mail", Account: "acct", Address: "ali@example.com", Summary: "Raporu gönder", SourceRef: "m1", DueAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	reply := Message{Channel: "mail", Account: "acct", EventID: "in1", Address: "ali@example.com", Body: "Tamam, bekliyorum", SentAt: time.Now().Add(time.Minute)}
	if err := s.RecordMessage(ctx, reply, ""); err != nil {
		t.Fatal(err)
	}
	all, _ := s.Commitments(ctx)
	found := false
	for _, c := range all {
		if c.Kind == "my_promise" {
			found = true
		}
	}
	if !found {
		t.Fatal("an incoming message closed the owner's promise")
	}
}

func TestPromiseValidation(t *testing.T) {
	ctx := context.Background()
	s, _ := promiseStore(t)
	good := Promise{Channel: "mail", Account: "acct", Address: "ali@example.com", Summary: "x", SourceRef: "m1", DueAt: time.Now().Add(time.Hour)}
	for name, mutate := range map[string]func(*Promise){
		"no summary":      func(p *Promise) { p.Summary = " " },
		"no due date":     func(p *Promise) { p.DueAt = time.Time{} },
		"already past":    func(p *Promise) { p.DueAt = time.Now().Add(-time.Hour) },
		"over a year":     func(p *Promise) { p.DueAt = time.Now().Add(400 * 24 * time.Hour) },
		"bad channel":     func(p *Promise) { p.Channel = "telegram" },
		"unknown contact": func(p *Promise) { p.Address = "nobody@example.com" },
	} {
		p := good
		mutate(&p)
		if ok, err := s.AddPromise(ctx, p); ok || err == nil {
			t.Errorf("%s accepted (ok=%v err=%v)", name, ok, err)
		}
	}
}

func TestTrackPromisesIsOffByDefaultAndSaved(t *testing.T) {
	ctx := context.Background()
	s := radarStore(t, true, 48)
	if p, _ := s.Preferences(ctx); p.Radar.TrackPromises {
		t.Fatal("promise tracking must default to off")
	}
	p, _ := s.Preferences(ctx)
	p.Radar.TrackPromises = true
	if err := s.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Preferences(ctx); !got.Radar.TrackPromises {
		t.Fatal("not saved")
	}
}
