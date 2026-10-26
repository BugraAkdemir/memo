// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"memo/internal/stats"
)

type recorded struct {
	Path, Method, ContentType, UserAgent string
	Header                                http.Header
	Body                                  []byte
}

// fakeServer stands in for tel.memocpp.com: it records every request and answers
// with whatever respond says (default: 202 for a report, 204 for a forget).
type fakeServer struct {
	*httptest.Server
	mu      sync.Mutex
	reqs    []recorded
	respond func(r recorded, n int) int
}

func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	f := &fakeServer{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		rec := recorded{Path: r.URL.Path, Method: r.Method, ContentType: r.Header.Get("Content-Type"), UserAgent: r.Header.Get("User-Agent"), Header: r.Header.Clone(), Body: body}
		f.reqs = append(f.reqs, rec)
		n := len(f.reqs)
		respond := f.respond
		f.mu.Unlock()
		status := http.StatusAccepted
		if rec.Path == "/v1/forget" {
			status = http.StatusNoContent
		}
		if respond != nil {
			status = respond(rec, n)
		}
		w.WriteHeader(status)
		if status == http.StatusAccepted {
			_, _ = w.Write([]byte(`{"status":"accepted"}`))
		}
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeServer) requests(path string) []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []recorded
	for _, r := range f.reqs {
		if r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

var now = time.Date(2026, 10, 27, 3, 0, 0, 0, time.UTC) // "today"; yesterday = 2026-10-26

func dayAt(offset int, h int) time.Time { // offset 1 = yesterday
	return time.Date(2026, 10, 27, h, 0, 0, 0, time.UTC).AddDate(0, 0, -offset)
}
func dayStr(offset int) string { return dayAt(offset, 0).Format("2006-01-02") }

func usedOn(offset int) stats.Event {
	return stats.Event{Timestamp: dayAt(offset, 10), Provider: "local", Model: "Qwen3-8B-Q4_K_M.gguf", Category: "chat", PromptTokens: 100, CompletionTokens: 40, TokensPerSecond: 50, TTFTMs: 600}
}

type fixture struct {
	m   *Manager
	srv *fakeServer
	src *fakeSources
}

func newFixture(t *testing.T, build string) *fixture {
	t.Helper()
	srv := newFakeServer(t)
	src := baseSources()
	m, err := New(Config{
		StatePath: filepath.Join(t.TempDir(), "telemetry_state.json"), Endpoint: srv.URL, Sources: src,
		Info: Info{Version: "5.0.0", Build: build, OS: "linux", Arch: "amd64"},
		Now:  func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{m: m, srv: srv, src: src}
}

// agreeOn records a "yes" given offset days ago.
func (f *fixture) agreeOn(t *testing.T, offset int) {
	t.Helper()
	f.m.now = func() time.Time { return dayAt(offset, 12) }
	if err := f.m.SetConsent(true); err != nil {
		t.Fatal(err)
	}
	f.m.now = func() time.Time { return now }
}

func TestTick_SendsNothingUntilThePersonHasAgreed(t *testing.T) {
	f := newFixture(t, BuildOfficial)
	f.src.events = []stats.Event{usedOn(1), usedOn(2)}
	if st := f.m.Status(); !st.NeedsAsk || st.Enabled || st.Consent != ConsentUnset {
		t.Fatalf("a new official install must ask and send nothing: %+v", st)
	}
	_ = f.m.Tick(context.Background())
	if n := len(f.srv.reqs); n != 0 {
		t.Fatalf("%d requests went out before the person answered", n)
	}
	// declining keeps it that way, and stops asking
	_ = f.m.SetConsent(false)
	_ = f.m.Tick(context.Background())
	if n := len(f.srv.reqs); n != 0 || f.m.Status().NeedsAsk {
		t.Fatalf("declined: requests=%d needsAsk=%v", n, f.m.Status().NeedsAsk)
	}
}

func TestTick_ABuildFromSourceNeverAsksAndNeverSendsByDefault(t *testing.T) {
	f := newFixture(t, BuildSource)
	f.src.events = []stats.Event{usedOn(1)}
	if st := f.m.Status(); st.NeedsAsk || st.Enabled {
		t.Fatalf("source build: %+v", st)
	}
	_ = f.m.Tick(context.Background())
	if len(f.srv.reqs) != 0 {
		t.Fatal("a source build sent something nobody asked for")
	}
	// ...but the person can turn it on, and then it is labelled as a source build.
	f.agreeOn(t, 3)
	_ = f.m.Tick(context.Background())
	reqs := f.srv.requests("/v1/report")
	if len(reqs) != 1 || !strings.Contains(string(reqs[0].Body), `"build":"source"`) {
		t.Fatalf("expected one report labelled build=source, got %d: %s", len(reqs), reqs)
	}
}

func TestTick_SendsYesterdayOnceAsJSONWithNoExtraHeaders(t *testing.T) {
	f := newFixture(t, BuildOfficial)
	f.src.events = []stats.Event{usedOn(1)}
	f.agreeOn(t, 5)
	if err := f.m.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	reqs := f.srv.requests("/v1/report")
	if len(reqs) != 1 {
		t.Fatalf("want 1 report, got %d", len(reqs))
	}
	r := reqs[0]
	if r.Method != "POST" || r.ContentType != "application/json" || r.UserAgent != "memo" {
		t.Errorf("request = %+v", r)
	}
	for k := range r.Header {
		switch k {
		case "Content-Type", "User-Agent", "Content-Length", "Accept-Encoding":
		default:
			t.Errorf("unexpected header %q — nothing besides the body should identify the install", k)
		}
	}
	var doc any
	if err := json.Unmarshal(r.Body, &doc); err != nil {
		t.Fatal(err)
	}
	s := loadSchema(t)
	if errs := validate(s, s, doc, "report"); len(errs) > 0 {
		t.Fatalf("sent a report the server would refuse:\n  %s", strings.Join(errs, "\n  "))
	}
	if doc.(map[string]any)["day"] != dayStr(1) {
		t.Errorf("day = %v, want %s", doc.(map[string]any)["day"], dayStr(1))
	}
	_ = f.m.Tick(context.Background())
	if n := len(f.srv.requests("/v1/report")); n != 1 {
		t.Fatalf("the same day was sent again (%d requests)", n)
	}
	if f.m.Status().LastSentDay != dayStr(1) {
		t.Errorf("LastSentDay = %q", f.m.Status().LastSentDay)
	}
}

func TestTick_NeverSendsADayFromBeforeTheAnswer(t *testing.T) {
	f := newFixture(t, BuildOfficial)
	f.src.events = []stats.Event{usedOn(1), usedOn(2), usedOn(6)}
	f.agreeOn(t, 2) // said yes two days ago: day 2 (the day of the answer) and older are off limits
	_ = f.m.Tick(context.Background())
	reqs := f.srv.requests("/v1/report")
	if len(reqs) != 1 || !strings.Contains(string(reqs[0].Body), `"day":"`+dayStr(1)+`"`) {
		t.Fatalf("only yesterday may go; got %d reports", len(reqs))
	}
}

func TestTick_CatchesUpOldestFirstAndSkipsIdleDaysWithoutARequest(t *testing.T) {
	f := newFixture(t, BuildOfficial)
	f.src.events = []stats.Event{usedOn(4), usedOn(2), usedOn(1)} // day 3 was idle
	f.agreeOn(t, 6)
	_ = f.m.Tick(context.Background())
	reqs := f.srv.requests("/v1/report")
	if len(reqs) != 3 {
		t.Fatalf("want 3 reports (days 4, 2, 1), got %d", len(reqs))
	}
	var days []string
	for _, r := range reqs {
		var d struct{ Day string }
		_ = json.Unmarshal(r.Body, &d)
		days = append(days, d.Day)
	}
	if strings.Join(days, ",") != strings.Join([]string{dayStr(4), dayStr(2), dayStr(1)}, ",") {
		t.Fatalf("order = %v", days)
	}
}

func TestTick_ServerTroubleKeepsTheDayForLater(t *testing.T) {
	f := newFixture(t, BuildOfficial)
	f.src.events = []stats.Event{usedOn(2), usedOn(1)}
	f.agreeOn(t, 4)
	f.srv.respond = func(recorded, int) int { return http.StatusServiceUnavailable }
	_ = f.m.Tick(context.Background())
	if n := len(f.srv.requests("/v1/report")); n != 1 {
		t.Fatalf("after a 503 the cycle must stop (1 request), got %d", n)
	}
	// the idle day before the failing one is rightly skipped; the failed day itself must not count as sent
	if got := f.m.Status().LastSentDay; got >= dayStr(2) {
		t.Fatalf("a failed day must not count as sent: LastSentDay = %q", got)
	}
	f.srv.respond = nil
	_ = f.m.Tick(context.Background())
	if n := len(f.srv.requests("/v1/report")); n != 1+2 {
		t.Fatalf("the next cycle should send both days, total requests = %d", n)
	}
}

func TestTick_ARefusedReportIsDroppedNotRetried(t *testing.T) {
	f := newFixture(t, BuildOfficial)
	f.src.events = []stats.Event{usedOn(2), usedOn(1)}
	f.agreeOn(t, 4)
	f.srv.respond = func(r recorded, n int) int {
		if n == 1 {
			return http.StatusBadRequest // the oldest day is refused
		}
		return http.StatusAccepted
	}
	_ = f.m.Tick(context.Background())
	reqs := f.srv.requests("/v1/report")
	if len(reqs) != 2 {
		t.Fatalf("a 400 drops that day and goes on: want 2 requests, got %d", len(reqs))
	}
	_ = f.m.Tick(context.Background())
	if n := len(f.srv.requests("/v1/report")); n != 2 {
		t.Fatalf("the refused bytes were sent again (%d requests)", n)
	}
}

func TestPreview_IsExactlyTheBytesThatAreSent(t *testing.T) {
	f := newFixture(t, BuildOfficial)
	f.src.events = []stats.Event{usedOn(1)}
	f.agreeOn(t, 5)
	p, err := f.m.Preview(context.Background())
	if err != nil || p.Empty {
		t.Fatalf("preview: %+v %v", p, err)
	}
	_ = f.m.Tick(context.Background())
	sent := f.srv.requests("/v1/report")
	if len(sent) != 1 || string(sent[0].Body) != string(p.JSON) {
		t.Fatalf("the preview is not what was sent:\npreview: %s\nsent:    %s", p.JSON, sent[0].Body)
	}
}

func TestPreview_ShowsWhatWouldGoEvenBeforeTheAnswerAndSendsNothing(t *testing.T) {
	f := newFixture(t, BuildOfficial)
	f.src.events = []stats.Event{usedOn(1)}
	p, err := f.m.Preview(context.Background())
	if err != nil || p.Empty || !strings.Contains(string(p.JSON), `"day":"`+dayStr(1)+`"`) {
		t.Fatalf("a person deciding must be able to see the report first: %+v %v", p, err)
	}
	if len(f.srv.reqs) != 0 {
		t.Fatal("previewing must never touch the network")
	}
}

func TestPreview_AQuietDayIsSaidSo(t *testing.T) {
	f := newFixture(t, BuildOfficial)
	p, err := f.m.Preview(context.Background())
	if err != nil || !p.Empty {
		t.Fatalf("want an empty preview: %+v %v", p, err)
	}
}

func TestEnvironmentSwitchWinsOverAnAnswer(t *testing.T) {
	f := newFixture(t, BuildOfficial)
	f.src.events = []stats.Event{usedOn(1)}
	f.agreeOn(t, 5)
	t.Setenv("MEMO_TELEMETRY", "off")
	st := f.m.Status()
	if !st.EnvOff || st.Enabled || st.NeedsAsk {
		t.Fatalf("MEMO_TELEMETRY=off must disable everything: %+v", st)
	}
	_ = f.m.Tick(context.Background())
	if len(f.srv.reqs) != 0 {
		t.Fatal("sent something although the environment switch is off")
	}
	f.m.NoteFeature("memo") // counters are not even kept
	if len(f.m.state.Snapshot().Days) != 0 {
		t.Fatal("counters were recorded while disabled")
	}
}

func TestCountersAreOnlyKeptWhileSharingIsOn(t *testing.T) {
	f := newFixture(t, BuildOfficial)
	f.m.NoteFeature("skills")
	f.m.NoteProviderErr("openai", "429")
	if len(f.m.state.Snapshot().Days) != 0 {
		t.Fatal("counters were recorded before the person agreed")
	}
	f.agreeOn(t, 3)
	f.m.NoteFeature("skills")
	if len(f.m.state.Snapshot().Days) != 1 {
		t.Fatal("counters must be recorded once sharing is on")
	}
}

func TestResetID_ForgetsTheOldIDFirstAndRetriesWhenOffline(t *testing.T) {
	f := newFixture(t, BuildOfficial)
	f.agreeOn(t, 3)
	old := f.m.state.Snapshot().TID

	f.srv.respond = func(r recorded, n int) int { return http.StatusServiceUnavailable } // offline-ish
	if err := f.m.ResetID(context.Background()); err != nil {
		t.Fatal(err)
	}
	cur := f.m.state.Snapshot()
	if cur.TID == old {
		t.Fatal("the id did not change")
	}
	if len(cur.PendingForget) != 1 || cur.PendingForget[0] != old {
		t.Fatalf("the old id must be remembered for a retry: %+v", cur.PendingForget)
	}
	f.srv.respond = nil
	_ = f.m.Tick(context.Background())
	forgets := f.srv.requests("/v1/forget")
	last := forgets[len(forgets)-1]
	if !strings.Contains(string(last.Body), old) {
		t.Fatalf("the retry did not carry the old id: %s", last.Body)
	}
	if len(f.m.state.Snapshot().PendingForget) != 0 {
		t.Fatal("the pending id was not cleared after the server forgot it")
	}
}

func TestDeleteData_ForgetsTheDeviceAndStopsSharing(t *testing.T) {
	f := newFixture(t, BuildOfficial)
	f.agreeOn(t, 3)
	old := f.m.state.Snapshot().TID
	if err := f.m.DeleteData(context.Background()); err != nil {
		t.Fatal(err)
	}
	forgets := f.srv.requests("/v1/forget")
	if len(forgets) != 1 || !strings.Contains(string(forgets[0].Body), old) {
		t.Fatalf("expected a forget of %s, got %v", old, forgets)
	}
	st := f.m.Status()
	if st.Enabled || st.Consent != ConsentNo || f.m.state.Snapshot().TID == old {
		t.Fatalf("after deleting: %+v", st)
	}
}

func TestForgetRequestIsJustTheID(t *testing.T) {
	f := newFixture(t, BuildOfficial)
	f.agreeOn(t, 3)
	_ = f.m.DeleteData(context.Background())
	var body map[string]string
	_ = json.Unmarshal(f.srv.requests("/v1/forget")[0].Body, &body)
	if len(body) != 1 || len(body["tid"]) != 32 {
		t.Fatalf("forget body = %v", body)
	}
}

func TestEncode_AnOversizeReportIsTrimmedNotRefused(t *testing.T) {
	r := fullReport()
	for i := 0; i < 190; i++ {
		r.Usage = append(r.Usage, UsageRow{Kind: "api", Provider: "openai", Model: "gpt-" + strings.Repeat("x", 30) + string(rune('a'+i%26)), Category: "chat", Turns: 1, InTokens: int64(i + 1), OutTokens: 1})
	}
	raw, err := encode(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > maxBody {
		t.Fatalf("%d bytes is over the server's limit of %d", len(raw), maxBody)
	}
	if !strings.Contains(string(raw), `"tid"`) {
		t.Fatal("trimming must keep the report valid")
	}
}
