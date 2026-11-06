package autonomy

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"memo/internal/twin"
)

// Found live (4 Nov): a contact with a long history made every question that
// needs contact_timeline fail as "reply_provider_failed", because the raw 50
// message timeline pushed the model input past the 7800-rune cap. Long incoming
// mails hit the same cap. Both must be trimmed to fit, not rejected.
func TestEngineLongTimelineEvidenceAndLongMailStillGetAReply(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"short question, long history", "What did we talk about last time?"},
		{"very long mail", "What did we talk about? " + strings.Repeat("Quoted older thread text. ", 1200)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, in := engineFixture(t)
			ctx := context.Background()
			in.Body = tc.body
			for i := 0; i < 60; i++ {
				m := twin.Message{Channel: "mail", Account: "account", EventID: fmt.Sprint("old", i), Address: in.Sender, Outgoing: i%2 == 0, Body: fmt.Sprintf("message %d ", i) + strings.Repeat("padding ", 150), SentAt: in.At.Add(-time.Duration(i+1) * time.Hour)}
				if err := e.Store.RecordMessage(ctx, m, "Friend"); err != nil {
					t.Fatal(err)
				}
			}
			maxInput := 0
			e.Generate = func(_ context.Context, system, user string) (string, error) {
				if n := len([]rune(user)); n > maxInput {
					maxInput = n
				}
				if system == analyzePrompt {
					return `{"kind":"question","tools":[{"name":"contact_timeline","args":{}}]}`, nil
				}
				if !strings.Contains(user, "message 0 ") {
					t.Error("the newest history must survive trimming")
				}
				return `{"body":"We last discussed this.","factual":true,"evidence_tools":["contact_timeline"]}`, nil
			}
			e.SaveDraft = func(context.Context, twin.Action) error { return nil }
			a, err := e.HandleIncomingEvent(ctx, in)
			if err != nil {
				t.Fatal(err)
			}
			if a == nil || a.State != "pending" || a.Reason != "drafted" || a.Body == "" {
				t.Fatalf("oversized input must still be answered, got %+v", a)
			}
			if a.ScoreStatus == "unavailable" {
				t.Errorf("trimmed evidence must stay small enough to be scored: %+v", a)
			}
			if maxInput > 7800 {
				t.Errorf("a model input exceeded the cap: %d runes", maxInput)
			}
		})
	}
}
