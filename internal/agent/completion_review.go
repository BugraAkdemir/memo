package agent

import (
	"context"
	"memo/internal/provider"
)

// CompletionReview evaluates a proposed final answer and the actual tool
// history. Feedback continues the same permission-checked agent loop.
type CompletionReview func(context.Context, []provider.Message, string) (CompletionAssessment, error)
type CompletionAssessment struct {
	Accepted bool
	Feedback string
	Summary  string
}
type completionReviewKey struct{}

func WithCompletionReview(ctx context.Context, review CompletionReview) context.Context {
	return context.WithValue(ctx, completionReviewKey{}, review)
}
func completionReviewFromContext(ctx context.Context) CompletionReview {
	review, _ := ctx.Value(completionReviewKey{}).(CompletionReview)
	return review
}
