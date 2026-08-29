package taskloop

import (
	"errors"
	"testing"
)

func TestClassifyProviderErr(t *testing.T) {
	cases := []struct {
		msg  string
		want providerFailKind
	}{
		{"provider rate limited", failRateLimited},
		{"status 429: Too Many Requests", failRateLimited},
		{"Error 429 (Resource has been exhausted (e.g. check quota))", failRateLimited},
		{"the model is overloaded, please try again", failRateLimited},
		{"authentication error (status 401): invalid x-api-key", failAuth},
		{"status 403: permission denied", failAuth},
		{"invalid API key provided", failAuth},
		{"model not found: gpt-9", failAuth},
		{"status 503: service unavailable", failTransient},
		{"context deadline exceeded", failTransient},
		{"dial tcp: connection refused", failTransient},
		{"some unrelated parsing failure", failOther},
		{"", failOther},
		// Agent-internal failures must NOT be treated as provider problems —
		// otherwise self-heal loops on a tool-permission timeout.
		{"işçi hatası: ⚠️ Agent execution cancelled (permission timeout)", failOther},
		{"AGENT [write_file] ERROR: Permission wait cancelled", failOther},
		{"tool call rejected by user", failOther},
		{"reached iteration limit (40)", failOther},
	}
	for _, c := range cases {
		var err error
		if c.msg != "" {
			err = errors.New(c.msg)
		}
		if got := classifyProviderErr(err); got != c.want {
			t.Errorf("classifyProviderErr(%q) = %d, want %d", c.msg, got, c.want)
		}
	}
}

func TestErrHelpers(t *testing.T) {
	if !IsRateLimitErr(errors.New("HTTP 429 quota exceeded")) {
		t.Error("IsRateLimitErr missed a 429")
	}
	if !IsAuthErr(errors.New("401 unauthorized")) {
		t.Error("IsAuthErr missed a 401")
	}
	if !IsTransientErr(errors.New("502 bad gateway")) {
		t.Error("IsTransientErr missed a 502")
	}
	if IsRateLimitErr(errors.New("401 unauthorized")) {
		t.Error("IsRateLimitErr false positive on a 401")
	}
}

func TestRetryAfterHint(t *testing.T) {
	cases := map[string]int{
		"Rate limited. Please try again in 34 seconds.": 34,
		"retry after 120s":                              120,
		"Retry-After: 5":                                5,
		"no hint here":                                  0,
		"":                                              0,
	}
	for msg, want := range cases {
		var err error
		if msg != "" {
			err = errors.New(msg)
		}
		if got := retryAfterHint(err); got != want {
			t.Errorf("retryAfterHint(%q) = %d, want %d", msg, got, want)
		}
	}
}
