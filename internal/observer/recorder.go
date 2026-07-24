// SPDX-License-Identifier: AGPL-3.0-or-later

package observer

import (
	"encoding/json"
	"memo/internal/logx"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// collectBuffer is the maximum number of buffered observations before the
// recorder starts dropping — prevents unbounded memory growth under heavy load.
const collectBuffer = 256

// Recorder is the integration hook used by the rest of the application. It
// wraps a Store and exposes intention-revealing methods (RecordMessage,
// RecordAgentRun, …) instead of raw Observation writes.
//
// Every method is safe to call on a nil *Recorder and writes happen on a
// single background goroutine, so recording never blocks or breaks the chat
// path even if the learning database is unavailable. If the buffer is full,
// the observation is dropped (with a warning) to avoid unbounded goroutine
// growth.
type Recorder struct {
	store     *Store
	recCh     chan Observation
	startOnce sync.Once
}

// NewRecorder wraps a store. Passing a nil store yields a no-op recorder.
func NewRecorder(store *Store) *Recorder {
	return &Recorder{
		store: store,
		recCh: make(chan Observation, collectBuffer),
	}
}

// enabled reports whether the recorder can actually write.
func (r *Recorder) enabled() bool {
	return r != nil && r.store != nil
}

// record enqueues a single observation to be written on the background worker.
func (r *Recorder) record(obs Observation) {
	if !r.enabled() {
		return
	}
	r.startOnce.Do(func() {
		logx.GoRecover("observer.Recorder.worker", r.worker)
	})
	select {
	case r.recCh <- obs:
	default:
		logx.Printf("OBSERVER: record buffer full, dropping (%s)", obs.ActivityType)
	}
}

// worker drains the observation channel and persists each one to the store.
// It runs for the lifetime of the Recorder.
func (r *Recorder) worker() {
	for obs := range r.recCh {
		// Recovered per-observation: a panic persisting one observation must
		// not permanently kill this loop for the rest of the process's
		// life — every future observation would silently stop being
		// recorded (same reasoning as internal/app's memorySaveWorker).
		func() {
			defer logx.Recover("observer.Recorder.worker/store.Record")
			if _, err := r.store.Record(obs); err != nil {
				logx.Printf("OBSERVER: record (%s): %v", obs.ActivityType, err)
			}
		}()
	}
}

// RecordMessage logs that the user sent a chat message. The message text is
// used only to derive a coarse topic and a word count — the text itself is not
// stored.
func (r *Recorder) RecordMessage(userMsg string) {
	if !r.enabled() {
		return
	}
	r.record(Observation{
		Timestamp:    time.Now(),
		ActivityType: ActivityChat,
		Topic:        ClassifyTopic(userMsg),
		WordCount:    wordCount(userMsg),
	})
}

// RecordAgentRun logs that the user triggered the agent pipeline.
func (r *Recorder) RecordAgentRun(userMsg string) {
	if !r.enabled() {
		return
	}
	r.record(Observation{
		Timestamp:    time.Now(),
		ActivityType: ActivityAgent,
		Topic:        ClassifyTopic(userMsg),
		WordCount:    wordCount(userMsg),
	})
}

// RecordOrchestraRun logs that the user triggered an orchestra workflow.
func (r *Recorder) RecordOrchestraRun(userMsg string) {
	if !r.enabled() {
		return
	}
	r.record(Observation{
		Timestamp:    time.Now(),
		ActivityType: ActivityOrchestra,
		Topic:        ClassifyTopic(userMsg),
		WordCount:    wordCount(userMsg),
	})
}

// IntentMeta holds the structured metadata stored with an intent observation.
// Using a plain struct (not importing intent package) avoids circular imports.
type IntentMeta struct {
	Summary     string `json:"summary"`
	Source      string `json:"source"`       // "chat" | "whatsapp"
	ContactName string `json:"contact_name"`
	IsHabit     bool   `json:"is_habit"`
	HabitTime   string `json:"habit_time,omitempty"` // "HH:MM"
}

// RecordIntent records a declared intent or habit. When it is a habit with a
// known time, TimeOfDaySeconds is set to that time so the pattern analyzer
// learns the declared schedule rather than the time the message was sent.
func (r *Recorder) RecordIntent(summary, source, contactName string, isHabit bool, habitTime *time.Time, msgTime time.Time) {
	if !r.enabled() {
		return
	}
	meta := IntentMeta{
		Summary:     summary,
		Source:      source,
		ContactName: contactName,
		IsHabit:     isHabit,
	}
	if habitTime != nil {
		meta.HabitTime = habitTime.Format("15:04")
	}
	metaJSON, _ := json.Marshal(meta)

	// Store.Record derives DayOfWeek and TimeOfDaySeconds from the timestamp, so
	// to make the pattern analyzer learn the *declared* schedule (e.g. "her gün
	// 21:00") rather than the time the message was sent, we encode the habit time
	// into the timestamp itself (keeping the message date).
	ts := msgTime
	if isHabit && habitTime != nil {
		ts = time.Date(msgTime.Year(), msgTime.Month(), msgTime.Day(),
			habitTime.Hour(), habitTime.Minute(), 0, 0, msgTime.Location())
	}

	r.record(Observation{
		Timestamp:    ts,
		ActivityType: ActivityIntent,
		Topic:        "general",
		Metadata:     string(metaJSON),
	})
}

// RecordWhatsAppMessage records a WhatsApp message observation (no intent).
func (r *Recorder) RecordWhatsAppMessage(text string, fromMe bool, msgTime time.Time) {
	if !r.enabled() {
		return
	}
	direction := "incoming"
	if fromMe {
		direction = "outgoing"
	}
	meta := map[string]string{"direction": direction}
	metaJSON, _ := json.Marshal(meta)

	r.record(Observation{
		Timestamp:    msgTime,
		ActivityType: ActivityWhatsApp,
		Topic:        ClassifyTopic(text),
		WordCount:    wordCount(text),
		Metadata:     string(metaJSON),
	})
}

// RecordSessionEnd logs the length of a completed chat session.
func (r *Recorder) RecordSessionEnd(lengthMin int) {
	if !r.enabled() {
		return
	}
	r.record(Observation{
		Timestamp:        time.Now(),
		ActivityType:     ActivitySession,
		SessionLengthMin: lengthMin,
	})
}

func wordCount(s string) int {
	return len(strings.Fields(s))
}

// topicKeywords maps a coarse topic label to trigger words (lower-case). The
// classifier is deliberately simple and local; later phases can replace it with
// something richer (e.g. the embedding model) without changing the schema.
//
// Deliberately excluded: bare Turkish word roots that are common, unrelated
// vocabulary in their own right and would otherwise fire on completely
// ordinary sentences — "git" (root of "gitmek", to go: "dün eve gittim"),
// "hata" (plain "mistake": "büyük bir hata yaptım"), "test" (generic,
// non-coding uses: "test ediyorum", "kan testi"), "nasıl" (root of the
// everyday greeting "nasılsın", how are you), "neden" (also just "reason"),
// "bul" (root of "bulmak", to find, used constantly outside any research
// context: "anahtarımı bulamadım"), "bug" (prefix of "bugün", today — one of
// the most common Turkish words there is: "bugün ne yaptın").
// See TestClassifyTopic_DoesNotMisfireOnCommonWords.
var topicKeywords = map[string][]string{
	"coding": {
		"kod", "code", "fonksiyon", "function", "derle", "compile",
		"refactor", "api", "fonksyon", "class", "struct", "deploy",
		"python", "golang", "javascript", "react", "sql", "docker",
	},
	"writing": {
		"yaz", "write", "metin", "makale", "article", "blog", "e-posta", "email",
		"mektup", "özet", "summary", "çevir", "translate", "düzelt", "edit",
	},
	"research": {
		"araştır", "research", "nedir", "what is", "açıkla", "explain",
		"how", "why", "kaynak", "source",
	},
	"planning": {
		"plan", "planla", "takvim", "calendar", "toplantı", "meeting", "görev",
		"task", "todo", "yapılacak", "schedule", "hatırlat", "remind",
	},
}

// ClassifyTopic returns a coarse topic label for a message, or "general" when
// nothing matches. Single-word keywords are matched as a whole-token prefix
// (the message is tokenized on Unicode letter/digit runs, so Turkish suffixes
// attach without breaking tokenization) rather than a raw substring scan —
// this still catches inflected forms ("yazıyorum" matches "yaz") but no
// longer matches a keyword root buried mid-word in an unrelated word (e.g.
// "beyaz", white, no longer matches "yaz"). Keyword phrases containing a
// space (e.g. "what is") are matched against the raw lowercased text, since
// tokenizing would break the phrase apart. Topics are scanned in a fixed
// (sorted) order so ties break deterministically.
func ClassifyTopic(text string) string {
	if strings.TrimSpace(text) == "" {
		return "general"
	}
	lower := strings.ToLower(text)
	tokens := strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	topics := make([]string, 0, len(topicKeywords))
	for topic := range topicKeywords {
		topics = append(topics, topic)
	}
	sort.Strings(topics)

	best := "general"
	bestHits := 0
	for _, topic := range topics {
		hits := 0
		for _, w := range topicKeywords[topic] {
			if strings.Contains(w, " ") {
				if strings.Contains(lower, w) {
					hits++
				}
				continue
			}
			for _, tok := range tokens {
				if strings.HasPrefix(tok, w) {
					hits++
					break
				}
			}
		}
		if hits > bestHits {
			bestHits = hits
			best = topic
		}
	}
	return best
}
