package models

import (
	"context"
	"time"
)

type quotaWaitKey struct{}

// WithQuotaWait asks a model-list call to wait up to d for fresh quota figures
// instead of answering from its cache — for the model picker's periodic
// refresh, which runs in the background where a second of waiting is invisible.
// Without it the list answers from what is cached and refreshes behind the
// scenes, so opening the picker never waits on a vendor.
func WithQuotaWait(ctx context.Context, d time.Duration) context.Context {
	return context.WithValue(ctx, quotaWaitKey{}, d)
}

// QuotaWaitFromContext is the wait WithQuotaWait set; zero when none.
func QuotaWaitFromContext(ctx context.Context) time.Duration {
	d, _ := ctx.Value(quotaWaitKey{}).(time.Duration)
	return d
}
