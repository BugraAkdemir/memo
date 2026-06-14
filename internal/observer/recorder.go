// SPDX-License-Identifier: AGPL-3.0-or-later

package observer

import (
	"log"
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
			log.Printf("OBSERVER: record (%s): %v", obs.ActivityType, err)
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
