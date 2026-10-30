package autonomy

import (
	"net/mail"
	"strings"
	"sync"
	"time"
)

type Envelope struct {
	Account, Sender, ThreadID string
	Headers                   map[string]string
	RecipientCount            int
	IsGroup                   bool
}

func address(raw string) string {
	if a, err := mail.ParseAddress(raw); err == nil {
		return strings.ToLower(a.Address)
	}
	return strings.ToLower(strings.TrimSpace(raw))
}

// AutoReplyPolicy restricts automatic sends only. It must never throttle a
// person's explicit Send button; group/CC messages require a draft and approval.
func AutoReplyPolicy(e Envelope) Verdict {
	if e.IsGroup || e.RecipientCount > 1 {
		return blocked("group_or_cc", "rules")
	}
	headers := map[string]string{}
	for k, v := range e.Headers {
		headers[strings.ToLower(k)] = strings.ToLower(strings.TrimSpace(v))
	}
	if v := headers["auto-submitted"]; v != "" && v != "no" {
		return blocked("automated_sender", "rules")
	}
	for _, k := range []string{"x-auto-response-suppress", "list-id", "list-unsubscribe", "x-loop"} {
		if headers[k] != "" {
			return blocked("automated_sender", "rules")
		}
	}
	switch headers["precedence"] {
	case "bulk", "list", "junk":
		return blocked("automated_sender", "rules")
	}
	sender := address(e.Sender)
	local, _, _ := strings.Cut(sender, "@")
	for _, p := range []string{"noreply", "no-reply", "mailer-daemon", "postmaster", "notifications"} {
		if local == p || strings.HasPrefix(local, p+"+") || strings.HasPrefix(local, p+".") {
			return blocked("automated_sender", "rules")
		}
	}
	if !strings.Contains(sender, "@") {
		return blocked("review_required", "rules")
	}
	return Verdict{Code: "clear", Check: "rules"}
}

// LoopBreaker atomically reserves an auto-reply attempt against BOTH contact
// and thread limits. Reservations count even if delivery later fails: retries
// must not turn an uncertain SMTP result into an automatic message storm.
// This process-local guard is supplemented by durable dedup in the next step.
type LoopBreaker struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func (l *LoopBreaker) ReserveAutoReply(e Envelope, now time.Time) Verdict {
	if v := AutoReplyPolicy(e); v.Blocked {
		return v
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.attempts == nil {
		l.attempts = map[string][]time.Time{}
	}
	for k, times := range l.attempts {
		live := times[:0]
		for _, t := range times {
			if t.After(now.Add(-24 * time.Hour)) {
				live = append(live, t)
			}
		}
		if len(live) == 0 {
			delete(l.attempts, k)
		} else {
			l.attempts[k] = live
		}
	}
	keys := []string{e.Account + "\x00sender\x00" + address(e.Sender)}
	if e.ThreadID != "" {
		keys = append(keys, e.Account+"\x00thread\x00"+e.ThreadID)
	}
	if len(l.attempts) > 4096 {
		return blocked("rate_limit", "rules")
	}
	for _, key := range keys {
		hour := 0
		for _, t := range l.attempts[key] {
			if t.After(now.Add(-time.Hour)) {
				hour++
			}
		}
		if hour >= 1 || len(l.attempts[key]) >= 3 {
			return blocked("rate_limit", "rules")
		}
	}
	for _, key := range keys {
		l.attempts[key] = append(l.attempts[key], now)
	}
	return Verdict{Code: "clear", Check: "rules"}
}
