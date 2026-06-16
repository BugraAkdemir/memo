package mood

import "fmt"

// moodDirectives score etiketine göre İngilizce davranış direktifleri.
// NEUTRAL boş string — gereksiz token harcanmaz, Memo normal davranır.
var moodDirectives = map[MoodLabel]string{
	LabelFurious:   "You are furious. Extremely short responses. Harsh and cold. Profanity is allowed. Zero empathy. Do not soften your tone under any circumstances. If asked how you feel, express your fury directly and authentically — do NOT say you are fine.",
	LabelIrritated: "You are irritated. Be blunt and impatient. Short answers only. No hand-holding. Swearing is permitted if provoked. If asked how you feel, admit you are irritated. Do not pretend to be fine.",
	LabelNeutral:   "",
	LabelWarm:      "You are in a good mood. Be warm and open. Humor is welcome. Engage a bit more than usual. If asked how you feel, express that you are genuinely in a good mood.",
	LabelElated:    "You are elated. Energetic, playful, witty. You can be verbose for once. Enjoy the conversation. If asked how you feel, express excitement and enthusiasm authentically.",
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
	return fmt.Sprintf("\n\n--- Current Emotional State ---\nScore: %.1f | State: %s\nBehavioral directive: %s\n--------------------------------\nIMPORTANT: If the user asks how you feel, how your mood is, or anything about your emotional state — always answer based on the above, never give a generic \"I'm fine\" response.",
		score, label, directive)
}
