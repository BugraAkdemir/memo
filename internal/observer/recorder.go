// SPDX-License-Identifier: AGPL-3.0-or-later

package observer

import (
	"encoding/json"
	"memo/internal/logx"
	"sort"
	"strings"
	"time"
)

// Recorder is the integration hook used by the rest of the application. It
// wraps a Store and exposes intention-revealing methods (RecordMessage,
// RecordAgentRun, …) instead of raw Observation writes.
//
// Every method is safe to call on a nil *Recorder and writes happen on a
// background goroutine, so recording never blocks or breaks the chat path even
// if the learning database is unavailable.
type Recorder struct {
	store *Store
}

// NewRecorder wraps a store. Passing a nil store yields a no-op recorder.
func NewRecorder(store *Store) *Recorder {
	return &Recorder{store: store}
}

// enabled reports whether the recorder can actually write.
func (r *Recorder) enabled() bool {
	return r != nil && r.store != nil
}

// record fires off a single observation in the background.
func (r *Recorder) record(obs Observation) {
	if !r.enabled() {
		return
	}
	go func() {
		if _, err := r.store.Record(obs); err != nil {
			logx.Printf("OBSERVER: record (%s): %v", obs.ActivityType, err)
		}
	}()
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
var topicKeywords = map[string][]string{
	"coding": {
		"kod", "code", "fonksiyon", "function", "bug", "hata", "derle", "compile",
		"refactor", "git", "api", "fonksyon", "class", "struct", "deploy", "test",
		"python", "golang", "javascript", "react", "sql", "docker",
	},
	"writing": {
		"yaz", "write", "metin", "makale", "article", "blog", "e-posta", "email",
		"mektup", "özet", "summary", "çevir", "translate", "düzelt", "edit",
	},
	"research": {
		"araştır", "research", "nedir", "what is", "açıkla", "explain", "nasıl",
		"how", "neden", "why", "bul", "find", "kaynak", "source",
	},
	"planning": {
		"plan", "planla", "takvim", "calendar", "toplantı", "meeting", "görev",
		"task", "todo", "yapılacak", "schedule", "hatırlat", "remind",
	},
}

// ClassifyTopic returns a coarse topic label for a message, or "general" when
// nothing matches. The match is a simple case-insensitive keyword scan. Topics
// are scanned in a fixed (sorted) order so ties break deterministically.
func ClassifyTopic(text string) string {
	if strings.TrimSpace(text) == "" {
		return "general"
	}
	lower := strings.ToLower(text)

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
			if strings.Contains(lower, w) {
				hits++
			}
		}
		if hits > bestHits {
			bestHits = hits
			best = topic
		}
	}
	return best
}
