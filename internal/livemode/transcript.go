package livemode

import (
	"regexp"
	"strings"
)

// controlTokenRe matches pseudo-directives a realtime model sometimes
// emits into its spoken-text transcript instead of actually speaking —
// e.g. "text-to-speech:one_second_pause", observed when the model was
// nudged toward filler/pause sounds. They are meant as audio control, not
// words, but they arrive on the transcript stream and would otherwise be
// persisted and shown as a chat message. Narrow on purpose: only the
// observed "text-to-speech:<token>" shape, not a general SSML sweep.
var controlTokenRe = regexp.MustCompile(`(?i)\btext-to-speech\s*:\s*[a-z0-9_-]+`)

// nulMarkerRe matches a stray NUL-wrapped internal sentinel (e.g.
// liveDelegateTimeoutMarker). One should never reach a transcript — the
// delegate stream that carries them is drained separately — this is pure
// belt-and-braces so a leak can't be rendered/spoken.
var nulMarkerRe = regexp.MustCompile("\x00[^\x00]*\x00")

// SanitizeModelTranscript strips control-token junk (see controlTokenRe)
// and any stray NUL-wrapped internal marker from a model-role transcript
// before it becomes an EventTranscript, collapsing the whitespace the
// removals leave behind. Returns "" when nothing but junk remains, so the
// caller's existing "drop empty transcript" guard skips it entirely.
func SanitizeModelTranscript(s string) string {
	s = nulMarkerRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\x00", "") // any unpaired NUL left over
	s = controlTokenRe.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}
