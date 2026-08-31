package taskloop

import "strings"

// providerFailKind is a best-effort classification of a worker-turn error.
// Only the OpenAI-family providers classify HTTP status centrally; Claude and
// Gemini surface plain "status NNN" strings, so this leans on substrings and
// is deliberately generous. It informs the loop's reaction: wait vs. switch
// provider vs. give up.
type providerFailKind int

const (
	failOther       providerFailKind = iota // unknown / not provider-related
	failRateLimited                         // 429 / quota — wait, do not switch
	failAuth                                // 401 / 403 / bad key — switch provider
	failTransient                           // 5xx / timeout — retry, then switch
)

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// classifyProviderErr buckets a worker error. err == nil returns failOther.
func classifyProviderErr(err error) providerFailKind {
	if err == nil {
		return failOther
	}
	s := strings.ToLower(err.Error())

	// Agent-internal failures (a tool permission wait timing out, an aborted
	// run, a tool rejection) are NOT provider problems — never self-heal or
	// wait-limit on them. Checked first because "permission timeout" / "cancelled"
	// would otherwise be misread as an auth or transient provider error.
	if containsAny(s, "permission wait cancelled", "permission timeout",
		"execution cancelled", "execution aborted", "agent execution cancelled",
		"tool call rejected", "denied by user", "user declined", "iteration limit") {
		return failOther
	}

	switch {
	// Permanent config faults — a bad model id, an unsupported model, a
	// malformed request. Retrying on a timer never fixes these, so bucket them
	// with auth: the provider-locked loop parks in waiting-user for the user to
	// fix. Checked before the transient bucket because some carry a "502"/"400"
	// status string that would otherwise read as transient.
	case containsAny(s, "not supported", "unsupported", "invalid_request_error",
		"invalid request error", `must be "type/model-id"`, "model must be",
		"is not supported when", "400 bad request", "status 400", `"status":400`,
		" 400,", "unknown model", "unrecognized model"):
		return failAuth
	case containsAny(s, "429", "rate limit", "rate-limit", "ratelimit", "quota",
		"too many requests", "please slow down", "resource exhausted",
		"resource_exhausted", "overloaded", "capacity"):
		return failRateLimited
	case containsAny(s, " 401", "401 ", "403", "unauthorized", "invalid api key",
		"invalid_api_key", "invalid x-api-key", "authentication",
		"api key not valid", "api key not configured", "no api key",
		"permission denied", "permission_denied",
		"expired", "no such model", "model not found"):
		return failAuth
	case containsAny(s, "500", "502", "503", "504", "timed out", "i/o timeout",
		"context deadline exceeded", "deadline exceeded", "connection refused",
		"connection reset", "unexpected eof", "temporarily unavailable",
		"service unavailable", "internal server error", "bad gateway", "eof"):
		return failTransient
	default:
		return failOther
	}
}

// IsRateLimitErr reports whether err looks like a provider rate-limit / quota
// error. Exported for the retry scheduler and host code.
func IsRateLimitErr(err error) bool {
	return classifyProviderErr(err) == failRateLimited
}

// IsAuthErr reports whether err looks like an authentication / bad-key /
// missing-model failure — the loop's cue to switch provider.
func IsAuthErr(err error) bool {
	return classifyProviderErr(err) == failAuth
}

// IsTransientErr reports whether err looks like a transient 5xx / timeout —
// worth a couple of retries before switching provider.
func IsTransientErr(err error) bool {
	return classifyProviderErr(err) == failTransient
}

// RetryAfterSeconds exposes any provider-supplied "try again in Ns" hint (0 if
// none). Used by the retry scheduler's notification text.
func RetryAfterSeconds(err error) int {
	return retryAfterHint(err)
}

// retryAfterHint tries to pull a "try again in Ns" / "retry after N seconds"
// delay out of a provider error string. Returns 0 when none is present.
func retryAfterHint(err error) int {
	if err == nil {
		return 0
	}
	s := strings.ToLower(err.Error())
	for _, marker := range []string{"try again in ", "retry after ", "retry-after: ", "retry_after: "} {
		i := strings.Index(s, marker)
		if i < 0 {
			continue
		}
		rest := s[i+len(marker):]
		num := 0
		seen := false
		for _, r := range rest {
			if r >= '0' && r <= '9' {
				num = num*10 + int(r-'0')
				seen = true
				continue
			}
			break
		}
		if seen {
			return num
		}
	}
	return 0
}
