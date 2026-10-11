package agent

import (
	"context"
	"errors"
	"memo/internal/provider"
	"strings"
	"testing"
)

func TestCompletionReviewBoundedFallback(t *testing.T) {
	for _, tc := range []struct {
		name           string
		acceptAt       int
		fail           bool
		reviews, calls int
	}{
		{"accept", 1, false, 1, 1}, {"repair", 2, false, 2, 2}, {"three rejections", 0, false, 3, 4}, {"three errors", 0, true, 3, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prov := &scriptedProvider{responses: []provider.ChatResponse{{Content: "draft 1"}, {Content: "draft 2"}, {Content: "draft 3"}, {Content: "self checked"}}}
			p := NewPipeline(NewRegistry(), NewPermissionManager(t.TempDir()), NewSandbox(DefaultSandboxConfig(t.TempDir())), prov, nil)
			p.maxIters = 1 // Review retries reserve bounded turns, even at the loop ceiling.
			reviews := 0
			hasFeedback := false
			ctx := WithCompletionReview(context.Background(), func(_ context.Context, messages []provider.Message, draft string) (CompletionAssessment, error) {
				reviews++
				if reviews > 1 {
					hasFeedback = strings.Contains(messages[len(messages)-1].Content.(string), "Review feedback")
				}
				if tc.fail {
					return CompletionAssessment{}, errors.New("bad verifier")
				}
				return CompletionAssessment{Accepted: reviews == tc.acceptAt, Feedback: "Button width did not change; inspect the code and run layout tests."}, nil
			})
			ch, err := p.RunStream(ctx, []provider.Message{{Role: "user", Content: "Make the button bigger"}}, "main", func(AgentEvent) {}, nil)
			if err != nil {
				t.Fatal(err)
			}
			output := ""
			done := false
			for chunk := range ch {
				output += chunk.Content
				if chunk.Done {
					done = true
				}
				if chunk.Error != "" {
					t.Fatal(chunk.Error)
				}
			}
			if reviews != tc.reviews || prov.calls != tc.calls || !done {
				t.Fatalf("reviews=%d calls=%d done=%t", reviews, prov.calls, done)
			}
			if tc.calls == 4 && output != "self checked" {
				t.Fatalf("draft leaked: %q", output)
			}
			if tc.calls > 1 && !hasFeedback {
				t.Fatal("repair lost the review feedback")
			}
		})
	}
}
