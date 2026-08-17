// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"memo/internal/api"
	"memo/internal/logx"
	"memo/internal/memory"
	moodpkg "memo/internal/mood"
)

// defaultInsightWindowDays is how far back GenerateSelfInsight looks by
// default (both the on-demand API call and the routine.ContextInsight path)
// — long enough for a pattern to surface above routine chit-chat noise,
// short enough that the digest still reads as "recent".
const defaultInsightWindowDays = 30

// maxInsightMemories caps how many raw memory rows get dumped into the
// synthesis prompt — same reasoning as identity.BuildSystemPrompt's own
// truncation (KNOWN_ISSUES.md M18): an unbounded dump on a long window risks
// blowing the model's context budget.
const maxInsightMemories = 60

// GenerateSelfInsight synthesizes a short "what did you notice about
// yourself" reflection from the user's own conversation memory + mood trend
// over the last windowDays days (<=0 uses defaultInsightWindowDays). Shared
// by the on-demand path (POST /api/memory/insight, handlers_flutter.go) and
// the proactive routine.ContextInsight path (internal/app/routine.go) —
// both just pick windowDays/lang, this is the actual content generation.
func (a *App) GenerateSelfInsight(ctx context.Context, windowDays int, lang string) (string, error) {
	if windowDays <= 0 {
		windowDays = defaultInsightWindowDays
	}
	since := time.Now().AddDate(0, 0, -windowDays)

	var memories []memory.MemoryResult
	if a.store != nil {
		var err error
		memories, err = a.store.RecentSince(ctx, since, maxInsightMemories)
		if err != nil {
			logx.Printf("insight: RecentSince: %v", err)
		}
	}

	var moodHistory []moodpkg.HistoryPoint
	if a.mood != nil && a.mood.Enabled() {
		var err error
		moodHistory, err = a.mood.HistorySince(ctx, since)
		if err != nil {
			logx.Printf("insight: HistorySince: %v", err)
		}
	}

	if len(memories) == 0 && len(moodHistory) == 0 {
		if routineLanguageIsEnglish(lang) {
			return "Not enough conversation history yet in this window to notice a pattern — check back after chatting a bit more.", nil
		}
		return "Bu zaman aralığında henüz bir kalıp fark edebilecek kadar sohbet geçmişi yok — biraz daha konuştuktan sonra tekrar dene.", nil
	}

	msgs := []api.Message{
		api.NewTextMessage("system", insightSystemPrompt(lang, windowDays)),
		api.NewTextMessage("user", formatInsightContext(memories, moodHistory, lang)),
	}
	reply := a.callLLMCategorized(ctx, msgs, categoryInsight)
	if isLLMErrorReply(reply) {
		return "", fmt.Errorf("insight: llm call failed: %s", reply)
	}
	return reply, nil
}

func insightSystemPrompt(lang string, windowDays int) string {
	if routineLanguageIsEnglish(lang) {
		return fmt.Sprintf(
			"You are looking back over the user's own conversation history and mood trend from the last %d days, provided below. "+
				"Notice one or two genuine patterns about the user themselves — not a summary of topics discussed, an actual observation "+
				"about them (a recurring concern, a shift in mood, something they said repeatedly without seeming to notice it). "+
				"Write it as a short, warm, second-person reflection (2-4 sentences), like a friend who's been paying attention. "+
				"If nothing genuine stands out, say so plainly instead of inventing a pattern.", windowDays)
	}
	return fmt.Sprintf(
		"Aşağıda kullanıcının son %d günlük sohbet geçmişi ve duygu durumu trendi var. "+
			"Kullanıcının kendisiyle ilgili gerçek bir ya da iki kalıba dikkat çek — konuşulan konuların özeti değil, kullanıcının kendisi hakkında "+
			"gerçek bir gözlem (tekrar eden bir kaygı, duygu durumunda bir değişim, fark etmeden sürekli söylediği bir şey). "+
			"Kısa, sıcak, ikinci tekil şahısla (\"sen\") yazılmış bir yansıma olsun (2-4 cümle), dikkatli bir arkadaş gibi. "+
			"Gerçekten dikkat çekici bir şey yoksa, uydurmak yerine bunu açıkça söyle.", windowDays)
}

func formatInsightContext(memories []memory.MemoryResult, moodHistory []moodpkg.HistoryPoint, lang string) string {
	var b strings.Builder
	if routineLanguageIsEnglish(lang) {
		b.WriteString("Conversation excerpts:\n")
	} else {
		b.WriteString("Sohbet kesitleri:\n")
	}
	if len(memories) == 0 {
		b.WriteString("(none)\n")
	}
	for _, m := range memories {
		b.WriteString("- ")
		b.WriteString(m.Content)
		b.WriteString("\n")
	}

	if len(moodHistory) > 0 {
		if routineLanguageIsEnglish(lang) {
			b.WriteString("\nMood trend (score range -10..10, most negative to most positive):\n")
		} else {
			b.WriteString("\nDuygu durumu trendi (skor aralığı -10..10, en olumsuzdan en olumluya):\n")
		}
		b.WriteString(summarizeMoodTrend(moodHistory))
	}
	return b.String()
}

// summarizeMoodTrend condenses a potentially long history slice into a few
// representative points (first, lowest, highest, last — deduplicated) rather
// than dumping every sample: the LLM needs the shape of the trend, not every
// data point.
func summarizeMoodTrend(history []moodpkg.HistoryPoint) string {
	if len(history) == 0 {
		return ""
	}
	firstIdx, lastIdx := 0, len(history)-1
	lowIdx, highIdx := 0, 0
	for i, p := range history {
		if p.Score < history[lowIdx].Score {
			lowIdx = i
		}
		if p.Score > history[highIdx].Score {
			highIdx = i
		}
	}

	label := func(p moodpkg.HistoryPoint) string {
		return fmt.Sprintf("- %s: %.1f (%s)\n", p.RecordedAt.Format("2006-01-02"), p.Score, moodpkg.Label(p.Score))
	}

	var b strings.Builder
	seen := make(map[int]bool, 4)
	for _, idx := range []int{firstIdx, lowIdx, highIdx, lastIdx} {
		if seen[idx] {
			continue
		}
		seen[idx] = true
		b.WriteString(label(history[idx]))
	}
	return b.String()
}
