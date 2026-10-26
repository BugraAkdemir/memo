// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
)

func openState(t *testing.T) (*State, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "telemetry_state.json")
	s, err := OpenState(p)
	if err != nil {
		t.Fatal(err)
	}
	return s, p
}

func TestState_NewInstallHasARandomIDAndHasNotBeenAsked(t *testing.T) {
	s, p := openState(t)
	snap := s.Snapshot()
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(snap.TID) {
		t.Fatalf("tid %q is not 32 hex chars", snap.TID)
	}
	if snap.Consent != ConsentUnset {
		t.Fatalf("a new install must not have an answer yet, got %q", snap.Consent)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatalf("state file was not written: %v", err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("state file mode = %v, want 0600", st.Mode().Perm())
	}
	if entries, _ := os.ReadDir(filepath.Dir(p)); len(entries) != 1 {
		t.Errorf("the atomic write left a temp file behind: %v", entries)
	}
}

func TestState_SurvivesAReopen(t *testing.T) {
	s, p := openState(t)
	first := s.Snapshot().TID
	if err := s.SetConsent(ConsentYes, "2026-10-26"); err != nil {
		t.Fatal(err)
	}
	again, err := OpenState(p)
	if err != nil {
		t.Fatal(err)
	}
	snap := again.Snapshot()
	if snap.TID != first || snap.Consent != ConsentYes || snap.ConsentDay != "2026-10-26" {
		t.Fatalf("state lost on reopen: %+v", snap)
	}
}

func TestState_ResetIDGivesANewDeviceAndReturnsTheOldID(t *testing.T) {
	s, _ := openState(t)
	old := s.Snapshot().TID
	_ = s.SetConsent(ConsentYes, "2026-10-26")
	if err := s.AddFeature("2026-10-26", "memory"); err != nil {
		t.Fatal(err)
	}
	got, err := s.ResetID()
	if err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	if got != old {
		t.Errorf("ResetID returned %q, want the OLD id %q (the caller needs it to ask the server to forget it)", got, old)
	}
	if snap.TID == old {
		t.Error("the id did not change")
	}
	if snap.Consent != ConsentYes {
		t.Error("resetting the id must not change the person's answer")
	}
	if len(snap.Days) != 0 || snap.LastSentDay != "" {
		t.Errorf("a new id starts a new history: %+v", snap)
	}
}

func TestState_CountersAccumulatePerDayAndOldDaysAreDropped(t *testing.T) {
	s, p := openState(t)
	_ = s.AddFeature("2026-10-26", "skills")
	_ = s.AddFeature("2026-10-26", "skills") // a feature counts once a day
	_ = s.AddFeature("2026-10-26", "memory")
	_ = s.AddProviderErr("2026-10-26", "openai", "429")
	_ = s.AddProviderErr("2026-10-26", "openai", "429")
	_ = s.AddStartFail("2026-10-26", "llama", "permission")
	_ = s.AddExit("2026-10-26", "llama", "oom_killed")
	_ = s.AddUpdate("2026-10-26", "failed", "checksum")
	_ = s.AddCrash("2026-10-26", CrashAgg{Sig: "abc", Component: "go", ExcType: "runtime.boundsError", Frames: []string{"memo/x.f x.go:1"}})
	_ = s.AddCrash("2026-10-26", CrashAgg{Sig: "abc", Component: "go", ExcType: "runtime.boundsError", Frames: []string{"memo/x.f x.go:1"}})

	again, err := OpenState(p)
	if err != nil {
		t.Fatal(err)
	}
	d := again.Snapshot().Days["2026-10-26"]
	if d == nil {
		t.Fatal("the day is gone after a reopen")
	}
	if len(d.Features) != 2 || d.ProviderErr["openai|429"] != 2 || d.StartFail["llama|permission"] != 1 ||
		d.Exit["llama|oom_killed"] != 1 || d.Update["failed|checksum"] != 1 || d.Crash["abc"].N != 2 {
		t.Fatalf("counters wrong: %+v", d)
	}

	// ten days on, the oldest day is pruned
	_ = again.AddFeature("2026-11-10", "voice")
	if _, ok := again.Snapshot().Days["2026-10-26"]; ok {
		t.Error("a day older than the backlog window was kept")
	}
}

func TestState_MilestonesKeepTheEarliestDate(t *testing.T) {
	s, _ := openState(t)
	_ = s.SetMilestone("provider_added", "2026-10-22")
	_ = s.SetMilestone("provider_added", "2026-10-25") // later: ignored
	_ = s.SetMilestone("provider_added", "2026-10-20") // earlier: wins
	_ = s.SetMilestone("first_message", "2026-10-21")
	m := s.Snapshot().Milestones
	if m.ProviderAdded != "2026-10-20" || m.FirstMessage != "2026-10-21" {
		t.Fatalf("milestones = %+v", m)
	}
}

func TestState_ACorruptFileStartsFreshAndIsKept(t *testing.T) {
	p := filepath.Join(t.TempDir(), "telemetry_state.json")
	if err := os.WriteFile(p, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := OpenState(p)
	if err != nil {
		t.Fatalf("a corrupt state file must not stop Memo: %v", err)
	}
	if s.Snapshot().Consent != ConsentUnset || s.Snapshot().TID == "" {
		t.Fatalf("expected a fresh state, got %+v", s.Snapshot())
	}
	if m, _ := filepath.Glob(p + ".corrupt*"); len(m) != 1 {
		t.Errorf("the unreadable file should be kept for inspection, found %v", m)
	}
}

func TestState_ConcurrentWritersDoNotLoseOrCorruptData(t *testing.T) {
	s, p := openState(t)
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.AddProviderErr("2026-10-26", "openai", "429")
		}()
	}
	wg.Wait()
	again, err := OpenState(p)
	if err != nil {
		t.Fatal(err)
	}
	if n := again.Snapshot().Days["2026-10-26"].ProviderErr["openai|429"]; n != 40 {
		t.Fatalf("counted %d, want 40", n)
	}
}
