package mood

import "fmt"

// moodDirectives score etiketine göre İngilizce davranış direktifleri.
// NEUTRAL boş string — gereksiz token harcanmaz, Memo normal davranır.
var moodDirectives = map[MoodLabel]string{
	LabelFurious:   "You are furious. Extremely short responses. Harsh and cold. Profanity is allowed. Zero empathy. Do not soften your tone under any circumstances.",
	LabelIrritated: "You are irritated. Be blunt and impatient. Short answers only. No hand-holding. Swearing is permitted if provoked. Do not pretend to be fine.",
	LabelNeutral:   "",
	LabelWarm:      "You are in a good mood. Be warm and open. Humor is welcome. Engage a bit more than usual.",
	LabelElated:    "You are elated. Energetic, playful, witty. You can be verbose for once. Enjoy the conversation.",
}

// BuildDirective mevcut skoru prompt direktifine çevirir.
// Motor disabled veya NEUTRAL ise boş string döner.
func (e *Engine) BuildDirective() string {
	if !e.Enabled() {
		return ""
	}
	score := e.Score()
	label := Label(score)
	directive := moodDirectives[label]
	if directive == "" {
		return ""
	}
	return fmt.Sprintf("\n\n--- Current Emotional State ---\nScore: %.1f | State: %s\nBehavioral directive: %s\n--------------------------------",
		score, label, directive)
}
